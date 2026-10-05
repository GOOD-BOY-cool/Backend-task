package models

import (
	"time"

	"gorm.io/gorm"
)

type Goods struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at" gorm:"index"`
	Title       string         `binding:"required" json:"title" gorm:"size:100"`
	Description string         `json:"description" gorm:"size:1000"`
	Price       float64        `binding:"required" json:"price"`
	Images      []string       `binding:"required" json:"images" gorm:"serializer:json"` //gorm:"serializer:json"数据存入数据库时将该字段序列化未JSON字符串；从数据库读取时在反序化为Go对象
	Category    string         `json:"category" gorm:"size:32"`
	Status      string         `json:"status" gorm:"size:20;default:'pending';index"` // pending/approved/rejected/deleted  初始默认为待审核状态（pending）,index可以加快存储速度，因为他会自建一个表{pending:1,2,5    ;approved:3,4     ;..........}
	UserID      uint           `json:"user_id"`
	SaleClosed  bool           `json:"sale_closed" gorm:"default:false"` // 二期新增：卖家是否已关闭售卖。
}
