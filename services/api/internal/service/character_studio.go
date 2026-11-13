package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/net/websocket"

	"github.com/starai/api/internal/billing"
	"github.com/starai/api/internal/util"
)

// 角色替换（实时换角）的接入层。
//
// 浏览器只拿到实时通道凭证与输出通道标识；上游密钥、控制连接与计费口径全部
// 留在服务端。所有返回给用户的文案都必须是中性术语，不出现上游品牌/产品名/
// 内部字段名。
const (
	characterStudioDefaultSalePrice      = 2.49
	characterStudioDefaultCreditPerSec   = 1.0
	characterStudioDefaultCreditPrice    = 0.03125
	characterStudioDefaultMaxMinutes     = 10
	characterStudioDefaultResolution     = "720p"
	characterStudioStartTimeout          = 90 * time.Second
	characterStudioHeartbeatTimeout      = 75 * time.Second
	characterStudioSwitchInterval        = 1200 * time.Millisecond
	characterStudioMaxCharacters         = 30
	characterStudioMaxTakes              = 12
	characterStudioRefType               = "character_studio"
	characterStudioEngineDefaultURL      = "https://g.alicdn.com/apsara-media-box/imp-web-rtc/7.1.9/aliyun-rtc-sdk.js"
	characterStudioEditingType           = "subject_replacement"
	characterStudioUpstreamCreatePath    = "/live/s_editing/realtime"
	characterStudioUpstreamUsagePath     = "/live/v1/lives/"
	characterStudioUpstreamWebSocketPath = "/live/ws/live/connect"
)

type characterStudioSettings struct {
	APIKey          string
	Host            string
	PublicBaseURL   string
	SalePerMinute   float64
	CreditPerSecond float64
	CreditPrice     float64
	MaxMinutes      int
	DefaultRes      string
	EngineURL          string
	SourceURL          string
	DefaultRoleBaseURL string
}

