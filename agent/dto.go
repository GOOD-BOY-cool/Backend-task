package agent

// 发消息请求
type ChatRequest struct {
	SessionID   uint   `json:"session_id"` // 为 0 表示自动新建
	Message     string `json:"message" binding:"required,max=1000"`
	ClientMsgID string `json:"client_msg_id" binding:"required,max=64"` // 幂等键，前端 UUID
}

// 非流式响应（SSE 关闭时使用，遵循现有的 {code,msg,data}）
type ChatResponse struct {
	SessionID uint        `json:"session_id"`
	Reply     string      `json:"reply"`
	Guide     *GuideCard  `json:"guide,omitempty"` // 操作引导卡片,omitempty以为意味着如果这个字段为空打包JSON时会忽略它
	Trace     []ToolTrace `json:"trace,omitempty"` // 本次用到过的工具，便于前端展示"已查询"
}

// 操作引导卡片：Agent 不写库，交给前端跳表单
type GuideCard struct {
	Action  string            `json:"action"`  // create_goods / report_post / sign_in ...
	Target  string            `json:"target"`  // 跳转路径
	Fields  map[string]string `json:"fields"`  // 预填字段
	Confirm string            `json:"confirm"` // 提示文案："请确认后点击发布"
}
