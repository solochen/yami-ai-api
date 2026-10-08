package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RenovationKB 软装知识库检索：优先 embedding 余弦相似度，
// embedding 模型不可用时自动降级为关键词评分，保证功能可用。
type RenovationKB struct {
	db   *pgxpool.Pool
	chat *ChatService

	mu            sync.Mutex
	loaded        bool
	embedModel    string
	embedBad      bool // embedding 模型探测失败，降级关键词模式
	entries       []kbEntry
}

type kbEntry struct {
	id      int64
	topic   string
	tags    []string
	content string
	vec     []float64
}

type RenovationKnowledgeHit struct {
	Topic   string   `json:"topic"`
	Tags    []string `json:"tags"`
	Content string   `json:"content"`
	Score   float64  `json:"score"`
	Source  string   `json:"source"`
}

func NewRenovationKB(db *pgxpool.Pool, chat *ChatService) *RenovationKB {
	return &RenovationKB{db: db, chat: chat}
}

// Search 检索知识库。modelCode 为 embedding 模型 code，为空则直接关键词模式。
func (kb *RenovationKB) Search(ctx context.Context, query, modelCode string, topK int) ([]RenovationKnowledgeHit, error) {
	kb.ensureLoaded(ctx, modelCode)

	kb.mu.Lock()
	defer kb.mu.Unlock()

	if kb.embedAvailable() && modelCode != "" {
		if qv, err := kb.embed(ctx, modelCode, []string{query}); err == nil && len(qv) == 1 {
			return kb.vectorSearch(qv[0], topK), nil
		} else if err != nil {
			log.Printf("[renovation_kb] embed query failed, fallback to keyword: %v", err)
		}
	}
	return kb.keywordSearch(query, topK), nil
}

func (kb *RenovationKB) embedAvailable() bool {
	if kb.embedBad || len(kb.entries) == 0 {
		return false
	}
	for i := range kb.entries {
		if kb.entries[i].vec == nil {
			return false
		}
	}
	return true
}

// ensureLoaded 懒加载知识库并为缺向量的条目回填 embedding。
func (kb *RenovationKB) ensureLoaded(ctx context.Context, modelCode string) {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	if kb.loaded && kb.embedModel == modelCode {
		return
	}
	kb.loaded = true
	kb.embedModel = modelCode

	rows, err := kb.db.Query(ctx, `SELECT id, topic, tags, content, embedding FROM renovation_knowledge ORDER BY id`)
	if err != nil {
		log.Printf("[renovation_kb] load knowledge failed: %v", err)
		return
	}
	defer rows.Close()
	entries := []kbEntry{}
	for rows.Next() {
		var e kbEntry
		var embedding []byte
		if err := rows.Scan(&e.id, &e.topic, &e.tags, &e.content, &embedding); err != nil {
			log.Printf("[renovation_kb] scan knowledge row failed: %v", err)
			return
		}
		if len(embedding) > 0 {
			var vec []float64
			if err := json.Unmarshal(embedding, &vec); err == nil && len(vec) > 0 {
				e.vec = vec
			}
		}
		entries = append(entries, e)
	}
	kb.entries = entries

	if modelCode == "" {
		return
	}
	missing := []kbEntry{}
	for _, e := range entries {
		if e.vec == nil {
			missing = append(missing, e)
		}
	}
	if len(missing) == 0 {
		return
	}
	inputs := make([]string, len(missing))
	for i, e := range missing {
		inputs[i] = e.topic + "：" + strings.Join(e.tags, ",") + "\n" + e.content
	}
	vecs, err := kb.embed(ctx, modelCode, inputs)
	if err != nil {
		kb.embedBad = true
		log.Printf("[renovation_kb] embedding model %q unavailable, keyword mode: %v", modelCode, err)
		return
	}
	for i, e := range missing {
		if i >= len(vecs) {
			break
		}
		b, err := json.Marshal(vecs[i])
		if err != nil {
			continue
		}
		if _, err := kb.db.Exec(ctx, `UPDATE renovation_knowledge SET embedding=$1::jsonb, embedding_model=$2, updated_at=now() WHERE id=$3`, string(b), modelCode, e.id); err != nil {
			log.Printf("[renovation_kb] persist embedding for %d failed: %v", e.id, err)
			continue
		}
		for j := range kb.entries {
			if kb.entries[j].id == e.id {
				kb.entries[j].vec = vecs[i]
				break
			}
		}
	}
}