type CharacterStudioCharacter struct {
	PublicID  string `json:"public_id"`
	Name      string `json:"name"`
	ImageURL  string `json:"image_url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type CharacterStudioChannel struct {
	AppID         string `json:"app_id"`
	ChannelID     string `json:"channel_id"`
	UserID        string `json:"user_id"`
	Token         string `json:"token"`
	TokenExpireAt string `json:"token_expire_at"`
}

type CharacterStudioTake struct {
	PublicID      string `json:"public_id"`
	URL           string `json:"url"`
	AssetPublicID string `json:"asset_public_id,omitempty"`
	DurationMS    int    `json:"duration_ms"`
	SizeBytes     int64  `json:"size_bytes"`
	CreatedAt     string `json:"created_at"`
}

type CharacterStudioSessionView struct {
	PublicID           string                    `json:"public_id"`
	Status             string                    `json:"status"`
	Resolution         string                    `json:"resolution"`
	Character          *CharacterStudioCharacter `json:"character,omitempty"`
	Channel            *CharacterStudioChannel   `json:"channel,omitempty"`
	OutputChannel      string                    `json:"output_channel,omitempty"`
	StartedAt          string                    `json:"started_at,omitempty"`
	ElapsedSeconds     int                       `json:"elapsed_seconds"`
	MaxSeconds         int                       `json:"max_seconds"`
	SalePricePerMinute float64                   `json:"sale_price_per_minute"`
	SwitchCount        int                       `json:"switch_count"`
	BilledSeconds      int                       `json:"billed_seconds"`
	Cost               float64                   `json:"cost"`
	Error              string                    `json:"error,omitempty"`
	EndReason          string                    `json:"end_reason,omitempty"`
	CanResume          bool                      `json:"can_resume,omitempty"`
	Takes              []CharacterStudioTake     `json:"takes,omitempty"`
	CreatedAt          string                    `json:"created_at"`
}

type CharacterStudioConfigView struct {
	SalePricePerMinute float64  `json:"sale_price_per_minute"`
	MaxMinutes         int      `json:"max_minutes"`
	DefaultResolution  string   `json:"default_resolution"`
	Resolutions        []string `json:"resolutions"`
	MaxCharacters      int      `json:"max_characters"`
	SourceURL          string   `json:"source_url,omitempty"`
	DefaultRoleBaseURL string   `json:"default_role_base_url,omitempty"`
}

type CharacterStudioInput struct {
	Name            string `json:"name"`
	ImageURL        string `json:"image_url"`
	RightsConfirmed bool   `json:"rights_confirmed"`
}

type CharacterStudioTakeInput struct {
	URL           string `json:"url"`
	AssetPublicID string `json:"asset_public_id"`
	DurationMS    int    `json:"duration_ms"`
	SizeBytes     int64  `json:"size_bytes"`
}

type characterStudioSessionRow struct {
	DBID               int64
	PublicID           string
	UserID             int64
	CharacterID        *int64
	CurrentCharacterID *int64
	LiveID             string
	ConnID             string
	RenderUID          string
	RTCPayload         []byte
	Resolution         string
	Status             string
	SwitchCount        int
	ErrorCode          string
	ErrorMessage       string
	BilledSeconds      int
	Credits            float64
	ProviderCost       float64
	Cost               float64
	StartedAt          *time.Time
	EndedAt            *time.Time
	LastHeartbeatAt    *time.Time
	CreatedAt          time.Time
}

type characterStudioLink struct {
	mu         sync.Mutex
	conn       *websocket.Conn
	liveID     string
	connID     string
	seq        int
	closed     bool
	lastSwitch time.Time
}

type characterStudioRuntime struct {
	mu            sync.Mutex
	cancel        context.CancelFunc
	link          *characterStudioLink
	userID        int64
	maxSeconds    int
	startedAt     time.Time
	reconnecting  bool
	reaperStarted bool
}

func (r *characterStudioRuntime) currentLink() *characterStudioLink {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.link
}

func (r *characterStudioRuntime) setLink(link *characterStudioLink) {
	r.mu.Lock()
	r.link = link
	r.mu.Unlock()
}

func (r *characterStudioRuntime) setMaxSeconds(seconds int) {
	r.mu.Lock()
	r.maxSeconds = seconds
	r.mu.Unlock()
}

func (r *characterStudioRuntime) maxSecondsValue() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maxSeconds
}

func (r *characterStudioRuntime) beginReconnect() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reconnecting {
		return false
	}
	r.reconnecting = true
	return true
}

func (r *characterStudioRuntime) endReconnect() {
	r.mu.Lock()
	r.reconnecting = false
	r.mu.Unlock()
}

func (r *characterStudioRuntime) beginReaper() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reaperStarted {
		return false
	}
	r.reaperStarted = true
	return true
}

type characterStudioError struct{ msg string }

func (e *characterStudioError) Error() string { return e.msg }

func characterStudioFail(msg string) error { return &characterStudioError{msg: msg} }

func AsCharacterStudioError(err error) (string, bool) {
	var friendly *characterStudioError
	if errors.As(err, &friendly) {
		return friendly.msg, true
	}
	return "", false
}

type characterStudioHubState struct {
	mu    sync.Mutex
	items map[string]*characterStudioRuntime
	ends  map[string]*sync.Mutex
}

var characterStudioHub = &characterStudioHubState{items: map[string]*characterStudioRuntime{}, ends: map[string]*sync.Mutex{}}

func (h *characterStudioHubState) endLock(id string) func() {
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

func (h *characterStudioHubState) get(id string) *characterStudioRuntime {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.items[id]
}

func (h *characterStudioHubState) put(id string, runtime *characterStudioRuntime) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items[id] = runtime
}

func (h *characterStudioHubState) del(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.items, id)
}

// characterStudioSettings 读取角色替换的计费与运行时配置。
// 密钥/域名复用已有的上游系统配置，不在前台暴露。
func (s *ChatService) characterStudioSettings(ctx context.Context) characterStudioSettings {
	settings := characterStudioSettings{
		Host:            "api.vidu.cn",
		SalePerMinute:   characterStudioDefaultSalePrice,
		CreditPerSecond: characterStudioDefaultCreditPerSec,
		CreditPrice:     characterStudioDefaultCreditPrice,
		MaxMinutes:      characterStudioDefaultMaxMinutes,
		DefaultRes:      characterStudioDefaultResolution,
		EngineURL:       characterStudioEngineDefaultURL,
		SourceURL:       "https://github.com/starai-platform/starai",
	}
	rows, err := s.db.Query(ctx, `
		SELECT key, value #>> '{}' FROM system_configs
		WHERE key LIKE 'character_studio_%'
		   OR key IN ('vidu_api_key','vidu_api_host','vidu_public_base_url','vidu_text_credit_rate')`)
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
	if n := parseConfigFloat(values["vidu_text_credit_rate"], -1); n > 0 {
		settings.CreditPrice = n
	}
	if n := parseConfigFloat(values["character_studio_sale_price_per_minute"], -1); n > 0 {
		settings.SalePerMinute = n
	}
	if n := parseConfigFloat(values["character_studio_credit_per_second"], -1); n > 0 {
		settings.CreditPerSecond = n
	}
	if n := parseConfigFloat(values["character_studio_max_minutes"], -1); n >= 1 {
		settings.MaxMinutes = int(math.Round(n))
	}
	if settings.MaxMinutes < 1 {
		settings.MaxMinutes = 1
	}
	if settings.MaxMinutes > 60 {
		settings.MaxMinutes = 60
	}
	if raw := values["character_studio_default_resolution"]; raw != "" {
		if res := normalizeCharacterStudioResolution(raw, ""); res != "" {
			settings.DefaultRes = res
		}
	}
	if raw := values["character_studio_engine_url"]; raw != "" {
		settings.EngineURL = raw
	}
	if raw := values["character_studio_source_url"]; raw != "" {
		settings.SourceURL = raw
	}
	if raw := strings.TrimRight(strings.TrimSpace(values["character_studio_default_role_base_url"]), "/"); raw != "" {
		settings.DefaultRoleBaseURL = raw
	}
	settings.SalePerMinute = math.Round(settings.SalePerMinute*1e6) / 1e6
	return settings
}

func normalizeCharacterStudioResolution(raw, fallback string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "540p", "720p", "1080p":
		return value
	case "2k":
		return "2K"
	}
	if strings.TrimSpace(strings.ToLower(fallback)) == "2k" {
		return "2K"
	}
	switch strings.TrimSpace(strings.ToLower(fallback)) {
	case "540p", "720p", "1080p":
		return strings.TrimSpace(strings.ToLower(fallback))
	}
	return ""
}

func (s *ChatService) CharacterStudioConfig(ctx context.Context) CharacterStudioConfigView {
	settings := s.characterStudioSettings(ctx)
	return CharacterStudioConfigView{
		SalePricePerMinute: settings.SalePerMinute,
		MaxMinutes:         settings.MaxMinutes,
		DefaultResolution:  settings.DefaultRes,
		Resolutions:        []string{"540p", "720p", "1080p"},
		MaxCharacters:      characterStudioMaxCharacters,
		SourceURL:          settings.SourceURL,
		DefaultRoleBaseURL: settings.DefaultRoleBaseURL,
	}
}

func (s *ChatService) CharacterStudioEngineURL(ctx context.Context) string {
	return s.characterStudioSettings(ctx).EngineURL
}

func (s *ChatService) ListCharacterStudioCharacters(ctx context.Context, userID int64) ([]CharacterStudioCharacter, error) {
	rows, err := s.db.Query(ctx, `
		SELECT public_id, name, image_url, created_at, updated_at
		FROM character_studio_characters WHERE user_id=$1
		ORDER BY updated_at DESC LIMIT 60`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CharacterStudioCharacter{}
	for rows.Next() {
		var item CharacterStudioCharacter
		var created, updated time.Time
		if err := rows.Scan(&item.PublicID, &item.Name, &item.ImageURL, &created, &updated); err != nil {
			return nil, err
		}
		item.CreatedAt = created.Format(time.RFC3339)
		item.UpdatedAt = updated.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *ChatService) SaveCharacterStudioCharacter(ctx context.Context, userID int64, publicID string, input CharacterStudioInput) (*CharacterStudioCharacter, error) {
	name := strings.TrimSpace(input.Name)
	imageURL := strings.TrimSpace(input.ImageURL)
	if name == "" {
		return nil, characterStudioFail("请给这个角色起个名字")
	}
	if len([]rune(name)) > 24 {
		return nil, characterStudioFail("角色名字不能超过 24 个字")
	}
	if publicID == "" {
		if !input.RightsConfirmed {
			return nil, characterStudioFail("请先确认你拥有该形象的使用授权")
		}
		if imageURL == "" {
			return nil, characterStudioFail("请上传一张角色参考图")
		}
		if len(imageURL) > 2048 {
			return nil, characterStudioFail("参考图地址过长，请重新上传")
		}
		var count int
		_ = s.db.QueryRow(ctx, `SELECT count(*) FROM character_studio_characters WHERE user_id=$1`, userID).Scan(&count)
		if count >= characterStudioMaxCharacters {
			return nil, characterStudioFail("角色卡最多保存 30 个，请先删除一些")
		}
		var item CharacterStudioCharacter
		var created, updated time.Time
		err := s.db.QueryRow(ctx, `
			INSERT INTO character_studio_characters (public_id, user_id, name, image_url, rights_confirmed_at)
			VALUES ($1,$2,$3,$4,now())
			RETURNING public_id, name, image_url, created_at, updated_at`,
			util.NewPublicID("csc"), userID, name, imageURL).Scan(&item.PublicID, &item.Name, &item.ImageURL, &created, &updated)
		if err != nil {
			return nil, err
		}
		item.CreatedAt = created.Format(time.RFC3339)
		item.UpdatedAt = updated.Format(time.RFC3339)
		return &item, nil
	}
	var current CharacterStudioCharacter
	var created, updated time.Time
	err := s.db.QueryRow(ctx, `
		SELECT public_id, name, image_url, created_at, updated_at
		FROM character_studio_characters WHERE user_id=$1 AND public_id=$2`, userID, publicID).
		Scan(&current.PublicID, &current.Name, &current.ImageURL, &created, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, characterStudioFail("这个角色不存在或已删除")
	}
	if err != nil {
		return nil, err
	}
	if imageURL == "" {
		imageURL = current.ImageURL
	}
	if len(imageURL) > 2048 {
		return nil, characterStudioFail("参考图地址过长，请重新上传")
	}
	err = s.db.QueryRow(ctx, `
		UPDATE character_studio_characters
		SET name=$3, image_url=$4, updated_at=now()
		WHERE user_id=$1 AND public_id=$2
		RETURNING public_id, name, image_url, created_at, updated_at`, userID, publicID, name, imageURL).
		Scan(&current.PublicID, &current.Name, &current.ImageURL, &created, &updated)
	if err != nil {
		return nil, err
	}
	current.CreatedAt = created.Format(time.RFC3339)
	current.UpdatedAt = updated.Format(time.RFC3339)
	return &current, nil
}

func (s *ChatService) DeleteCharacterStudioCharacter(ctx context.Context, userID int64, publicID string) error {
	var active int
	if err := s.db.QueryRow(ctx, `
		SELECT count(*) FROM character_studio_sessions
		WHERE user_id=$1 AND status IN ('starting','live','reconnecting','wrapping')
		  AND (character_id=(SELECT id FROM character_studio_characters WHERE user_id=$1 AND public_id=$2)
		       OR current_character_id=(SELECT id FROM character_studio_characters WHERE user_id=$1 AND public_id=$2))`,
		userID, publicID).Scan(&active); err != nil {
		return err
	}
	if active > 0 {
		return characterStudioFail("这个角色正在使用中，先结束会话再删除")
	}
	tag, err := s.db.Exec(ctx, `DELETE FROM character_studio_characters WHERE user_id=$1 AND public_id=$2`, userID, publicID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return characterStudioFail("这个角色不存在或已删除")
	}
	return nil
}

func (s *ChatService) StartCharacterStudioSession(ctx context.Context, userID int64, characterPublicID, resolution string) (*CharacterStudioSessionView, error) {
	settings := s.characterStudioSettings(ctx)
	if settings.APIKey == "" {
		return nil, characterStudioFail("实时渲染服务还没有配置密钥，请联系客服")
	}
	character, err := s.characterStudioCharacter(ctx, userID, characterPublicID)
	if err != nil {
		return nil, err
	}
	imageURL := characterStudioImageURL(character.ImageURL, settings.PublicBaseURL)
	if imageURL == "" {
		return nil, characterStudioFail("角色参考图需要公网可访问的绝对地址，请在角色卡里改用「公网链接」或换一张图")
	}
	if active := s.activeCharacterStudioSession(ctx, userID); active != nil {
		if active.Status == "starting" && time.Since(active.CreatedAt) > 2*time.Minute {
			// 上一次启动流程没有收尾（例如服务重启），先释放它，避免永久占位。
			_, _ = s.db.Exec(ctx, `UPDATE character_studio_sessions SET status='failed', ended_at=now(), error_code='start_timeout', error_message='启动超时' WHERE id=$1 AND status='starting'`, active.DBID)
			_ = s.billing.Unfreeze(ctx, userID, settings.SalePerMinute, characterStudioRefType, active.PublicID)
		} else {
			return nil, characterStudioFail("请先结束当前会话")
		}
	}
	res := normalizeCharacterStudioResolution(resolution, settings.DefaultRes)
	if res == "" {
		return nil, characterStudioFail("清晰度参数不正确")
	}
	publicID := util.NewPublicID("cs")
	connID := util.NewPublicID("conn")
	var dbID int64
	err = s.db.QueryRow(ctx, `
		INSERT INTO character_studio_sessions
			(public_id, user_id, character_id, current_character_id, conn_id, resolution, status)
		VALUES ($1,$2,(SELECT id FROM character_studio_characters WHERE user_id=$2 AND public_id=$3),
			(SELECT id FROM character_studio_characters WHERE user_id=$2 AND public_id=$3),$4,$5,'starting')
		RETURNING id`, publicID, userID, characterPublicID, connID, res).Scan(&dbID)
	if err != nil {
		return nil, err
	}
	if err = s.billing.Freeze(ctx, userID, settings.SalePerMinute, characterStudioRefType, publicID); err != nil {
		_, _ = s.db.Exec(ctx, `UPDATE character_studio_sessions SET status='failed', ended_at=now(), error_code='start_failed' WHERE id=$1`, dbID)
		return nil, err
	}
	runtimeCtx, cancel := context.WithCancel(context.Background())
	characterStudioHub.put(publicID, &characterStudioRuntime{
		cancel:     cancel,
		userID:     userID,
		maxSeconds: settings.MaxMinutes * 60,
	})
	go s.runCharacterStudioStart(runtimeCtx, dbID, publicID, userID, character, imageURL, res, settings)
	return &CharacterStudioSessionView{
		PublicID:           publicID,
		Status:             "starting",
		Resolution:         res,
		Character:          &character,
		MaxSeconds:         settings.MaxMinutes * 60,
		SalePricePerMinute: settings.SalePerMinute,
		CreatedAt:          time.Now().Format(time.RFC3339),
	}, nil
}

func (s *ChatService) runCharacterStudioStart(ctx context.Context, dbID int64, publicID string, userID int64, character CharacterStudioCharacter, imageURL, resolution string, settings characterStudioSettings) {
	fail := func(code, message string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		tag, err := s.db.Exec(ctx, `
			UPDATE character_studio_sessions
			SET status='failed', ended_at=now(), error_code=$2, error_message=$3
			WHERE id=$1 AND status='starting'`, dbID, code, message)
		if err == nil && tag.RowsAffected() > 0 {
			_ = s.billing.Unfreeze(ctx, userID, settings.SalePerMinute, characterStudioRefType, publicID)
		}
		characterStudioHub.del(publicID)
	}
	createCtx, cancel := context.WithTimeout(ctx, 70*time.Second)
	defer cancel()
	body := map[string]any{
		"image_url":    imageURL,
		"editing_type": characterStudioEditingType,
		"resolution":   resolution,
	}
	payload, status, err := s.characterStudioJSON(createCtx, settings, http.MethodPost, characterStudioUpstreamCreatePath, body, 65*time.Second)
	if err != nil {
		log.Printf("character studio create: %v", err)
		fail("create_failed", "实时渲染服务连接超时，请稍后再试")
		return
	}
	if status >= 300 {
		log.Printf("character studio create rejected: status=%d body=%s", status, characterStudioLogBody(payload))
		fail("create_rejected", characterStudioUpstreamError(status, payload))
		return
	}
	var created struct {
		Live struct {
			ID           string `json:"id"`
			LiveDuration int    `json:"live_duration"`
		} `json:"live"`
		RTC       map[string]any `json:"rtc"`
		RenderUID string         `json:"render_uid"`
	}
	if json.Unmarshal(payload, &created) != nil || created.Live.ID == "" {
		fail("create_failed", "实时渲染服务没有返回可用会话")
		return
	}
	rtcRaw, _ := json.Marshal(created.RTC)
	tag, err := s.db.Exec(ctx, `
		UPDATE character_studio_sessions
		SET live_id=$2, rtc_payload=$3, render_uid=$4
		WHERE id=$1 AND status='starting'`, dbID, created.Live.ID, rtcRaw, created.RenderUID)
	if err == nil && tag.RowsAffected() == 0 {
		// 启动过程中用户已取消/结束，不再继续建链。
		characterStudioHub.del(publicID)
		return
	}
	if err != nil {
		fail("create_failed", "会话保存失败，请稍后再试")
		return
	}
	connID := s.characterStudioConnID(ctx, dbID)
	if connID == "" {
		fail("create_failed", "会话保存失败，请稍后再试")
		return
	}
	link, _, err := s.openCharacterStudioLink(ctx, settings, created.Live.ID, connID, 0)
	if err != nil {
		if ctx.Err() != nil {
			characterStudioHub.del(publicID)
			return
		}
		fail("handshake_failed", err.Error())
		return
	}
	maxSeconds := settings.MaxMinutes * 60
	if created.Live.LiveDuration > 0 && created.Live.LiveDuration < maxSeconds {
		maxSeconds = created.Live.LiveDuration
	}
	tag, err = s.db.Exec(ctx, `
		UPDATE character_studio_sessions SET status='live', started_at=now(), last_heartbeat_at=now()
		WHERE id=$1 AND status='starting'`, dbID)
	if err != nil || tag.RowsAffected() == 0 {
		link.close(true)
		characterStudioHub.del(publicID)
		return
	}
	runtime := characterStudioHub.get(publicID)
	if runtime == nil {
		runtime = &characterStudioRuntime{userID: userID, maxSeconds: maxSeconds}
	}
	runtime.setLink(link)
	runtime.setMaxSeconds(maxSeconds)
	runtime.mu.Lock()
	runtime.startedAt = time.Now()
	runtime.userID = userID
	runtime.mu.Unlock()
	characterStudioHub.put(publicID, runtime)
	go s.readCharacterStudioLink(publicID, userID, link, connID)
	go s.watchCharacterStudioSession(publicID, userID, maxSeconds)
}

func (s *ChatService) characterStudioConnID(ctx context.Context, dbID int64) string {
	var connID string
	_ = s.db.QueryRow(ctx, `SELECT conn_id FROM character_studio_sessions WHERE id=$1`, dbID).Scan(&connID)
	return connID
}

func (s *ChatService) activeCharacterStudioSession(ctx context.Context, userID int64) *characterStudioSessionRow {
	row, err := s.scanCharacterStudioSession(ctx, `
		SELECT s.id,s.public_id,s.user_id,s.character_id,s.current_character_id,s.live_id,s.conn_id,s.render_uid,
		       s.rtc_payload,s.resolution,s.status,s.switch_count,s.error_code,s.error_message,s.billed_seconds,
		       s.credits,s.provider_cost,s.cost,s.started_at,s.ended_at,s.last_heartbeat_at,s.created_at
		FROM character_studio_sessions s
		WHERE s.user_id=$1 AND s.status IN ('starting','live','reconnecting','wrapping')
		ORDER BY s.created_at DESC LIMIT 1`, userID)
	if err != nil || row == nil {
		return nil
	}
	return row
}

func (s *ChatService) GetCharacterStudioSession(ctx context.Context, userID int64, publicID string) (*CharacterStudioSessionView, error) {
	row, err := s.ownedCharacterStudioSession(ctx, userID, publicID)
	if err != nil {
		return nil, err
	}
	settings := s.characterStudioSettings(ctx)
	character, err := s.currentCharacterStudioCharacter(ctx, userID, row.CurrentCharacterID)
	if err != nil {
		return nil, err
	}
	takes, err := s.listCharacterStudioTakes(ctx, row.DBID)
	if err != nil {
		return nil, err
	}
	return s.characterStudioSessionView(ctx, row, character, takes, settings), nil
}

func (s *ChatService) ActiveCharacterStudioSession(ctx context.Context, userID int64) (*CharacterStudioSessionView, error) {
	row := s.activeCharacterStudioSession(ctx, userID)
	if row == nil {
		return nil, nil
	}
	settings := s.characterStudioSettings(ctx)
	character, err := s.currentCharacterStudioCharacter(ctx, userID, row.CurrentCharacterID)
	if err != nil {
		return nil, err
	}
	takes, err := s.listCharacterStudioTakes(ctx, row.DBID)
	if err != nil {
		return nil, err
	}
	return s.characterStudioSessionView(ctx, row, character, takes, settings), nil
}

func (s *ChatService) SwitchCharacterStudioCharacter(ctx context.Context, userID int64, sessionPublicID, characterPublicID string) (*CharacterStudioCharacter, error) {
	row, err := s.ownedCharacterStudioSession(ctx, userID, sessionPublicID)
	if err != nil {
		return nil, err
	}
	if row.Status != "live" && row.Status != "reconnecting" {
		return nil, characterStudioFail("会话还没有开播，请稍后再试")
	}
	settings := s.characterStudioSettings(ctx)
	character, err := s.characterStudioCharacter(ctx, userID, characterPublicID)
	if err != nil {
		return nil, err
	}
	if row.CurrentCharacterID != nil && *row.CurrentCharacterID != 0 {
		var currentPublicID string
		_ = s.db.QueryRow(ctx, `SELECT public_id FROM character_studio_characters WHERE id=$1`, *row.CurrentCharacterID).Scan(&currentPublicID)
		if currentPublicID == character.PublicID {
			return &character, nil
		}
	}
	imageURL := characterStudioImageURL(character.ImageURL, settings.PublicBaseURL)
	if imageURL == "" {
		return nil, characterStudioFail("角色参考图需要公网可访问的绝对地址，请在角色卡里改用「公网链接」或换一张图")
	}
	runtime := characterStudioHub.get(sessionPublicID)
	if runtime == nil || runtime.currentLink() == nil {
		return nil, characterStudioFail("实时通道还没有就绪，请稍后再试")
	}
	if err := runtime.currentLink().switchPrompt(imageURL); err != nil {
		if msg, ok := AsCharacterStudioError(err); ok {
			return nil, characterStudioFail(msg)
		}
		return nil, characterStudioFail("实时通道已断开，无法切换角色")
	}
	_, err = s.db.Exec(ctx, `
		UPDATE character_studio_sessions
		SET current_character_id=(SELECT id FROM character_studio_characters WHERE user_id=$2 AND public_id=$3),
		    switch_count=switch_count+1, error_code='', error_message=''
		WHERE id=$1`, row.DBID, userID, character.PublicID)
	if err != nil {
		return nil, err
	}
	return &character, nil
}

func (s *ChatService) HeartbeatCharacterStudioSession(ctx context.Context, userID int64, publicID string) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE character_studio_sessions SET last_heartbeat_at=now()
		WHERE user_id=$1 AND public_id=$2 AND status IN ('live','reconnecting')`, userID, publicID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return characterStudioFail("会话已经结束")
	}
	return nil
}

