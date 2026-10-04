package controllers

import (
	"backend/agent"
	"backend/models"
	"backend/utils"
	"encoding/json"
	"errors"
	"fmt"
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

// AgentChatStream 发消息，回答一个字一个字地流回来（SSE）
//
//	响应不是 JSON，而是 SSE 事件流：
//	  event: start  data: {"session_id":0}          已开始处理
//	  event: delta  data: {"text":"你"}               每次一小段正文，重复多次
//	  event: done   data: {"session_id":12,"trace":[...]} 结束，带工具痕迹
//	  event: error  data: {"message":"助手暂时无法回答，请稍后再试"}
//
//	前端用 EventSource 只能发 GET，要 POST 得用 fetch + ReadableStream，
//	或者直接用 axios 的 onDownloadProgress。实在嫌麻烦可以先用 /chat 的非流式版本。
func AgentChatStream(c *gin.Context) {
	var req agent.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "参数错误："+err.Error())
		return
	}

	uid, ok := currentUserID(c)
	if !ok {
		return
	}

	// SSE 三件套响应头。
	// X-Accel-Buffering: no 是给 nginx 看的，不然它会把响应攒着一起发，
	// 流式就退化成"等全部完成才一次性显示"。
	c.Header("Content-Type", "text/event-stream") //告诉浏览器这是 SSE 流式响应
	c.Header("Cache-Control", "no-cache")         //告诉浏览器不要缓存响应
	c.Header("Connection", "keep-alive")          //告诉浏览器保持连接，别断开
	c.Header("X-Accel-Buffering", "no")           //告诉 nginx 不要缓冲响应，直接往前端发

	w := c.Writer
	flusher, ok := w.(http.Flusher) //因为Go的http.ResponseWriter存在缓冲区BUFFER，flusher后，不管有多少数据立刻发给客户端
	if !ok {
		utils.Fail(c, http.StatusInternalServerError, "当前连接不支持流式输出")
		return
	}

	// 每发一个事件都要立刻 Flush，否则内容会卡在缓冲区里不出来
	emit := func(event string, data interface{}) {
		b, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(b))
		flusher.Flush()
	}

	reqCtx := c.Request.Context() //直接拿浏览器的请求上下文，浏览器断开连接时这个 ctx 会被取消，后续的工具调用就能及时停掉，省下 token
	emit("start", gin.H{"session_id": req.SessionID})

	resp, err := agent.ChatStream(reqCtx, uid, req, func(text string) error {
		//匿名函数回调了stream中的onDelta函数，onDelta函数里又回调了这个匿名函数
		// 这个匿名函数里又调用了emit函数把text发给前端
		// 这个匿名函数里还要检查浏览器有没有断开连接，如果断开就返回错误让上层停掉
		// 用户关掉页面 / 按了停止，请求上下文会被取消。
		// 这时候返回一个错误，让上层别再往下生成了，省下的 token 是真金白银
		if reqCtx.Err() != nil {
			return reqCtx.Err()
		}
		emit("delta", gin.H{"text": text})
		return nil
	})

	if err != nil {
		// 已经开始往外面写响应了，不能再走 utils.Fail（那会重写响应头报 http: superfluous 错误），
		// 只能用一个 error 事件告诉前端
		c.Error(err)
		emit("error", gin.H{"message": "助手暂时无法回答，请稍后再试"})
		return
	}

	emit("done", gin.H{"session_id": resp.SessionID, "trace": resp.Trace})
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
