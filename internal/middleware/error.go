package middleware

import (
	"ahop/pkg/logger"
	"ahop/pkg/response"

	"github.com/gin-gonic/gin"
)

// ErrorHandler 错误处理中间件 - 主要处理panic
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				appLogger := logger.GetLogger()
				appLogger.Errorf("Panic recovered: %v", err)
				response.ServerError(c, "服务器内部错误")
				c.Abort()
			}
		}()

		c.Next()
	}
}