func (s *ChatService) EndCharacterStudioSession(ctx context.Context, userID int64, publicID, code, message string) (*CharacterStudioSessionView, error) {
	unlock := characterStudioHub.endLock(publicID)
	defer unlock()
	row, err := s.ownedCharacterStudioSession(ctx, userID, publicID)
	if err != nil {
		return nil, err
	}
	settings := s.characterStudioSettings(ctx)
	if row.Status == "ended" || row.Status == "failed" {
		character, _ := s.currentCharacterStudioCharacter(ctx, userID, row.CurrentCharacterID)
		takes, _ := s.listCharacterStudioTakes(ctx, row.DBID)
		return s.characterStudioSessionView(ctx, row, character, takes, settings), nil
	}
	if code == "" {
		code = "user_end"
	}
	if row.Status == "starting" {
		if runtime := characterStudioHub.get(publicID); runtime != nil {
			if runtime.cancel != nil {
				runtime.cancel()
			}
			characterStudioHub.del(publicID)
		}
		_, _ = s.db.Exec(ctx, `UPDATE character_studio_sessions SET status='wrapping' WHERE id=$1 AND status='starting'`, row.DBID)
		if err := s.billing.Unfreeze(ctx, userID, settings.SalePerMinute, characterStudioRefType, publicID); err != nil && !errors.Is(err, billing.ErrFreezeNotFound) {
			log.Printf("character studio unfreeze %s: %v", publicID, err)
		}
		_, _ = s.db.Exec(ctx, `
			UPDATE character_studio_sessions
			SET status='ended', ended_at=now(), billed_seconds=0, credits=0, provider_cost=0, cost=0,
			    error_code=$2, error_message=$3
			WHERE id=$1`, row.DBID, code, message)
		row.Status = "ended"
		row.ErrorCode = code
		row.ErrorMessage = message
		updated, loadErr := s.ownedCharacterStudioSession(ctx, userID, publicID)
		if loadErr != nil || updated == nil {
			updated = row
		}
		character, _ := s.currentCharacterStudioCharacter(ctx, userID, updated.CurrentCharacterID)
		return s.characterStudioSessionView(ctx, updated, character, nil, settings), nil
	}
	_, _ = s.db.Exec(ctx, `UPDATE character_studio_sessions SET status='wrapping' WHERE id=$1 AND status IN ('live','reconnecting')`, row.DBID)
	if runtime := characterStudioHub.get(publicID); runtime != nil {
		if link := runtime.currentLink(); link != nil {
			link.close(true)
		}
		characterStudioHub.del(publicID)
	}
	billed, credits := s.pollCharacterStudioUsage(settings, row.LiveID, 5)
	elapsed := 0
	if row.StartedAt != nil {
		elapsed = int(time.Since(*row.StartedAt).Seconds())
	}
	if billed <= 0 {
		billed = elapsed
	}
	if billed < 0 {
		billed = 0
	}
	if row.StartedAt != nil && time.Since(*row.StartedAt) > time.Duration(settings.MaxMinutes)*time.Minute {
		if max := settings.MaxMinutes * 60; billed > max {
			billed = max
		}
	}
	cost := characterStudioCost(settings.SalePerMinute, billed)
	providerCost := characterStudioProviderCost(billed, credits, settings)
	if cost <= 0 {
		if err := s.billing.Unfreeze(ctx, userID, settings.SalePerMinute, characterStudioRefType, publicID); err != nil && !errors.Is(err, billing.ErrFreezeNotFound) {
			log.Printf("character studio unfreeze %s: %v", publicID, err)
		}
	} else if err := s.billing.Charge(ctx, userID, settings.SalePerMinute, cost, characterStudioRefType, publicID, "video_usage", "角色替换"); err != nil && !errors.Is(err, billing.ErrFreezeNotFound) {
		log.Printf("character studio charge %s: %v", publicID, err)
	}
	_, _ = s.db.Exec(ctx, `
		UPDATE character_studio_sessions
		SET status='ended', ended_at=now(), billed_seconds=$2, credits=$3, provider_cost=$4, cost=$5,
		    error_code=$6, error_message=$7
		WHERE id=$1`, row.DBID, billed, credits, providerCost, cost, code, message)
	row.Status = "ended"
	row.BilledSeconds = billed
	row.Credits = credits
	row.ProviderCost = providerCost
	row.Cost = cost
	row.ErrorCode = code
	row.ErrorMessage = message
	updated, loadErr := s.ownedCharacterStudioSession(ctx, userID, publicID)
	if loadErr != nil || updated == nil {
		updated = row
	}
	character, _ := s.currentCharacterStudioCharacter(ctx, userID, updated.CurrentCharacterID)
	takes, _ := s.listCharacterStudioTakes(ctx, updated.DBID)
	return s.characterStudioSessionView(ctx, updated, character, takes, settings), nil
}

