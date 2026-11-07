package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starai/modelcontract"
)

func agentModelsFixture(t *testing.T) (*AgentService, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("AGENT_DRAFT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AGENT_DRAFT_TEST_DATABASE_URL for isolated model-slot integration")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("agent_models_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	_, err = pool.Exec(ctx, `
CREATE TABLE models(id bigserial PRIMARY KEY,code text UNIQUE,public_alias text,display_name text,category text,request_mode text,is_enabled bool,sort_order int DEFAULT 0,runtime_rule jsonb DEFAULT '{}',input_schema jsonb DEFAULT '{}',new_api_endpoint text);
CREATE TABLE model_routes(id bigserial PRIMARY KEY,model_id bigint,is_enabled bool,runtime_rule jsonb DEFAULT '{}',endpoint text);
CREATE TABLE workflow_definitions(id bigserial PRIMARY KEY,code text UNIQUE,name text,description text,icon text,category text DEFAULT 'workflow',nodes jsonb DEFAULT '[]',input_schema jsonb DEFAULT '{}',price_rule jsonb DEFAULT '{"billing_type":"model_actual"}',display_config jsonb DEFAULT '{}',runtime_config jsonb DEFAULT '{}',is_enabled bool DEFAULT true,sort_order int DEFAULT 0,updated_at timestamptz DEFAULT now());
CREATE TABLE workflow_projects(id bigserial PRIMARY KEY,public_id text,user_id bigint,workflow_id bigint,status text,inputs jsonb DEFAULT '{}',outputs jsonb DEFAULT '{}');
INSERT INTO models(code,public_alias,display_name,category,request_mode,is_enabled,runtime_rule,input_schema,new_api_endpoint) VALUES
('plain',NULL,'纯文本','chat','chat_completions',true,'{}','{}','/v1/chat/completions'),
('vision',NULL,'视频图片理解','chat','chat_completions',true,'{"capabilities":{"vision":true,"video_analysis":true}}','{}','/v1/chat/completions'),
('image','public-image','参考图生成','image','images',true,'{"image":{"max_reference_images":4}}','{}','/images'),
('image-text',NULL,'纯文生图','image','images',true,'{}','{}','/images'),
('video',NULL,'首帧视频','video','video',true,'{}','{"properties":{"generation_mode":{"enum":["text","image"]}}}','/videos'),
('video-text',NULL,'纯文生视频','video','video',true,'{}','{}','/videos'),
('speech',NULL,'语音合成','audio','audio',true,'{}','{}','/v1/audio/speech'),
('music',NULL,'音乐生成','audio','audio',true,'{}','{}','/v1/music_generation'),
('off',NULL,'已停用','chat','chat_completions',false,'{"capabilities":{"vision":true,"video_analysis":true}}','{}','/v1/chat/completions');
INSERT INTO model_routes(model_id,is_enabled) SELECT id,true FROM models;
INSERT INTO workflow_definitions(code,name,category,runtime_config,nodes,is_enabled) VALUES
('viral_video_breakdown','爆款拆解','tool','{"agent_mode":"viral_video_breakdown","analysis_model_code":"vision"}','[]',true),
('ai_comic_drama','漫剧','video','{"agent_mode":"comic_drama","analysis_model_code":"plain","dialogue_model_codes":["plain","vision"],"image_model_code":"image","video_model_code":"video","generation_model_code":"video","narration_model_code":"speech","audio_strategy":"hybrid","agent_policy":{"version":3}}',
 '[{"id":"comic_plan","type":"llm","name":"剧本","model_code":"plain"},{"id":"keyframes","type":"image","name":"关键帧","model_code":"image","custom_field":{"keep":true},"prompt_template":"keep"},{"id":"video_segments","type":"video","name":"视频片段","model_code":"video"},{"id":"narrations","type":"audio","name":"配音","model_code":"speech"}]',true),
('general_creative_agent','通用创作','chat','{"agent_mode":"creative_chat","system_workspace":true,"analysis_model_code":"vision","image_model_code":"image","video_model_code":"video","speech_model_code":"speech","music_model_code":"music"}','[]',true),
('infinite_canvas','画布','workflow','{"agent_mode":"infinite_canvas","system_workspace":true}','[]',false);
INSERT INTO workflow_projects(public_id,user_id,workflow_id,status,inputs,outputs) SELECT 'failed',1,id,'failed','{}','{"keyframes":[{"id":"S01","image_url":"keep"}]}' FROM workflow_definitions WHERE code='ai_comic_drama';
`)
	if err != nil {
		t.Fatal(err)
	}
	return &AgentService{db: pool}, pool
}

func TestAgentModelOverviewAndCandidates(t *testing.T) {
	s, _ := agentModelsFixture(t)
	items, err := s.ListAgentModelBindings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("got %d agents", len(items))
	}
	for _, item := range items {
		if item.Code == "infinite_canvas" {
			if item.Enabled || !item.SystemWorkspace || len(item.Slots) < 5 {
				t.Fatal("disabled workspace omitted")
			}
		}
		for _, slot := range item.Slots {
			if slot.Purpose == "" || len(slot.Required) == 0 || len(slot.Sources) == 0 {
				t.Fatalf("incomplete slot %+v", slot)
			}
			if item.Code == "viral_video_breakdown" && slot.ID == "analysis_model_code" {
				if len(slot.Candidates) != 1 || slot.Candidates[0].Code != "vision" {
					t.Fatalf("bad viral candidates %+v", slot.Candidates)
				}
			}
			if item.Code == "ai_comic_drama" && slot.ID == "narration_model_code" {
				if len(slot.Candidates) != 1 || slot.Candidates[0].Code != "speech" {
					t.Fatal("music appeared in voice candidates")
				}
			}
		}
	}
}

func TestAgentModelSlotUpdateRejectsIncompatibleAndSynchronizes(t *testing.T) {
	s, pool := agentModelsFixture(t)
	ctx := context.Background()
	for _, tc := range []struct{ agent, slot, code string }{{"viral_video_breakdown", "analysis_model_code", "plain"}, {"ai_comic_drama", "image_model_code", "image-text"}, {"ai_comic_drama", "video_model_code", "video-text"}, {"ai_comic_drama", "narration_model_code", "music"}, {"viral_video_breakdown", "analysis_model_code", "off"}, {"ai_comic_drama", "invented", "image"}} {
		if _, err := s.UpdateAgentModelSlot(ctx, tc.agent, tc.slot, AgentModelSlotUpdate{ModelCode: tc.code}); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	before := "plain"
	if _, err := s.UpdateAgentModelSlot(ctx, "ai_comic_drama", "analysis_model_code", AgentModelSlotUpdate{ModelCode: "vision", ExpectedModelCode: &before}); err != nil {
		t.Fatal(err)
	}
	var analysis, primary, backup, node, policy string
	if err := pool.QueryRow(ctx, `SELECT runtime_config->>'analysis_model_code',runtime_config#>>'{dialogue_model_codes,0}',runtime_config#>>'{dialogue_model_codes,1}',nodes#>>'{0,model_code}',runtime_config#>>'{agent_policy,version}' FROM workflow_definitions WHERE code='ai_comic_drama'`).Scan(&analysis, &primary, &backup, &node, &policy); err != nil {
		t.Fatal(err)
	}
	if analysis != "vision" || primary != "vision" || backup != "vision" || node != "vision" || policy != "3" {
		t.Fatalf("mirrors/policy: %s %s %s %s %s", analysis, primary, backup, node, policy)
	}
	if _, err := s.UpdateAgentModelSlot(ctx, "ai_comic_drama", "analysis_model_code", AgentModelSlotUpdate{ModelCode: "plain", ExpectedModelCode: &before}); err == nil {
		t.Fatal("stale write accepted")
	}
	if _, err := s.UpdateAgentModelSlot(ctx, "ai_comic_drama", "image_model_code", AgentModelSlotUpdate{ModelCode: "public-image"}); err != nil {
		t.Fatal(err)
	}
	var custom, prompt, code string
	if err := pool.QueryRow(ctx, `SELECT nodes#>>'{1,custom_field,keep}',nodes#>>'{1,prompt_template}',runtime_config->>'image_model_code' FROM workflow_definitions WHERE code='ai_comic_drama'`).Scan(&custom, &prompt, &code); err != nil {
		t.Fatal(err)
	}
	if custom != "true" || prompt != "keep" || code != "image" {
		t.Fatal("unrelated node data lost or alias not normalized")
	}
}

