package controllers

import (
	"backend/models"
	"backend/services"
	"backend/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

func CreateReport(c *gin.Context) {
	pid64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(c, 400, "帖子无效")
		return
	}
	pid := uint(pid64)

	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	var goods models.Goods
	if err := DB.First(&goods, pid).Error; err != nil {
		utils.Fail(c, 400, "帖子不存在")
		return
	}

	if goods.UserID == uid {
		utils.Fail(c, 400, "不能举报自己的帖子")
		return
	}

	var existing models.Report
	if err := DB.Where("user_id=? AND post_id=?", uid, pid).First(&existing).Error; err == nil {
		utils.Fail(c, 400, "你已经举报过这个帖子")
		return
	}

	reason := c.PostForm("reason")
	if reason == "" {
		utils.Fail(c, 400, "举报原因不能为空")
		return
	}

	DB.Create(&models.Report{
		UserID: uid,
		PostID: pid,
		Reason: reason,
		Status: "pending",
	})

	utils.Success(c, "举报成功,等待管理员处理")

}

func ListReport(c *gin.Context) {
	var reports []models.Report
	DB.Order("created_at ASC").Find(&reports)
	utils.Success(c, reports)

}

func AdminHandleReport(c *gin.Context) {
	rid64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(c, 400, "举报ID无效")
		return
	}
	rid := uint(rid64)

	action := c.PostForm("action") //valid/invalid 有效/无效
	if action != "valid" && action != "invalid" {
		utils.Fail(c, 400, "action只能是valid或invalid")
		return

	}

	var report models.Report
	if err := DB.First(&report, rid).Error; err != nil {
		utils.Fail(c, 404, "举报记录不存在")
		return
	}
	if action == "valid" {
		DB.Model(&models.Goods{}).Where("id=?", report.PostID).Update("status", "deleted")

		var goods models.Goods
		DB.First(&goods, report.PostID)
		if err := services.AddExp(goods.UserID, -5); err != nil {
			utils.Fail(c, 500, "处理失败")
			return
		}
		if err := services.AddExp(report.UserID, 5); err != nil {
			utils.Fail(c, 500, "处理失败")
			return
		}
	}

	DB.Model(&report).Update("status", action)
	utils.Success(c, "处理完成")

}
