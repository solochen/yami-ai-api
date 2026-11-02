package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	urlpkg "net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------- types ----------

type UpstreamProvider struct {
	Provider            string                 `json:"provider"`
	DisplayName         string                 `json:"display_name"`
	SourceType          string                 `json:"source_type"`
	PriceEndpoint       string                 `json:"price_endpoint"`
	Currency            string                 `json:"currency"`
	RechargeRate        float64                `json:"recharge_rate"`
	USDCNYRate          float64                `json:"usd_cny_rate"`
	IsEnabled           bool                   `json:"is_enabled"`
	Config              map[string]interface{} `json:"config"`
	LastSyncAt          *time.Time             `json:"last_sync_at,omitempty"`
	LastStatus          string                 `json:"last_status"`
	ConsecutiveFailures int                    `json:"consecutive_failures"`
}

type PriceSyncRun struct {
	ID                int64      `json:"id"`
	Trigger           string     `json:"trigger"`
	Status            string     `json:"status"`
	ProviderTotal     int        `json:"provider_total"`
	ProviderSucceeded int        `json:"provider_succeeded"`
	ProviderFailed    int        `json:"provider_failed"`
	ItemsTotal        int        `json:"items_total"`
	ItemsMatched      int        `json:"items_matched"`
	ErrorSummary      string     `json:"error_summary"`
	CreatedBy         *int64     `json:"created_by,omitempty"`
	StartedAt         time.Time  `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
	Providers         []PriceSyncProviderRun `json:"providers,omitempty"`
}

type PriceSyncProviderRun struct {
	ID         int64      `json:"id"`
	SyncRunID  int64      `json:"sync_run_id"`
	Provider   string     `json:"provider"`
	Status     string     `json:"status"`
	HTTPStatus int        `json:"http_status"`
	Items      int        `json:"items"`
	LatencyMS  int        `json:"latency_ms"`
	Attempts   int        `json:"attempts"`
	Error      string     `json:"error"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type UpstreamPrice struct {
	Provider            string    `json:"provider"`
	UpstreamModel       string    `json:"upstream_model"`
	GroupName           string    `json:"group_name"`
	BillingType         string    `json:"billing_type"`
	SourceCurrency      string    `json:"source_currency"`
	SourceUnit          string    `json:"source_unit"`
	InputPrice          float64   `json:"input_price"`
	OutputPrice         float64   `json:"output_price"`
	CacheReadPrice      float64   `json:"cache_read_price"`
	CacheWritePrice     float64   `json:"cache_write_price"`
	UnitPrice           float64   `json:"unit_price"`
	BillingMultiplier   float64   `json:"billing_multiplier"`
	FXRate              float64   `json:"fx_rate"`
	CNYInputPerM        float64   `json:"cny_input_per_m"`
	CNYOutputPerM       float64   `json:"cny_output_per_m"`
	CNYCacheReadPerM    float64   `json:"cny_cache_read_per_m"`
	CNYCacheWritePerM   float64   `json:"cny_cache_write_per_m"`
	CNYUnitPrice        float64   `json:"cny_unit_price"`
	SourceURL           string    `json:"source_url"`
	SourceType          string    `json:"source_type"`
	IsTrusted           bool      `json:"is_trusted"`
	ObservedAt          time.Time `json:"observed_at"`
	IsStale             bool      `json:"is_stale"`
	ModelID             *int64    `json:"model_id,omitempty"`
	ModelCode           string    `json:"model_code,omitempty"`
	ModelName           string    `json:"model_name,omitempty"`
	Raw                 []byte    `json:"-"`
}

type PriceCost struct {
	BillingType string  `json:"billing_type"`
	Input       float64 `json:"input"`
	Output      float64 `json:"output"`
	CacheRead   float64 `json:"cache_read"`
	CacheWrite  float64 `json:"cache_write"`
	Unit        float64 `json:"unit"`
}

type RoutePriceBrief struct {
	RouteID      int64      `json:"route_id"`
	RouteName    string     `json:"route_name"`
	Provider     string     `json:"provider"`
	UpstreamModel string    `json:"upstream_model"`
	PriceGroup   string     `json:"price_group"`
	Priority     int        `json:"priority"`
	HealthStatus string     `json:"health_status"`
	Cost         PriceCost  `json:"cost"`
	CostSource   string     `json:"cost_source"`
	IsTrusted    bool       `json:"is_trusted"`
	ObservedAt   *time.Time `json:"observed_at,omitempty"`
	IsStale      bool       `json:"is_stale"`
	UserCost     *PriceCost `json:"user_cost,omitempty"`
}

type PotentialPrice struct {
	Provider      string    `json:"provider"`
	UpstreamModel string    `json:"upstream_model"`
	GroupName     string    `json:"group_name"`
	Cost          PriceCost `json:"cost"`
	ObservedAt    time.Time `json:"observed_at"`
}

type ModelUsageSummary struct {
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	ImageCount       int64   `json:"image_count"`
	VideoSeconds     float64 `json:"video_seconds"`
	RequestCount     int64   `json:"request_count"`
	ProviderCost     float64 `json:"provider_cost"`
}

type PriceAnalysisRow struct {
	ModelID          int64               `json:"model_id"`
	Code             string              `json:"code"`
	DisplayName      string              `json:"display_name"`
	Primary          RoutePriceBrief     `json:"primary"`
	Cheapest         RoutePriceBrief     `json:"cheapest"`
	Potential        *PotentialPrice     `json:"potential,omitempty"`
	GapPercent       float64             `json:"gap_percent"`
	DailySaving      float64             `json:"daily_saving"`
	MonthlySaving    float64             `json:"monthly_saving"`
	Usage            ModelUsageSummary   `json:"usage"`
	RouteCount       int                 `json:"route_count"`
	HasUserPrice     bool                `json:"has_user_price"`
	MonthlyRevenue   float64             `json:"monthly_revenue"`
	MonthlyCost      float64             `json:"monthly_cost"`
	MonthlyProfit    float64             `json:"monthly_profit"`
	Status           string              `json:"status"`
	MarginPercent    *float64            `json:"margin_percent,omitempty"`
	Backups          []RoutePriceBrief   `json:"backups"`
}

type RouteSwitchLog struct {
	ID                     int64      `json:"id"`
	ModelID                int64      `json:"model_id"`
	ModelCode              string     `json:"model_code,omitempty"`
	FromRouteID            *int64     `json:"from_route_id,omitempty"`
	ToRouteID              int64      `json:"to_route_id"`
	FromRouteName          string     `json:"from_route_name,omitempty"`
	ToRouteName            string     `json:"to_route_name,omitempty"`
	Reason                 string     `json:"reason"`
	EstimatedDailySaving   float64    `json:"estimated_daily_saving"`
	EstimatedMonthlySaving float64    `json:"estimated_monthly_saving"`
	AdminID                *int64     `json:"admin_id,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	RolledBackAt           *time.Time `json:"rolled_back_at,omitempty"`
	RollbackOfID           *int64     `json:"rollback_of_id,omitempty"`
}

type routeRow struct {
	ID           int64
	ModelID      int64
	Code         string
	DisplayName  string
	RouteName    string
	Provider     string
	UpstreamModel string
	PriceGroup   string
	Priority     int
	HealthStatus string
	CostRule     map[string]interface{}
	UserPrice    map[string]interface{}
}

// ---------- provider config ----------

func (s *AdminService) ListUpstreamProviders(ctx context.Context) ([]UpstreamProvider, error) {
	rows, err := s.db.Query(ctx, `SELECT provider,display_name,source_type,price_endpoint,currency,
		recharge_rate,usd_cny_rate,is_enabled,config,last_sync_at,last_status,consecutive_failures
		FROM upstream_providers ORDER BY provider`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UpstreamProvider{}
	for rows.Next() {
		var p UpstreamProvider
		var raw []byte
		if err := rows.Scan(&p.Provider, &p.DisplayName, &p.SourceType, &p.PriceEndpoint, &p.Currency,
			&p.RechargeRate, &p.USDCNYRate, &p.IsEnabled, &raw, &p.LastSyncAt, &p.LastStatus, &p.ConsecutiveFailures); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &p.Config)
		out = append(out, p)
	}
	return out, rows.Err()
}

type UpdateUpstreamProviderInput struct {
	DisplayName   *string                `json:"display_name"`
	SourceType    *string                `json:"source_type"`
	PriceEndpoint *string                `json:"price_endpoint"`
	Currency      *string                `json:"currency"`
	RechargeRate  *float64               `json:"recharge_rate"`
	USDCNYRate    *float64               `json:"usd_cny_rate"`
	IsEnabled     *bool                  `json:"is_enabled"`
	Config        map[string]interface{} `json:"config"`
}

func (s *AdminService) UpdateUpstreamProvider(ctx context.Context, provider string, in UpdateUpstreamProviderInput) (*UpstreamProvider, error) {
	if strings.TrimSpace(provider) == "" {
		return nil, errors.New("厂商不能为空")
	}
	var raw []byte
	if in.Config != nil {
		raw, _ = json.Marshal(in.Config)
	} else {
		raw = []byte("{}")
	}
	_, err := s.db.Exec(ctx, `UPDATE upstream_providers SET
		display_name=COALESCE($2,display_name),
		source_type=COALESCE($3,source_type),
		price_endpoint=COALESCE($4,price_endpoint),
		currency=COALESCE($5,currency),
		recharge_rate=COALESCE($6,recharge_rate),
		usd_cny_rate=COALESCE($7,usd_cny_rate),
		is_enabled=COALESCE($8,is_enabled),
		config=CASE WHEN $9::jsonb = '{}'::jsonb THEN config ELSE $9::jsonb END,
		updated_at=now()
		WHERE provider=$1`, provider, in.DisplayName, in.SourceType, in.PriceEndpoint, in.Currency,
		in.RechargeRate, in.USDCNYRate, in.IsEnabled, raw)
	if err != nil {
		return nil, err
	}
	list, err := s.ListUpstreamProviders(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].Provider == provider {
			return &list[i], nil
		}
	}
	return nil, errors.New("厂商不存在")
}

// ---------- sync ----------

func (s *AdminService) StartPriceSync(ctx context.Context, adminID int64, providers []string) (int64, error) {
	var running int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM price_sync_runs WHERE status='running' AND started_at > now()-interval '15 minutes'`).Scan(&running); err != nil {
		return 0, err
	}
	if running > 0 {
		return 0, errors.New("已有价格同步任务在运行，请稍后刷新")
	}
	all, err := s.ListUpstreamProviders(ctx)
	if err != nil {
		return 0, err
	}
	targets := []string{}
	want := map[string]bool{}
	for _, p := range providers {
		want[strings.TrimSpace(p)] = true
	}
	for _, p := range all {
		if !p.IsEnabled {
			continue
		}
		if len(want) > 0 && !want[p.Provider] {
			continue
		}
		if p.SourceType != "new_api" && p.SourceType != "openrouter" && p.SourceType != "new_api_hub" && p.SourceType != "codexauv_page" {
			continue
		}
		targets = append(targets, p.Provider)
	}
	if len(targets) == 0 {
		return 0, errors.New("没有可同步的厂商（仅支持 new_api / openrouter）")
	}
	var runID int64
	if err := s.db.QueryRow(ctx, `INSERT INTO price_sync_runs(trigger,status,provider_total,created_by,started_at)
		VALUES('manual','running',$1,$2,now()) RETURNING id`, len(targets), adminID).Scan(&runID); err != nil {
		return 0, err
	}
	go s.runPriceSync(runID, targets)
	return runID, nil
}

