package models

type Favorite struct {
	ID     uint `gorm:"primarykey"`
	UserID uint `json:"user_id"`
	PostID uint `json:"post_id"`
}
