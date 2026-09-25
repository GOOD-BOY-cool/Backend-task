package controllers

import (
	"backend/models"
	"backend/utils"

	"strconv"

	"github.com/gin-gonic/gin"
)

// 商品发布函数
func CreateGoods(c *gin.Context) {
	var goods models.Goods
	if err := c.ShouldBindJSON(&goods); err != nil {
		utils.Fail(c, 400, "参数错误："+err.Error())
		return
	}
	// 从 JWT 中间件透传的用户ID
	userID, _ := c.Get("user_id")
	uid := userID.(uint)
	goods.UserID = uid

	var user models.User
	DB.First(&user, uid)
	if user.Level >= 2 {
		goods.Status = "approved"
	} else {
		goods.Status = "pending"
	}

	if err := DB.Create(&goods).Error; err != nil {
		utils.Fail(c, 500, "发布失败")
		return
	}

	msg := "发布成功"
	if goods.Status == "pending" {
		msg = "发布成功,等待管理员审核"
	}
	utils.Success(c, gin.H{"msg": msg, "goods": goods})
}

// 商品列表（包含查询功能与分页功能）
func ListGoods(c *gin.Context) {
	keyword := c.Query("keyword")                          //c.get拿c.set放置的东西(一次请求结束后就没了，是我自己装进上下文的，数据存在请求内存)，c.Query拿URL中问号？后面的参数
	page, err := strconv.Atoi(c.DefaultQuery("page", "1")) //strconv.Atoi()将字符串转换为整数，c.DefaultQuery("page","1"）当输入为空时默认page为1，防止未输入值导致的程序崩溃
	if err != nil {
		utils.Fail(c, 400, "请输入数字")
	}
	limit := 10                  //每次拿10条
	offset := (page - 1) * limit //拿取你输入页码的数据

	var goods []models.Goods //用数组来对应数据库多行数据，因为列表要装多条数据，为Find做准备

	query := DB.Model(&models.Goods{}).Where("status=?", "approved")

	if keyword != "" {
		query = query.Where("title LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%") //动态模糊查询，LIKE是模糊匹配，%=”通配符“，%keyword%意味着任意位置包含keyword就行，问号是占位符
	}
	query.Limit(limit).Offset(offset).Find(&goods)
	utils.Success(c, goods)
}

// 商品详情
func DetailGoods(c *gin.Context) {
	id := c.Param("id") //c.Param取URL路径里的参数(在问号前面)
	var goods models.Goods
	if err := DB.Where("id=? AND status = ?", id, "approved").First(&goods).Error; err != nil {
		utils.Fail(c, 404, "商品不存在")
		return
	} //Find查找不到返回空对象或者切片，First查不到会报错
	utils.Success(c, goods)
}

// 发布者更新商品
func UpdateGoods(c *gin.Context) {
	id := c.Param("id")
	var goods models.Goods
	if err := DB.First(&goods, id).Error; err != nil {
		utils.Fail(c, 404, "商品不存在")
		return
	}
	userID, _ := c.Get("user_id")
	if goods.UserID != userID.(uint) {
		utils.Fail(c, 403, "无权限")
		return
	}
	var updateData models.Goods
	c.ShouldBindJSON(&updateData)
	DB.Model(&goods).Updates(updateData) //锁定这条商品并且更新他
	utils.Success(c, goods)

}

// 删除商品
func DeleteGoods(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")
	uid := userID.(uint)
	var goods models.Goods
	if err := DB.First(&goods, id).Error; err != nil {
		utils.Fail(c, 404, "商品不存在")
		return
	}
	if goods.UserID != uid {
		utils.Fail(c, 403, "你无权限删除别人的商品")
	}
	DB.Model(&models.Goods{}).Where("id=?", id).Update("status", "deleted")
	utils.Success(c, nil)
}
