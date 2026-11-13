package handler

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/starai/api/internal/billing"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

const characterStudioEngineCacheTTL = 6 * time.Hour

var characterStudioEngineCache struct {
	mu        sync.Mutex
	source    string
	body      []byte
	updatedAt time.Time
}

func (h *Handler) CharacterStudioConfig(c *gin.Context) {
	util.OK(c, h.chat.CharacterStudioConfig(c.Request.Context()))
}

func (h *Handler) ListCharacterStudioCharacters(c *gin.Context) {
	items, err := h.chat.ListCharacterStudioCharacters(c.Request.Context(), c.GetInt64("user_id"))
	if err != nil {
		h.writeCharacterStudioError(c, err)
		return
	}
	util.OK(c, items)
}

func (h *Handler) SaveCharacterStudioCharacter(c *gin.Context) {
	var req service.CharacterStudioInput
	if c.ShouldBindJSON(&req) != nil {
		util.BadRequest(c, "参数错误")
		return
	}
	item, err := h.chat.SaveCharacterStudioCharacter(c.Request.Context(), c.GetInt64("user_id"), c.Param("id"), req)
	if err != nil {
		h.writeCharacterStudioError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *Handler) DeleteCharacterStudioCharacter(c *gin.Context) {
	if err := h.chat.DeleteCharacterStudioCharacter(c.Request.Context(), c.GetInt64("user_id"), c.Param("id")); err != nil {
		h.writeCharacterStudioError(c, err)
		return
	}
	util.OK(c, gin.H{"ok": true})
}

func (h *Handler) StartCharacterStudioSession(c *gin.Context) {
	var req struct {
		CharacterID string `json:"character_id"`
		Resolution  string `json:"resolution"`
	}
	if c.ShouldBindJSON(&req) != nil {
		util.BadRequest(c, "参数错误")
		return
	}
	item, err := h.chat.StartCharacterStudioSession(c.Request.Context(), c.GetInt64("user_id"), req.CharacterID, req.Resolution)
	if err != nil {
		h.writeCharacterStudioError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *Handler) GetCharacterStudioSession(c *gin.Context) {
	item, err := h.chat.GetCharacterStudioSession(c.Request.Context(), c.GetInt64("user_id"), c.Param("id"))
	if err != nil {
		h.writeCharacterStudioError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *Handler) ActiveCharacterStudioSession(c *gin.Context) {
	item, err := h.chat.ActiveCharacterStudioSession(c.Request.Context(), c.GetInt64("user_id"))
	if err != nil {
		h.writeCharacterStudioError(c, err)
		return
	}
	util.OK(c, gin.H{"session": item})
}

func (h *Handler) SwitchCharacterStudioCharacter(c *gin.Context) {
	var req struct {
		CharacterID string `json:"character_id"`
	}
	if c.ShouldBindJSON(&req) != nil {
		util.BadRequest(c, "参数错误")
		return
	}
	item, err := h.chat.SwitchCharacterStudioCharacter(c.Request.Context(), c.GetInt64("user_id"), c.Param("id"), req.CharacterID)
	if err != nil {
		h.writeCharacterStudioError(c, err)
		return
	}
	util.OK(c, gin.H{"character": item, "accepted": true})
}

func (h *Handler) HeartbeatCharacterStudioSession(c *gin.Context) {
	if err := h.chat.HeartbeatCharacterStudioSession(c.Request.Context(), c.GetInt64("user_id"), c.Param("id")); err != nil {
		h.writeCharacterStudioError(c, err)
		return
	}
	util.OK(c, gin.H{"ok": true})
}

func (h *Handler) EndCharacterStudioSession(c *gin.Context) {
	item, err := h.chat.EndCharacterStudioSession(c.Request.Context(), c.GetInt64("user_id"), c.Param("id"), "user_end", "你结束了本次会话")
	if err != nil {
		h.writeCharacterStudioError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *Handler) AddCharacterStudioTake(c *gin.Context) {
	var req service.CharacterStudioTakeInput
	if c.ShouldBindJSON(&req) != nil {
		util.BadRequest(c, "参数错误")
		return
	}
	userID := c.GetInt64("user_id")
	item, err := h.chat.AddCharacterStudioTake(c.Request.Context(), userID, c.Param("id"), req)
	if err != nil {
		h.writeCharacterStudioError(c, err)
		return
	}
	// 片段上传只落 assets；这里同步生成 works 记录，才能出现在「我的作品」。
	if item.AssetPublicID != "" && h.works != nil {
		retentionDays := 7
		if h.admin != nil {
			if configs, configErr := h.admin.GetRawSystemConfigs(c.Request.Context()); configErr == nil {
				retentionDays = service.ParseWorkRetentionDays(configs["work_retention_days"], retentionDays)
			}
		}
		title := "角色替换片段"
		if _, workErr := h.works.CreateFromAssetWithSource(c.Request.Context(), userID, item.AssetPublicID, title, "video", item.URL, "character_studio", retentionDays); workErr != nil {
			log.Printf("character studio take work failed: user_id=%d take=%s asset=%s error=%v", userID, item.PublicID, item.AssetPublicID, workErr)
		}
	}
	util.OK(c, item)
}

// RealtimeEngine 用中性路径代理浏览器运行时，避免页面源码里直接出现第三方 CDN 域名。
// 代理源地址由后台配置；只允许 http(s)，并限制响应大小与缓存时间。
func (h *Handler) RealtimeEngine(c *gin.Context) {
	source := strings.TrimSpace(h.chat.CharacterStudioEngineURL(c.Request.Context()))
	target, err := url.Parse(source)
	if err != nil || (target.Scheme != "https" && target.Scheme != "http") || target.Host == "" {
		util.InternalError(c, "实时运行服务暂时不可用")
		return
	}
	now := time.Now()
	characterStudioEngineCache.mu.Lock()
	if characterStudioEngineCache.source == source && len(characterStudioEngineCache.body) > 0 && now.Sub(characterStudioEngineCache.updatedAt) < characterStudioEngineCacheTTL {
		body := characterStudioEngineCache.body
		characterStudioEngineCache.mu.Unlock()
		writeRealtimeEngine(c, body)
		return
	}
	stale := characterStudioEngineCache.body
	characterStudioEngineCache.mu.Unlock()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		writeRealtimeEngineStale(c, stale)
		return
	}
	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		writeRealtimeEngineStale(c, stale)
		return
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		writeRealtimeEngineStale(c, stale)
		return
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil || len(body) == 0 {
		writeRealtimeEngineStale(c, stale)
		return
	}
	characterStudioEngineCache.mu.Lock()
	characterStudioEngineCache.source = source
	characterStudioEngineCache.body = body
	characterStudioEngineCache.updatedAt = now
	characterStudioEngineCache.mu.Unlock()
	writeRealtimeEngine(c, body)
}

func writeRealtimeEngine(c *gin.Context, body []byte) {
	if len(body) == 0 {
		util.InternalError(c, "实时运行服务暂时不可用")
		return
	}
	c.Header("Cache-Control", "public, max-age=21600")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/javascript; charset=utf-8", body)
}

func writeRealtimeEngineStale(c *gin.Context, body []byte) {
	if len(body) > 0 {
		writeRealtimeEngine(c, body)
		return
	}
	util.InternalError(c, "实时运行服务暂时不可用")
}

func (h *Handler) writeCharacterStudioError(c *gin.Context, err error) {
	if msg, ok := service.AsCharacterStudioError(err); ok {
		util.BadRequest(c, msg)
		return
	}
	if errors.Is(err, billing.ErrInsufficientBalance) {
		util.Fail(c, http.StatusPaymentRequired, 402, billing.InsufficientBalanceMsg)
		return
	}
	util.InternalError(c, "角色替换服务暂时不可用")
}
