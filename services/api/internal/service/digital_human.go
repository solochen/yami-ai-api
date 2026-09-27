package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/starai/api/internal/billing"
	"golang.org/x/net/websocket"
)

const (
	digitalHumanVoiceDefault = 100.0
	digitalHumanVideoDefault = 3500.0
	digitalHumanPersonaLimit = 50000
)

type digitalHumanSettings struct {
	APIKey         string
	Host           string
	PublicBaseURL  string
	VoicePerMinute float64
	VideoPerMinute float64
	TextCreditRate float64
}

type DigitalHumanRole struct {
	PublicID       string                   `json:"public_id"`
	Name           string                   `json:"name"`
	Relation       string                   `json:"relation"`
	UserTitle      string                   `json:"user_title"`
	Memory         string                   `json:"memory"`
	Style          string                   `json:"style"`
	Persona        string                   `json:"persona"`
	Voice          string                   `json:"voice"`
	AvatarURL      string                   `json:"avatar_url"`
	KnowledgeTitle string                   `json:"knowledge_title"`
	UpdatedAt      string                   `json:"updated_at"`
	Messages       []DigitalHumanMessage    `json:"messages,omitempty"`
	Knowledge      []DigitalHumanKnowledge  `json:"knowledge,omitempty"`
	ActiveSession  *DigitalHumanSessionView `json:"active_session,omitempty"`
}

type DigitalHumanMessage struct {
	ID        int64  `json:"id"`
	Speaker   string `json:"speaker"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type DigitalHumanKnowledge struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type DigitalHumanSessionView struct {
	PublicID string         `json:"public_id"`
	CallMode string         `json:"call_mode"`
	Status   string         `json:"status"`
	Cost     float64        `json:"cost"`
	RTC      map[string]any `json:"rtc,omitempty"`
}

type DigitalHumanInput struct {
	Name           string `json:"name"`
	Relation       string `json:"relation"`
	UserTitle      string `json:"user_title"`
	Memory         string `json:"memory"`
	Style          string `json:"style"`
	Persona        string `json:"persona"`
	Voice          string `json:"voice"`
	AvatarURL      string `json:"avatar_url"`
	KnowledgeTitle string `json:"knowledge_title"`
}

type digitalHumanError struct{ msg string }

func (e *digitalHumanError) Error() string { return e.msg }

func digitalHumanFail(msg string) error { return &digitalHumanError{msg: msg} }

func AsDigitalHumanError(err error) (string, bool) {
	var friendly *digitalHumanError
	if errors.As(err, &friendly) {
		return friendly.msg, true
	}
	return "", false
}

type knowledgeDoc struct {
	ID      int64
	Title   string
	Content string
}

type KnowledgeHit struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Type       string  `json:"type"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
}

type liveLink struct {
	mu       sync.Mutex
	conn     *websocket.Conn
	seq      int
	connID   string
	liveID   string
	userID   string
	roleDBID int64
	lastSent string
	saved    map[string]string
	closed   bool
	reply    chan string
}

type digitalHumanHub struct {
	mu    sync.Mutex
	links map[string]*liveLink
	ends  map[string]*sync.Mutex
}

var digitalHumans = &digitalHumanHub{links: map[string]*liveLink{}, ends: map[string]*sync.Mutex{}}

func (h *digitalHumanHub) endLock(id string) func() {
	h.mu.Lock()
	lock := h.ends[id]
	if lock == nil {
		lock = &sync.Mutex{}
		h.ends[id] = lock
	}
	h.mu.Unlock()
	lock.Lock()
	return lock.Unlock
}

func (s *ChatService) digitalHumanSettings(ctx context.Context) digitalHumanSettings {
	settings := digitalHumanSettings{Host: "api.vidu.cn", VoicePerMinute: digitalHumanVoiceDefault, VideoPerMinute: digitalHumanVideoDefault, TextCreditRate: 1}
	rows, err := s.db.Query(ctx, `SELECT key, value #>> '{}' FROM system_configs WHERE key LIKE 'vidu_%'`)
	if err != nil {
		return settings
	}
	defer rows.Close()
	values := map[string]string{}
	for rows.Next() {
		var key, value string
		if rows.Scan(&key, &value) == nil {
			values[key] = strings.TrimSpace(value)
		}
	}
	if values["vidu_api_key"] != "" {
		settings.APIKey = values["vidu_api_key"]
	}
	if host := strings.TrimPrefix(strings.TrimPrefix(values["vidu_api_host"], "https://"), "http://"); host != "" {
		settings.Host = strings.Trim(host, "/")
	}
	settings.PublicBaseURL = strings.TrimRight(values["vidu_public_base_url"], "/")
	if n := parseConfigFloat(values["vidu_voice_price_per_minute"], -1); n >= 0 {
		settings.VoicePerMinute = n
	}
	if n := parseConfigFloat(values["vidu_video_price_per_minute"], -1); n >= 0 {
		settings.VideoPerMinute = n
	}
	if n := parseConfigFloat(values["vidu_text_credit_rate"], -1); n >= 0 {
		settings.TextCreditRate = n
	}
	return settings
}

func parseConfigFloat(raw string, fallback float64) float64 {
	if raw == "" {
		return fallback
	}
	var n float64
	if _, err := fmt.Sscan(raw, &n); err != nil {
		return fallback
	}
	return n
}

func (s *ChatService) DigitalHumanPrices(ctx context.Context) (voice, video float64) {
	settings := s.digitalHumanSettings(ctx)
	return settings.VoicePerMinute, settings.VideoPerMinute
}

