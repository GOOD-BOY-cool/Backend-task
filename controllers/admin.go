package controllers

import (
	"backend/models"
	"backend/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func PendingPosts(c *gin.Context) {
	var goods []models.Goods
	DB.Where("status=?", "pending").Order("created_at ASC").Find(&goods) //ASC升序DESC降序
	utils.Success(c, goods)
}

func checkLeval(user *models.User) {
	levalexp := []int{0, 0, 100, 300, 600, 1000, 1500}
	for i := 6; i > 1; i-- {
		if user.Exp >= levalexp[i] && user.Level < i {
			DB.Model(user).Update("leval", i)
			break
		}
	}
} //Auditpost内层所引用的函数

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
		DB.Model(&models.User{}).Where("id=?", goods.UserID).Update("exp", gorm.Expr("exp+10")) //gorm.Expr()专门用于字段增减

		var author models.User
		DB.First(&author, goods.UserID)
		checkLeval(&author)
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
