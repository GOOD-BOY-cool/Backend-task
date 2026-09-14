package router

import (
	"backend/controllers"
	"backend/middleware"

	"github.com/gin-gonic/gin"
)

// *gin.Engine 和路由有关的结构体内部感觉很复杂
func SetupRouter() *gin.Engine {
	r := gin.Default()
	//gin.Default()=gin.New()(裸引擎) + Logger()(打印请求日志) + Recovery()（保证panic不崩溃）  gin.New()（返回一个空的gin.Engine结构体），Logger()和Recovery()是gin的中间件
	//recovery主要靠内部的defer + recover()和c.Abort()来保证panic不崩溃)
	auth := r.Group("/api/auth")
	{
		auth.POST("/register", controllers.Register)
		auth.POST("/login", controllers.Login)
		auth.GET("/me", middleware.JWTAuth(), func(c *gin.Context) {
			c.JSON(200, gin.H{
				"code": 0,
				"msg":  "ok",
			})
		})
	}

	return r
}