func (s *ChatService) ListDigitalHumanRoles(ctx context.Context, userID int64) ([]DigitalHumanRole, error) {
	rows, err := s.db.Query(ctx, `
		SELECT public_id, name, relation, user_title, voice, avatar_url, knowledge_title, updated_at
		FROM digital_human_roles WHERE user_id=$1 ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DigitalHumanRole
	for rows.Next() {
		var item DigitalHumanRole
		var updated time.Time
		if err := rows.Scan(&item.PublicID, &item.Name, &item.Relation, &item.UserTitle, &item.Voice, &item.AvatarURL, &item.KnowledgeTitle, &updated); err != nil {
			return nil, err
		}
		item.UpdatedAt = updated.Format(time.RFC3339)
		items = append(items, item)
	}
	if items == nil {
		items = []DigitalHumanRole{}
	}
	return items, rows.Err()
}

func (s *ChatService) GetDigitalHumanRole(ctx context.Context, userID int64, publicID string) (*DigitalHumanRole, error) {
	item, dbID, err := s.loadDigitalHumanRole(ctx, userID, publicID, true)
	if err != nil {
		return nil, err
	}
	messages, err := s.listDigitalHumanMessages(ctx, dbID, 80)
	if err != nil {
		return nil, err
	}
	knowledge, err := s.listDigitalHumanKnowledge(ctx, dbID)
	if err != nil {
		return nil, err
	}
	item.Messages = messages
	item.Knowledge = knowledge
	session, err := s.activeDigitalHumanSession(ctx, userID, dbID)
	if err != nil {
		return nil, err
	}
	item.ActiveSession = session
	return item, nil
}

func (s *ChatService) SaveDigitalHumanRole(ctx context.Context, userID int64, publicID string, input DigitalHumanInput) (*DigitalHumanRole, error) {
	input = normalizeDigitalHumanInput(input)
	if input.Name == "" {
		return nil, digitalHumanFail("请填写角色名字")
	}
	if publicID == "" {
		publicID = newDigitalHumanID("dhr")
		var id int64
		err := s.db.QueryRow(ctx, `
			INSERT INTO digital_human_roles (public_id, user_id, name, relation, user_title, memory, style, persona, voice, avatar_url, knowledge_title)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
			publicID, userID, input.Name, input.Relation, input.UserTitle, input.Memory, input.Style, input.Persona, input.Voice, input.AvatarURL, input.KnowledgeTitle).Scan(&id)
		if err != nil {
			return nil, err
		}
		return s.GetDigitalHumanRole(ctx, userID, publicID)
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE digital_human_roles
		SET name=$3, relation=$4, user_title=$5, memory=$6, style=$7, persona=$8, voice=$9, avatar_url=$10, knowledge_title=$11, updated_at=now()
		WHERE user_id=$1 AND public_id=$2`,
		userID, publicID, input.Name, input.Relation, input.UserTitle, input.Memory, input.Style, input.Persona, input.Voice, input.AvatarURL, input.KnowledgeTitle)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, digitalHumanFail("角色不存在")
	}
	return s.GetDigitalHumanRole(ctx, userID, publicID)
}

func (s *ChatService) DeleteDigitalHumanRole(ctx context.Context, userID int64, publicID string) error {
	role, dbID, err := s.loadDigitalHumanRole(ctx, userID, publicID, false)
	if err != nil {
		return err
	}
	if session, _ := s.activeDigitalHumanSession(ctx, userID, dbID); session != nil {
		_, _ = s.EndDigitalHumanSession(ctx, userID, session.PublicID)
	}
	_ = role
	tag, err := s.db.Exec(ctx, `DELETE FROM digital_human_roles WHERE user_id=$1 AND public_id=$2`, userID, publicID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return digitalHumanFail("角色不存在")
	}
	return nil
}

func (s *ChatService) AddDigitalHumanKnowledge(ctx context.Context, userID int64, roleID, title, content string) (*DigitalHumanKnowledge, error) {
	_, dbID, err := s.loadDigitalHumanRole(ctx, userID, roleID, false)
	if err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" || content == "" {
		return nil, digitalHumanFail("请填写资料标题和正文")
	}
	if len([]rune(content)) > 20000 {
		return nil, digitalHumanFail("单份资料不能超过 20000 字")
	}
	item := &DigitalHumanKnowledge{Title: title, Content: content}
	var created time.Time
	err = s.db.QueryRow(ctx, `
		INSERT INTO digital_human_knowledge (role_id, title, content) VALUES ($1,$2,$3)
		RETURNING id, created_at`, dbID, title, content).Scan(&item.ID, &created)
	if err != nil {
		return nil, err
	}
	item.CreatedAt = created.Format(time.RFC3339)
	_, _ = s.db.Exec(ctx, `UPDATE digital_human_roles SET updated_at=now() WHERE id=$1`, dbID)
	return item, nil
}

func (s *ChatService) DeleteDigitalHumanKnowledge(ctx context.Context, userID int64, roleID string, docID int64) error {
	_, dbID, err := s.loadDigitalHumanRole(ctx, userID, roleID, false)
	if err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx, `DELETE FROM digital_human_knowledge WHERE id=$1 AND role_id=$2`, docID, dbID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return digitalHumanFail("资料不存在")
	}
	return nil
}

func (s *ChatService) StartDigitalHumanSession(ctx context.Context, userID int64, rolePublicID, mode, publicBase string) (*DigitalHumanSessionView, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "audio" && mode != "text" && mode != "video" {
		return nil, digitalHumanFail("通话方式不正确")
	}
	role, dbID, err := s.loadDigitalHumanRole(ctx, userID, rolePublicID, true)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(role.AvatarURL) == "" {
		return nil, digitalHumanFail("请先填写数字人可以访问的头像图片地址")
	}
	if active, _ := s.activeDigitalHumanSession(ctx, userID, dbID); active != nil {
		return nil, digitalHumanFail("请先结束当前通话")
	}
	var starting int
	_ = s.db.QueryRow(ctx, `SELECT count(*) FROM digital_human_sessions WHERE user_id=$1 AND role_id=$2 AND status='starting' AND started_at > now() - interval '3 minutes'`, userID, dbID).Scan(&starting)
	if starting > 0 {
		return nil, digitalHumanFail("上一通正在接通，请稍等")
	}
	settings := s.digitalHumanSettings(ctx)
	if settings.APIKey == "" {
		return nil, digitalHumanFail("还没有配置 Vidu 密钥，请先在后台系统配置里填写")
	}
	messages, err := s.listDigitalHumanMessages(ctx, dbID, 12)
	if err != nil {
		return nil, err
	}
	persona := buildDigitalHumanPersona(*role, messages)
	if mode == "video" {
		persona += "用户让你做动作、换姿势、靠近、后退或蹲下时，立刻用身体表演，嘴里只说短句。不要把动作和神态写进回复，也不要念括号里的字。例如用户说蹲下，直接蹲下，只说「好哒，我这就蹲下」。"
	}
	if len([]rune(persona)) > digitalHumanPersonaLimit {
		persona = string([]rune(persona)[:digitalHumanPersonaLimit])
	}
	knowledgeCount, err := s.countDigitalHumanKnowledge(ctx, dbID)
	if err != nil {
		return nil, err
	}
	base := settings.PublicBaseURL
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(publicBase), "/")
	}
	token := randomDigitalHumanToken()
	publicID := newDigitalHumanID("dhs")
	viduMode := "audio"
	freeze := settings.TextCreditRate
	if mode == "audio" {
		freeze = settings.VoicePerMinute
		if freeze <= 0 {
			freeze = digitalHumanVoiceDefault
		}
	}
	if mode == "video" {
		viduMode = "video"
		freeze = settings.VideoPerMinute
		if freeze <= 0 {
			freeze = digitalHumanVideoDefault
		}
	}
	if freeze < 0 {
		freeze = 0
	}
	var sessionDBID int64
	err = s.db.QueryRow(ctx, `
		INSERT INTO digital_human_sessions (public_id, user_id, role_id, call_mode, status, knowledge_token)
		VALUES ($1,$2,$3,$4,'starting',$5) RETURNING id`,
		publicID, userID, dbID, mode, token).Scan(&sessionDBID)
	if err != nil {
		return nil, err
	}
	fail := func(msg string) (*DigitalHumanSessionView, error) {
		_, _ = s.db.Exec(ctx, `UPDATE digital_human_sessions SET status='ended', ended_at=now() WHERE id=$1`, sessionDBID)
		return nil, digitalHumanFail(msg)
	}
	avatar := map[string]any{
		"persona":   persona,
		"image_uri": role.AvatarURL,
		"voice":     role.Voice,
	}
	if mode == "video" {
		avatar["idle_action"] = true
	}
	body := map[string]any{
		"call_mode": viduMode,
		"avatar":    avatar,
		"audio":                map[string]any{"enable_transcription": true},
		"llm":                  map[string]any{"max_tokens": 200, "temperature": 0.7},
		"idle_timeout_seconds": 600,
	}
	if knowledgeCount > 0 && base != "" && !strings.Contains(base, "localhost") && !strings.Contains(base, "127.0.0.1") {
		body["knowledge_retrieval"] = map[string]any{
			"enabled":       true,
			"endpoint":      base + "/api/digital-human/knowledge/search",
			"authorization": "Bearer " + token,
			"timeout_ms":    3000,
		}
	}
	payload, status, err := s.viduJSON(ctx, settings, http.MethodPost, "/live/v1/lives", body, 120*time.Second)
	if err != nil {
		log.Printf("digital human create live: %v", err)
		return fail("连接数字人服务超时，请再试一次")
	}
	if status >= 300 {
		return fail(viduErrorMessage(payload))
	}
	var created struct {
		Live struct {
			ID string `json:"id"`
		} `json:"live"`
		RTC map[string]any `json:"rtc"`
	}
	if json.Unmarshal(payload, &created) != nil || created.Live.ID == "" {
		return fail("数字人服务没有返回会话")
	}
	rtcRaw, _ := json.Marshal(created.RTC)
	if _, err = s.db.Exec(ctx, `UPDATE digital_human_sessions SET live_id=$2, rtc_payload=$3 WHERE id=$1`, sessionDBID, created.Live.ID, rtcRaw); err != nil {
		return fail("保存会话失败")
	}
	link, err := s.openDigitalHumanLink(settings, created.Live.ID, rtcUserID(created.RTC), dbID)
	if err != nil {
		return fail(err.Error())
	}
	if err = s.billing.Freeze(ctx, userID, freeze, "digital_human", publicID); err != nil {
		s.closeDigitalHumanLink(publicID, link, true)
		if errors.Is(err, billing.ErrInsufficientBalance) {
			_, _ = s.db.Exec(ctx, `UPDATE digital_human_sessions SET status='ended', ended_at=now() WHERE id=$1`, sessionDBID)
			return nil, err
		}
		return fail("冻结余额失败")
	}
	if _, err = s.db.Exec(ctx, `UPDATE digital_human_sessions SET status='active' WHERE id=$1`, sessionDBID); err != nil {
		s.closeDigitalHumanLink(publicID, link, true)
		_ = s.billing.Unfreeze(ctx, userID, freeze, "digital_human", publicID)
		return fail("保存会话失败")
	}
	digitalHumans.mu.Lock()
	digitalHumans.links[publicID] = link
	digitalHumans.mu.Unlock()
	go s.readDigitalHumanLink(userID, publicID, link)
	return &DigitalHumanSessionView{PublicID: publicID, CallMode: mode, Status: "active", RTC: created.RTC}, nil
}

func (s *ChatService) SendDigitalHumanText(ctx context.Context, userID int64, sessionID, content string) (*DigitalHumanMessage, *DigitalHumanMessage, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil, digitalHumanFail("请输入内容")
	}
	if len([]rune(content)) > 2000 {
		return nil, nil, digitalHumanFail("单条消息不能超过 2000 字")
	}
	session, err := s.ownedDigitalHumanSession(ctx, userID, sessionID)
	if err != nil {
		return nil, nil, err
	}
	if session.Status != "active" {
		return nil, nil, digitalHumanFail("通话已经结束")
	}
	link := s.digitalHumanLink(sessionID)
	if link == nil {
		return nil, nil, digitalHumanFail("控制连接已断开，请结束通话后重试")
	}
	reply := make(chan string, 1)
	if err = link.sendText(content, reply); err != nil {
		return nil, nil, digitalHumanFail("文字没有送出，请稍后重试")
	}
	userMessage, err := s.insertDigitalHumanMessage(ctx, session.RoleDBID, "user", content)
	if err != nil {
		return nil, nil, err
	}
	select {
	case text := <-reply:
		text = strings.TrimSpace(text)
		if text == "" {
			return userMessage, nil, nil
		}
		return userMessage, &DigitalHumanMessage{Speaker: "assistant", Content: text, CreatedAt: time.Now().Format(time.RFC3339)}, nil
	case <-time.After(20 * time.Second):
		link.cancelReply(reply)
		return userMessage, nil, nil
	case <-ctx.Done():
		link.cancelReply(reply)
		return userMessage, nil, nil
	}
}

func (s *ChatService) EndDigitalHumanSession(ctx context.Context, userID int64, sessionID string) (*DigitalHumanSessionView, error) {
	unlock := digitalHumans.endLock(sessionID)
	defer unlock()
	session, err := s.ownedDigitalHumanSession(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	if session.Status == "ended" {
		return &DigitalHumanSessionView{PublicID: session.PublicID, CallMode: session.CallMode, Status: "ended", Cost: session.Cost}, nil
	}
	link := s.digitalHumanLink(sessionID)
	s.closeDigitalHumanLink(sessionID, link, true)
	settings := s.digitalHumanSettings(ctx)
	billed, credits := s.pollViduUsage(settings, session.LiveID, 5, time.Second)
	cost := digitalHumanCost(session.CallMode, billed, credits, settings.VoicePerMinute, settings.VideoPerMinute, settings.TextCreditRate)
	if cost <= 0 && session.LiveID != "" && (session.CallMode == "audio" || session.CallMode == "video") {
		go s.settleDigitalHumanBill(userID, session.DBID, session.PublicID, session.CallMode, session.LiveID, settings)
		_, _ = s.db.Exec(ctx, `UPDATE digital_human_sessions SET status='ended', ended_at=now() WHERE id=$1 AND status<>'ended'`, session.DBID)
		return &DigitalHumanSessionView{PublicID: session.PublicID, CallMode: session.CallMode, Status: "ended"}, nil
	}
	s.chargeDigitalHumanSession(ctx, userID, session.DBID, session.PublicID, session.CallMode, cost, settings)
	return &DigitalHumanSessionView{PublicID: session.PublicID, CallMode: session.CallMode, Status: "ended", Cost: cost}, nil
}

func (s *ChatService) pollViduUsage(settings digitalHumanSettings, liveID string, attempts int, gap time.Duration) (int, float64) {
	if attempts < 1 {
		attempts = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	billed, credits := s.viduUsage(ctx, settings, liveID)
	for i := 1; i < attempts && billed <= 0; i++ {
		time.Sleep(gap)
		billed, credits = s.viduUsage(ctx, settings, liveID)
	}
	return billed, credits
}

func (s *ChatService) settleDigitalHumanBill(userID, sessionDBID int64, publicID, mode, liveID string, settings digitalHumanSettings) {
	billed, credits := s.pollViduUsage(settings, liveID, 8, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cost := digitalHumanCost(mode, billed, credits, settings.VoicePerMinute, settings.VideoPerMinute, settings.TextCreditRate)
	if cost <= 0 {
		freeze := settings.TextCreditRate
		if mode == "audio" {
			freeze = settings.VoicePerMinute
		}
		if mode == "video" {
			freeze = settings.VideoPerMinute
		}
		if err := s.billing.Unfreeze(ctx, userID, freeze, "digital_human", publicID); err != nil {
			log.Printf("digital human unfreeze %s: %v", publicID, err)
		}
		_, _ = s.db.Exec(ctx, `UPDATE digital_human_sessions SET status='ended', ended_at=COALESCE(ended_at, now()), cost=0 WHERE id=$1`, sessionDBID)
		return
	}
	s.chargeDigitalHumanSession(ctx, userID, sessionDBID, publicID, mode, cost, settings)
}

func (s *ChatService) chargeDigitalHumanSession(ctx context.Context, userID, sessionDBID int64, publicID, mode string, cost float64, settings digitalHumanSettings) {
	freeze := settings.TextCreditRate
	remark := "数字人文字"
	txType := "audio_usage"
	switch mode {
	case "audio":
		freeze = settings.VoicePerMinute
		remark = "数字人语音"
	case "video":
		freeze = settings.VideoPerMinute
		remark = "数字人视频"
		txType = "video_usage"
	}
	if err := s.billing.Charge(ctx, userID, freeze, cost, "digital_human", publicID, txType, remark); err != nil && !errors.Is(err, billing.ErrFreezeNotFound) {
		log.Printf("digital human charge %s: %v", publicID, err)
	}
	_, _ = s.db.Exec(ctx, `UPDATE digital_human_sessions SET status='ended', ended_at=COALESCE(ended_at, now()), cost=$2 WHERE id=$1`, sessionDBID, cost)
}

func (s *ChatService) SearchDigitalHumanKnowledge(ctx context.Context, token, liveID, query string, limit int) ([]KnowledgeHit, error) {
	token = strings.TrimSpace(token)
	if token == "" || liveID == "" {
		return nil, digitalHumanFail("unauthorized")
	}
	var roleID int64
	err := s.db.QueryRow(ctx, `
		SELECT role_id FROM digital_human_sessions
		WHERE knowledge_token=$1 AND live_id=$2 AND status='active'`, token, liveID).Scan(&roleID)
	if err != nil {
		return nil, digitalHumanFail("unauthorized")
	}
	rows, err := s.db.Query(ctx, `SELECT id, title, content FROM digital_human_knowledge WHERE role_id=$1 ORDER BY id ASC`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []knowledgeDoc
	for rows.Next() {
		var doc knowledgeDoc
		if rows.Scan(&doc.ID, &doc.Title, &doc.Content) == nil {
			docs = append(docs, doc)
		}
	}
	return rankDigitalHumanKnowledge(docs, query, limit), rows.Err()
}

func digitalHumanCost(mode string, billedSeconds int, credits, voicePerMinute, videoPerMinute, creditRate float64) float64 {
	rate := 0.0
	switch mode {
	case "audio":
		rate = voicePerMinute
	case "video":
		rate = videoPerMinute
	default:
		if credits <= 0 || creditRate <= 0 {
			return 0
		}
		return math.Round(credits*creditRate*1e6) / 1e6
	}
	if billedSeconds <= 0 || rate <= 0 {
		return 0
	}
	return math.Round(float64(billedSeconds)/60*rate*1e6) / 1e6
}

func buildDigitalHumanPersona(role DigitalHumanRole, messages []DigitalHumanMessage) string {
	relation := strings.TrimSpace(role.Relation)
	if relation == "" {
		relation = "朋友"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "你是%s，与用户的关系是%s。", strings.TrimSpace(role.Name), relation)
	if title := strings.TrimSpace(role.UserTitle); title != "" {
		fmt.Fprintf(&b, "你称呼用户为%s。", title)
	}
	if memory := strings.TrimSpace(role.Memory); memory != "" {
		fmt.Fprintf(&b, "核心记忆：%s", memory)
	}
	if style := strings.TrimSpace(role.Style); style != "" {
		fmt.Fprintf(&b, "说话风格：%s", style)
	}
	if persona := strings.TrimSpace(role.Persona); persona != "" {
		fmt.Fprintf(&b, "人设：%s", persona)
	}
	b.WriteString("用中文自然对话。不要提及自己是程序。用户问起资料里的内容时，依据检索到的知识回答；没有检索到就直接说不知道。说出口的话里不要出现括号、旁白、动作或表情描写，括号里的内容不要念出来。")
	if len(messages) > 0 {
		b.WriteString("最近的对话：")
		for _, message := range messages {
			speaker := "用户"
			content := stripSpokenStageDirections(message.Content)
			if content == "" {
				continue
			}
			if message.Speaker == "assistant" {
				speaker = "你"
			}
			fmt.Fprintf(&b, "%s：%s", speaker, content)
		}
	}
	return b.String()
}

var spokenStageDirectionPattern = regexp.MustCompile(`（[^（）]*）|\([^()]*\)`)

func stripSpokenStageDirections(text string) string {
	cleaned := spokenStageDirectionPattern.ReplaceAllString(text, "")
	return strings.TrimSpace(cleaned)
}

func rankDigitalHumanKnowledge(docs []knowledgeDoc, query string, limit int) []KnowledgeHit {
	query = strings.TrimSpace(query)
	if query == "" || len(docs) == 0 {
		return []KnowledgeHit{}
	}
	if limit <= 0 || limit > 10 {
		limit = 5
	}
	type scored struct {
		doc   knowledgeDoc
		score float64
	}
	var ranked []scored
	for _, doc := range docs {
		score := scoreKnowledge(doc.Title+"\n"+doc.Content, query)
		if score >= 0.2 {
			ranked = append(ranked, scored{doc: doc, score: score})
		}
	}
	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].score > ranked[i].score {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	hits := make([]KnowledgeHit, 0, len(ranked))
	for _, item := range ranked {
		content := item.doc.Content
		if len([]rune(content)) > 800 {
			content = string([]rune(content)[:800])
		}
		hits = append(hits, KnowledgeHit{
			ID: fmt.Sprintf("doc_%d", item.doc.ID), Title: item.doc.Title, Content: content,
			Type: "document", Source: "role_knowledge", Confidence: math.Round(item.score*100) / 100,
		})
	}
	return hits
}

func scoreKnowledge(content, query string) float64 {
	content = strings.ToLower(content)
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return 0
	}
	if strings.Contains(content, query) {
		return 1
	}
	grams := knowledgeGrams(query)
	if len(grams) == 0 {
		return 0
	}
	hit := 0
	for _, gram := range grams {
		if strings.Contains(content, gram) {
			hit++
		}
	}
	return float64(hit) / float64(len(grams))
}

func knowledgeGrams(query string) []string {
	var parts []string
	var word strings.Builder
	flush := func() {
		if word.Len() >= 2 {
			parts = append(parts, strings.ToLower(word.String()))
		}
		word.Reset()
	}
	runes := []rune(query)
	for _, r := range runes {
		if unicode.Is(unicode.Han, r) {
			flush()
			parts = append(parts, string(r))
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			word.WriteRune(unicode.ToLower(r))
			continue
		}
		flush()
	}
	flush()
	if len(parts) < 2 {
		return parts
	}
	grams := make([]string, 0, len(parts)-1)
	for i := 0; i < len(parts)-1; i++ {
		grams = append(grams, parts[i]+parts[i+1])
	}
	return grams
}

func normalizeDigitalHumanInput(input DigitalHumanInput) DigitalHumanInput {
	input.Name = trimRunes(input.Name, 64)
	input.Relation = trimRunes(input.Relation, 32)
	input.UserTitle = trimRunes(input.UserTitle, 64)
	input.Memory = trimRunes(input.Memory, 4000)
	input.Style = trimRunes(input.Style, 2000)
	input.Persona = trimRunes(input.Persona, 8000)
	input.Voice = trimRunes(input.Voice, 128)
	input.AvatarURL = strings.TrimSpace(input.AvatarURL)
	input.KnowledgeTitle = trimRunes(input.KnowledgeTitle, 128)
	if input.Voice == "" {
		input.Voice = "Tina"
	}
	return input
}

func trimRunes(value string, max int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > max {
		return string(runes[:max])
	}
	return value
}

func (s *ChatService) loadDigitalHumanRole(ctx context.Context, userID int64, publicID string, full bool) (*DigitalHumanRole, int64, error) {
	query := `SELECT id, public_id, name, relation, user_title, voice, avatar_url, knowledge_title, updated_at FROM digital_human_roles WHERE user_id=$1 AND public_id=$2`
	if full {
		query = `SELECT id, public_id, name, relation, user_title, memory, style, persona, voice, avatar_url, knowledge_title, updated_at FROM digital_human_roles WHERE user_id=$1 AND public_id=$2`
	}
	var item DigitalHumanRole
	var id int64
	var updated time.Time
	var err error
	if full {
		err = s.db.QueryRow(ctx, query, userID, publicID).Scan(&id, &item.PublicID, &item.Name, &item.Relation, &item.UserTitle, &item.Memory, &item.Style, &item.Persona, &item.Voice, &item.AvatarURL, &item.KnowledgeTitle, &updated)
	} else {
		err = s.db.QueryRow(ctx, query, userID, publicID).Scan(&id, &item.PublicID, &item.Name, &item.Relation, &item.UserTitle, &item.Voice, &item.AvatarURL, &item.KnowledgeTitle, &updated)
	}
	if err != nil {
		return nil, 0, digitalHumanFail("角色不存在")
	}
	item.UpdatedAt = updated.Format(time.RFC3339)
	return &item, id, nil
}

func (s *ChatService) listDigitalHumanMessages(ctx context.Context, roleID int64, limit int) ([]DigitalHumanMessage, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, speaker, content, created_at FROM (
			SELECT id, speaker, content, created_at FROM digital_human_messages WHERE role_id=$1 ORDER BY id DESC LIMIT $2
		) recent ORDER BY id ASC`, roleID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DigitalHumanMessage{}
	for rows.Next() {
		var item DigitalHumanMessage
		var created time.Time
		if err := rows.Scan(&item.ID, &item.Speaker, &item.Content, &created); err != nil {
			return nil, err
		}
		item.CreatedAt = created.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *ChatService) listDigitalHumanKnowledge(ctx context.Context, roleID int64) ([]DigitalHumanKnowledge, error) {
	rows, err := s.db.Query(ctx, `SELECT id, title, content, created_at FROM digital_human_knowledge WHERE role_id=$1 ORDER BY id ASC`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DigitalHumanKnowledge{}
	for rows.Next() {
		var item DigitalHumanKnowledge
		var created time.Time
		if rows.Scan(&item.ID, &item.Title, &item.Content, &created) == nil {
			item.CreatedAt = created.Format(time.RFC3339)
			items = append(items, item)
		}
	}
	return items, rows.Err()
}

func (s *ChatService) countDigitalHumanKnowledge(ctx context.Context, roleID int64) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM digital_human_knowledge WHERE role_id=$1`, roleID).Scan(&n)
	return n, err
}

func (s *ChatService) insertDigitalHumanMessage(ctx context.Context, roleID int64, speaker, content string) (*DigitalHumanMessage, error) {
	item := &DigitalHumanMessage{Speaker: speaker, Content: content}
	var created time.Time
	err := s.db.QueryRow(ctx, `
		INSERT INTO digital_human_messages (role_id, speaker, content) VALUES ($1,$2,$3) RETURNING id, created_at`,
		roleID, speaker, content).Scan(&item.ID, &created)
	if err != nil {
		return nil, err
	}
	item.CreatedAt = created.Format(time.RFC3339)
	return item, nil
}

type digitalHumanSessionRow struct {
	DBID     int64
	PublicID string
	RoleDBID int64
	LiveID   string
	CallMode string
	Status   string
	Cost     float64
	RTC      map[string]any
}

func (s *ChatService) activeDigitalHumanSession(ctx context.Context, userID, roleID int64) (*DigitalHumanSessionView, error) {
	row, err := s.scanDigitalHumanSession(ctx, `SELECT id, public_id, role_id, live_id, call_mode, status, cost, rtc_payload FROM digital_human_sessions WHERE user_id=$1 AND role_id=$2 AND status IN ('active','starting') ORDER BY id DESC LIMIT 1`, userID, roleID)
	if err != nil || row == nil || row.Status != "active" {
		return nil, err
	}
	return &DigitalHumanSessionView{PublicID: row.PublicID, CallMode: row.CallMode, Status: row.Status, Cost: row.Cost, RTC: row.RTC}, nil
}

func (s *ChatService) ownedDigitalHumanSession(ctx context.Context, userID int64, publicID string) (*digitalHumanSessionRow, error) {
	row, err := s.scanDigitalHumanSession(ctx, `SELECT id, public_id, role_id, live_id, call_mode, status, cost, rtc_payload FROM digital_human_sessions WHERE user_id=$1 AND public_id=$2`, userID, publicID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, digitalHumanFail("通话不存在")
	}
	return row, nil
}

func (s *ChatService) scanDigitalHumanSession(ctx context.Context, query string, args ...any) (*digitalHumanSessionRow, error) {
	var row digitalHumanSessionRow
	var rtc []byte
	err := s.db.QueryRow(ctx, query, args...).Scan(&row.DBID, &row.PublicID, &row.RoleDBID, &row.LiveID, &row.CallMode, &row.Status, &row.Cost, &rtc)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if len(rtc) > 0 {
		_ = json.Unmarshal(rtc, &row.RTC)
	}
	return &row, nil
}

func (s *ChatService) viduJSON(ctx context.Context, settings digitalHumanSettings, method, path string, body any, timeout time.Duration) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://"+settings.Host+path, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Token "+settings.APIKey)
	req.Header.Set("Content-Type", "application/json")
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	res, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return payload, res.StatusCode, err
}

func (s *ChatService) viduUsage(ctx context.Context, settings digitalHumanSettings, liveID string) (int, float64) {
	if liveID == "" || settings.APIKey == "" {
		return 0, 0
	}
	payload, status, err := s.viduJSON(ctx, settings, http.MethodGet, "/live/v1/lives/"+url.PathEscape(liveID), nil, 20*time.Second)
	if err != nil || status >= 300 {
		return 0, 0
	}
	var parsed struct {
		Live struct {
			BilledSeconds int     `json:"billed_seconds"`
			CreditsCost   float64 `json:"credits_cost"`
		} `json:"live"`
	}
	if json.Unmarshal(payload, &parsed) != nil {
		return 0, 0
	}
	return parsed.Live.BilledSeconds, parsed.Live.CreditsCost
}

func viduErrorMessage(payload []byte) string {
	var parsed struct {
		Message string `json:"message"`
		Reason  string `json:"reason"`
	}
	if json.Unmarshal(payload, &parsed) == nil {
		if parsed.Message != "" {
			return "数字人服务拒绝了这次通话：" + parsed.Message
		}
		if parsed.Reason != "" {
			return "数字人服务拒绝了这次通话：" + parsed.Reason
		}
	}
	return "数字人服务拒绝了这次通话"
}

func (s *ChatService) openDigitalHumanLink(settings digitalHumanSettings, liveID, rtcUser string, roleDBID int64) (*liveLink, error) {
	connID := "starai-" + randomDigitalHumanToken()[:8]
	wsURL := fmt.Sprintf("wss://%s/live/ws/live/connect?live_id=%s&conn_id=%s", settings.Host, url.QueryEscape(liveID), url.QueryEscape(connID))
	cfg, err := websocket.NewConfig(wsURL, "https://"+settings.Host)
	if err != nil {
		return nil, digitalHumanFail("控制连接配置失败")
	}
	cfg.Header.Set("Authorization", "Token "+settings.APIKey)
	conn, err := websocket.DialConfig(cfg)
	if err != nil {
		return nil, digitalHumanFail("控制连接没有建立")
	}
	link := &liveLink{conn: conn, connID: connID, liveID: liveID, userID: rtcUser, roleDBID: roleDBID}
	if err = link.write(1, map[string]any{"conn_init": map[string]any{"version": 1}}); err != nil {
		conn.Close()
		return nil, digitalHumanFail("控制连接初始化失败")
	}
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	for {
		var frame map[string]any
		if err = websocket.JSON.Receive(conn, &frame); err != nil {
			conn.Close()
			return nil, digitalHumanFail("数字人控制链路没有就绪")
		}
		frameType, _ := frame["type"].(float64)
		if int(frameType) != 2 {
			continue
		}
		payload, _ := frame["payload"].(map[string]any)
		ack, _ := payload["conn_init_ack"].(map[string]any)
		success, _ := ack["success"].(bool)
		if success {
			_ = conn.SetDeadline(time.Time{})
			return link, nil
		}
		if code, _ := ack["error_code"].(string); code == "NOT_READY" {
			continue
		}
		conn.Close()
		msg, _ := ack["error_msg"].(string)
		if msg == "" {
			msg = "数字人控制链路没有就绪"
		}
		return nil, digitalHumanFail(msg)
	}
}

func (s *ChatService) readDigitalHumanLink(userID int64, sessionID string, link *liveLink) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("digital human read panic: %v", rec)
		}
	}()
	for {
		var frame map[string]any
		if err := websocket.JSON.Receive(link.conn, &frame); err != nil {
			return
		}
		frameType, _ := frame["type"].(float64)
		switch int(frameType) {
		case 1, 2, 5, 7, 97, 98:
			continue
		case 6:
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				_, _ = s.EndDigitalHumanSession(ctx, userID, sessionID)
			}()
			return
		}
		payload, _ := json.Marshal(frame["payload"])
		speaker := ""
		switch int(frameType) {
		case 9:
			speaker = "user"
		case 10:
			speaker = "assistant"
		}
		for _, text := range extractLiveTexts(payload) {
			text.Content = strings.TrimSpace(text.Content)
			if speaker != "" {
				text.Speaker = speaker
			}
			if text.Content == "" || text.Content == link.lastSentText() || link.seen(text.Speaker, text.Content) {
				continue
			}
			if text.Speaker == "assistant" {
				link.deliver(text.Content)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, _ = s.insertDigitalHumanMessage(ctx, link.roleDBID, text.Speaker, text.Content)
			cancel()
		}
	}
}

func (s *ChatService) digitalHumanLink(sessionID string) *liveLink {
	digitalHumans.mu.Lock()
	defer digitalHumans.mu.Unlock()
	return digitalHumans.links[sessionID]
}

func (s *ChatService) closeDigitalHumanLink(sessionID string, link *liveLink, hangup bool) {
	digitalHumans.mu.Lock()
	if current := digitalHumans.links[sessionID]; current != nil {
		link = current
		delete(digitalHumans.links, sessionID)
	}
	digitalHumans.mu.Unlock()
	if link == nil {
		return
	}
	if hangup {
		_ = link.write(5, map[string]any{"hangup": map[string]any{"hangup_reason": "user_end"}})
	}
	link.mu.Lock()
	link.closed = true
	if link.conn != nil {
		_ = link.conn.Close()
	}
	link.mu.Unlock()
}

func (l *liveLink) sendText(content string, reply chan string) error {
	l.mu.Lock()
	l.lastSent = content
	l.reply = reply
	l.mu.Unlock()
	return l.write(99, map[string]any{"text_msg": map[string]any{
		"msg_id":    newDigitalHumanID("msg"),
		"content":   content,
		"timestamp": time.Now().UnixMilli(),
	}})
}

func (l *liveLink) deliver(content string) {
	l.mu.Lock()
	reply := l.reply
	l.reply = nil
	l.mu.Unlock()
	if reply == nil {
		return
	}
	select {
	case reply <- content:
	default:
	}
}

func (l *liveLink) cancelReply(reply chan string) {
	l.mu.Lock()
	if l.reply == reply {
		l.reply = nil
	}
	l.mu.Unlock()
}

func (l *liveLink) seen(speaker, content string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.saved == nil {
		l.saved = map[string]string{}
	}
	if l.saved[speaker] == content {
		return true
	}
	l.saved[speaker] = content
	return false
}

func (l *liveLink) lastSentText() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lastSent
}

func (l *liveLink) write(frameType int, payload any) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.conn == nil {
		return errors.New("closed")
	}
	l.seq++
	frame := map[string]any{
		"type": frameType, "live_id": l.liveID, "user_id": l.userID,
		"conn_id": l.connID, "seq_id": l.seq, "payload": payload,
	}
	return websocket.JSON.Send(l.conn, frame)
}

type liveText struct {
	Speaker string
	Content string
}

func extractLiveTexts(payload []byte) []liveText {
	var generic any
	if json.Unmarshal(payload, &generic) != nil {
		return nil
	}
	var found []liveText
	var walk func(any, string)
	walk = func(node any, path string) {
		switch value := node.(type) {
		case map[string]any:
			for key, child := range value {
				next := path + "." + strings.ToLower(key)
				if text, ok := child.(string); ok && isLiveTextKey(key) && strings.TrimSpace(text) != "" {
					found = append(found, liveText{Speaker: liveSpeaker(next), Content: text})
				}
				walk(child, next)
			}
		case []any:
			for _, child := range value {
				walk(child, path)
			}
		}
	}
	walk(generic, "")
	return found
}

func isLiveTextKey(key string) bool {
	switch strings.ToLower(key) {
	case "content", "text", "transcript", "transcription", "output_text", "asr_text":
		return true
	default:
		return false
	}
}

func liveSpeaker(path string) string {
	if strings.Contains(path, "user") || strings.Contains(path, "input") || strings.Contains(path, "asr") {
		return "user"
	}
	return "assistant"
}

func rtcUserID(rtc map[string]any) string {
	if rtc == nil {
		return ""
	}
	if value, ok := rtc["user_id"].(string); ok {
		return value
	}
	return ""
}

func newDigitalHumanID(prefix string) string {
	return prefix + "_" + randomDigitalHumanToken()[:12]
}

func randomDigitalHumanToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
