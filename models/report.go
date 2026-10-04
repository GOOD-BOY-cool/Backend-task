package models

import "time"

type Report struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"uniqueIndex:idx_user_goods"`
	PostID    uint      `json:"post_id" gorm:"uniqueIndex:idx_user_goods;index"` // 排序分里的子查询要用
	Reason    string    `json:"reason" gorm:"size:500"`
	Status    string    `json:"status" gorm:"size:20;default:'pending';index"`
	CreatedAt time.Time `json:"created_at"`
}
