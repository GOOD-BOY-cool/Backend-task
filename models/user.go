package models

import "gorm.io/gorm"

type User struct {
	gorm.Model          //包含 ID（自增主键）、CreatedAt（记录创建时间）、UpdatedAt（记录更新时间）、DeletedAt（软删除字段，执行Delet()时只记录删除时间，不真正删除数据） 字段
	Account      string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null" json:"-"`
	QQ           string `json:"qq"`
	Email        string `json:"email"`
	Level        int    `gorm:"default:1" json:"level"`
	Exp          int    `gorm:"default:0" json:"exp"`
	Role         string `gorm:"default:'user'" json:"role"`
	Status       string `gorm:"default:'active'" json:"status"`
}
