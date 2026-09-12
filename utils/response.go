package utils

import "github.com/gin-gonic/gin"

func Success(c *gin.Context, data interface{}) {
	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "成功",
		"data": data,
	})
} //成功响应

func Fail(c *gin.Context, code int, msg string) {
	c.JSON(200, gin.H{
		"code": code,
		"msg":  msg,
		"data": nil,
	})

} //失败响应