func (s *ChatService) AddCharacterStudioTake(ctx context.Context, userID int64, sessionPublicID string, input CharacterStudioTakeInput) (*CharacterStudioTake, error) {
	row, err := s.ownedCharacterStudioSession(ctx, userID, sessionPublicID)
	if err != nil {
		return nil, err
	}
	if row.Status == "failed" {
		return nil, characterStudioFail("这次会话没有成功开播")
	}
	rawURL := strings.TrimSpace(input.URL)
	if rawURL == "" || len(rawURL) > 2048 {
		return nil, characterStudioFail("片段地址不可用")
	}
	if input.DurationMS < 0 {
		input.DurationMS = 0
	}
	if input.DurationMS > 120000 {
		input.DurationMS = 120000
	}
	if input.SizeBytes < 0 {
		input.SizeBytes = 0
	}
	var count int
	_ = s.db.QueryRow(ctx, `SELECT count(*) FROM character_studio_takes WHERE session_id=$1`, row.DBID).Scan(&count)
	if count >= characterStudioMaxTakes {
		return nil, characterStudioFail("这段会话保存的片段已达上限")
	}
	var item CharacterStudioTake
	var created time.Time
	err = s.db.QueryRow(ctx, `
		INSERT INTO character_studio_takes
			(public_id, session_id, user_id, character_id, asset_public_id, url, duration_ms, size_bytes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING public_id, url, asset_public_id, duration_ms, size_bytes, created_at`,
		util.NewPublicID("cst"), row.DBID, userID, row.CurrentCharacterID, strings.TrimSpace(input.AssetPublicID), rawURL, input.DurationMS, input.SizeBytes).
		Scan(&item.PublicID, &item.URL, &item.AssetPublicID, &item.DurationMS, &item.SizeBytes, &created)
	if err != nil {
		return nil, err
	}
	item.CreatedAt = created.Format(time.RFC3339)
	return &item, nil
}

