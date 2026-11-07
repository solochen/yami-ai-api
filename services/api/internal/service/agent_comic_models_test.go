package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 该回归只验证自愈逻辑本身，不依赖完整业务表；设置 AGENT_DRAFT_TEST_DATABASE_URL
// 后会在独立 schema 中建最小表并清理，未设置时跳过。
func TestHealComicDramaModels(t *testing.T) {
	dsn := os.Getenv("AGENT_DRAFT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AGENT_DRAFT_TEST_DATABASE_URL for isolated schema regression")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("comic_models_test_%d", time.Now().UnixNano())
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
		CREATE TABLE models(id bigserial PRIMARY KEY, code text UNIQUE, public_alias text, category text, request_mode text, is_enabled boolean, sort_order int, runtime_rule jsonb DEFAULT '{}', input_schema jsonb DEFAULT '{}', new_api_endpoint text);
		CREATE TABLE model_routes(id bigserial PRIMARY KEY, model_id bigint, is_enabled boolean);
		CREATE TABLE workflow_definitions(code text PRIMARY KEY, runtime_config jsonb);
		INSERT INTO models(code, public_alias, category, request_mode, is_enabled, sort_order) VALUES
		  ('chat_demo_v1', NULL, 'chat', 'chat_completions', false, 1),
		  ('deepseek-v4-pro-0813', NULL, 'chat', 'chat_completions', true, 50),
		  ('deepseek-v4-1-flash-1017', NULL, 'chat', 'chat_completions', true, 60),
		  ('image_fast_v1', NULL, 'image', 'images', false, 1),
		  ('seedream-5-0-lite', NULL, 'image', 'images', true, 4),
		  ('video_demo_v1', NULL, 'video', 'video', false, 1),
		  ('seedance-2-0-fast', NULL, 'video', 'video', true, 4),
		  ('audio_minimax_speech_28_hd', NULL, 'audio', 'audio', true, 75);
		UPDATE models SET runtime_rule='{"image":{"max_reference_images":4}}' WHERE category='image';
        UPDATE models SET runtime_rule='{"video":{"upload_profile":"first_last"}}' WHERE category='video';
        UPDATE models SET new_api_endpoint='/v1/audio/speech' WHERE category='audio';
        INSERT INTO model_routes(model_id, is_enabled) SELECT id, true FROM models WHERE is_enabled;
		INSERT INTO workflow_definitions(code, runtime_config) VALUES ('general_creative_agent',
		  '{"analysis_model_code":"deepseek-v4-pro-0813","image_model_code":"seedream-5-0-lite","video_model_code":"seedance-2-0-fast","speech_model_code":"audio_minimax_speech_28_hd"}');
	`)
	if err != nil {
		t.Fatal(err)
	}
	s := &AgentService{db: pool}
	inputs := map[string]interface{}{
		"image_model_code":     "image_fast_v1",
		"video_model_code":     "video_demo_v1",
		"narration_model_code": "audio_minimax_speech_28_hd",
		"dialogue_model_codes": []string{"chat_demo_v1"},
		"audio_strategy":       "hybrid",
	}
	s.healComicDramaModels(ctx, inputs)
	for key, want := range map[string]string{
		"image_model_code":     "seedream-5-0-lite",
		"video_model_code":     "seedance-2-0-fast",
		"narration_model_code": "audio_minimax_speech_28_hd",
		"audio_strategy":       "hybrid",
	} {
		if got := stringValue(inputs[key]); got != want {
			t.Fatalf("%s: got %q want %q", key, got, want)
		}
	}
	if codes := agentStringSlice(inputs["dialogue_model_codes"], nil); len(codes) != 1 || codes[0] != "deepseek-v4-pro-0813" {
		t.Fatalf("dialogue models not healed: %v", codes)
	}

	// 有效选择必须原样保留，不能每次都改写成“首选模型”。
	inputs["image_model_code"] = "seedream-5-0-lite"
	inputs["video_model_code"] = "seedance-2-0-fast"
	inputs["dialogue_model_codes"] = []string{"deepseek-v4-1-flash-1017"}
	s.healComicDramaModels(ctx, inputs)
	if got := stringValue(inputs["image_model_code"]); got != "seedream-5-0-lite" {
		t.Fatalf("valid image model replaced: %q", got)
	}
	if got := stringValue(inputs["video_model_code"]); got != "seedance-2-0-fast" {
		t.Fatalf("valid video model replaced: %q", got)
	}
	if codes := agentStringSlice(inputs["dialogue_model_codes"], nil); len(codes) != 1 || codes[0] != "deepseek-v4-1-flash-1017" {
		t.Fatalf("valid dialogue model replaced: %v", codes)
	}

	// 没有任何可用配音模型时，音频策略必须降级，不能再卡在“需要配音模型”。
	inputs["narration_model_code"] = "audio_minimax_speech_28_hd"
	inputs["audio_strategy"] = "hybrid"
	if _, err = pool.Exec(ctx, `UPDATE models SET is_enabled=false WHERE category='audio'`); err != nil {
		t.Fatal(err)
	}
	s.healComicDramaModels(ctx, inputs)
	if got := stringValue(inputs["audio_strategy"]); got != "video_native" {
		t.Fatalf("audio strategy not degraded: %q", got)
	}
}

// 历史演示模型即使仍被标记为启用，也不能继续作为 AI 漫剧的生产模型。
func TestHealComicDramaModelsRetiresDemoCodes(t *testing.T) {
	dsn := os.Getenv("AGENT_DRAFT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AGENT_DRAFT_TEST_DATABASE_URL for isolated schema regression")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("comic_demo_test_%d", time.Now().UnixNano())
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
		CREATE TABLE models(id bigserial PRIMARY KEY, code text UNIQUE, public_alias text, category text, request_mode text, is_enabled boolean, sort_order int, runtime_rule jsonb DEFAULT '{}', input_schema jsonb DEFAULT '{}', new_api_endpoint text);
		CREATE TABLE model_routes(id bigserial PRIMARY KEY, model_id bigint, is_enabled boolean);
		CREATE TABLE workflow_definitions(code text PRIMARY KEY, runtime_config jsonb);
		INSERT INTO models(code, category, request_mode, is_enabled, sort_order) VALUES
		  ('chat_demo_v1', 'chat', 'chat_completions', true, 1),
		  ('image_fast_v1', 'image', 'images', true, 1),
		  ('video_demo_v1', 'video', 'video', true, 1),
		  ('deepseek-v4-1-flash-1017', 'chat', 'chat_completions', true, 60),
		  ('seedream-5-0-lite', 'image', 'images', true, 4),
		  ('seedance-2-0-fast', 'video', 'video', true, 4),
		  ('audio_minimax_speech_28_hd', 'audio', 'audio', true, 75);
		UPDATE models SET runtime_rule='{"image":{"max_reference_images":4}}' WHERE category='image';
        UPDATE models SET runtime_rule='{"video":{"upload_profile":"first_last"}}' WHERE category='video';
        UPDATE models SET new_api_endpoint='/v1/audio/speech' WHERE category='audio';
        INSERT INTO model_routes(model_id, is_enabled) SELECT id, true FROM models;
	`)
	if err != nil {
		t.Fatal(err)
	}
	s := &AgentService{db: pool}
	inputs := map[string]interface{}{
		"image_model_code":     "image_fast_v1",
		"video_model_code":     "video_demo_v1",
		"dialogue_model_codes": []string{"chat_demo_v1"},
	}
	s.healComicDramaModels(ctx, inputs)
	for key, want := range map[string]string{
		"image_model_code": "seedream-5-0-lite",
		"video_model_code": "seedance-2-0-fast",
	} {
		if got := stringValue(inputs[key]); got != want {
			t.Fatalf("%s: got %q want %q", key, got, want)
		}
	}
	if codes := agentStringSlice(inputs["dialogue_model_codes"], nil); len(codes) != 1 || codes[0] != "deepseek-v4-1-flash-1017" {
		t.Fatalf("demo dialogue model not retired: %v", codes)
	}
}