func (s *AdminService) runPriceSync(runID int64, targets []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	all, err := s.ListUpstreamProviders(ctx)
	if err != nil {
		s.finishPriceSync(ctx, runID, "failed", 0, 0, 0, 0, err.Error())
		return
	}
	byProvider := map[string]UpstreamProvider{}
	for _, p := range all {
		byProvider[p.Provider] = p
	}

	concurrency := s.configInt(ctx, "price_sync_concurrency", 5)
	if concurrency < 1 || concurrency > 10 {
		concurrency = 5
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	succeeded, failed, items, matched := 0, 0, 0, 0
	var errs []string

	for _, provider := range targets {
		p, ok := byProvider[provider]
		if !ok {
			continue
		}
		wg.Add(1)
		go func(p UpstreamProvider) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			prID, itemsN, matchedN, err := s.syncOneProvider(ctx, runID, p)
			mu.Lock()
			defer mu.Unlock()
			items += itemsN
			matched += matchedN
			if err != nil {
				failed++
				errs = append(errs, fmt.Sprintf("%s: %v", p.Provider, err))
				_ = prID
			} else {
				succeeded++
			}
		}(p)
	}
	wg.Wait()

	status := "success"
	if failed > 0 && succeeded > 0 {
		status = "partial"
	} else if failed > 0 && succeeded == 0 {
		status = "failed"
	}
	summary := strings.Join(errs, "; ")
	if len(summary) > 2000 {
		summary = summary[:2000]
	}
	s.finishPriceSync(ctx, runID, status, succeeded, failed, items, matched, summary)
	s.cleanupPriceSnapshots(ctx)
}

