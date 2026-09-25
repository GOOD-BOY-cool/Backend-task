package router

import (
	"backend/controllers"
	"backend/middleware"

	"github.com/gin-gonic/gin"
)

// *gin.Engine 和路由有关的结构体内部感觉很复杂
func SetupRouter() *gin.Engine {
	r := gin.Default()
	r.Static("/uploads", "./uploads") //把服务器本地的 ./uploads 文件夹，映射成可以通过浏览器直接访问的静态资源路径 /uploads
	//gin.Default()=gin.New()(裸引擎) + Logger()(打印请求日志) + Recovery()（保证panic不崩溃）  gin.New()（返回一个空的gin.Engine结构体），Logger()和Recovery()是gin的中间件
	//recovery主要靠内部的defer + recover()和c.Abort()来保证panic不崩溃)
	auth := r.Group("/api/auth")
	{
		auth.POST("/register", controllers.Register) //注册
		auth.POST("/login", controllers.Login)       //登录
		auth.GET("/me", middleware.JWTAuth(), func(c *gin.Context) {
			c.JSON(200, gin.H{
				"code": 0,
				"msg":  "ok",
			}) //鉴权测试
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
		authGoods.POST("", controllers.CreateGoods)        // 发布(有等级限制)
		authGoods.PUT("/:id", controllers.UpdateGoods)     // 修改    冒号开头表示这是一个动态占位符，名字叫id（可以任意变换）
		authGoods.DELETE("/:id", controllers.DeleteGoods)  // 下架
		authGoods.POST("/upload", controllers.UploadImage) // 上传图片
	}

	adminGroup := r.Group("/api/admin")
	adminGroup.Use(middleware.JWTAuth(), middleware.AdminAuth())
	{
		adminGroup.GET("/post/pending", controllers.PendingPosts)    //拿到等待决定的帖子
		adminGroup.POST("/posts/:id/audit", controllers.Auditpost)   //对帖子进行approve与reject的决定（包含成功时增加经验值的功能）
		adminGroup.DELETE("/posts/:id", controllers.AdminDeletePost) //管理员删除指定帖子

	}

	favorite := r.Group("/api/posts")
	favorite.Use(middleware.JWTAuth())
	{
		favorite.POST("/:id/favorite", controllers.FavoritePost)     //对项目进行收藏
		favorite.DELETE("/:id/favorite", controllers.UnfavoritePost) //删除列表中的已收藏的物品
		favorite.GET("/my-favorite", controllers.MyFavorites)        //生成收藏列表
	}

	//普通用户举报
	report := r.Group("/api/posts")
	report.Use(middleware.JWTAuth())
	{
		report.POST("/:id/report", controllers.CreateReport)
	}

	//管理员处理
	adminReport := r.Group("/api/admin/reports")
	adminReport.Use(middleware.JWTAuth(), middleware.AdminAuth())
	{
		adminReport.GET("", controllers.ListReport)
		adminReport.POST("/:id/handle", controllers.AdminHandleReport)
	}

	return r
}