func (s *ChatService) characterStudioSessionView(ctx context.Context, row *characterStudioSessionRow, character *CharacterStudioCharacter, takes []CharacterStudioTake, settings characterStudioSettings) *CharacterStudioSessionView {
	view := &CharacterStudioSessionView{
		PublicID:           row.PublicID,
		Status:             row.Status,
		Resolution:         row.Resolution,
		Character:          character,
		ElapsedSeconds:     0,
		MaxSeconds:         settings.MaxMinutes * 60,
		SalePricePerMinute: settings.SalePerMinute,
		SwitchCount:        row.SwitchCount,
		BilledSeconds:      row.BilledSeconds,
		Cost:               row.Cost,
		Error:              row.ErrorMessage,
		CreatedAt:          row.CreatedAt.Format(time.RFC3339),
		Takes:              takes,
	}
	if runtime := characterStudioHub.get(row.PublicID); runtime != nil {
		if max := runtime.maxSecondsValue(); max > 0 {
			view.MaxSeconds = max
		}
	}
	if row.StartedAt != nil {
		view.StartedAt = row.StartedAt.Format(time.RFC3339)
		elapsed := int(time.Since(*row.StartedAt).Seconds())
		if elapsed < 0 {
			elapsed = 0
		}
		if view.MaxSeconds > 0 && elapsed > view.MaxSeconds {
			elapsed = view.MaxSeconds
		}
		view.ElapsedSeconds = elapsed
	}
	if row.Status == "ended" || row.Status == "failed" {
		view.EndReason = characterStudioEndReason(row.ErrorCode)
		if row.BilledSeconds > 0 {
			view.ElapsedSeconds = row.BilledSeconds
		}
	}
	if row.Status == "live" || row.Status == "reconnecting" {
		if channel := characterStudioChannel(row.RTCPayload); channel != nil {
			view.Channel = channel
		}
		view.OutputChannel = row.RenderUID
		view.CanResume = view.Channel != nil && view.OutputChannel != ""
	}
	if row.Status == "starting" {
		view.Cost = 0
	}
	return view
}