func (s *AdminService) syncOneProvider(ctx context.Context, runID int64, p UpstreamProvider) (int64, int, int, error) {
	started := time.Now()
	var prID int64
	if err := s.db.QueryRow(ctx, `INSERT INTO price_sync_provider_runs(sync_run_id,provider,status,started_at)
		VALUES($1,$2,'running',now()) RETURNING id`, runID, p.Provider).Scan(&prID); err != nil {
		return 0, 0, 0, err
	}
	timeout := time.Duration(s.configInt(ctx, "price_sync_timeout_seconds", 60)) * time.Second
	retries := s.configInt(ctx, "price_sync_retries", 2)
	var prices []UpstreamPrice
	var httpStatus int
	var err error
	attempts := 0
	for attempt := 0; attempt <= retries; attempt++ {
		attempts++
		prices, httpStatus, err = s.fetchProviderPrices(ctx, p, timeout)
		if err == nil {
			break
		}
		if attempt < retries {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}
	latency := int(time.Since(started).Milliseconds())
	status := "success"
	if err != nil {
		status = "failed"
		_, _ = s.db.Exec(ctx, `UPDATE upstream_providers SET last_status='failed', consecutive_failures=consecutive_failures+1, updated_at=now() WHERE provider=$1`, p.Provider)
		_, _ = s.db.Exec(ctx, `UPDATE upstream_price_current SET is_stale=true WHERE provider=$1`, p.Provider)
	} else {
		_, _ = s.db.Exec(ctx, `UPDATE upstream_providers SET last_sync_at=now(), last_status='success', consecutive_failures=0, updated_at=now() WHERE provider=$1`, p.Provider)
	}
	_, _ = s.db.Exec(ctx, `UPDATE price_sync_provider_runs SET status=$1,http_status=$2,items=$3,latency_ms=$4,attempts=$5,error=$6,finished_at=now() WHERE id=$7`,
		status, httpStatus, len(prices), latency, attempts, errString(err), prID)

	if err != nil {
		return prID, 0, 0, err
	}
	matched := 0
	for _, item := range prices {
		item.ObservedAt = time.Now()
		if err := s.saveUpstreamPrice(ctx, &runID, item); err != nil {
			return prID, len(prices), matched, err
		}
		var modelID *int64
		_ = s.db.QueryRow(ctx, `SELECT model_id FROM upstream_model_mappings WHERE provider=$1 AND upstream_model=$2`, item.Provider, item.UpstreamModel).Scan(&modelID)
		if modelID != nil {
			matched++
		}
	}
	return prID, len(prices), matched, nil
}

func (s *AdminService) fetchProviderPrices(ctx context.Context, p UpstreamProvider, timeout time.Duration) ([]UpstreamPrice, int, error) {
	switch p.SourceType {
	case "new_api":
		return s.fetchNewAPIPrices(ctx, p, timeout)
	case "openrouter":
		return s.fetchOpenRouterPrices(ctx, p, timeout)
	case "new_api_hub":
		return s.fetchNewAPIHubPrices(ctx, p, timeout)
	case "codexauv_page":
		return s.fetchCodexAuvPrices(ctx, p, timeout)
	default:
		return nil, 0, errors.New("不支持的同步来源")
	}
}

func (s *AdminService) fetchNewAPIPrices(ctx context.Context, p UpstreamProvider, timeout time.Duration) ([]UpstreamPrice, int, error) {
	base := strings.TrimRight(strings.TrimSpace(p.PriceEndpoint), "/")
	if base == "" {
		return nil, 0, errors.New("缺少 price_endpoint")
	}
	var status map[string]interface{}
	code, err := httpGetJSON(ctx, base+"/api/status", timeout, &status)
	if err != nil {
		return nil, code, err
	}
	var pricing map[string]interface{}
	code, err = httpGetJSON(ctx, base+"/api/pricing", timeout, &pricing)
	if err != nil {
		return nil, code, err
	}
	statusData, _ := status["data"].(map[string]interface{})
	qpu := numAny(statusData["quota_per_unit"])
	if qpu <= 0 {
		qpu = 500000
	}
	statusPrice := numAny(statusData["price"])
	settlement := p.RechargeRate
	if p.Currency == "CNY" && statusPrice > 0 {
		settlement = statusPrice
	}
	if p.Currency == "USD" {
		settlement = p.USDCNYRate
	}
	if settlement <= 0 {
		settlement = 1
	}
	groupRatio, _ := pricing["group_ratio"].(map[string]interface{})
	trustedGroups := stringSlice(p.Config["trusted_groups"])
	data, _ := pricing["data"].([]interface{})
	out := []UpstreamPrice{}
	for _, item := range data {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name := strAny(m["model_name"])
		if name == "" {
			continue
		}
		groups := stringSlice(m["enable_groups"])
		if len(groups) == 0 {
			groups = []string{"default"}
		}
		for _, group := range groups {
			ratio := numAny(groupRatio[group])
			if ratio <= 0 {
				ratio = 1
			}
			trusted := true
			if len(trustedGroups) > 0 {
				trusted = stringInSlice(trustedGroups, group)
			}
			price := buildNewAPIPrice(m, group, ratio, settlement, qpu, p.Provider, name, trusted)
			if price == nil {
				continue
			}
			out = append(out, *price)
		}
	}
	return out, 200, nil
}

var exprCoefRe = regexp.MustCompile(`\b(p|c|cr|cc)\s*\*\s*([0-9.]+)`)
var fixedRe = regexp.MustCompile(`fixed\(([0-9.]+)\)`)
var secondsRe = regexp.MustCompile(`u\("seconds"\)\s*\*\s*([0-9.]+)`)

func buildNewAPIPrice(m map[string]interface{}, group string, ratio, settlement, qpu float64, provider, name string, trusted bool) *UpstreamPrice {
	expr := strAny(m["billing_expr"])
	billingType := "per_token"
	sourceUnit := "per_1m_tokens"
	quotaType := int(numAny(m["quota_type"]))
	lowerName := strings.ToLower(name)
	if strings.Contains(expr, "u(\"seconds\")") {
		billingType = "per_second"
		sourceUnit = "per_second"
	} else if quotaType == 1 || strings.Contains(lowerName, "image") || strings.Contains(lowerName, "seedream") || strings.Contains(expr, "image_count") {
		billingType = "per_image"
		sourceUnit = "per_image"
	} else if strings.Contains(expr, "fixed(") && !strings.Contains(expr, "p *") {
		billingType = "per_request"
		sourceUnit = "per_request"
	}
	p := &UpstreamPrice{
		Provider: provider, UpstreamModel: name, GroupName: group,
		BillingType: billingType, SourceCurrency: "CNY", SourceUnit: sourceUnit,
		BillingMultiplier: ratio, FXRate: 1, SourceType: "sync", IsTrusted: trusted,
		SourceURL: "",
	}
	base := 1.0
	if billingType == "per_token" {
		coefs := parseExprCoefs(expr)
		ratioBase := numAny(m["model_ratio"]) * (1000000 / qpu)
		pv := coefs["p"]
		if pv <= 0 {
			pv = ratioBase
		}
		cv := coefs["c"]
		if cv <= 0 {
			cv = pv * numAny(m["completion_ratio"])
		}
		crv := coefs["cr"]
		if crv <= 0 && m["cache_ratio"] != nil {
			crv = pv * numAny(m["cache_ratio"])
		}
		ccv := coefs["cc"]
		if ccv <= 0 && m["create_cache_ratio"] != nil {
			ccv = pv * numAny(m["create_cache_ratio"])
		}
		p.InputPrice = pv
		p.OutputPrice = cv
		p.CacheReadPrice = crv
		p.CacheWritePrice = ccv
	} else if billingType == "per_second" {
		base = 0
		if mm := secondsRe.FindStringSubmatch(expr); len(mm) == 2 {
			base, _ = strconv.ParseFloat(mm[1], 64)
		}
		if base <= 0 {
			base = numAny(m["model_price"])
		}
		p.UnitPrice = base
	} else {
		base = numAny(m["model_price"])
		if base <= 0 {
			if mm := fixedRe.FindStringSubmatch(expr); len(mm) == 2 {
				base, _ = strconv.ParseFloat(mm[1], 64)
			}
		}
		p.UnitPrice = base
	}
	if p.InputPrice <= 0 && p.OutputPrice <= 0 && p.UnitPrice <= 0 {
		return nil
	}
	mult := ratio * settlement
	p.CNYInputPerM = p.InputPrice * mult
	p.CNYOutputPerM = p.OutputPrice * mult
	p.CNYCacheReadPerM = p.CacheReadPrice * mult
	p.CNYCacheWritePerM = p.CacheWritePrice * mult
	p.CNYUnitPrice = p.UnitPrice * mult
	return p
}

func parseExprCoefs(expr string) map[string]float64 {
	out := map[string]float64{}
	for _, m := range exprCoefRe.FindAllStringSubmatch(expr, -1) {
		if len(m) == 3 {
			v, _ := strconv.ParseFloat(m[2], 64)
			if v > 0 {
				out[m[1]] = v
			}
		}
	}
	return out
}

func (s *AdminService) fetchNewAPIHubPrices(ctx context.Context, p UpstreamProvider, timeout time.Duration) ([]UpstreamPrice, int, error) {
	base := strings.TrimRight(strings.TrimSpace(p.PriceEndpoint), "/")
	if base == "" {
		return nil, 0, errors.New("缺少 inspector 地址（price_endpoint）")
	}
	group := "default"
	if v, ok := p.Config["group"].(string); ok && strings.TrimSpace(v) != "" {
		group = strings.TrimSpace(v)
	}
	days := 7
	if v, ok := p.Config["days"]; ok {
		if n := int(numAny(v)); n > 0 {
			days = n
		}
	}
	url := fmt.Sprintf("%s/internal/upstreams?all=1&group=%s&days=%d", base, group, days)
	var payload map[string]interface{}
	code, err := httpGetJSON(ctx, url, timeout, &payload)
	if err != nil {
		return nil, code, err
	}
	models, _ := payload["models"].([]interface{})
	out := []UpstreamPrice{}
	seen := map[string]bool{}
	for _, item := range models {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		primary, _ := m["primary"].(map[string]interface{})
		if primary == nil {
			continue
		}
		pc, _ := primary["price_cny"].(map[string]interface{})
		if pc == nil {
			continue
		}
		upstream := strings.TrimSpace(strAny(primary["upstream_model"]))
		if upstream == "" {
			upstream = strings.TrimSpace(strAny(m["model"]))
		}
		if upstream == "" {
			continue
		}
		key := upstream + "\x00" + group
		if seen[key] {
			continue
		}
		seen[key] = true
		bt := strings.TrimSpace(strAny(pc["billing_type"]))
		if bt == "" {
			bt = "per_token"
		}
		unit := "per_1m_tokens"
		if bt != "per_token" {
			unit = bt
		}
		providerName := strings.TrimSpace(strAny(primary["provider"]))
		if providerName == "" {
			providerName = "unknown"
		}
		raw, _ := json.Marshal(map[string]interface{}{
			"channel_id": primary["channel_id"], "channel_name": primary["channel_name"],
			"channel_type": primary["channel_type"], "provider": providerName,
			"upstream_model": upstream, "usage": primary["usage"],
		})
		out = append(out, UpstreamPrice{
			Provider: p.Provider, UpstreamModel: upstream, GroupName: group,
			BillingType: bt, SourceCurrency: "CNY", SourceUnit: unit,
			InputPrice: numAny(pc["input_per_m"]), OutputPrice: numAny(pc["output_per_m"]),
			CacheReadPrice: numAny(pc["cache_read_per_m"]), CacheWritePrice: numAny(pc["cache_write_per_m"]),
			UnitPrice: numAny(pc["unit_cost"]), BillingMultiplier: 1, FXRate: 1,
			CNYInputPerM: numAny(pc["input_per_m"]), CNYOutputPerM: numAny(pc["output_per_m"]),
			CNYCacheReadPerM: numAny(pc["cache_read_per_m"]), CNYCacheWritePerM: numAny(pc["cache_write_per_m"]),
			CNYUnitPrice: numAny(pc["unit_cost"]), SourceURL: base,
			SourceType: "hub:" + providerName, IsTrusted: true,
			Raw: raw,
		})
	}
	if len(out) == 0 {
		return nil, code, errors.New("hub 没有返回可用价格")
	}
	return out, code, nil
}

func (s *AdminService) fetchCodexAuvPrices(ctx context.Context, p UpstreamProvider, timeout time.Duration) ([]UpstreamPrice, int, error) {
	base := strings.TrimRight(strings.TrimSpace(p.PriceEndpoint), "/")
	if base == "" {
		base = "https://codexauv.com"
	}
	url := base + "/api/v1/model-plaza"
	client := &http.Client{Timeout: timeout}
	if proxy, _ := p.Config["proxy_url"].(string); strings.TrimSpace(proxy) != "" {
		if pu, err := urlpkg.Parse(strings.TrimSpace(proxy)); err == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(pu)}
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, resp.StatusCode, err
	}
	data, _ := payload["data"].(map[string]interface{})
	groups, _ := data["groups"].([]interface{})
	out := []UpstreamPrice{}
	seen := map[string]bool{}
	for _, git := range groups {
		g, ok := git.(map[string]interface{})
		if !ok {
			continue
		}
		groupName := strings.TrimSpace(strAny(g["name"]))
		rate := numAny(g["rate_multiplier"])
		if rate <= 0 {
			rate = 1
		}
		models, _ := g["models"].([]interface{})
		for _, mit := range models {
			m, ok := mit.(map[string]interface{})
			if !ok {
				continue
			}
			name := strings.TrimSpace(strAny(m["name"]))
			if name == "" {
				continue
			}
			pricing, _ := m["pricing"].(map[string]interface{})
			if pricing == nil {
				continue
			}
			mode := strings.ToLower(strings.TrimSpace(strAny(pricing["billing_mode"])))
			currency, fx := codexAuvCurrency(name, p.USDCNYRate)
			bt, unit := "per_token", "per_1m_tokens"
			inPrice := numAny(pricing["input_price"])
			outPrice := numAny(pricing["output_price"])
			cacheRead := numAny(pricing["cache_read_price"])
			cacheWrite := numAny(pricing["cache_write_price"])
			unitPrice := numAny(pricing["per_request_price"])
			if mode == "request" || (inPrice <= 0 && outPrice <= 0 && unitPrice > 0) {
				bt, unit = "per_request", "per_request"
				unitPrice = unitPrice * rate * fx
			} else {
				inPrice = inPrice * rate * fx * 1e6
				outPrice = outPrice * rate * fx * 1e6
				cacheRead = cacheRead * rate * fx * 1e6
				cacheWrite = cacheWrite * rate * fx * 1e6
			}
			if inPrice <= 0 && outPrice <= 0 && unitPrice <= 0 {
				continue
			}
			key := name + "\x00" + groupName
			if seen[key] {
				continue
			}
			seen[key] = true
			raw, _ := json.Marshal(map[string]interface{}{
				"group_id": g["id"], "platform": g["platform"], "rate_multiplier": rate,
				"is_channel_priced": m["is_channel_priced"], "official_pricing": m["official_pricing"],
			})
			out = append(out, UpstreamPrice{
				Provider: p.Provider, UpstreamModel: name, GroupName: groupName,
				BillingType: bt, SourceCurrency: currency, SourceUnit: unit,
				InputPrice: inPrice, OutputPrice: outPrice, CacheReadPrice: cacheRead, CacheWritePrice: cacheWrite,
				UnitPrice: unitPrice, BillingMultiplier: rate, FXRate: fx,
				CNYInputPerM: inPrice, CNYOutputPerM: outPrice, CNYCacheReadPerM: cacheRead, CNYCacheWritePerM: cacheWrite,
				CNYUnitPrice: unitPrice, SourceURL: url, SourceType: "codexauv_page", IsTrusted: true,
				Raw: raw,
			})
		}
	}
	if len(out) == 0 {
		return nil, resp.StatusCode, errors.New("CodexAuv 没有返回可用价格")
	}
	return out, resp.StatusCode, nil
}

