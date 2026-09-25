package models

type Favorite struct {
	ID     uint `gorm:"primarykey"`
	UserID uint `json:"user_id"  gorm:"uniqueIndex:idx_user_goods"`
	PostID uint `json:"post_id" gorm:"uniqueIndex:idx_user_goods"`
}
