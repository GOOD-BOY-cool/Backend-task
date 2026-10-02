package controllers

import (
	"backend/agent"
	"backend/models"
	"backend/utils"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// 控制器只干三件事：收参数、取当前用户、把结果包成 JSON 返回

// 控制器里每个函数都要重复做的一步，抽出来省得漏掉
func currentUserID(c *gin.Context) (uint, bool) {
	v, exists := c.Get("user_id")
	if !exists {
		utils.Fail(c, http.StatusUnauthorized, "未登录")
		return 0, false
	}
	uid, ok := v.(uint)
	if !ok {
		utils.Fail(c, http.StatusUnauthorized, "登录信息异常")
		return 0, false
	}
	return uid, true
}

// 把 URL 里的字符串 ID 转成数字，顺手处理"传了个 abc 过来"的情况
func pathID(c *gin.Context, key string) (uint, bool) {
	id64, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "ID 无效")
		return 0, false
	}
	return uint(id64), true
}

// AgentChat 发消息并拿到助手回答
//
//	请求：{"session_id":0,"message":"怎么发布商品？","client_msg_id":"前端生成的唯一ID"}
//	说明：session_id 传 0 会自动新建会话
func AgentChat(c *gin.Context) {
	var req agent.ChatRequest

	// binding 标签会自动校验：message 非空且不超过 1000 字，client_msg_id 必填
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "参数错误："+err.Error())
		return
	}

	// user_id 来自 JWT 中间件，前端传什么都不影响，改不了
	uid, ok := currentUserID(c)
	if !ok {
		return
	}

	resp, err := agent.Chat(uid, req)
	if err != nil {
		// 详细错误留在服务端日志，别把内部细节（比如数据库报错）甩给前端
		c.Error(err)
		utils.Fail(c, http.StatusInternalServerError, "助手暂时无法回答，请稍后再试")
		return
	}

	utils.Success(c, resp)
}

// CreateChatSession 新建一个空会话
//
//	请求：{"title":"咨询发布流程"}   title 可省略
func CreateChatSession(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}

	var body struct {
		Title string `json:"title"`
	}
	c.ShouldBindJSON(&body) // 这里允许没传 body，所以用 Should 而不是 Must

	session, err := agent.CreateSession(uid, body.Title)
	if err != nil {
		c.Error(err)
		utils.Fail(c, http.StatusInternalServerError, "创建会话失败")
		return
	}

	utils.Success(c, session)
}

// ListChatSessions 我的会话列表
func ListChatSessions(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}

	list, err := agent.ListSessions(uid, 20)
	if err != nil {
		c.Error(err)
		utils.Fail(c, http.StatusInternalServerError, "获取会话列表失败")
		return
	}

	// 没数据时返回空数组而不是 null，否则前端要写额外的判空逻辑
	if list == nil {
		list = []models.ChatSession{}
	}
	utils.Success(c, list)
}

// ListChatMessages 某个会话的历史消息，刷新页面时用
func ListChatMessages(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	sessionID, ok := pathID(c, "id")
	if !ok {
		return
	}

	list, err := agent.ListHistory(uid, sessionID)
	if err != nil {
		// 查不到时统一说"不存在"：既可能是真没有，也可能是别人的会话，
		// 返回同一种措辞可以避免别人试探哪些 ID 存在
		utils.Fail(c, http.StatusNotFound, "会话不存在")
		return
	}

	if list == nil {
		list = []models.ChatMessage{}
	}
	utils.Success(c, list)
}

// ArchiveChatSession 归档会话（不真删除）
func ArchiveChatSession(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	sessionID, ok := pathID(c, "id")
	if !ok {
		return
	}

	if err := agent.ArchiveSession(uid, sessionID); err != nil {
		if errors.Is(err, agent.ErrNotFound) {
			utils.Fail(c, http.StatusNotFound, "会话不存在")
			return
		}
		// errors.Is(err, agent.ErrNotFound)，查找错误链里有没有 ErrNotFound，找到了就返回 404，否则就是数据库报错，返回 500
		c.Error(err)
		utils.Fail(c, http.StatusInternalServerError, "归档失败")
		return
	}

	utils.Success(c, nil)
}