func codexAuvCurrency(model string, usdCNY float64) (string, float64) {
	n := strings.ToLower(model)
	for _, prefix := range []string{"deepseek", "glm", "qwen", "kimi", "minimax", "mimo", "hunyuan", "doubao", "seed", "wan", "hailuo", "vidu", "kling"} {
		if strings.HasPrefix(n, prefix) {
			return "CNY", 1
		}
	}
	if usdCNY <= 0 {
		usdCNY = 6.75
	}
	return "USD", usdCNY
}

func (s *AdminService) fetchOpenRouterPrices(ctx context.Context, p UpstreamProvider, timeout time.Duration) ([]UpstreamPrice, int, error) {
	base := strings.TrimRight(strings.TrimSpace(p.PriceEndpoint), "/")
	if base == "" {
		base = "https://openrouter.ai/api/v1"
	}
	var payload map[string]interface{}
	code, err := httpGetJSON(ctx, base+"/models", timeout, &payload)
	if err != nil {
		return nil, code, err
	}
	fx := p.USDCNYRate
	if fx <= 0 {
		fx = 6.75
	}
	data, _ := payload["data"].([]interface{})
	out := []UpstreamPrice{}
	for _, item := range data {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id := strAny(m["id"])
		pricing, _ := m["pricing"].(map[string]interface{})
		if id == "" || pricing == nil {
			continue
		}
		prompt := numAny(pricing["prompt"]) * 1000000
		completion := numAny(pricing["completion"]) * 1000000
		cacheRead := numAny(pricing["input_cache_read"]) * 1000000
		cacheWrite := numAny(pricing["input_cache_write"]) * 1000000
		unit := numAny(pricing["image"]) + numAny(pricing["request"])
		if prompt <= 0 && completion <= 0 && unit <= 0 {
			continue
		}
		billingType := "per_token"
		sourceUnit := "per_1m_tokens"
		if prompt <= 0 && completion <= 0 {
			billingType = "per_request"
			sourceUnit = "per_request"
		}
		price := UpstreamPrice{
			Provider: p.Provider, UpstreamModel: id, GroupName: "default",
			BillingType: billingType, SourceCurrency: "USD", SourceUnit: sourceUnit,
			InputPrice: prompt, OutputPrice: completion, CacheReadPrice: cacheRead, CacheWritePrice: cacheWrite,
			UnitPrice: unit, BillingMultiplier: 1, FXRate: fx,
			CNYInputPerM: prompt * fx, CNYOutputPerM: completion * fx,
			CNYCacheReadPerM: cacheRead * fx, CNYCacheWritePerM: cacheWrite * fx, CNYUnitPrice: unit * fx,
			SourceType: "sync", IsTrusted: true,
		}
		out = append(out, price)
	}
	return out, 200, nil
}

func (s *AdminService) saveUpstreamPrice(ctx context.Context, runID *int64, p UpstreamPrice) error {
	raw, _ := json.Marshal(p)
	var snapID int64
	if err := s.db.QueryRow(ctx, `INSERT INTO upstream_price_snapshots
		(sync_run_id,provider,upstream_model,group_name,billing_type,source_currency,source_unit,
		 input_price,output_price,cache_read_price,cache_write_price,unit_price,
		 billing_multiplier,fx_rate,cny_input_per_m,cny_output_per_m,cny_cache_read_per_m,cny_cache_write_per_m,cny_unit_price,
		 source_url,source_type,is_trusted,observed_at,raw)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)
		RETURNING id`,
		runID, p.Provider, p.UpstreamModel, p.GroupName, p.BillingType, p.SourceCurrency, p.SourceUnit,
		p.InputPrice, p.OutputPrice, p.CacheReadPrice, p.CacheWritePrice, p.UnitPrice,
		p.BillingMultiplier, p.FXRate, p.CNYInputPerM, p.CNYOutputPerM, p.CNYCacheReadPerM, p.CNYCacheWritePerM, p.CNYUnitPrice,
		p.SourceURL, p.SourceType, p.IsTrusted, p.ObservedAt, raw).Scan(&snapID); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, `INSERT INTO upstream_price_current
		(provider,upstream_model,group_name,snapshot_id,billing_type,source_currency,source_unit,
		 input_price,output_price,cache_read_price,cache_write_price,unit_price,
		 billing_multiplier,fx_rate,cny_input_per_m,cny_output_per_m,cny_cache_read_per_m,cny_cache_write_per_m,cny_unit_price,
		 source_url,source_type,is_trusted,observed_at,is_stale)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,false)
		ON CONFLICT (provider,upstream_model,group_name) DO UPDATE SET
		 snapshot_id=EXCLUDED.snapshot_id,billing_type=EXCLUDED.billing_type,source_currency=EXCLUDED.source_currency,
		 source_unit=EXCLUDED.source_unit,input_price=EXCLUDED.input_price,output_price=EXCLUDED.output_price,
		 cache_read_price=EXCLUDED.cache_read_price,cache_write_price=EXCLUDED.cache_write_price,unit_price=EXCLUDED.unit_price,
		 billing_multiplier=EXCLUDED.billing_multiplier,fx_rate=EXCLUDED.fx_rate,
		 cny_input_per_m=EXCLUDED.cny_input_per_m,cny_output_per_m=EXCLUDED.cny_output_per_m,
		 cny_cache_read_per_m=EXCLUDED.cny_cache_read_per_m,cny_cache_write_per_m=EXCLUDED.cny_cache_write_per_m,
		 cny_unit_price=EXCLUDED.cny_unit_price,source_url=EXCLUDED.source_url,source_type=EXCLUDED.source_type,
		 is_trusted=EXCLUDED.is_trusted,observed_at=EXCLUDED.observed_at,is_stale=false`,
		p.Provider, p.UpstreamModel, p.GroupName, snapID, p.BillingType, p.SourceCurrency, p.SourceUnit,
		p.InputPrice, p.OutputPrice, p.CacheReadPrice, p.CacheWritePrice, p.UnitPrice,
		p.BillingMultiplier, p.FXRate, p.CNYInputPerM, p.CNYOutputPerM, p.CNYCacheReadPerM, p.CNYCacheWritePerM, p.CNYUnitPrice,
		p.SourceURL, p.SourceType, p.IsTrusted, p.ObservedAt)
	return err
}

