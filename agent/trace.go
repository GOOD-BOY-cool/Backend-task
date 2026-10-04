package agent

// ToolTrace 一次工具调用的痕迹，用于前端展示 "助手刚刚查了什么" 和事后排查问题
type ToolTrace struct {
	Name   string `json:"name"`    // 工具名，例如 search_goods
	OK     bool   `json:"ok"`      // 这次调用是否成功
	CostMS int64  `json:"cost_ms"` // 耗时毫秒
	ErrMsg string `json:"err_msg"` // 失败原因，成功时为空
}
