package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/starai/api/internal/config"
)

func TestExtractAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		header string
		query  string
		want   string
	}{
		{
			name:   "standard bearer",
			header: "Bearer sk-starai-test",
			want:   "sk-starai-test",
		},
		{
			name:   "case insensitive bearer with extra spaces",
			header: "  bearer   sk-starai-test  ",
			want:   "sk-starai-test",
		},
		{
			name:  "token query fallback",
			query: "?token=sk-starai-query",
			want:  "sk-starai-query",
		},
		{
			name:  "api key query fallback",
			query: "?api_key=sk-starai-query",
			want:  "sk-starai-query",
		},
		{
			name: "empty",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions"+tt.query, nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			c.Request = req

			if got := extractAPIKey(c); got != tt.want {
				t.Fatalf("extractAPIKey()=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorkbenchSessionMissingUserClearsCookieAndReturnsUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/workbench/session", nil)

	handler := &Handler{cfg: &config.Config{}}
	handler.failWorkbenchSession(ctx, pgx.ErrNoRows)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != http.StatusUnauthorized || body.Message != "登录状态已失效，请重新登录" {
		t.Fatalf("unexpected response: %+v", body)
	}
	cookie := recorder.Result().Cookies()
	if len(cookie) != 1 || cookie[0].Name != "starai_session" || cookie[0].MaxAge >= 0 {
		t.Fatalf("session cookie was not cleared: %+v", cookie)
	}
}

func TestWorkbenchSessionUnexpectedFailureRemainsServerError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/workbench/session", nil)

	handler := &Handler{cfg: &config.Config{}}
	handler.failWorkbenchSession(ctx, errors.New("database unavailable"))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