func (s *AdminService) finishPriceSync(ctx context.Context, runID int64, status string, succeeded, failed, items, matched int, summary string) {
	_, _ = s.db.Exec(ctx, `UPDATE price_sync_runs SET status=$2,provider_succeeded=$3,provider_failed=$4,
		items_total=$5,items_matched=$6,error_summary=$7,finished_at=now() WHERE id=$1`,
		runID, status, succeeded, failed, items, matched, summary)
}

func (s *AdminService) cleanupPriceSnapshots(ctx context.Context) {
	days := s.configInt(ctx, "price_snapshot_retention_days", 60)
	if days < 1 {
		return
	}
	_, _ = s.db.Exec(ctx, `DELETE FROM upstream_price_snapshots WHERE observed_at < now()-make_interval(days=>$1)`, days)
}

// ---------- sync runs / catalog ----------

func (s *AdminService) ListPriceSyncRuns(ctx context.Context, page, pageSize int) ([]PriceSyncRun, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM price_sync_runs`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(ctx, `SELECT id,trigger,status,provider_total,provider_succeeded,provider_failed,
		items_total,items_matched,COALESCE(error_summary,''),created_by,started_at,finished_at
		FROM price_sync_runs ORDER BY started_at DESC LIMIT $1 OFFSET $2`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []PriceSyncRun{}
	for rows.Next() {
		var r PriceSyncRun
		if err := rows.Scan(&r.ID, &r.Trigger, &r.Status, &r.ProviderTotal, &r.ProviderSucceeded, &r.ProviderFailed,
			&r.ItemsTotal, &r.ItemsMatched, &r.ErrorSummary, &r.CreatedBy, &r.StartedAt, &r.FinishedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

func (s *AdminService) GetPriceSyncRun(ctx context.Context, id int64) (*PriceSyncRun, error) {
	var r PriceSyncRun
	err := s.db.QueryRow(ctx, `SELECT id,trigger,status,provider_total,provider_succeeded,provider_failed,
		items_total,items_matched,COALESCE(error_summary,''),created_by,started_at,finished_at
		FROM price_sync_runs WHERE id=$1`, id).Scan(&r.ID, &r.Trigger, &r.Status, &r.ProviderTotal,
		&r.ProviderSucceeded, &r.ProviderFailed, &r.ItemsTotal, &r.ItemsMatched, &r.ErrorSummary,
		&r.CreatedBy, &r.StartedAt, &r.FinishedAt)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT id,sync_run_id,provider,status,COALESCE(http_status,0),items,latency_ms,attempts,
		COALESCE(error,''),started_at,finished_at FROM price_sync_provider_runs WHERE sync_run_id=$1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p PriceSyncProviderRun
		if err := rows.Scan(&p.ID, &p.SyncRunID, &p.Provider, &p.Status, &p.HTTPStatus, &p.Items, &p.LatencyMS,
			&p.Attempts, &p.Error, &p.StartedAt, &p.FinishedAt); err != nil {
			return nil, err
		}
		r.Providers = append(r.Providers, p)
	}
	return &r, rows.Err()
}

func (s *AdminService) ListUpstreamPrices(ctx context.Context, page, pageSize int, provider, q, group string, trustedOnly bool) ([]UpstreamPrice, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	where := []string{"1=1"}
	args := []interface{}{}
	if provider != "" {
		args = append(args, provider)
		where = append(where, fmt.Sprintf("c.provider=$%d", len(args)))
	}
	if q != "" {
		args = append(args, "%"+q+"%")
		where = append(where, fmt.Sprintf("(c.upstream_model ILIKE $%d OR m.code ILIKE $%d OR m.display_name ILIKE $%d)", len(args), len(args), len(args)))
	}
	if group != "" {
		args = append(args, group)
		where = append(where, fmt.Sprintf("c.group_name=$%d", len(args)))
	}
	if trustedOnly {
		where = append(where, "c.is_trusted=true")
	}
	cond := strings.Join(where, " AND ")
	var total int
	countQ := `SELECT COUNT(*) FROM upstream_price_current c
		LEFT JOIN upstream_model_mappings mp ON mp.provider=c.provider AND mp.upstream_model=c.upstream_model
		LEFT JOIN models m ON m.id=mp.model_id WHERE ` + cond
	if err := s.db.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	query := `SELECT c.provider,c.upstream_model,c.group_name,c.billing_type,c.source_currency,c.source_unit,
		COALESCE(c.input_price,0),COALESCE(c.output_price,0),COALESCE(c.cache_read_price,0),COALESCE(c.cache_write_price,0),COALESCE(c.unit_price,0),
		c.billing_multiplier,c.fx_rate,COALESCE(c.cny_input_per_m,0),COALESCE(c.cny_output_per_m,0),
		COALESCE(c.cny_cache_read_per_m,0),COALESCE(c.cny_cache_write_per_m,0),COALESCE(c.cny_unit_price,0),
		COALESCE(c.source_url,''),c.source_type,c.is_trusted,c.observed_at,c.is_stale,mp.model_id,COALESCE(m.code,''),COALESCE(m.display_name,'')
		FROM upstream_price_current c
		LEFT JOIN upstream_model_mappings mp ON mp.provider=c.provider AND mp.upstream_model=c.upstream_model
		LEFT JOIN models m ON m.id=mp.model_id
		WHERE ` + cond + ` ORDER BY c.cny_input_per_m NULLS LAST, c.cny_unit_price NULLS LAST, c.provider, c.upstream_model
		LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []UpstreamPrice{}
	for rows.Next() {
		var p UpstreamPrice
		if err := rows.Scan(&p.Provider, &p.UpstreamModel, &p.GroupName, &p.BillingType, &p.SourceCurrency, &p.SourceUnit,
			&p.InputPrice, &p.OutputPrice, &p.CacheReadPrice, &p.CacheWritePrice, &p.UnitPrice,
			&p.BillingMultiplier, &p.FXRate, &p.CNYInputPerM, &p.CNYOutputPerM, &p.CNYCacheReadPerM, &p.CNYCacheWritePerM, &p.CNYUnitPrice,
			&p.SourceURL, &p.SourceType, &p.IsTrusted, &p.ObservedAt, &p.IsStale, &p.ModelID, &p.ModelCode, &p.ModelName); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// ---------- manual price ----------

type ManualPriceInput struct {
	Provider       string  `json:"provider"`
	UpstreamModel  string  `json:"upstream_model"`
	GroupName      string  `json:"group_name"`
	BillingType    string  `json:"billing_type"`
	Currency       string  `json:"currency"`
	InputPrice     float64 `json:"input_price"`
	OutputPrice    float64 `json:"output_price"`
	CacheReadPrice float64 `json:"cache_read_price"`
	CacheWritePrice float64 `json:"cache_write_price"`
	UnitPrice      float64 `json:"unit_price"`
	BillingMultiplier float64 `json:"billing_multiplier"`
	FXRate         float64 `json:"fx_rate"`
	SourceURL      string  `json:"source_url"`
	IsTrusted      *bool   `json:"is_trusted"`
}

func (s *AdminService) UpsertManualPrice(ctx context.Context, in ManualPriceInput) (*UpstreamPrice, error) {
	if strings.TrimSpace(in.Provider) == "" || strings.TrimSpace(in.UpstreamModel) == "" {
		return nil, errors.New("provider 和 upstream_model 不能为空")
	}
	bt := strings.TrimSpace(in.BillingType)
	if bt == "" {
		bt = "per_token"
	}
	if in.BillingMultiplier <= 0 {
		in.BillingMultiplier = 1
	}
	if in.FXRate <= 0 {
		in.FXRate = 1
	}
	unit := "per_1m_tokens"
	if bt != "per_token" {
		unit = bt
	}
	trusted := true
	if in.IsTrusted != nil {
		trusted = *in.IsTrusted
	}
	p := UpstreamPrice{
		Provider: in.Provider, UpstreamModel: in.UpstreamModel, GroupName: in.GroupName,
		BillingType: bt, SourceCurrency: in.Currency, SourceUnit: unit,
		InputPrice: in.InputPrice, OutputPrice: in.OutputPrice, CacheReadPrice: in.CacheReadPrice,
		CacheWritePrice: in.CacheWritePrice, UnitPrice: in.UnitPrice,
		BillingMultiplier: in.BillingMultiplier, FXRate: in.FXRate,
		SourceURL: in.SourceURL, SourceType: "manual", IsTrusted: trusted, ObservedAt: time.Now(),
	}
	p.CNYInputPerM = p.InputPrice * p.BillingMultiplier * p.FXRate
	p.CNYOutputPerM = p.OutputPrice * p.BillingMultiplier * p.FXRate
	p.CNYCacheReadPerM = p.CacheReadPrice * p.BillingMultiplier * p.FXRate
	p.CNYCacheWritePerM = p.CacheWritePrice * p.BillingMultiplier * p.FXRate
	p.CNYUnitPrice = p.UnitPrice * p.BillingMultiplier * p.FXRate
	if err := s.saveUpstreamPrice(ctx, nil, p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ApplyRealCost 把网关真实成本写入指定线路的 cost_rule。仅人工触发，不自动改账。
func (s *AdminService) ApplyRealCost(ctx context.Context, modelID, routeID int64) (*UpstreamPrice, error) {
	if modelID <= 0 || routeID <= 0 {
		return nil, errors.New("模型或线路 ID 无效")
	}
	var provider, upstream, priceGroup string
	if err := s.db.QueryRow(ctx, `SELECT provider,upstream_model,COALESCE(price_group,'') FROM model_routes WHERE id=$1 AND model_id=$2`, routeID, modelID).Scan(&provider, &upstream, &priceGroup); err != nil {
		return nil, errors.New("线路不存在")
	}
	var p UpstreamPrice
	query := `SELECT provider,upstream_model,group_name,billing_type,source_currency,source_unit,
		COALESCE(input_price,0),COALESCE(output_price,0),COALESCE(cache_read_price,0),COALESCE(cache_write_price,0),COALESCE(unit_price,0),
		billing_multiplier,fx_rate,COALESCE(cny_input_per_m,0),COALESCE(cny_output_per_m,0),
		COALESCE(cny_cache_read_per_m,0),COALESCE(cny_cache_write_per_m,0),COALESCE(cny_unit_price,0),
		COALESCE(source_url,''),source_type,is_trusted,observed_at,is_stale
		FROM upstream_price_current WHERE provider=$1 AND upstream_model=$2`
	args := []interface{}{provider, upstream}
	if priceGroup != "" {
		query += ` AND group_name=$3`
		args = append(args, priceGroup)
	} else {
		query += ` AND source_type LIKE 'hub:%'`
	}
	query += ` ORDER BY observed_at DESC LIMIT 1`
	err := s.db.QueryRow(ctx, query, args...).Scan(&p.Provider, &p.UpstreamModel, &p.GroupName, &p.BillingType, &p.SourceCurrency, &p.SourceUnit,
		&p.InputPrice, &p.OutputPrice, &p.CacheReadPrice, &p.CacheWritePrice, &p.UnitPrice,
		&p.BillingMultiplier, &p.FXRate, &p.CNYInputPerM, &p.CNYOutputPerM,
		&p.CNYCacheReadPerM, &p.CNYCacheWritePerM, &p.CNYUnitPrice,
		&p.SourceURL, &p.SourceType, &p.IsTrusted, &p.ObservedAt, &p.IsStale)
	if err != nil {
		return nil, errors.New("没有可用的真实上游价格，请先同步")
	}
	rule := map[string]interface{}{
		"currency": "CREDITS", "billing_type": p.BillingType, "compute_per_cny": 1,
		"source_currency": "CNY", "price_unit_normalized": true,
	}
	if p.BillingType == "per_token" {
		rule["input_cost_per_m"] = p.CNYInputPerM
		rule["output_cost_per_m"] = p.CNYOutputPerM
		rule["cache_read_cost_per_m"] = p.CNYCacheReadPerM
		rule["cache_write_cost_per_m"] = p.CNYCacheWritePerM
		rule["source_input_cost_per_m"] = p.CNYInputPerM
		rule["source_output_cost_per_m"] = p.CNYOutputPerM
		rule["source_cache_read_cost_per_m"] = p.CNYCacheReadPerM
		rule["source_cache_write_cost_per_m"] = p.CNYCacheWritePerM
	} else {
		rule["unit_cost"] = p.CNYUnitPrice
		rule["source_unit_cost"] = p.CNYUnitPrice
	}
	raw, _ := json.Marshal(rule)
	if _, err := s.db.Exec(ctx, `UPDATE model_routes SET cost_rule=$2,updated_at=now() WHERE id=$1 AND model_id=$3`, routeID, raw, modelID); err != nil {
		return nil, err
	}
	return &p, nil
}

// ---------- mappings ----------

func (s *AdminService) ListUpstreamMappings(ctx context.Context, unmatched bool) ([]map[string]interface{}, error) {
	where := "1=1"
	if unmatched {
		where += " AND (mp.model_id IS NULL OR mp.is_confirmed=false)"
	}
	rows, err := s.db.Query(ctx, `SELECT c.provider,c.upstream_model,mp.model_id,COALESCE(m.code,''),COALESCE(m.display_name,''),
		COALESCE(mp.match_method,''),COALESCE(mp.confidence,0),COALESCE(mp.is_confirmed,false)
		FROM (SELECT DISTINCT provider,upstream_model FROM upstream_price_current) c
		LEFT JOIN upstream_model_mappings mp ON mp.provider=c.provider AND mp.upstream_model=c.upstream_model
		LEFT JOIN models m ON m.id=mp.model_id WHERE `+where+` ORDER BY c.provider,c.upstream_model`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var provider, upstream, code, name, method string
		var modelID *int64
		var confidence float64
		var confirmed bool
		if err := rows.Scan(&provider, &upstream, &modelID, &code, &name, &method, &confidence, &confirmed); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"provider": provider, "upstream_model": upstream, "model_id": modelID,
			"model_code": code, "model_name": name, "match_method": method,
			"confidence": confidence, "is_confirmed": confirmed,
		})
	}
	return out, rows.Err()
}

func (s *AdminService) UpdateUpstreamMapping(ctx context.Context, provider, upstreamModel string, modelID int64) error {
	if strings.TrimSpace(provider) == "" || strings.TrimSpace(upstreamModel) == "" || modelID <= 0 {
		return errors.New("provider、upstream_model、model_id 不能为空")
	}
	_, err := s.db.Exec(ctx, `INSERT INTO upstream_model_mappings(provider,upstream_model,model_id,match_method,confidence,is_confirmed)
		VALUES($1,$2,$3,'manual',1,true)
		ON CONFLICT (provider,upstream_model) DO UPDATE SET
			model_id=EXCLUDED.model_id,match_method='manual',confidence=1,is_confirmed=true,updated_at=now()`,
		provider, upstreamModel, modelID)
	return err
}

// ---------- analysis ----------

func (s *AdminService) refreshUsageDaily(ctx context.Context) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM model_usage_daily WHERE day >= current_date-30`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO model_usage_daily (model_id, day, input_tokens, output_tokens, request_count, provider_cost)
		SELECT model_id, date(created_at), COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0),
		       COUNT(*), COALESCE(SUM(provider_cost),0)
		FROM ai_call_logs
		WHERE model_id IS NOT NULL AND status='success' AND created_at >= current_date-30
		GROUP BY model_id, date(created_at)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO model_usage_daily (model_id, day, image_count, video_seconds, request_count, provider_cost)
		SELECT model_id, date(created_at),
		       COUNT(*) FILTER (WHERE type='image'),
		       COALESCE(SUM(CASE WHEN type='video' THEN COALESCE((input->>'target_duration_sec')::numeric, (input->>'duration')::numeric, 0) ELSE 0 END),0),
		       COUNT(*), COALESCE(SUM(provider_cost),0)
		FROM tasks
		WHERE model_id IS NOT NULL AND status='succeeded' AND created_at >= current_date-30
		GROUP BY model_id, date(created_at)
		ON CONFLICT (model_id, day) DO UPDATE SET
			image_count = EXCLUDED.image_count,
			video_seconds = EXCLUDED.video_seconds,
			request_count = model_usage_daily.request_count + EXCLUDED.request_count,
			provider_cost = model_usage_daily.provider_cost + EXCLUDED.provider_cost`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *AdminService) loadAnalysisRows(ctx context.Context) ([]routeRow, map[string][]UpstreamPrice, map[int64]ModelUsageSummary, error) {
	rows, err := s.db.Query(ctx, `SELECT r.id,r.model_id,m.code,COALESCE(m.display_name,''),r.route_name,r.provider,
		r.upstream_model,COALESCE(r.price_group,''),r.priority,r.health_status,r.cost_rule,COALESCE(r.user_price_rule,'{}'::jsonb)
		FROM model_routes r JOIN models m ON m.id=r.model_id
		WHERE m.is_enabled=true AND r.is_enabled=true
		ORDER BY r.model_id,r.priority,r.id`)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	routes := []routeRow{}
	for rows.Next() {
		var r routeRow
		var costRaw, userRaw []byte
		if err := rows.Scan(&r.ID, &r.ModelID, &r.Code, &r.DisplayName, &r.RouteName, &r.Provider,
			&r.UpstreamModel, &r.PriceGroup, &r.Priority, &r.HealthStatus, &costRaw, &userRaw); err != nil {
			return nil, nil, nil, err
		}
		_ = json.Unmarshal(costRaw, &r.CostRule)
		_ = json.Unmarshal(userRaw, &r.UserPrice)
		routes = append(routes, r)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}

	priceRows, err := s.db.Query(ctx, `SELECT c.provider,c.upstream_model,c.group_name,c.billing_type,c.source_currency,c.source_unit,
		COALESCE(c.input_price,0),COALESCE(c.output_price,0),COALESCE(c.cache_read_price,0),COALESCE(c.cache_write_price,0),COALESCE(c.unit_price,0),
		c.billing_multiplier,c.fx_rate,COALESCE(c.cny_input_per_m,0),COALESCE(c.cny_output_per_m,0),
		COALESCE(c.cny_cache_read_per_m,0),COALESCE(c.cny_cache_write_per_m,0),COALESCE(c.cny_unit_price,0),
		COALESCE(c.source_url,''),c.source_type,c.is_trusted,c.observed_at,c.is_stale,
		mp.model_id,COALESCE(m.code,''),COALESCE(m.display_name,'')
		FROM upstream_price_current c
		LEFT JOIN upstream_model_mappings mp ON mp.provider=c.provider AND mp.upstream_model=c.upstream_model
		LEFT JOIN models m ON m.id=mp.model_id`)
	if err != nil {
		return nil, nil, nil, err
	}
	defer priceRows.Close()
	prices := map[string][]UpstreamPrice{}
	for priceRows.Next() {
		var p UpstreamPrice
		if err := priceRows.Scan(&p.Provider, &p.UpstreamModel, &p.GroupName, &p.BillingType, &p.SourceCurrency, &p.SourceUnit,
			&p.InputPrice, &p.OutputPrice, &p.CacheReadPrice, &p.CacheWritePrice, &p.UnitPrice,
			&p.BillingMultiplier, &p.FXRate, &p.CNYInputPerM, &p.CNYOutputPerM, &p.CNYCacheReadPerM, &p.CNYCacheWritePerM, &p.CNYUnitPrice,
			&p.SourceURL, &p.SourceType, &p.IsTrusted, &p.ObservedAt, &p.IsStale, &p.ModelID, &p.ModelCode, &p.ModelName); err != nil {
			return nil, nil, nil, err
		}
		key := p.Provider + "\x00" + p.UpstreamModel
		prices[key] = append(prices[key], p)
	}
	if err := priceRows.Err(); err != nil {
		return nil, nil, nil, err
	}

	usageRows, err := s.db.Query(ctx, `SELECT model_id,COALESCE(SUM(input_tokens),0),COALESCE(SUM(output_tokens),0),
		COALESCE(SUM(cache_read_tokens),0),COALESCE(SUM(cache_write_tokens),0),COALESCE(SUM(image_count),0),
		COALESCE(SUM(video_seconds),0),COALESCE(SUM(request_count),0),COALESCE(SUM(provider_cost),0)
		FROM model_usage_daily WHERE day >= current_date-30 GROUP BY model_id`)
	if err != nil {
		return nil, nil, nil, err
	}
	defer usageRows.Close()
	usage := map[int64]ModelUsageSummary{}
	for usageRows.Next() {
		var id int64
		var u ModelUsageSummary
		if err := usageRows.Scan(&id, &u.InputTokens, &u.OutputTokens, &u.CacheReadTokens, &u.CacheWriteTokens,
			&u.ImageCount, &u.VideoSeconds, &u.RequestCount, &u.ProviderCost); err != nil {
			return nil, nil, nil, err
		}
		usage[id] = u
	}
	return routes, prices, usage, usageRows.Err()
}

func (s *AdminService) priceAnalysis(ctx context.Context) ([]PriceAnalysisRow, error) {
	if err := s.refreshUsageDaily(ctx); err != nil {
		// 用量刷新失败不阻断价格分析；页面仍可展示价格与线路。
		_ = err
	}
	routes, prices, usage, err := s.loadAnalysisRows(ctx)
	if err != nil {
		return nil, err
	}
	staleHours := s.configInt(ctx, "price_sync_stale_hours", 24)
	lossThreshold := s.configFloat(ctx, "price_loss_margin_threshold_percent", 5)
	trustedOnly := true

	byModel := map[int64][]routeRow{}
	order := []int64{}
	for _, r := range routes {
		if _, ok := byModel[r.ModelID]; !ok {
			order = append(order, r.ModelID)
		}
		byModel[r.ModelID] = append(byModel[r.ModelID], r)
	}
	out := []PriceAnalysisRow{}
	for _, modelID := range order {
		rs := byModel[modelID]
		briefs := make([]RoutePriceBrief, 0, len(rs))
		for _, r := range rs {
			briefs = append(briefs, s.routeBrief(r, prices, staleHours))
		}
		sort.SliceStable(briefs, func(i, j int) bool { return briefs[i].Priority < briefs[j].Priority })
		if len(briefs) == 0 {
			continue
		}
		// 用户售价是模型级配置，可能只挂在其中一条线路；主线路缺省时复用同模型任意线路的售价。
		var modelUser *PriceCost
		for i := range rs {
			if len(rs[i].UserPrice) > 0 {
				c := ruleCost(rs[i].UserPrice)
				modelUser = &c
				break
			}
		}
		if modelUser != nil {
			for i := range briefs {
				if briefs[i].UserCost == nil {
					briefs[i].UserCost = modelUser
				}
			}
		}
		primary := briefs[0]
		cheapest := primary
		u := usage[modelID]
		for _, b := range briefs[1:] {
			if !b.IsTrusted || costValue(b.Cost) <= 0 {
				continue
			}
			if comparable(primary.Cost.BillingType, b.Cost.BillingType) &&
				(costValue(cheapest.Cost) <= 0 || comparisonCost(b.Cost, u) < comparisonCost(cheapest.Cost, u)) {
				cheapest = b
			}
		}
		potential := s.findPotential(rs[0].ModelID, primary.Cost.BillingType, prices, trustedOnly, briefs)
		daily := estimateSaving(u, primary.Cost, cheapest.Cost)
		row := PriceAnalysisRow{
			ModelID: modelID, Code: rs[0].Code, DisplayName: rs[0].DisplayName,
			Primary: primary, Cheapest: cheapest, Potential: potential,
			DailySaving: daily, MonthlySaving: daily * 30, Usage: u, RouteCount: len(briefs),
			Backups: briefs[1:],
		}
		if primary.UserCost != nil {
			row.HasUserPrice = true
			row.MonthlyRevenue = usageCost(*primary.UserCost, u)
			row.MonthlyCost = usageCost(primary.Cost, u)
			row.MonthlyProfit = row.MonthlyRevenue - row.MonthlyCost
		}
		if cheapestVal := comparisonCost(cheapest.Cost, u); cheapestVal > 0 {
			row.GapPercent = (comparisonCost(primary.Cost, u) - cheapestVal) / cheapestVal * 100
		}
		row.Status = "ok"
		if costValue(primary.Cost) <= 0 {
			row.Status = "missing_price"
		} else if row.GapPercent > 0.5 {
			row.Status = "not_lowest"
		}
		if primary.IsStale {
			row.Status = "stale"
		}
		if primary.UserCost != nil {
			uv := usageCost(*primary.UserCost, u)
			if uv <= 0 {
				uv = costValue(*primary.UserCost)
			}
			if uv > 0 {
				cv := usageCost(primary.Cost, u)
				if cv <= 0 {
					cv = costValue(primary.Cost)
				}
				margin := (uv - cv) / uv * 100
				row.MarginPercent = &margin
				if margin < -0.0001 {
					row.Status = "loss"
				} else if margin < lossThreshold {
					row.Status = "low_margin"
				}
			}
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if math.Abs(out[i].MonthlySaving-out[j].MonthlySaving) > 0.0001 {
			return out[i].MonthlySaving > out[j].MonthlySaving
		}
		return out[i].GapPercent > out[j].GapPercent
	})
	return out, nil
}

func (s *AdminService) routeBrief(r routeRow, prices map[string][]UpstreamPrice, staleHours int) RoutePriceBrief {
	b := RoutePriceBrief{
		RouteID: r.ID, RouteName: r.RouteName, Provider: r.Provider, UpstreamModel: r.UpstreamModel,
		PriceGroup: r.PriceGroup, Priority: r.Priority, HealthStatus: r.HealthStatus,
		Cost: ruleCost(r.CostRule), CostSource: "config", IsTrusted: true,
	}
	if len(r.UserPrice) > 0 {
		uc := ruleCost(r.UserPrice)
		b.UserCost = &uc
	}
	key := r.Provider + "\x00" + r.UpstreamModel
	candidates := prices[key]
	var matched *UpstreamPrice
	// 网关真实成本优先于手工/配置成本。
	for i := range candidates {
		c := &candidates[i]
		if r.PriceGroup != "" && c.GroupName != r.PriceGroup {
			continue
		}
		if strings.HasPrefix(c.SourceType, "hub:") {
			matched = c
			break
		}
	}
	if matched == nil && r.Provider == "codexauv" && r.PriceGroup == "" {
		// CodexAuv 的分组由账号 Key 决定，未绑定分组时不能用广场里的最低分组冒充实际成本。
		for i := range candidates {
			c := &candidates[i]
			if c.GroupName == "" || c.SourceType == "config" {
				matched = c
				break
			}
		}
	}
	if matched == nil {
		for i := range candidates {
			c := &candidates[i]
			if r.PriceGroup == "" || c.GroupName == r.PriceGroup {
				if matched == nil || c.CNYInputPerM+c.CNYUnitPrice < matched.CNYInputPerM+matched.CNYUnitPrice {
					matched = c
				}
			}
		}
	}
	if matched != nil && (matched.CNYInputPerM > 0 || matched.CNYUnitPrice > 0) {
		b.Cost = priceCost(*matched)
		b.CostSource = matched.SourceType
		b.IsTrusted = matched.IsTrusted
		t := matched.ObservedAt
		b.ObservedAt = &t
		b.IsStale = matched.IsStale || time.Since(t) > time.Duration(staleHours)*time.Hour
	}
	return b
}

func (s *AdminService) findPotential(modelID int64, billingType string, prices map[string][]UpstreamPrice, trustedOnly bool, briefs []RoutePriceBrief) *PotentialPrice {
	bound := map[string]bool{}
	for _, b := range briefs {
		bound[b.Provider+"\x00"+b.UpstreamModel+"\x00"+b.PriceGroup] = true
	}
	var best *PotentialPrice
	for _, list := range prices {
		for _, p := range list {
			if p.ModelID == nil || *p.ModelID != modelID {
				continue
			}
			if trustedOnly && !p.IsTrusted {
				continue
			}
			if !comparable(billingType, p.BillingType) {
				continue
			}
			if bound[p.Provider+"\x00"+p.UpstreamModel+"\x00"+p.GroupName] {
				continue
			}
			if costValue(priceCost(p)) <= 0 {
				continue
			}
			if best == nil || costValue(priceCost(p)) < costValue(best.Cost) {
				best = &PotentialPrice{Provider: p.Provider, UpstreamModel: p.UpstreamModel, GroupName: p.GroupName, Cost: priceCost(p), ObservedAt: p.ObservedAt}
			}
		}
	}
	return best
}

func (s *AdminService) PriceAnalysis(ctx context.Context) ([]PriceAnalysisRow, error) {
	return s.priceAnalysis(ctx)
}

func (s *AdminService) PriceModelDetail(ctx context.Context, modelID int64) (*PriceAnalysisRow, error) {
	rows, err := s.priceAnalysis(ctx)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].ModelID == modelID {
			return &rows[i], nil
		}
	}
	return nil, errors.New("模型没有可用线路")
}

