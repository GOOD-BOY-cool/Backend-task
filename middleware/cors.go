package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS 允许跨域请求：前端不在 8080 同源时，浏览器会直接拦掉请求
// 预检请求（OPTIONS）直接返回 204，不进入业务处理器
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

//这里设置了返回初始返回请求的格式
//c.JSON() gin帮你自动c.writer与初始的结构不会冲突二者相互补充
