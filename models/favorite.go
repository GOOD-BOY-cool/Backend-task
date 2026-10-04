package models

type Favorite struct {
	ID     uint `json:"id" gorm:"primaryKey"`
	UserID uint `json:"user_id" gorm:"uniqueIndex:idx_user_goods"`
	PostID uint `json:"post_id" gorm:"uniqueIndex:idx_user_goods;index"` // 同上
}
