package models

import (
	"gorm.io/gorm"
)

type Goods struct {
	gorm.Model
	Title       string   `binding:"required" json:"title"`
	Description string   ` json:"description"`
	Price       float64  `binding:"required" json:"price"`
	Images      []string `binding:"required" json:"images" gorm:"serializer:json"` //gorm:"serializer:json"数据存入数据库时将该字段序列化未JSON字符串；从数据库读取时在反序化为Go对象
	Category    string   `json:"category"`
	Status      string   `json:"status" gorm:"default:pending"` // pending/approved/rejected/deleted  初始默认为待审核状态（pending）
	UserID      uint     `json:"user_id"`
}