func (s *ChatService) characterStudioCharacter(ctx context.Context, userID int64, publicID string) (CharacterStudioCharacter, error) {
	var item CharacterStudioCharacter
	var created, updated time.Time
	err := s.db.QueryRow(ctx, `
		SELECT public_id, name, image_url, created_at, updated_at
		FROM character_studio_characters WHERE user_id=$1 AND public_id=$2`, userID, publicID).
		Scan(&item.PublicID, &item.Name, &item.ImageURL, &created, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, characterStudioFail("这个角色不存在或已删除")
	}
	if err != nil {
		return item, err
	}
	item.CreatedAt = created.Format(time.RFC3339)
	item.UpdatedAt = updated.Format(time.RFC3339)
	return item, nil
}

func (s *ChatService) currentCharacterStudioCharacter(ctx context.Context, userID int64, id *int64) (*CharacterStudioCharacter, error) {
	if id == nil {
		return nil, nil
	}
	var item CharacterStudioCharacter
	var created, updated time.Time
	err := s.db.QueryRow(ctx, `
		SELECT public_id, name, image_url, created_at, updated_at
		FROM character_studio_characters WHERE id=$1 AND user_id=$2`, *id, userID).
		Scan(&item.PublicID, &item.Name, &item.ImageURL, &created, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.CreatedAt = created.Format(time.RFC3339)
	item.UpdatedAt = updated.Format(time.RFC3339)
	return &item, nil
}

func (s *ChatService) ownedCharacterStudioSession(ctx context.Context, userID int64, publicID string) (*characterStudioSessionRow, error) {
	row, err := s.scanCharacterStudioSession(ctx, `
		SELECT s.id,s.public_id,s.user_id,s.character_id,s.current_character_id,s.live_id,s.conn_id,s.render_uid,
		       s.rtc_payload,s.resolution,s.status,s.switch_count,s.error_code,s.error_message,s.billed_seconds,
		       s.credits,s.provider_cost,s.cost,s.started_at,s.ended_at,s.last_heartbeat_at,s.created_at
		FROM character_studio_sessions s
		WHERE s.user_id=$1 AND s.public_id=$2`, userID, publicID)
	if errors.Is(err, pgx.ErrNoRows) || row == nil {
		return nil, characterStudioFail("这个会话不存在或已结束")
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *ChatService) scanCharacterStudioSession(ctx context.Context, query string, args ...any) (*characterStudioSessionRow, error) {
	var row characterStudioSessionRow
	err := s.db.QueryRow(ctx, query, args...).Scan(
		&row.DBID, &row.PublicID, &row.UserID, &row.CharacterID, &row.CurrentCharacterID,
		&row.LiveID, &row.ConnID, &row.RenderUID, &row.RTCPayload, &row.Resolution, &row.Status,
		&row.SwitchCount, &row.ErrorCode, &row.ErrorMessage, &row.BilledSeconds,
		&row.Credits, &row.ProviderCost, &row.Cost, &row.StartedAt, &row.EndedAt, &row.LastHeartbeatAt, &row.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *ChatService) listCharacterStudioTakes(ctx context.Context, sessionDBID int64) ([]CharacterStudioTake, error) {
	rows, err := s.db.Query(ctx, `
		SELECT public_id, url, asset_public_id, duration_ms, size_bytes, created_at
		FROM character_studio_takes WHERE session_id=$1 ORDER BY created_at ASC`, sessionDBID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CharacterStudioTake{}
	for rows.Next() {
		var item CharacterStudioTake
		var created time.Time
		if err := rows.Scan(&item.PublicID, &item.URL, &item.AssetPublicID, &item.DurationMS, &item.SizeBytes, &created); err != nil {
			return nil, err
		}
		item.CreatedAt = created.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

// ---------- 上游调用与控制连接 ----------

func (s *ChatService) characterStudioJSON(ctx context.Context, settings characterStudioSettings, method, path string, body any, timeout time.Duration) ([]byte, int, error) {
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

func (s *ChatService) pollCharacterStudioUsage(settings characterStudioSettings, liveID string, attempts int) (int, float64) {
	if liveID == "" || settings.APIKey == "" {
		return 0, 0
	}
	if attempts < 1 {
		attempts = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	billed, credits := s.characterStudioUsage(ctx, settings, liveID)
	for i := 1; i < attempts && billed <= 0; i++ {
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return billed, credits
		}
		billed, credits = s.characterStudioUsage(ctx, settings, liveID)
	}
	return billed, credits
}

func (s *ChatService) characterStudioUsage(ctx context.Context, settings characterStudioSettings, liveID string) (int, float64) {
	payload, status, err := s.characterStudioJSON(ctx, settings, http.MethodGet, characterStudioUpstreamUsagePath+url.PathEscape(liveID), nil, 15*time.Second)
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

func (s *ChatService) openCharacterStudioLink(ctx context.Context, settings characterStudioSettings, liveID, connID string, initialSeq int) (*characterStudioLink, int, error) {
	wsURL := fmt.Sprintf("wss://%s%s?live_id=%s&conn_id=%s",
		settings.Host, characterStudioUpstreamWebSocketPath, url.QueryEscape(liveID), url.QueryEscape(connID))
	cfg, err := websocket.NewConfig(wsURL, "https://"+settings.Host)
	if err != nil {
		return nil, initialSeq, characterStudioFail("实时通道配置失败")
	}
	cfg.Header.Set("Authorization", "Token "+settings.APIKey)
	conn, err := websocket.DialConfig(cfg)
	if err != nil {
		return nil, initialSeq, characterStudioFail("实时通道没有建立，请稍后再试")
	}
	link := &characterStudioLink{conn: conn, liveID: liveID, connID: connID, seq: initialSeq}
	deadline := time.Now().Add(75 * time.Second)
	for {
		if err := ctx.Err(); err != nil {
			conn.Close()
			return nil, link.seq, err
		}
		if err := link.write(1, characterStudioConnInitPayload()); err != nil {
			conn.Close()
			return nil, link.seq, characterStudioFail("实时通道没有就绪，请稍后再试")
		}
		readDeadline := time.Now().Add(5 * time.Second)
		if readDeadline.After(deadline) {
			readDeadline = deadline
		}
		_ = conn.SetReadDeadline(readDeadline)
		var frame map[string]any
		if err := websocket.JSON.Receive(conn, &frame); err != nil {
			if time.Now().After(deadline) {
				conn.Close()
				return nil, link.seq, characterStudioFail("实时通道准备超时，请稍后再试")
			}
			// 读超时后短暂等待再重发初始化信令；真正的 NOT_READY 也会走这里。
			select {
			case <-time.After(1500 * time.Millisecond):
			case <-ctx.Done():
				conn.Close()
				return nil, link.seq, ctx.Err()
			}
			continue
		}
		if characterStudioFrameType(frame) != 2 {
			continue
		}
		ack := characterStudioMap(frame["payload"], "conn_init_ack")
		if ack == nil {
			continue
		}
		if success, _ := ack["success"].(bool); success {
			_ = conn.SetReadDeadline(time.Time{})
			return link, link.seq, nil
		}
		code := characterStudioString(ack["error_code"])
		if code == "NOT_READY" {
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				conn.Close()
				return nil, link.seq, ctx.Err()
			}
			continue
		}
		conn.Close()
		return nil, link.seq, characterStudioFail(characterStudioHandshakeError(code))
	}
}

func (s *ChatService) readCharacterStudioLink(publicID string, userID int64, link *characterStudioLink, connID string) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("character studio read panic: %v", rec)
		}
	}()
	for {
		link.mu.Lock()
		conn := link.conn
		closed := link.closed
		link.mu.Unlock()
		if closed || conn == nil {
			return
		}
		var frame map[string]any
		if err := websocket.JSON.Receive(conn, &frame); err != nil {
			if link.isClosed() {
				return
			}
			if s.characterStudioCanReconnect(context.Background(), publicID) {
				if s.reconnectCharacterStudioLink(publicID, userID, connID) {
					return
				}
			}
			s.endCharacterStudioAsync(userID, publicID, "connection_lost", "实时通道已断开")
			return
		}
		switch characterStudioFrameType(frame) {
		case 14:
			ack := characterStudioMap(frame["payload"], "switch_prompt_ack")
			if ack == nil {
				continue
			}
			if success, _ := ack["success"].(bool); !success {
				message := characterStudioSwitchError(characterStudioString(ack["error_code"]))
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, _ = s.db.Exec(ctx, `UPDATE character_studio_sessions SET error_code='switch_failed', error_message=$2 WHERE public_id=$1 AND status IN ('live','reconnecting')`, publicID, message)
				cancel()
			}
		case 6:
			s.endCharacterStudioAsync(userID, publicID, "render_ended", "渲染节点已结束这次会话")
			return
		}
	}
}

func (s *ChatService) reconnectCharacterStudioLink(publicID string, userID int64, connID string) bool {
	runtime := characterStudioHub.get(publicID)
	if runtime == nil {
		return false
	}
	if !runtime.beginReconnect() {
		return true
	}
	defer runtime.endReconnect()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	settings := s.characterStudioSettings(ctx)
	row, err := s.scanCharacterStudioSession(ctx, `
		SELECT s.id,s.public_id,s.user_id,s.character_id,s.current_character_id,s.live_id,s.conn_id,s.render_uid,
		       s.rtc_payload,s.resolution,s.status,s.switch_count,s.error_code,s.error_message,s.billed_seconds,
		       s.credits,s.provider_cost,s.cost,s.started_at,s.ended_at,s.last_heartbeat_at,s.created_at
		FROM character_studio_sessions s WHERE s.public_id=$1`, publicID)
	if err != nil || row == nil || (row.Status != "live" && row.Status != "reconnecting") {
		return false
	}
	seq := 0
	if old := runtime.currentLink(); old != nil {
		old.mu.Lock()
		seq = old.seq
		old.closed = true
		if old.conn != nil {
			_ = old.conn.Close()
		}
		old.mu.Unlock()
	}
	link, nextSeq, err := s.openCharacterStudioLink(ctx, settings, row.LiveID, connID, seq)
	if err != nil {
		return false
	}
	_ = nextSeq
	runtime.setLink(link)
	go s.readCharacterStudioLink(publicID, userID, link, connID)
	return true
}

func (s *ChatService) characterStudioCanReconnect(ctx context.Context, publicID string) bool {
	var status string
	if err := s.db.QueryRow(ctx, `SELECT status FROM character_studio_sessions WHERE public_id=$1`, publicID).Scan(&status); err != nil {
		return false
	}
	return status == "live" || status == "reconnecting"
}

func (s *ChatService) endCharacterStudioAsync(userID int64, publicID, code, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _ = s.EndCharacterStudioSession(ctx, userID, publicID, code, message)
}

func (s *ChatService) watchCharacterStudioSession(publicID string, userID int64, maxSeconds int) {
	runtime := characterStudioHub.get(publicID)
	if runtime == nil {
		return
	}
	if !runtime.beginReaper() {
		return
	}
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		var status string
		var startedAt, heartbeatAt *time.Time
		err := s.db.QueryRow(ctx, `SELECT status, started_at, last_heartbeat_at FROM character_studio_sessions WHERE public_id=$1`, publicID).
			Scan(&status, &startedAt, &heartbeatAt)
		cancel()
		if err != nil || (status != "live" && status != "reconnecting") {
			return
		}
		now := time.Now()
		if startedAt != nil && maxSeconds > 0 && now.Sub(*startedAt) >= time.Duration(maxSeconds)*time.Second {
			s.endCharacterStudioAsync(userID, publicID, "timeout", "达到单场时长上限")
			return
		}
		last := startedAt
		if heartbeatAt != nil {
			last = heartbeatAt
		}
		if last != nil && now.Sub(*last) > characterStudioHeartbeatTimeout {
			s.endCharacterStudioAsync(userID, publicID, "connection_lost", "实时通道已断开")
			return
		}
	}
}

// ---------- 纯函数 ----------

func (l *characterStudioLink) write(frameType int, payload any) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.writeLocked(frameType, payload)
}

func (l *characterStudioLink) writeLocked(frameType int, payload any) error {
	if l.closed || l.conn == nil {
		return errors.New("closed")
	}
	l.seq++
	frame := map[string]any{
		"type":    frameType,
		"live_id": l.liveID,
		"conn_id": l.connID,
		"seq_id":  l.seq,
		"payload": payload,
	}
	return websocket.JSON.Send(l.conn, frame)
}

func (l *characterStudioLink) switchPrompt(imageURL string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.conn == nil {
		return errors.New("closed")
	}
	if time.Since(l.lastSwitch) < characterStudioSwitchInterval {
		return characterStudioFail("切换太快，请稍候再试")
	}
	if err := l.writeLocked(13, characterStudioSwitchPayload(imageURL)); err != nil {
		return err
	}
	l.lastSwitch = time.Now()
	return nil
}

func (l *characterStudioLink) close(hangup bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	if hangup && l.conn != nil {
		_ = l.writeLocked(5, characterStudioHangupPayload())
	}
	l.closed = true
	if l.conn != nil {
		_ = l.conn.Close()
	}
}

func (l *characterStudioLink) isClosed() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.closed
}

func characterStudioConnInitPayload() map[string]any {
	return map[string]any{"conn_init": map[string]any{"version": 1}}
}

func characterStudioSwitchPayload(imageURL string) map[string]any {
	return map[string]any{
		"switch_prompt": map[string]any{
			"prompts":      []any{map[string]any{"type": "image", "content": imageURL}},
			"editing_type": characterStudioEditingType,
		},
	}
}

func characterStudioHangupPayload() map[string]any {
	return map[string]any{"hangup": map[string]any{"hangup_reason": "user_end"}}
}

func characterStudioFrameType(frame map[string]any) int {
	switch value := frame["type"].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(value))
		return n
	}
	return 0
}