func (kb *RenovationKB) vectorSearch(queryVec []float64, topK int) []RenovationKnowledgeHit {
	hits := make([]RenovationKnowledgeHit, 0, len(kb.entries))
	for _, e := range kb.entries {
		if e.vec == nil {
			continue
		}
		hits = append(hits, RenovationKnowledgeHit{
			Topic: e.topic, Tags: e.tags, Content: e.content,
			Score: cosineSimilarity(queryVec, e.vec), Source: "renovation_knowledge",
		})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}

func (kb *RenovationKB) keywordSearch(query string, topK int) []RenovationKnowledgeHit {
	q := strings.ToLower(strings.TrimSpace(query))
	grams := queryGrams(q)
	hits := []RenovationKnowledgeHit{}
	for _, e := range kb.entries {
		text := strings.ToLower(e.topic + " " + strings.Join(e.tags, " ") + " " + e.content)
		score := 0.0
		if strings.Contains(text, q) {
			score += 5
		}
		for _, tag := range e.tags {
			if strings.Contains(q, strings.ToLower(tag)) {
				score += 3
			}
		}
		for _, g := range grams {
			if strings.Count(text, g) > 0 {
				score++
			}
		}
		if score > 0 {
			hits = append(hits, RenovationKnowledgeHit{Topic: e.topic, Tags: e.tags, Content: e.content, Score: score, Source: "renovation_knowledge"})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}

// embed 调 embedding 模型（走 ModelRoute 线路，第一条成功即返回）。
func (kb *RenovationKB) embed(ctx context.Context, modelCode string, inputs []string) ([][]float64, error) {
	model, err := kb.chat.models.GetFullByCode(ctx, modelCode)
	if err != nil {
		return nil, fmt.Errorf("embedding model %q not found: %w", modelCode, err)
	}
	routes, err := kb.chat.modelRuntimeRoutes(ctx, model)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for i := range routes {
		route := routes[i]
		name := route.UpstreamModel
		if name == "" {
			name = model.NewAPIModel
		}
		if name == "" {
			name = model.Code
		}
		vecs, callErr := kb.chat.runtime.Embeddings(ctx, name, inputs, route.RequestExtraForRequest(model, ""))
		if callErr == nil {
			return vectorsToF64(vecs), nil
		}
		lastErr = callErr
	}
	return nil, lastErr
}

func vectorsToF64(vecs [][]float32) [][]float64 {
	out := make([][]float64, len(vecs))
	for i, v := range vecs {
		out[i] = make([]float64, len(v))
		for j, x := range v {
			out[i][j] = float64(x)
		}
	}
	return out
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// queryGrams 中文查询按二元组分词，英文按空格分词小写化。
func queryGrams(q string) []string {
	runes := []rune(q)
	grams := []string{}
	for i := 0; i+1 < len(runes); i++ {
		if r := runes[i]; isHan(r) && isHan(runes[i+1]) {
			grams = append(grams, string(runes[i:i+2]))
		}
	}
	for _, w := range strings.Fields(q) {
		for _, part := range strings.Split(w, ",") {
			if p := strings.TrimSpace(part); p != "" {
				grams = append(grams, strings.ToLower(p))
			}
		}
	}
	return grams
}

func isHan(r rune) bool {
	return r >= 0x4e00 && r <= 0x9fff
}
