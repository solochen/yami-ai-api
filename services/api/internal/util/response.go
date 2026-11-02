package util

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: data})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, APIResponse{Code: 0, Message: "ok", Data: data})
}

func Fail(c *gin.Context, status int, code int, message string) {
	// 统一出口：任何错误文案都可能夹带上游地址/厂商名，返回前先脱敏。
	c.JSON(status, APIResponse{Code: code, Message: SanitizeMessage(message)})
}

func FailWithData(c *gin.Context, status int, code int, message string, data interface{}) {
	c.JSON(status, APIResponse{Code: code, Message: SanitizeMessage(message), Data: data})
}

func BadRequest(c *gin.Context, message string) {
	Fail(c, http.StatusBadRequest, 400, message)
}

func Unauthorized(c *gin.Context, message string) {
	Fail(c, http.StatusUnauthorized, 401, message)
}

func Forbidden(c *gin.Context, message string) {
	Fail(c, http.StatusForbidden, 403, message)
}

func NotFound(c *gin.Context, message string) {
	Fail(c, http.StatusNotFound, 404, message)
}

func InternalError(c *gin.Context, message string) {
	Fail(c, http.StatusInternalServerError, 500, message)
}
