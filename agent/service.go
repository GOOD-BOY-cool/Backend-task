package agent

import (
	"backend/database"
	"backend/models"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// ErrNotFound 会话不存在
// 定义成一个"哨兵错误"，之后可用errors.Is(err, agent.ErrNotFound) 判断，
// 而不是去比对错误字符串（字符串一改就全崩，很脆弱）
var ErrNotFound = errors.New("会话不存在")

// SystemPrompt 系统提示词，用来给助手定人设、划边界
// 这一句会永远出现在每次请求的 messages 最前面
const SystemPrompt = `你是"校园二手交易市场"的智能助手，服务于在校大学生之间的二手物品交易。

你的职责：
- 解答平台规则：怎么发布商品、审核要多久、等级经验怎么算、怎么举报违规
- 给出交易建议：定价参考、当面交易注意事项、如何识别虚假商品
- 引导用户操作：告诉用户该功能在哪个入口，而不是替他们操作

你的边界（必须遵守）：
- 绝不索要、猜测或提供任何人的手机号、QQ、微信号、宿舍地址
- 不承诺平台担保、先行赔付或退款
- 不代替用户发布、修改、删除任何内容
- 不知道就说不知道，不要编造平台规则
- 回答简洁，控制在 200 字以内，能用分点就用分点`

const (
	// historyLimit 最多带最近多少条历史消息给模型
	// 带得越多越聪明，但花钱越多、也越容易被塞爆上下文
	// 10 条大约等于最近 5 轮对话，够用且便宜
	historyLimit = 10

	titleMaxLen = 20 // 会话标题取用户首句话的前多少个字
)

// Chat 处理一次完整对话，等模型全部生成完才一次性返回。
// userID 来自 JWT 中间件，不是前端传的这样用户就伪造不了身份。
func Chat(userID uint, req ChatRequest) (*ChatResponse, error) {
	return runChat(context.Background(), userID, req, nil)
}

// ChatStream 同上，但模型吐出的每个字都通过 onDelta 回调推出去，
// 前端能做成"打字机"效果。onDelta 为 nil 时等价于 Chat。
// ctx 由 HTTP 请求带过来：用户关掉页面，ctx 立刻取消，请求也就停了。
func ChatStream(ctx context.Context, userID uint, req ChatRequest, onDelta DeltaFunc) (*ChatResponse, error) {
	return runChat(ctx, userID, req, onDelta)
}

// runChat 流式与非流式共用的那一段流程。
// 差别只有最后"怎么调模型"这一步，所以收在一个函数里，避免改一处忘另一处。
func runChat(ctx context.Context, userID uint, req ChatRequest, onDelta DeltaFunc) (*ChatResponse, error) {
	// 网络卡顿时前端会重发同一条消息，如果不管，模型就会被重复调用两次（花两次钱），
	// 用户也会看到两条一样的回答。用 client_msg_id 挡住。
	//所以用以下函数防止幂等
	if old, found := findDuplicate(userID, req.ClientMsgID); found {
		return old, nil
	}
	//获取会话
	session, err := ensureSession(userID, req.SessionID, req.Message)
	if err != nil {
		return nil, err
	}

	// 把用户消息存下来
	// 顺序很重要：必须先存再调模型。万一模型那边超时崩了，
	// 用户刷新页面还能看到自己说过的话，而不是凭空消失
	userMsg := models.ChatMessage{
		SessionID:   session.ID,
		UserID:      userID,
		Role:        "user",
		Content:     req.Message,
		Status:      "done",
		ClientMsgID: &req.ClientMsgID, // 只有用户消息才有幂等键，助手消息留 NULL
	}
	if err := database.DB.Create(&userMsg).Error; err != nil {
		return nil, err
	}

	// 拼装要给模型的完整对话历史
	messages, err := buildMessages(session, req.Message)
	if err != nil {
		return nil, err
	}

	// 调模型LLM，拿到回答、工具痕迹和 token 消耗量
	// 用 RunWithTools 而不是 Ask：前者会把工具清单一起发给模型，
	// 模型需要查数据时能自己发起调用（search_goods / get_my_profile）
	var (
		replyText string
		trace     []ToolTrace
		tokens    int
	)
	//err 在这个函数前面已经声明过了，再声明一次会报 redeclared

	//if 的大括号是一个内层作用域，在它里面声明的变量，出了右花括号就销毁。
	// return 在函数层级，看不到它们。
	//所以必须把变量声明在外层，然后在分支里只赋值：

	if onDelta != nil {
		// 注意是 = 不是 :=
		replyText, trace, tokens, err = RunWithToolsStream(ctx, userID, messages, onDelta)
		if errors.Is(err, ErrToolUnsupported) {
			// 带工具这版被拒了，退回流式纯问答：查不了商品，但聊天不受影响
			replyText, tokens, err = AskStream(ctx, messages, onDelta)
		}
	} else {
		replyText, trace, tokens, err = RunWithTools(ctx, userID, messages)
		if errors.Is(err, ErrToolUnsupported) {
			// 同上，非流式的兜底通道就是 llm.go 里的 Ask
			replyText, tokens, err = Ask(messages)
		}
	}
	if err != nil {
		// 把失败也记下来，方便事后排查，同时不让脏数据混进上下文
		database.DB.Create(&models.ChatMessage{
			SessionID: session.ID,
			UserID:    userID,
			Role:      "assistant",
			Content:   "",
			Status:    "failed",
			ErrMsg:    truncate(err.Error(), 500),
		})
		return nil, err
	}

	// 把助手回答也存下来
	assistantMsg := models.ChatMessage{
		SessionID: session.ID,
		UserID:    userID,
		Role:      "assistant",
		Content:   replyText,
		Status:    "done",
	}
	if err := database.DB.Create(&assistantMsg).Error; err != nil {
		return nil, err
	}

	// 更新会话的统计信息
	// 用 gorm.Expr 让数据库自己累加（msg_count + 2），2包含了用户提问和助手回答两条消息，
	// 而不是算好再写进去从而避免两个人同时聊天时互相覆盖
	database.DB.Model(&models.ChatSession{}).
		Where("id = ?", session.ID).
		Updates(map[string]interface{}{
			"msg_count":   gorm.Expr("msg_count + ?", 2),
			"token_used":  gorm.Expr("token_used + ?", tokens),
			"last_msg_at": time.Now(),
		})

	return &ChatResponse{
		SessionID: session.ID,
		Reply:     replyText,
		Trace:     trace,
	}, nil
}

// ListHistory 返回某个会话的历史消息，前端刷新时用得上
func ListHistory(userID, sessionID uint) ([]models.ChatMessage, error) {
	var session models.ChatSession
	if err := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		return nil, ErrNotFound
	}
	var list []models.ChatMessage
	err := database.DB.Where("session_id = ? AND role != ?", sessionID, "system").
		Order("id ASC").Limit(200).Find(&list).Error
	return list, err
}

