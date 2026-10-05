package controllers

import (
	"math/rand"
	"time"

	"backend/models"
	"backend/utils"

	"github.com/gin-gonic/gin"
)

// 一个关键设计：position 挂在"请求"上而不是"商品"上。
// sellRequestPrefix 生成 request_id 用，格式对齐文档里的 req_10086
const sellRequestPrefix = "req_"

// genRequestID 生成一个对外可见的购买请求编号。
//
//	自增 ID 会泄露业务量（别人一看 req_5 就知道你才成交 5 单）
func genRequestID() string {
	const charset = "0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return sellRequestPrefix + string(b)
}

// 发起购买请求
//
//	请求体：{}（买家身份完全从 Token 里取）
func CreateSellRequest(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}

	goodsID, ok := pathID(c, "goods_id")
	if !ok {
		return
	}

	// 商品得存在，而且得是审核通过、还没被下架的状态
	var goods models.Goods
	if err := DB.First(&goods, goodsID).Error; err != nil {
		utils.Fail(c, 404, "商品不存在")
		return
	}
	if goods.Status != "approved" {
		utils.Fail(c, 400, "该商品当前不可购买")
		return
	}

	// 自己不能买自己的东西
	if goods.UserID == uid {
		utils.Fail(c, 400, "不能购买自己发布的商品")
		return
	}

	// 卖家已经关闭售卖
	if goods.SaleClosed {
		utils.Fail(c, 400, "卖家已关闭该商品的售卖")
		return
	}

	// 同一个买家对同一件商品只能有一条"进行中"的请求，防止手抖连点生成一堆
	var dup int64
	DB.Model(&models.SellRequest{}).
		Where("goods_id = ? AND buyer_id = ? AND status = ?", goodsID, uid, "pending").
		Count(&dup)
	if dup > 0 {
		utils.Fail(c, 400, "你已经发过请求了，请等待卖家处理")
		return
	}

	// 拿买家账号填进 BuyerAccount
	var buyer models.User
	if err := DB.First(&buyer, uid).Error; err != nil {
		utils.Fail(c, 500, "用户信息异常")
		return
	}

	req := models.SellRequest{
		RequestID:    genRequestID(),
		GoodsID:      goodsID,
		BuyerID:      uid,
		BuyerAccount: buyer.Account,
		Status:       "pending",
	}
	if err := DB.Create(&req).Error; err != nil {
		utils.Fail(c, 500, "发送失败，请重试")
		return
	}

	utils.Success(c, gin.H{"message": "购买请求已发送"})
}

// 卖家查看购买通知
//
//	返回当前登录用户（作为卖家）收到的、还没处理的请求
func ListSellNotices(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}

	// 先找出"属于我"的商品 ID 列表。
	// 用 Pluck 只取 id 一列，比把整个 Goods 结构体查出来轻很多
	var goodsIDs []uint
	if err := DB.Model(&models.Goods{}).
		Where("user_id = ?", uid).
		Pluck("id", &goodsIDs).Error; err != nil {
		utils.Fail(c, 500, "获取失败")
		return
	}

	// 没有发布过商品，通知列表就是空数组。
	// 这里必须返回 [] 而不是 null：前端拿到 null 去 .map() 会直接崩
	if len(goodsIDs) == 0 {
		utils.Success(c, []gin.H{})
		return
	}

	var requests []models.SellRequest
	if err := DB.Where("goods_id IN ? AND status = ?", goodsIDs, "pending").
		Order("created_at DESC").
		Find(&requests).Error; err != nil {
		utils.Fail(c, 500, "获取失败")
		return
	}

	// 响应形状手工拼一遍，避免把 buyer_id / 内部 id 这些字段泄给前端
	notices := make([]gin.H, 0, len(requests))
	for _, r := range requests {
		notices = append(notices, gin.H{
			"request_id": r.RequestID,
			"account":    r.BuyerAccount,
			"goods_id":   r.GoodsID,
		})
	}

	utils.Success(c, notices)
}

