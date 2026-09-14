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
		"exp":     time.Now().Add(time.Second * config.JWTExpire).Unix(), //转变为unix时间戳,因为JWT标准要求字段必须是时间戳
	}) //token即是header.payload.signature的组合，header（包含alg:签名算法,typ:token类型几乎永远是JWT）和payload是base64编码(方便网上传输)的，signature是用header和payload以及密钥生成的签名
	tokenStr, _ := token.SignedString([]byte(config.JWTSecret))

	utils.Success(c, gin.H{
		"token": tokenStr,
		"user": gin.H{
			"level": user.Level,
			"role":  user.Role,
		},
	})

} //这里用私钥后期有时间可以考虑公钥   多个授权服务考虑公钥，因为当密码当多个授权后私钥被破解概率会极大上升（这或许和破译密码学有关）
//或许我对密码学感兴趣
