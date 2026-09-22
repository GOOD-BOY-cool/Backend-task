package main

import (
	"backend/config"
	"backend/controllers"
	"backend/models"
	router "backend/routes"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	dsn := config.DBUser + ":" +
		config.DBPassword + "@tcp(" +
		config.DBHost + ":" + config.DBPort + ")/" +
		config.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
	//utf8mb4是utf8的超集，支持更多字符，尤其是emoji表情
	//parseTime=True是将MySQL时间字段转为go的time.Time类型
	//loc=Local是使用本地时区
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	// gorm.Open()是gorm的函数建立与数据库的链接，mysql.Open(dsn)是mysql驱动的函数	，把MySql数据库的底层连结参数准备好
	// dsn是Data Source Name的缩写，表示数据源名称，包含了连接数据库所需的各种信息，如用户名、密码、主机地址、端口号、数据库名称等。dsn的格式为"user:password@tcp(host:port)/dbname?param1=value1&param2=value2"
	//&gorm.Config{}代表默认配置如默认复数表明,且字段名默认是驼峰转下划线,如UserName->user_name
	//小驼峰第一个字母小写后面每个单词首字母大写,大驼峰每个单词首字母大写,下划线命名所有字母小写，单词之间用下划线分隔,如user_name
	if err != nil {
		panic("数据库连接失败")
	}
	db.AutoMigrate(&models.User{})     //生成用户表
	db.AutoMigrate(&models.Goods{})    //生成物品表
	db.AutoMigrate(&models.AuditLog{}) //生成审计日志（管理员）
	//按照模型结构体自动创建表，若表已存在则不创建
	controllers.InitDB(db)

	r := router.SetupRouter()
	r.Run(":8080")
}

//官方默认规则（约定）
//只要你没用 gorm:"column:xxx" 标签强行指定列名，GORM 就会按以下规则自动转换：
//字段名：驼峰命名（CamelCase）自动转蛇形小写（snake_case）。比如 CreatedAt → created_at，UserName → user_name。
//表名：结构体名字转蛇形，并且默认变复数。比如 Goods → goods（你之前的商品表），User → users。
//c.Get返回interface{},c.Param返回string
