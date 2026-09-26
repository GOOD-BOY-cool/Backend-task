package services

import (
	"backend/database"
	"backend/models"
)

// AddExp 加/减经验，自动算等级，exp 不低于 0
func AddExp(userID uint, exp int) error {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return err
	}
	user.Exp += exp
	if user.Exp < 0 {
		user.Exp = 0
	}
	if user.Level <= 99 {
		user.Level = user.Exp/100 + 1
		user.Exp -= exp
	}
	return database.DB.Model(models.User{}).Where("id=?", userID).Updates(map[string]interface{}{"exp": user.Exp, "level": user.Level}).Error
}
