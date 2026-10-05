package models

import "time"

// 一次"购买请求"就是一条 SellRequest 记录。
// 一个商品可以有很多条（多个买家同时抢），所以 request_id 必须是每条独立生成，
// 卖家在通知列表里点"同意"时，后端才知道他点的到底是哪个买家的请求。
type SellRequest struct {
	ID uint `json:"id" gorm:"primaryKey"`

	// 对外的请求编号，形如 req_10086。
	// uniqueIndex 是因为卖家同意/拒绝时前端回传的就是这个字符串，撞了就会答错人
	RequestID string `json:"request_id" gorm:"size:40;uniqueIndex"`

	GoodsID uint `json:"goods_id" gorm:"index"` // 被请求的商品
	BuyerID uint `json:"buyer_id" gorm:"index"` // 发起请求的买家
	// 冗余存一份买家账号，省得 /sell/notice 列表再 JOIN 一次 users 表
	BuyerAccount string `json:"account" gorm:"size:50"`

	// pending  等待卖家处理
	// approved 卖家已同意，可以来线下交易了
	// rejected 卖家已拒绝
	// closed   卖家直接关闭了这件商品的售卖，所有人都没机会了
	Status string `json:"status" gorm:"size:20;default:'pending';index"`

	// 线下交易地点，固定顺序 [经度, 纬度]。
	// serializer:json 让 GORM 把它当 JSON 字符串存进 MySQL，读出来自动变回 []float64。
	// 不用 gorm:"-" 是因为这个字段确实要落库（卖家同意后买家下次进来还得看得到）
	Position []float64 `json:"position,omitempty" gorm:"serializer:json"`

	// 卖家处理这条请求的时间，没处理就是 NULL，所以用指针
	HandledAt *time.Time `json:"handled_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
