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
	//公开接口
	goods := r.Group("/api/goods")
	{
		goods.GET("", controllers.ListGoods)       // 搜索 ?keyword=手机&page=1
		goods.GET("/:id", controllers.DetailGoods) // 详情
	}

	// 需登录接口
	authGoods := r.Group("/api/goods")
	authGoods.Use(middleware.JWTAuth()) //Use给路由挂载中间件，意味着这个分组的所有接口都先执行这个函数
	{
		authGoods.POST("", controllers.CreateGoods)       // 发布
		authGoods.PUT("/:id", controllers.UpdateGoods)    // 修改    冒号开头表示这是一个动态占位符，名字叫id（可以任意变换）
		authGoods.DELETE("/:id", controllers.DeleteGoods) // 下架
		r.POST("/api/upload", controllers.UploadImage)    // 上传图片（也可放这里）
	}

	return r
}
