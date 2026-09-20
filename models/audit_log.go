package models

import "time"

type AuditLog struct {
	ID        uint      `gorm:"primaryKey"`
	AdminID   uint      `json:"admin_id"`
	PostID    uint      `json:"post_id"`
	Action    string    `json:"action"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"created_at"`
}

//审计表（管理员）
