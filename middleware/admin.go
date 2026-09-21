package middleware

import (
	"backend/controllers"
	"backend/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 401, "message": "未登录"})
			c.Abort()
			return
		}
		var user models.User
		controllers.DB.First(&user, userID)
		if user.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "需要管理员权限"})
			c.Abort()
			return
		}
		c.Next()

	}
}
