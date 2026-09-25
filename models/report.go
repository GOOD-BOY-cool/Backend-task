package models

import "time"

type Report struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `json:"user_id" `
	PostID    uint      `json:"post_id" `
	Reason    string    `json:"reason"`
	Status    string    `json:"status" gorm:"default:pending"`
	CreatedAt time.Time `json:"created_at"`
}

//后期可以考虑联合唯一索引从数据库物理层面防止高并发请况
