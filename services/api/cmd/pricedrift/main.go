// Command pricedrift 巡检上游价格漂移。
//
// 用户售价与上游成本解耦之后（model_routes.user_price_rule），上游涨价不会再
// 自动反映到用户售价上，毛利会被悄悄吃掉。本命令拉取 OpenRouter 的实时价格，
// 与本地线路配置的上游成本逐项比对，偏差超过阈值即写入 price_drift_alerts，
// 并在配置了收件人时发送告警邮件。
//
// 用法：
//
//	go run ./cmd/pricedrift            # 巡检并写库，超阈值时发邮件
//	go run ./cmd/pricedrift -dry-run   # 只打印，不写库不发邮件
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/starai/api/internal/mailer"
)

const (
	openRouterModelsURL = "https://openrouter.ai/api/v1/models"
	openRouterImagesURL = "https://openrouter.ai/api/v1/images/models"
	openRouterVideosURL = "https://openrouter.ai/api/v1/videos/models"
	defaultThreshold    = 10.0
)

type routeRow struct {
	ID            int64
	ModelCode     string
	Provider      string
	UpstreamModel string
	CostRule      map[string]interface{}
}

type drift struct {
	ModelCode     string
	RouteID       int64
	Provider      string
	UpstreamModel string
	Field         string
	Configured    float64
	Observed      float64
	Deviation     float64
	SourceURL     string
}

func main() {
	dryRun := flag.Bool("dry-run", false, "只打印结果，不写库、不发邮件")
	flag.Parse()

	_ = godotenv.Load("../../.env.local", "../../.env", ".env.local", ".env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://starai:starai@localhost:5432/starai?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	threshold := configFloat(ctx, pool, "price_drift_threshold_percent", defaultThreshold)
	alertEmail := configString(ctx, pool, "price_drift_alert_email")
	if alertEmail == "" {
		alertEmail = configString(ctx, pool, "smtp_from")
	}

	routes, err := loadOpenRouterRoutes(ctx, pool)
	if err != nil {
		log.Fatalf("load routes: %v", err)
	}
	fmt.Printf("巡检 %d 条 OpenRouter 线路，偏差阈值 %.1f%%\n\n", len(routes), threshold)

	catalog, err := fetchCatalog(ctx)
	if err != nil {
		log.Fatalf("fetch openrouter catalog: %v", err)
	}

	var drifts []drift
	for _, route := range routes {
		drifts = append(drifts, compareRoute(ctx, route, catalog, threshold)...)
	}
	sort.Slice(drifts, func(i, j int) bool { return math.Abs(drifts[i].Deviation) > math.Abs(drifts[j].Deviation) })

	if len(drifts) == 0 {
		fmt.Println("✅ 未发现超过阈值的价格漂移")
		return
	}
	fmt.Printf("⚠️  发现 %d 项价格漂移：\n", len(drifts))
	for _, d := range drifts {
		fmt.Printf("  %-28s %-14s 配置 %.6f → 现价 %.6f（%+.1f%%）\n",
			d.ModelCode, d.Field, d.Configured, d.Observed, d.Deviation)
	}

	if *dryRun {
		fmt.Println("\n（dry-run：未写库、未发邮件）")
		return
	}
	for _, d := range drifts {
		if err := insertAlert(ctx, pool, d); err != nil {
			log.Printf("insert alert: %v", err)
		}
	}
	if alertEmail != "" {
		if err := sendAlertEmail(ctx, pool, alertEmail, threshold, drifts); err != nil {
			log.Printf("send alert email: %v", err)
		} else {
			fmt.Printf("\n已发送告警邮件到 %s\n", alertEmail)
		}
	} else {
		fmt.Println("\n未配置 price_drift_alert_email，已仅写入 price_drift_alerts 表")
	}
}

func loadOpenRouterRoutes(ctx context.Context, pool *pgxpool.Pool) ([]routeRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT r.id, m.code, r.provider, r.upstream_model, r.cost_rule
		FROM model_routes r
		JOIN models m ON m.id = r.model_id
		WHERE r.is_enabled AND r.provider = 'openrouter'
		ORDER BY m.code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []routeRow
	for rows.Next() {
		var row routeRow
		var raw []byte
		if err := rows.Scan(&row.ID, &row.ModelCode, &row.Provider, &row.UpstreamModel, &raw); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &row.CostRule)
		out = append(out, row)
	}
	return out, rows.Err()
}

type catalog struct {
	models map[string]map[string]interface{}
	images map[string][]imagePrice
	videos map[string]map[string]interface{}
}

type imagePrice struct {
	variant string
	costUSD float64
}

