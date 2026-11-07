package handler

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starai/api/internal/service"
)

func TestAgentModelHTTPRejectsBypassAndLogsSuccessfulUpdate(t *testing.T) {
	dsn := os.Getenv("AGENT_DRAFT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AGENT_DRAFT_TEST_DATABASE_URL for isolated HTTP integration")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("agent_models_http_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `
CREATE TABLE models(id bigserial PRIMARY KEY,code text,category text,request_mode text,is_enabled bool,runtime_rule jsonb,input_schema jsonb);
CREATE TABLE model_routes(id bigserial PRIMARY KEY,model_id bigint,is_enabled bool,runtime_rule jsonb);
CREATE TABLE workflow_definitions(id bigserial PRIMARY KEY,code text UNIQUE,name text,description text,icon text,category text,nodes jsonb,input_schema jsonb,price_rule jsonb,display_config jsonb,runtime_config jsonb,is_enabled bool,sort_order int,updated_at timestamptz DEFAULT now());
CREATE TABLE admin_operation_logs(admin_id bigint,action text,target_type text,target_id text,detail jsonb);
INSERT INTO models(code,category,request_mode,is_enabled,runtime_rule,input_schema) VALUES ('plain','chat','chat_completions',true,'{}','{}'),('capable','chat','chat_completions',true,'{"capabilities":{"vision":true,"video_analysis":true}}','{}');
INSERT INTO model_routes(model_id,is_enabled,runtime_rule) SELECT id,true,'{}' FROM models;
INSERT INTO workflow_definitions(code,name,category,nodes,input_schema,price_rule,display_config,runtime_config,is_enabled,sort_order) VALUES ('viral_video_breakdown','爆款拆解','tool','[]','{}','{"billing_type":"duration_second","unit_price":1}','{}','{"agent_mode":"viral_video_breakdown","analysis_model_code":"capable"}',true,0);
`)
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{agents: service.NewAgentService(pool, nil, nil, nil), admin: service.NewAdminService(pool, nil, "")}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("admin_id", int64(7)); c.Next() })
	router.GET("/admin/api/agent-models", h.AdminListAgentModels)
	router.GET("/admin/api/agents/:code/models/:slot/candidates", h.AdminAgentModelCandidates)
	router.PUT("/admin/api/agents/:code/models/:slot", h.AdminUpdateAgentModel)
	router.PUT("/admin/api/agents/:code", h.AdminUpdateAgent)
	router.POST("/admin/api/agents", h.AdminCreateAgent)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		res := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		return res
	}
	path := "/admin/api/agents/viral_video_breakdown/models/analysis_model_code"
	for _, body := range []string{`{"model_code":"plain"}`, `{"model_code":"plain","required_capabilities":[]}`, `{"model_code":"plain","capabilities":{"vision":true,"video_analysis":true}}`, `{"model_code":"capable"} {}`, `{"model_code":"missing"}`} {
		if res := request("PUT", path, body); res.Code != 400 {
			t.Fatalf("bypass accepted: %s => %d %s", body, res.Code, res.Body.String())
		}
	}
	for _, method := range []string{"PUT", "POST"} {
		url := "/admin/api/agents"
		if method == "PUT" {
			url += "/viral_video_breakdown"
		}
		body := `{"code":"viral_video_breakdown","name":"bad","runtime_config":{"agent_mode":"prompt_chat","analysis_model_code":"plain"},"price_rule":{"billing_type":"duration_second"}}`
		if res := request(method, url, body); res.Code != 400 {
			t.Fatalf("legacy bypass: %s %d %s", method, res.Code, res.Body.String())
		}
	}
	if res := request("GET", "/admin/api/agents/viral_video_breakdown/models/analysis_model_code/candidates", ""); res.Code != 200 || strings.Contains(res.Body.String(), `"code":"plain"`) {
		t.Fatalf("bad candidates %s", res.Body.String())
	}
	if res := request("PUT", path, `{"model_code":"capable","expected_model_code":"capable"}`); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM admin_operation_logs WHERE admin_id=7 AND action='update_agent_model' AND detail->>'before'='capable' AND detail->>'after'='capable'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("missing audit: %d %v", count, err)
	}
	if res := request("GET", "/admin/api/agent-models", ""); res.Code != 200 || !strings.Contains(res.Body.String(), "爆款结构分析") {
		t.Fatalf("overview failed %s", res.Body.String())
	}
}