// 卖家同意 / 拒绝
//
//	{"status":"approve","position":[120.123456,30.123456]}
//	{"status":"reject"}
func ReplySellRequest(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}

	requestID := c.Param("request_id")
	if requestID == "" {
		utils.Fail(c, 400, "无效的请求编号")
		return
	}

	var body struct {
		Status   string    `binding:"required" json:"status"`
		Position []float64 `json:"position"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Fail(c, 400, "参数错误："+err.Error())
		return
	}

	if body.Status != "approve" && body.Status != "reject" {
		utils.Fail(c, 400, "status 只能是 approve 或 reject")
		return
	}

	// 同意就必须给交易地点，而且要 [经度, 纬度] 两个数
	if body.Status == "approve" && len(body.Position) != 2 {
		utils.Fail(c, 400, "同意交易时必须提供 position，格式为 [经度, 纬度]")
		return
	}

	// 经度范围 -180~180，纬度范围 -90~90。
	// 不校验的话，前端把经纬度传反了也能"成功"
	if len(body.Position) == 2 {
		lng, lat := body.Position[0], body.Position[1]
		if lng < -180 || lng > 180 || lat < -90 || lat > 90 {
			utils.Fail(c, 400, "position 超出合法经纬度范围")
			return
		}
	}

	var req models.SellRequest
	if err := DB.Where("request_id = ?", requestID).First(&req).Error; err != nil {
		utils.Fail(c, 404, "请求不存在")
		return
	}

	// 校验这条请求背后的商品是不是当前用户的，防止越权同意别人的单子
	var goods models.Goods
	if err := DB.First(&goods, req.GoodsID).Error; err != nil {
		utils.Fail(c, 404, "商品不存在")
		return
	}
	if goods.UserID != uid {
		utils.Fail(c, 403, "你无权处理该请求")
		return
	}

	// 已经处理过的不能再改，避免卖家反复横跳（也避免把 position 改来改去）
	if req.Status != "pending" {
		utils.Fail(c, 400, "该请求已处理过")
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"handled_at": &now,
		"status":     "approved",
	}
	if body.Status == "approve" {
		// 同意 A 买家的同时，把同一件商品其他还在 pending 的请求统统拒掉。
		// 一件二手货只有一个买家能买走，留着别的 pending 只会让卖家以为"我还没处理"
		DB.Model(&models.SellRequest{}).
			Where("goods_id = ? AND status = ? AND id <> ?", req.GoodsID, "pending", req.ID).
			Updates(map[string]interface{}{"status": "rejected", "handled_at": &now})
		updates["position"] = body.Position
	} else {
		updates["status"] = "rejected"
	}

	if err := DB.Model(&req).Updates(updates).Error; err != nil {
		utils.Fail(c, 500, "处理失败")
		return
	}

	utils.Success(c, gin.H{"message": "已处理"})
}

// 买家查看结果及交易地点
//
//	{"status":"approved","position":[120.123456,30.123456]}
func GetSellResult(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}

	goodsID, ok := pathID(c, "goods_id")
	if !ok {
		return
	}

	// 只查"我自己的"那条请求。
	// 同一商品可能有多个买家的请求，不按 buyer_id 过滤就会把别人的交易地点给出去
	var req models.SellRequest
	if err := DB.Where("goods_id = ? AND buyer_id = ?", goodsID, uid).
		Order("created_at DESC").
		First(&req).Error; err != nil {
		utils.Fail(c, 404, "你还没有对该商品发起购买请求")
		return
	}

	// closed 是个特例：卖家关闭时已经把全部请求刷成 closed 了，
	// 但按文档语义它应该优先于 rejected 告诉买家"这件不卖了"
	utils.Success(c, gin.H{
		"status":   req.Status,
		"position": req.Position,
	})
}

// 卖家关闭售卖
//
//	关闭后该商品不再接受新的购买请求
func CloseSell(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}

	goodsID, ok := pathID(c, "goods_id")
	if !ok {
		return
	}

	var goods models.Goods
	if err := DB.First(&goods, goodsID).Error; err != nil {
		utils.Fail(c, 404, "商品不存在")
		return
	}
	if goods.UserID != uid {
		utils.Fail(c, 403, "你无权关闭该商品的售卖")
		return
	}

	// 已经关闭过的没必要再关一次。
	if goods.SaleClosed {
		utils.Fail(c, 400, "该商品已经关闭售卖了")
		return
	}

	now := time.Now()

	// 已经有请求的：全部置为 closed。
	//    注意这里要连 approved 的一起关掉：卖家反悔了，买家那边也得看到最新状态
	DB.Model(&models.SellRequest{}).
		Where("goods_id = ?", goodsID).
		Updates(map[string]interface{}{"status": "closed", "handled_at": &now})

	// 商品本身也标记一下。
	//goods.status 是给管理员审核用的
	//（pending/approved/rejected/deleted），拿它来存"售卖状态"会把审核状态覆盖掉。
	// 所以额外加一个字段，两者互不干扰
	DB.Model(&goods).Update("sale_closed", true)

	utils.Success(c, gin.H{"message": "售卖已关闭"})
}

// 卖家查看自己商品的售卖概览（
func ListMySellRequests(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}

	var goodsIDs []uint
	if err := DB.Model(&models.Goods{}).
		Where("user_id = ?", uid).
		Pluck("id", &goodsIDs).Error; err != nil {
		utils.Fail(c, 500, "获取失败")
		return
	}
	if len(goodsIDs) == 0 {
		utils.Success(c, []gin.H{})
		return
	}

	var requests []models.SellRequest
	if err := DB.Where("goods_id IN ?", goodsIDs).
		Order("created_at DESC").
		Find(&requests).Error; err != nil {
		utils.Fail(c, 500, "获取失败")
		return
	}

	list := make([]gin.H, 0, len(requests))
	for _, r := range requests {
		list = append(list, gin.H{
			"request_id": r.RequestID,
			"account":    r.BuyerAccount,
			"goods_id":   r.GoodsID,
			"status":     r.Status,
			"position":   r.Position,
			"created_at": r.CreatedAt,
		})
	}
	utils.Success(c, list)
}
