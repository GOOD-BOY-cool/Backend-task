package controllers

import (
	"backend/models"
	"backend/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

func FavoritePost(c *gin.Context) {
	postID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(c, 400, "帖子ID无效")
	}
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	var goods models.Goods
	if err := DB.First(&goods, postID).Error; err != nil {
		utils.Fail(c, 404, "帖子不存在")
		return

	}
	if goods.UserID == uid {
		utils.Fail(c, 400, "不能收藏自己的贴子")
		return
	}

	var existing models.Favorite
	if err := DB.Where("user_id = ? AND post_id =?", uid, postID).First(&existing).Error; err == nil {
		utils.Fail(c, 400, "已经收藏过了")
		return
	}

	DB.Create(&models.Favorite{
		UserID: uid,
		PostID: uint(postID),
	})
	utils.Success(c, "收藏成功")
}

func UnfavoritePost(c *gin.Context) {
	postID := c.Param("id")
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	result := DB.Where("user_id=? AND psot_id =?", uid, postID).Delete(models.Favorite{}) //Delete(models.Favorite{})指定对这个库进行删除
	if result.RowsAffected == 0 {
		utils.Fail(c, 400, "你还没收藏这个帖子")
		return
	} //result.RowsAffected代表受到影响的行数即被删掉的行数
	utils.Success(c, "已取消收藏")
}

func MyFavorites(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	var favorites []models.Favorite
	DB.Where("user_id=?", uid).Find(&favorites)

	if len(favorites) == 0 {
		utils.Success(c, []interface{}{}) //传一个空数组给前端
		return
	}

	postIDs := make([]uint, len(favorites))
	for i, f := range favorites {
		postIDs[i] = f.PostID
	}

	var goods []models.Goods
	DB.Where("id IN ? AND status =?", postIDs, "approved").Find(&goods)
	utils.Success(c, goods)

}
