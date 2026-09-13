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
		}
		//判断是否登录

		tokenStr := strings.TrimPrefix(header, "Bearer")
		//剥去Bearer前缀，因为前端传来的token中含Bearer但是jwt.Parse只认字符串
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return []byte(config.JWTSecret), nil
			//tokenStr为前端传来的，func告诉库用哪吧密钥签名，jwt.Parse会自动告诉你用什么签名算法  注意；func内的nil不是外面err的值
			//token为验完后返回的结构体

		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Token无效"})
			c.Abort()
			return
		}
		//Valid代表是否有效,由jwt. Parse解析得到
		c.Next()
	}

}
