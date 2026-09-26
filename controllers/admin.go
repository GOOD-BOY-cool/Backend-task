package controllers

import (
	"backend/models"
	"backend/services"
	"backend/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func PendingPosts(c *gin.Context) {
	var goods []models.Goods
	DB.Where("status=?", "pending").Order("created_at ASC").Find(&goods) //ASC升序DESC降序
	utils.Success(c, goods)
}

func Auditpost(c *gin.Context) {
	id := c.Param("id")
	action := c.PostForm("action") //c.PostForm从前端表单拿取数据，分为reject和approve

	var goods models.Goods
	if err := DB.First(&goods, id).Error; err != nil {
		utils.Fail(c, 404, "帖子不存在")
		return
	}

	adminID, _ := c.Get("user_id")

	if action == "approve" {
		DB.Model(&goods).Update("status", "approved")
		// DB.Model(&models.User{}).Where("id=?", goods.UserID).Update("exp", gorm.Expr("exp+10")) //gorm.Expr()专门用于字段增减

		if err := services.AddExp(goods.UserID, 10); err != nil {
			utils.Fail(c, 500, "处理失败")
			return
		}
	} else {
		DB.Model(&goods).Update("status", "rejected")
	}

	DB.Create(&models.AuditLog{
		AdminID: adminID.(uint),
		PostID:  goods.ID,
		Action:  action})
	utils.Success(c, "操作成功")
}

func AdminDeletePost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "格式错误",
		})
		return

	}
	adminID, _ := c.Get("user_id")
	DB.Model(&models.Goods{}).Where("id=?", id).Update("status", "deleted")
	DB.Create(&models.AuditLog{
		AdminID: adminID.(uint),
		PostID:  uint(id),
		Action:  "delete",
	})
	utils.Success(c, "删除成功")

}
