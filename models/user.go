package models

import "gorm.io/gorm"

type User struct {
	gorm.Model          //包含 ID（自增主键）、CreatedAt（记录创建时间）、UpdatedAt（记录更新时间）、DeletedAt（软删除字段，执行Delet()时只记录删除时间，不真正删除数据） 字段
	Account      string `gorm:"uniqueIndex;not null;size:50" ` //GORM 对 string 类型，如果没指定 size，默认会映射成 longtext（或 text），而不是 varchar
	PasswordHash string `gorm:"not null;size:255" json:"-"`    //TEXT / BLOB / LONGTEXT：MySQL 规定这类大文本字段不能直接建普通索引或唯一索引，必须指定“前缀长度”
	QQ           string `gorm:"size:20" json:"qq"`
	Email        string `gorm:"size:100" json:"email"`
	Level        int    `gorm:"default:1" json:"level"`
	Exp          int    `gorm:"default:0" json:"exp"`
	Role         string `gorm:"default:'user'" json:"role"`
	// Status       string `gorm:"default:'active'" json:"status"`
}