func TestLegacyAgentSaveAndOverridesCannotBypass(t *testing.T) {
	s, pool := agentModelsFixture(t)
	ctx := context.Background()
	for _, runtime := range []map[string]interface{}{
		{"agent_mode": "prompt_chat", "analysis_model_code": "plain"},
		{"agent_mode": "viral_video_breakdown", "analysis_model_code": "vision", "fallback_model_code": "plain"},
	} {
		err := s.Upsert(ctx, AgentUpsertInput{Code: "viral_video_breakdown", Name: "bad", RuntimeConfig: runtime, PriceRule: map[string]interface{}{"billing_type": "model_actual"}})
		if err == nil {
			t.Fatal("legacy save bypassed")
		}
	}
	w, err := s.Get(ctx, "ai_comic_drama")
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range []WorkflowNode{{ID: "keyframes", Type: "llm", ModelCode: "plain"}, {ID: "narrations", Type: "audio", ModelCode: "music"}} {
		if err := s.Upsert(ctx, AgentUpsertInput{Code: w.Code, Name: w.Name, RuntimeConfig: w.RuntimeConfig, Nodes: []WorkflowNode{node}, PriceRule: w.PriceRule}); err == nil {
			t.Fatal("normalization hid incompatible node")
		}
	}
	if _, err := s.CreateProject(ctx, 1, "ai_comic_drama", map[string]interface{}{"image_model_code": "image-text"}); err == nil || !strings.Contains(err.Error(), "参考图") {
		t.Fatalf("bad submission must fail before billing: %v", err)
	}
	if err := s.RetryProjectNode(ctx, 1, "failed", "keyframes", map[string]string{"image_model_code": "image-text"}); err == nil {
		t.Fatal("retry override accepted")
	}
	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT outputs FROM workflow_projects WHERE public_id='failed'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	_ = json.Unmarshal(raw, &out)
	if out["keyframes"] == nil {
		t.Fatal("invalid retry changed checkpoints")
	}
}