// CreateSession 手动新建一个空会话（前端"新对话"按钮会用到）
func CreateSession(userID uint, title string) (models.ChatSession, error) {
	if title == "" {
		title = "新会话"
	}
	s := models.ChatSession{
		UserID:    userID,
		Title:     truncate(title, 50),
		Status:    "active",
		LastMsgAt: time.Now(),
	}
	err := database.DB.Create(&s).Error
	return s, err
}

// ListSessions 我的会话列表，最近聊过的排最前面
func ListSessions(userID uint, limit int) ([]models.ChatSession, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var list []models.ChatSession
	err := database.DB.
		Where("user_id = ? AND status = ?", userID, "active").
		Order("last_msg_at DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}

// ArchiveSession 归档会话。
// 这里是"归档"不是"删除"：数据全留着，只是列表里不显示了
// 这样将来要查历史、要恢复，都还有救
func ArchiveSession(userID, sessionID uint) error {
	res := database.DB.Model(&models.ChatSession{}).
		Where("id = ? AND user_id = ?", sessionID, userID).
		Update("status", "archived")
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

//  以下为这次调用的内部函数

// ensureSession 找到会话就返回，找不到（或 sessionID 为 0）就新建一个。
// 查询条件里带 user_id，限制了只能查自己的会话，避免别人试探别人的会话 ID
func ensureSession(userID, sessionID uint, firstMessage string) (models.ChatSession, error) {
	if sessionID > 0 {
		var s models.ChatSession
		err := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).First(&s).Error
		if err == nil {
			return s, nil
		}
		// 查不到就往下走，当作新会话处理
	}

	title := firstMessage
	if len([]rune(title)) > titleMaxLen {
		title = string([]rune(title)[:titleMaxLen]) + "..."
	} //rune()是为了支持中文字符，避免截断时出现乱码,一个字符不管是中文还是英文都算一个rune

	s := models.ChatSession{
		UserID:    userID,
		Title:     title,
		Status:    "active",
		MsgCount:  0,
		LastMsgAt: time.Now(),
	}
	err := database.DB.Create(&s).Error
	return s, err
}

// buildMessages 拼装完整的 messages
//
//	system（人设） + 历史对话（最近的若干条） + 当前提问
func buildMessages(session models.ChatSession, userMessage string) ([]Message, error) {
	msgs := []Message{{Role: "system", Content: SystemPrompt}}

	var history []models.ChatMessage
	err := database.DB.
		Where("session_id = ? AND role IN ? AND status = ?", session.ID, []string{"user", "assistant"}, "done").
		Order("id DESC"). // 从新到旧取
		Limit(historyLimit).
		Find(&history).Error
	if err != nil {
		return nil, err
	}

	// 数据库里是新到旧，喂给模型必须旧到新，所以倒过来遍历
	for i := len(history) - 1; i >= 0; i-- {
		msgs = append(msgs, Message{
			Role:    history[i].Role,
			Content: history[i].Content,
		})
	}

	msgs = append(msgs, Message{Role: "user", Content: userMessage})
	return msgs, nil
}

// findDuplicate 检查这条消息是不是已经发过了
// 命中就把上次的回答原样返回，不重复花钱调模型
func findDuplicate(userID uint, clientMsgID string) (*ChatResponse, bool) {
	if clientMsgID == "" {
		return nil, false
	}

	var old models.ChatMessage
	err := database.DB.
		Where("user_id = ? AND client_msg_id = ? AND role = ?", userID, clientMsgID, "user").
		First(&old).Error
	if err != nil {
		return nil, false // 没找到，正常继续
	}

	// 找到这条用户消息之后的第一条助手回答
	var reply models.ChatMessage
	if err := database.DB.
		Where("session_id = ? AND role = ? AND id > ?", old.SessionID, "assistant", old.ID).
		Order("id ASC").First(&reply).Error; err != nil {
		return nil, false // 上次可能失败了，这次重新来一遍
	}

	return &ChatResponse{SessionID: old.SessionID, Reply: reply.Content}, true
}

// truncate 截断字符串，支持中文字符
func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
} //rune()是为了支持中文字符，避免截断时出现乱码,一个字符不管是中文还是英文都算一个rune