// imageOutputPerToken 返回图片端点按 token 计费的输出价（USD/token），
// 按张计费的图片模型返回 0。
func imageOutputPerToken(ctx context.Context, c *catalog, modelID string) float64 {
	var payload struct {
		Endpoints []struct {
			Pricing []struct {
				Billable string  `json:"billable"`
				Unit     string  `json:"unit"`
				CostUSD  float64 `json:"cost_usd"`
			} `json:"pricing"`
		} `json:"endpoints"`
	}
	url := fmt.Sprintf("https://openrouter.ai/api/v1/images/models/%s/endpoints", modelID)
	if err := getJSON(ctx, url, &payload); err != nil {
		return 0
	}
	for _, ep := range payload.Endpoints {
		for _, p := range ep.Pricing {
			if p.Billable == "output_image" && p.Unit == "token" && p.CostUSD > 0 {
				return p.CostUSD
			}
		}
	}
	return 0
}

func fetchCatalog(ctx context.Context) (*catalog, error) {
	c := &catalog{models: map[string]map[string]interface{}{}, images: map[string][]imagePrice{}, videos: map[string]map[string]interface{}{}}

	var models struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := getJSON(ctx, openRouterModelsURL, &models); err != nil {
		return nil, err
	}
	for _, m := range models.Data {
		if id, ok := m["id"].(string); ok {
			c.models[id] = m
		}
	}

	var videos struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := getJSON(ctx, openRouterVideosURL, &videos); err != nil {
		return nil, err
	}
	for _, m := range videos.Data {
		if id, ok := m["id"].(string); ok {
			c.videos[id] = m
		}
	}

	var images struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := getJSON(ctx, openRouterImagesURL, &images); err != nil {
		return nil, err
	}
	for _, m := range images.Data {
		id, ok := m["id"].(string)
		if !ok {
			continue
		}
		c.images[id] = fetchImagePrices(ctx, id)
	}
	return c, nil
}

func fetchImagePrices(ctx context.Context, modelID string) []imagePrice {
	var payload struct {
		Endpoints []struct {
			Pricing []struct {
				Billable string  `json:"billable"`
				Unit     string  `json:"unit"`
				CostUSD  float64 `json:"cost_usd"`
				Variant  string  `json:"variant"`
			} `json:"pricing"`
		} `json:"endpoints"`
	}
	url := fmt.Sprintf("https://openrouter.ai/api/v1/images/models/%s/endpoints", modelID)
	if err := getJSON(ctx, url, &payload); err != nil {
		return nil
	}
	var out []imagePrice
	for _, ep := range payload.Endpoints {
		for _, p := range ep.Pricing {
			if p.Billable == "output_image" && p.Unit == "image" && p.CostUSD > 0 {
				out = append(out, imagePrice{variant: p.Variant, costUSD: p.CostUSD})
			}
		}
	}
	return out
}