func (s *AdminService) ModelRouteOverview(ctx context.Context) ([]PriceAnalysisRow, error) {
	return s.priceAnalysis(ctx)
}

func (s *AdminService) PromotePreview(ctx context.Context, modelID, routeID int64) (*PriceAnalysisRow, *RoutePriceBrief, float64, float64, error) {
	row, err := s.PriceModelDetail(ctx, modelID)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	for i := range row.Backups {
		if row.Backups[i].RouteID == routeID {
			daily := estimateSaving(row.Usage, row.Primary.Cost, row.Backups[i].Cost)
			return row, &row.Backups[i], daily, daily * 30, nil
		}
	}
	if row.Primary.RouteID == routeID {
		return row, &row.Primary, 0, 0, nil
	}
	return nil, nil, 0, 0, errors.New("线路不属于该模型或未启用")
}

// ---------- helpers ----------

func (s *AdminService) configFloat(ctx context.Context, key string, def float64) float64 {
	var raw []byte
	if err := s.db.QueryRow(ctx, `SELECT value FROM system_configs WHERE key=$1`, key).Scan(&raw); err != nil {
		return def
	}
	var v float64
	if json.Unmarshal(raw, &v) == nil && v > 0 {
		return v
	}
	return def
}

func (s *AdminService) configInt(ctx context.Context, key string, def int) int {
	return int(s.configFloat(ctx, key, float64(def)))
}