func TestComicFallbackOnlyChoosesCapableModels(t *testing.T) {
	s, pool := agentModelsFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE models SET sort_order=-100 WHERE code IN ('image-text','video-text','music')`); err != nil {
		t.Fatal(err)
	}
	inputs := map[string]interface{}{"image_model_code": "missing", "video_model_code": "missing", "narration_model_code": "missing", "audio_strategy": "hybrid"}
	s.healComicDramaModels(ctx, inputs)
	for key, want := range map[string]string{"image_model_code": "image", "video_model_code": "video", "narration_model_code": "speech"} {
		if inputs[key] != want {
			t.Fatalf("%s=%v", key, inputs[key])
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE model_routes SET runtime_rule='{"capabilities":{"reference_images":false}}' WHERE model_id=(SELECT id FROM models WHERE code='image')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateAgentModelSlot(ctx, "ai_comic_drama", "image_model_code", AgentModelSlotUpdate{ModelCode: "image"}); err == nil {
		t.Fatal("route mismatch accepted")
	}
}

func TestAgentModelSpecializedMirrorsAndLegacyOverrides(t *testing.T) {
	s, pool := agentModelsFixture(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
INSERT INTO models(code,display_name,category,request_mode,is_enabled,runtime_rule) VALUES
('upscale','超分','video','video',true,'{"capabilities":{"video_upscale":true}}');
INSERT INTO workflow_definitions(code,name,runtime_config,nodes) VALUES
('video_upscale','超分','{"agent_mode":"video_upscale","generation_model_code":"upscale"}','[{"id":"upscale","type":"video","model_code":"upscale"}]'),
('ai_novel_workshop','小说','{"agent_mode":"novel_workshop","analysis_model_code":"plain","generation_model_code":"plain"}','[{"id":"planning","type":"llm","model_code":"plain"},{"id":"writing","type":"llm","model_code":"plain"},{"id":"polishing","type":"llm","model_code":"plain"},{"id":"archiving","type":"llm","model_code":"plain"}]'),
('ai_photo_studio','写真','{"agent_mode":"photo_studio","analysis_model_code":"vision","generation_model_code":"image"}','[{"id":"styling","type":"llm","model_code":"vision"},{"id":"generate","type":"image","model_code":"image"}]');
UPDATE workflow_definitions SET nodes='[{"id":"analysis","type":"llm","model_code":"plain"},{"id":"generate","type":"image","model_code":"image-text"}]',runtime_config=runtime_config || '{"generation_model_code":"image-text"}' WHERE code='general_creative_agent';
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ agent, slot, code string }{
		{"video_upscale", "generation_model_code", "upscale"},
		{"ai_novel_workshop", "generation_model_code", "vision"},
		{"ai_photo_studio", "analysis_model_code", "vision"},
		{"general_creative_agent", "image_model_code", "image"},
	} {
		if _, err := s.UpdateAgentModelSlot(ctx, tc.agent, tc.slot, AgentModelSlotUpdate{ModelCode: tc.code}); err != nil {
			t.Fatalf("%s/%s: %v", tc.agent, tc.slot, err)
		}
		w, err := s.Get(ctx, tc.agent)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range w.Nodes {
			if modelcontract.NodeRuntimeField(contractDefinition(*w), modelcontract.Node{ID: n.ID}) == tc.slot && n.ModelCode != tc.code {
				t.Fatalf("descriptive node did not follow actual runtime: %+v", n)
			}
		}
		for _, row := range buildAgentModelBindings(*w, nil).Slots {
			if strings.HasPrefix(row.ID, "node.") {
				t.Fatalf("misleading duplicate binding: %s/%s", w.Code, row.ID)
			}
		}
	}
	for _, code := range []string{"ai_photo_studio", "ai_virtual_tryon", "ai_novel_workshop"} {
		if err := s.validateModelOverrides(ctx, WorkflowDTO{Code: code}, map[string]interface{}{"model_code": "image-text"}); err == nil {
			t.Fatalf("%s legacy model override bypassed capabilities", code)
		}
	}
	if err := s.validateModelOverrides(ctx, WorkflowDTO{Code: "ai_comic_drama"}, map[string]interface{}{"dialogue_model_code": "image"}); err == nil {
		t.Fatal("singular retry dialogue override bypassed chat requirement")
	}
	// An absent runtime mirror must be created when only a descriptive node exists.
	if _, err := pool.Exec(ctx, `UPDATE workflow_definitions SET runtime_config='{"agent_mode":"video_upscale"}' WHERE code='video_upscale'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateAgentModelSlot(ctx, "video_upscale", "generation_model_code", AgentModelSlotUpdate{ModelCode: "upscale"}); err != nil {
		t.Fatal(err)
	}
	var runtimeCode string
	if err := pool.QueryRow(ctx, `SELECT runtime_config->>'generation_model_code' FROM workflow_definitions WHERE code='video_upscale'`).Scan(&runtimeCode); err != nil || runtimeCode != "upscale" {
		t.Fatalf("runtime binding not repaired: %s %v", runtimeCode, err)
	}
}
