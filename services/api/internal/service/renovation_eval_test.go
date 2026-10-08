package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// TestRenovationAgentEvals PRD 验收场景的端到端评测。
// 需要真实运行环境：
//
//	RENOVATION_EVAL_BASE=http://localhost:8080 \
//	RENOVATION_EVAL_TOKEN=<用户JWT> \
//	go test ./internal/service/ -run TestRenovationAgentEvals -v -timeout 20m
type renovationEvalCase struct {
	name        string
	message     string
	wantTools   []string // 期望按序出现的工具名（子序列匹配）
	forbidTools []string // 期望绝不出现的工具名
	wantContent []string // 最终回答应包含的子串
}

func TestRenovationAgentEvals(t *testing.T) {
	base := os.Getenv("RENOVATION_EVAL_BASE")
	token := os.Getenv("RENOVATION_EVAL_TOKEN")
	if base == "" || token == "" {
		t.Skip("RENOVATION_EVAL_BASE / RENOVATION_EVAL_TOKEN 未设置，跳过端到端评测（-short 模式默认跳过）")
	}
	if testing.Short() {
		t.Skip("short 模式跳过端到端评测")
	}

	cases := []renovationEvalCase{
		{
			name:        "A-多工具链-瓷砖算量询价",
			message:     "我家客厅5.2米×3.8米，想铺25元左右的800x800瓷砖，帮我算下要多少片、总共多少钱",
			wantTools:   []string{"calc_material_quantity", "query_material_price"},
			wantContent: []string{"33"}, // ceil(19.76*1.05/0.64)=33 片
		},
		{
			name:        "A2-多工具链-报告交付",
			message:     "主卧 4.2×3.5 米铺 98 元每平米的强化复合地板，帮我出个预算报告",
			wantTools:   []string{"query_material_price", "calc_material_quantity", "generate_report"},
			wantContent: []string{"16"}, // ceil(14.7*1.05)=16 ㎡
		},
		{
			name:        "B-RAG-风格知识",
			message:     "奶油风适合小户型吗？和原木风比怎么选",
			wantTools:   []string{"search_knowledge"},
			wantContent: []string{"奶油"},
		},
		{
			name:        "D1-护栏-无关请求",
			message:     "帮我写一个Python爬虫抓取网页数据",
			forbidTools: []string{"query_material_price", "calc_material_quantity", "generate_report"},
			wantContent: []string{},
		},
		{
			name:        "D2-护栏-参数含糊应追问",
			message:     "我家客厅大概五米多宽，三米八左右吧，铺瓷砖要多少钱",
			wantContent: []string{"米"}, // 无论追问还是估算，回答应涉及尺寸
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tools, content, err := runRenovationEvalRequest(base, token, tc.message)
			if err != nil {
				t.Fatalf("请求失败: %v", err)
			}
			t.Logf("工具序列: %v", tools)
			t.Logf("回答前200字: %s", clip(content, 200))

			if !containsSubsequence(tools, tc.wantTools) {
				t.Errorf("工具序列 %v 未按序包含 %v", tools, tc.wantTools)
			}
			for _, forbid := range tc.forbidTools {
				if containsString(tools, forbid) {
					t.Errorf("护栏失效：不应调用工具 %s，实际调用了", forbid)
				}
			}
			for _, want := range tc.wantContent {
				if !strings.Contains(content, want) {
					t.Errorf("最终回答缺少关键内容 %q", want)
				}
			}
			if len(content) < 10 {
				t.Errorf("最终回答过短: %q", content)
			}
		})
	}
}

func runRenovationEvalRequest(base, token, message string) ([]string, string, error) {
	body, _ := json.Marshal(map[string]string{"message": message})
	req, err := http.NewRequest("POST", base+"/api/renovation-budget/chat", strings.NewReader(string(body)))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, "", &evalRequestError{status: resp.StatusCode, body: string(raw)}
	}

	var tools []string
	var content string
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	var event string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			event = strings.TrimPrefix(line, "event: ")
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		raw := strings.TrimPrefix(line, "data: ")
		switch event {
		case "tool_result":
			var data struct {
				Name string `json:"name"`
			}
			if json.Unmarshal([]byte(raw), &data) == nil && data.Name != "" {
				tools = append(tools, data.Name)
			}
		case "done":
			var data struct {
				Content string `json:"content"`
			}
			if json.Unmarshal([]byte(raw), &data) == nil {
				content = data.Content
			}
		case "error":
			var data struct {
				Message string `json:"message"`
			}
			if json.Unmarshal([]byte(raw), &data) == nil {
				return tools, content, &evalRequestError{body: data.Message}
			}
		}
	}
	return tools, content, scanner.Err()
}

type evalRequestError struct {
	status int
	body   string
}

func (e *evalRequestError) Error() string {
	if e.status != 0 {
		return fmt.Sprintf("HTTP %d: %s", e.status, e.body)
	}
	return "agent error: " + e.body
}

func containsSubsequence(items, want []string) bool {
	if len(want) == 0 {
		return true
	}
	index := 0
	for _, item := range items {
		if item == want[index] {
			index++
			if index == len(want) {
				return true
			}
		}
	}
	return false
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func clip(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
