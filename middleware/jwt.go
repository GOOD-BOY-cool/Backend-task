package middleware

import (
	"backend/config"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer") {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
			c.Abort()
			return
		} //判断是否登录

		tokenStr := strings.TrimPrefix(header, "Bearer") //剥去Bearer前缀
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return []byte(config.JWTSecret), nil

		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Token无效"})
			c.Abort()
			return
		}
		c.Next()
	}

}