func httpGetJSON(ctx context.Context, url string, timeout time.Duration, out interface{}) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "orbit-e-price-monitor/1.0")
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	dec := json.NewDecoder(resp.Body)
	return resp.StatusCode, dec.Decode(out)
}

func ruleCost(rule map[string]interface{}) PriceCost {
	bt := strAny(rule["billing_type"])
	if bt == "" {
		bt = "per_token"
	}
	return PriceCost{
		BillingType: bt,
		Input:       numAny(rule["input_cost_per_m"]),
		Output:      numAny(rule["output_cost_per_m"]),
		CacheRead:   numAny(rule["cache_read_cost_per_m"]),
		CacheWrite:  numAny(rule["cache_write_cost_per_m"]),
		Unit:        numAny(rule["unit_cost"]),
	}
}

func priceCost(p UpstreamPrice) PriceCost {
	return PriceCost{BillingType: p.BillingType, Input: p.CNYInputPerM, Output: p.CNYOutputPerM,
		CacheRead: p.CNYCacheReadPerM, CacheWrite: p.CNYCacheWritePerM, Unit: p.CNYUnitPrice}
}

func costValue(c PriceCost) float64 {
	if c.BillingType == "per_token" || c.BillingType == "" {
		return c.Input
	}
	return c.Unit
}

