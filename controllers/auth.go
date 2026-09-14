package controllers

import (
	"backend/config"
	"backend/models"
	"backend/utils"

	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(db *gorm.DB) {
	DB = db
}
func Register(c *gin.Context) {
	var body struct {
		Account  string `json:"account"`
		Password string `json:"password"`
		QQ       string `json:"qq"`
		Email    string `json:"email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Fail(c, 400, "参数错误")
		return
	}
	hash, _ := utils.HashPassword(body.Password)
	user := models.User{
		Account:      body.Account,
		PasswordHash: hash,
		QQ:           body.QQ,
		Email:        body.Email,
	}
	if err := DB.Create(&user).Error; err != nil {
		utils.Fail(c, 500, "注册失败账号已经存在")
		return
	}
	utils.Success(c, nil)

}
func Login(c *gin.Context) {
	var body struct {
		Account  string `json:"accont"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Fail(c, 400, "参数错误")
		return
	}
	var user models.User
	if err := DB.Where("account=?", body.Account).First(&user).Error; err != nil {
		utils.Fail(c, 401, "账号或密码错误")
		return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     time.Now().Add(time.Second * config.JWTExpire).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(config.JWTSecret))

	utils.Success(c, gin.H{
		"token": tokenStr,
		"user": gin.H{
			"level": user.Level,
			"role":  user.Role,
		},
	})

}
