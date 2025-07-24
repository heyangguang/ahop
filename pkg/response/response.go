package response

import (
	"net/http"

	"ahop/pkg/errors"
	"ahop/pkg/pagination"

	"github.com/gin-gonic/gin"
)

// Response 统一返回格式
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// PageInfo 分页信息
type PageInfo struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// PageResponse 分页返回格式
type PageResponse struct {
	Code     int         `json:"code"`
	Message  string      `json:"message"`
	Data     interface{} `json:"data,omitempty"`
	PageInfo PageInfo    `json:"page_info"`
}

// ========== 基础返回方法 ==========

// Success 成功返回
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    errors.CodeSuccess,
		Message: "success",
		Data:    data,
	})
}

// SuccessWithMessage 成功返回（自定义消息）
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    errors.CodeSuccess,
		Message: message,
		Data:    data,
	})
}

// SuccessWithPage 分页成功返回
func SuccessWithPage(c *gin.Context, data interface{}, pageInfo *pagination.PageInfo) {
	c.JSON(http.StatusOK, gin.H{
		"code":      errors.CodeSuccess,
		"message":   "success",
		"data":      data,
		"page_info": pageInfo,
	})
}

// Error 通用错误返回
func Error(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
	})
}

// ========== HTTP错误快捷方法 ==========

func BadRequest(c *gin.Context, message string) {
	Error(c, errors.CodeInvalidParam, message)
}

func Unauthorized(c *gin.Context, message string) {
	Error(c, errors.CodeUnauthorized, message)
}

func Forbidden(c *gin.Context, message string) {
	Error(c, errors.CodeForbidden, message)
}

func NotFound(c *gin.Context, message string) {
	Error(c, errors.CodeNotFound, message)
}

func ServerError(c *gin.Context, message string) {
	Error(c, errors.CodeServerError, message)
}
