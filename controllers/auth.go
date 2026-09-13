package controllers

import (
	"backend/utils"

	"github.com/gin-gonic/gin"
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
}