func compareRoute(ctx context.Context, route routeRow, c *catalog, threshold float64) []drift {
	rule := route.CostRule
	billingType := strings.ToLower(stringValue(rule["billing_type"]))
	usdRate := floatValue(rule["usd_to_compute"])
	if usdRate <= 0 {
		usdRate = 7.2
	}
	// 配置值优先取 source_*（USD 原值），否则用算力值折算回 USD。
	configuredUSD := func(sourceKey, normalizedKey string) float64 {
		if v := floatValue(rule[sourceKey]); v > 0 {
			return v
		}
		return floatValue(rule[normalizedKey]) / usdRate
	}
	add := func(field string, configured, observed float64, source string) []drift {
		if configured <= 0 || observed <= 0 {
			return nil
		}
		deviation := (observed - configured) / configured * 100
		if math.Abs(deviation) < threshold {
			return nil
		}
		return []drift{{
			ModelCode: route.ModelCode, RouteID: route.ID, Provider: route.Provider,
			UpstreamModel: route.UpstreamModel, Field: field,
			Configured: configured, Observed: observed, Deviation: deviation, SourceURL: source,
		}}
	}

	switch billingType {
	case "per_token":
		model, ok := c.models[route.UpstreamModel]
		if !ok {
			return nil
		}
		pricing, _ := model["pricing"].(map[string]interface{})
		var out []drift
		out = append(out, add("输入/1M tokens", configuredUSD("source_input_cost_per_m", "input_cost_per_m"), floatValue(pricing["prompt"])*1e6, openRouterModelsURL)...)
		// 按 token 计费但输出是图片的模型（如 gemini-image 系列），
		// 输出价在图片端点里（output_image 每 token），文本 completion 不适用。
		if imageOutput := imageOutputPerToken(ctx, c, route.UpstreamModel); imageOutput > 0 {
			out = append(out, add("输出/1M image tokens", configuredUSD("source_output_cost_per_m", "output_cost_per_m"), imageOutput*1e6,
				fmt.Sprintf("https://openrouter.ai/api/v1/images/models/%s/endpoints", route.UpstreamModel))...)
		} else {
			out = append(out, add("输出/1M tokens", configuredUSD("source_output_cost_per_m", "output_cost_per_m"), floatValue(pricing["completion"])*1e6, openRouterModelsURL)...)
		}
		out = append(out, add("缓存读取/1M tokens", configuredUSD("source_cache_read_cost_per_m", "cache_read_cost_per_m"), floatValue(pricing["input_cache_read"])*1e6, openRouterModelsURL)...)
		return out

	case "per_image":
		prices := c.images[route.UpstreamModel]
		if len(prices) == 0 {
			return nil
		}
		base := prices[0]
		for _, p := range prices {
			if p.variant == "" {
				base = p
				break
			}
		}
		field := "每张图片"
		if base.variant != "" {
			field = "每张图片(" + base.variant + ")"
		}
		return add(field, configuredUSD("source_unit_cost", "unit_cost"), base.costUSD,
			fmt.Sprintf("https://openrouter.ai/api/v1/images/models/%s/endpoints", route.UpstreamModel))

	case "per_second":
		video, ok := c.videos[route.UpstreamModel]
		if !ok {
			return nil
		}
		skus, _ := video["pricing_skus"].(map[string]interface{})
		var observed float64
		var field string
		for key, value := range skus {
			lowered := strings.ToLower(key)
			switch {
			case strings.Contains(lowered, "cents_per_second") && !strings.Contains(lowered, "480p") && !strings.Contains(lowered, "720p") && !strings.Contains(lowered, "1080p") && !strings.Contains(lowered, "4k"):
				observed = floatValue(value) / 100
				field = "每秒"
			case lowered == "duration_seconds":
				observed = floatValue(value)
				field = "每秒"
			}
			if observed > 0 {
				break
			}
		}
		if observed <= 0 {
			return nil // 按 token 计费的视频线路无法直接比对，跳过
		}
		return add(field, configuredUSD("source_unit_cost", "unit_cost"), observed,
			fmt.Sprintf("https://openrouter.ai/api/v1/videos/models/%s", route.UpstreamModel))
	}
	return nil
}

func insertAlert(ctx context.Context, pool *pgxpool.Pool, d drift) error {
	// 同一线路同一字段 20 小时内不重复告警。
	var exists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM price_drift_alerts
		 WHERE route_id=$1 AND field=$2 AND created_at > now() - interval '20 hours')`,
		d.RouteID, d.Field).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO price_drift_alerts
		 (model_code, route_id, provider, upstream_model, field, configured, observed, deviation_percent, observed_currency, source_url)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'USD',$9)`,
		d.ModelCode, d.RouteID, d.Provider, d.UpstreamModel, d.Field,
		d.Configured, d.Observed, d.Deviation, d.SourceURL)
	return err
}

func sendAlertEmail(ctx context.Context, pool *pgxpool.Pool, to string, threshold float64, drifts []drift) error {
	svc := mailer.New(pool)
	cfg := svc.LoadConfig(ctx)
	if !cfg.Enabled {
		return fmt.Errorf("邮件通道未启用")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "价格漂移巡检发现 %d 项偏差（阈值 %.1f%%）：\n\n", len(drifts), threshold)
	for _, d := range drifts {
		fmt.Fprintf(&b, "模型 %s（%s）\n  字段：%s\n  配置成本：%.6f USD\n  上游现价：%.6f USD\n  偏差：%+.1f%%\n  来源：%s\n\n",
			d.ModelCode, d.UpstreamModel, d.Field, d.Configured, d.Observed, d.Deviation, d.SourceURL)
	}
	b.WriteString("说明：售价与上游成本解耦的线路（配了用户售价规则），上游涨价不会自动反映到用户售价，毛利会被压缩；请及时调整售价规则或切换线路。\n")
	return svc.Send(ctx, cfg, to, "[Orbit-E] 上游价格漂移告警", b.String())
}

func getJSON(ctx context.Context, url string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func configFloat(ctx context.Context, pool *pgxpool.Pool, key string, fallback float64) float64 {
	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT value FROM system_configs WHERE key=$1`, key).Scan(&raw); err != nil {
		return fallback
	}
	var value float64
	if json.Unmarshal(raw, &value) != nil || value <= 0 {
		return fallback
	}
	return value
}

func configString(ctx context.Context, pool *pgxpool.Pool, key string) string {
	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT value FROM system_configs WHERE key=$1`, key).Scan(&raw); err != nil {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func floatValue(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case string:
		var f float64
		if _, err := fmt.Sscan(v, &f); err == nil {
			return f
		}
	}
	return 0
}

func stringValue(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
