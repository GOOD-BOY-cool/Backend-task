package controllers

import (
	"backend/models"
	"backend/utils"
	"time"

	"github.com/gin-gonic/gin"
)

func SignIn(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Fail(c, 401, "未登录")
		return
	}
	uid := userID.(uint)

	var user models.User
	DB.First(&user, uid)

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	if user.LastSignIn != nil {
		lastDate := time.Date(
			user.LastSignIn.Year(),
			user.LastSignIn.Month(),
			user.LastSignIn.Day(),
			0, 0, 0, 0, time.Local,
		)
		if lastDate.Equal(today) {
			utils.Fail(c, 400, "今天已签到")
			return
		} //时间比较用Equal更安全和准确
	}

	expGain := 5
	streak := user.SignInStreak

	if user.LastSignIn != nil {
		yesterday := today.AddDate(0, 0, -1)
		lastDate := time.Date(
			user.LastSignIn.Year(),
			user.LastSignIn.Month(),
			user.LastSignIn.Day(),
			0, 0, 0, 0, time.Local,
		)

		if lastDate.Equal(yesterday) {
			streak++
			if streak >= 7 && streak < 14 {
				expGain = 10
			} else if streak >= 14 && streak < 21 {
				expGain = 20
			} else if streak >= 21 {
				expGain = 30
			} else {
				streak = 1
			}
		} else {
			streak = 1
		}
	}
	user.Exp += expGain
	user.Level = user.Exp/100 + 1
	user.LastSignIn = &now //LastSignIn为指针类型
	user.SignInStreak = streak

	if err := DB.Save(&user).Error; err != nil {
		utils.Fail(c, 500, "签到失败")
		return
	}

	utils.Success(c, gin.H{
		"exp_gain": expGain,
		"user_exp": user.Exp,
		"level":    user.Level,
		"streak":   streak,
		"msg":      "签到成功",
	})
}
