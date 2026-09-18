package controllers

import (
	"backend/utils"
	"os"
	"path/filepath"
	"strconv"
	"time"

	// "uuid"  //第二种

	"github.com/gin-gonic/gin"
)

func UploadImage(c *gin.Context) {
	file, err := c.FormFile("image") //从表单获取名为"image"的文件（前端需要传from-data,key为image）
	if err != nil {
		utils.Fail(c, 400, "请选择文件")
		return
	}
	os.MkdirAll("uploads", os.ModePerm) //创建uploads文件夹（如果不存在）,os.ModePerm表示最大权限
	ext := filepath.Ext(file.Filename)  //获取原文件的扩展名（.jpg,.png）

	//第一种
	fileName := strconv.FormatInt(time.Now().Unix(), 10) + ext
	//用法：time.Now().Unix() 获取当前秒级时间戳（如 1695000000），转字符串后(根据括号中数字进行转换，如10即为转换为10进制)拼接扩展名。
	//意义：生成类似 1695000000.jpg 的唯一名，避免同名文件覆盖（但并发极高时时间戳可能重复，更严谨可用 UUID）。

	//第二种
	// newUUID := uuid.New().String()
	// fileName := newUUID + ext
	//更好的方案

	dst := filepath.Join("uploads", fileName)
	//用法：跨平台拼接路径（Linux 为 uploads/xxx.jpg，Windows 为 uploads\xxx.jpg）。
	//意义：得到最终服务器保存的完整相对路径，赋值给 dst
	if err := c.SaveUploadedFile(file, dst); err != nil {
		utils.Fail(c, 500, "上传失败")
		return
	} //将内存中的文件写入 dst 路径,真正完成文件从前端到服务器的落地存储
	utils.Success(c, gin.H{"url": "/" + dst}) //gin.H 构造对象，"/" + dst 生成可访问 URL（如 /uploads/1695000000.jpg）。

}