func characterStudioMap(payload any, key string) map[string]any {
	raw, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	child, _ := raw[key].(map[string]any)
	return child
}

func characterStudioString(value any) string {
	switch item := value.(type) {
	case string:
		return item
	case json.Number:
		return item.String()
	case float64:
		return strconv.FormatInt(int64(item), 10)
	case int:
		return strconv.Itoa(item)
	case int64:
		return strconv.FormatInt(item, 10)
	}
	return ""
}

func characterStudioChannel(raw []byte) *CharacterStudioChannel {
	if len(raw) == 0 {
		return nil
	}
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	channel := &CharacterStudioChannel{
		AppID:         characterStudioString(value["app_id"]),
		ChannelID:     characterStudioString(value["channel_id"]),
		UserID:        characterStudioString(value["user_id"]),
		Token:         characterStudioString(value["token"]),
		TokenExpireAt: characterStudioString(value["token_expire_at"]),
	}
	if channel.AppID == "" || channel.ChannelID == "" || channel.UserID == "" || channel.Token == "" {
		return nil
	}
	return channel
}

func characterStudioImageURL(raw, publicBase string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "data:") || strings.HasPrefix(value, "ssupload:") {
		return value
	}
	if strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") {
		return value
	}
	if strings.HasPrefix(value, "//") {
		return "https:" + value
	}
	base := strings.TrimRight(strings.TrimSpace(publicBase), "/")
	if base != "" && strings.HasPrefix(value, "/") {
		return base + value
	}
	return ""
}

