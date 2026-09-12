package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Account      string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null" json:"-"`
	QQ           string `json:"qq"`
	Email        string `json:"email"`
	Level        int    `gorm:"default:1" json:"level"`
	Exp          int    `gorm:"default:0" json:"exp"`
	Role         string `gorm:"default:'user'" json:"role"`
	Status       string `gorm:"default:'active'" json:"status"`
}
