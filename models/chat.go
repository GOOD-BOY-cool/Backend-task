package models

import "time"

type ChatSession struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `json:"user_id" gorm:"index"`
	Title        string    `json:"title" gorm:"size:100"`
	Status       string    `json:"status" gorm:"size:20;default:'active'"`
	Summary      string    `json:"summary" gorm:"type:text"`
	SummarizedID uint      `json:"summarize_id"`
	MsgCount     int       `json:"msg_count" gorm:"default:0"`
	TokenUsed    int       `json:"token_used" gorm:"default:0"`
	LastMsgAt    time.Time `json:"last_msg_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ChatMessage struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	SessionID   uint      `json:"session_id" gorm:"index"`
	UserID      uint      `json:"user_id" gorm:"index"`
	Role        uint      `json:"role" gorm:"size:20"`
	Content     string    `json:"content" gorm:"type:text"`
	ToolName    string    `json:"tool_name" gorm:"size:50"`
	ToolArgs    string    `json:"tool_args" gorm:"type:text"` //工具参数通常是JSON
	Status      string    `json:"status" gorm:"size:20;default:'done'"`
	ErrMsg      string    `json:"err_msg" gorm:"size:500"`
	ClientMsgID string    `json:"client_msg_id" gorm:"uinqueIndex:idx_client_msg;size:64"`
	CreatedaAt  time.Time `json:"created_at"`
}