func characterStudioCost(salePerMinute float64, billedSeconds int) float64 {
	if billedSeconds <= 0 || salePerMinute <= 0 {
		return 0
	}
	return math.Round(float64(billedSeconds)/60*salePerMinute*1e6) / 1e6
}

func characterStudioProviderCost(billedSeconds int, credits float64, settings characterStudioSettings) float64 {
	if credits > 0 {
		return math.Round(credits*settings.CreditPrice*1e6) / 1e6
	}
	if billedSeconds <= 0 {
		return 0
	}
	return math.Round(float64(billedSeconds)*settings.CreditPerSecond*settings.CreditPrice*1e6) / 1e6
}

func characterStudioEndReason(code string) string {
	switch strings.TrimSpace(code) {
	case "user_end", "user_hangup":
		return "你结束了本次会话"
	case "timeout":
		return "达到单场时长上限"
	case "audit_violation":
		return "内容安全校验未通过"
	case "credit_insufficient":
		return "算力余额不足"
	case "client_reconnect_timeout", "client_disconnect_before_ready", "connection_lost":
		return "连接中断，会话已结束"
	case "render_ready_timeout", "render_reconnect_timeout", "render_internal_error", "render_ended":
		return "渲染节点异常，会话已结束"
	case "start_timeout":
		return "启动超时"
	case "create_failed", "create_rejected", "handshake_failed":
		return "这次会话没有成功开播"
	}
	return ""
}

func characterStudioHandshakeError(code string) string {
	switch strings.TrimSpace(code) {
	case "SIP_READY_TIMEOUT", "LIVE_ENDED", "LIVE_CONN_INIT_FAILED":
		return "实时通道准备失败，请稍后再试"
	case "NOT_READY":
		return "渲染节点还在准备，请稍后再试"
	}
	return "实时通道没有就绪，请稍后再试"
}

func characterStudioSwitchError(code string) string {
	switch strings.TrimSpace(code) {
	case "SWITCH_PROMPT_INVALID":
		return "这次切换的参数不可用，请换一个角色再试"
	case "LIVE_NOT_ACTIVE":
		return "会话已经结束"
	case "RENDER_NOT_READY":
		return "渲染节点还在准备，请稍后再试"
	case "SWITCH_PROMPT_EXPIRED":
		return "切换指令已过期，请再试一次"
	case "SWITCH_PROMPT_TIMEOUT":
		return "切换处理超时，请再试一次"
	case "RENDER_WRITE_FAILED", "SWITCH_PROMPT_INTERNAL_ERROR":
		return "渲染节点暂时没有响应，请再试一次"
	}
	return "角色切换没有生效，请再试一次"
}

func characterStudioUpstreamError(status int, payload []byte) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "实时渲染服务的凭证无效，请联系客服"
	case http.StatusTooManyRequests:
		return "当前同时进行的会话较多，请稍后再试"
	case http.StatusPaymentRequired:
		return "算力余额不足，请充值后再试"
	}
	var parsed struct {
		Code    string `json:"code"`
		Reason  string `json:"reason"`
		Message string `json:"message"`
	}
	if json.Unmarshal(payload, &parsed) == nil {
		text := parsed.Message
		if text == "" {
			text = parsed.Reason
		}
		lower := strings.ToLower(text)
		if strings.Contains(lower, "credit") || strings.Contains(lower, "insufficient") {
			return "算力余额不足，请充值后再试"
		}
		if strings.Contains(lower, "audit") || strings.Contains(lower, "face") || strings.Contains(lower, "image") {
			return "参考形象未通过校验，请换一张正面清晰的人像"
		}
	}
	return "实时渲染服务暂时不可用，请稍后再试"
}

func characterStudioLogBody(payload []byte) string {
	text := strings.TrimSpace(string(payload))
	if text == "" {
		return "<empty>"
	}
	if runes := []rune(text); len(runes) > 512 {
		text = string(runes[:512]) + "..."
	}
	return util.SanitizeMessage(text)
}