func comparable(a, b string) bool {
	if a == "" {
		a = "per_token"
	}
	if b == "" {
		b = "per_token"
	}
	return a == b
}

func usageCost(c PriceCost, u ModelUsageSummary) float64 {
	switch c.BillingType {
	case "per_token", "":
		return float64(u.InputTokens)/1000000*c.Input +
			float64(u.OutputTokens)/1000000*c.Output +
			float64(u.CacheReadTokens)/1000000*c.CacheRead +
			float64(u.CacheWriteTokens)/1000000*c.CacheWrite
	case "per_image":
		return float64(u.ImageCount) * c.Unit
	case "per_second":
		return u.VideoSeconds * c.Unit
	case "per_request":
		return float64(u.RequestCount) * c.Unit
	}
	return 0
}

func comparisonCost(c PriceCost, u ModelUsageSummary) float64 {
	if v := usageCost(c, u); v > 0 {
		return v
	}
	return costValue(c)
}

func estimateSaving(u ModelUsageSummary, from, to PriceCost) float64 {
	if !comparable(from.BillingType, to.BillingType) {
		return 0
	}
	switch from.BillingType {
	case "per_token", "":
		v := float64(u.InputTokens)/1000000*(from.Input-to.Input) +
			float64(u.OutputTokens)/1000000*(from.Output-to.Output) +
			float64(u.CacheReadTokens)/1000000*(from.CacheRead-to.CacheRead) +
			float64(u.CacheWriteTokens)/1000000*(from.CacheWrite-to.CacheWrite)
		if v < 0 {
			return 0
		}
		return v
	case "per_image":
		v := float64(u.ImageCount) * (from.Unit - to.Unit)
		if v < 0 {
			return 0
		}
		return v
	case "per_second":
		v := u.VideoSeconds * (from.Unit - to.Unit)
		if v < 0 {
			return 0
		}
		return v
	case "per_request":
		v := float64(u.RequestCount) * (from.Unit - to.Unit)
		if v < 0 {
			return 0
		}
		return v
	}
	return 0
}

func numAny(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	}
	return 0
}

func strAny(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func stringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		if xs, ok := v.([]string); ok {
			return xs
		}
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func stringInSlice(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Ensure unused imports are referenced when this file is compiled in isolation.
var _ = pgx.ErrNoRows
var _ *pgxpool.Pool
