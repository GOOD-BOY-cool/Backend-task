package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// maxToolRounds 最大的循环次数
// 模型有可能陷入"调工具，不满意，再调"的死循环，必须给个上限兜底，
// 防止一次对话就能把你账户里的钱烧光。
const maxToolRounds = 3

// Msg 支持工具调用的完整消息格式。
// llm.go 里的 Message 只有 role/content，这里多了三个字段
type Msg struct {
	Role       string     `json:"role"`                   // system / user / assistant / tool
	Content    string     `json:"content,omitempty"`      // 说的话；调工具时可能为空
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // 模型要求调用的工具
	ToolCallID string     `json:"tool_call_id,omitempty"` // 回传结果时对应哪个调用
}

// ToolCall 模型发起的一次工具调用
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`      // 工具名
		Arguments string `json:"arguments"` // 参数，注意是 JSON 字符串而不是对象,想在go里用它就需要反序列化
	} `json:"function"`
}

//ToolCall和Msg是模型和工具之间的通信格式，模型发起调用时会生成（返回）ToolCall，工具执行后会把结果放回Msg的Content中，并带上ToolCallID以对应调用。

type toolRequest struct {
	Model      string                   `json:"model"`
	Messages   []Msg                    `json:"messages"`
	Tools      []map[string]interface{} `json:"tools,omitempty"`
	ToolChoice string                   `json:"tool_choice,omitempty"`
	Stream     bool                     `json:"stream"`
}

type toolResponse struct {
	Choices []struct {
		Message Msg `json:"message"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// RunWithTools 带工具的完整对话入口。
// 参数 messages 是 llm.go 里那种普通消息（buildMessages 拼好的），
// 这里会转成带工具能力的格式，然后进入循环。
func RunWithTools(userID uint, messages []Message) (string, int, error) {
	msgs := make([]Msg, 0, len(messages)+4)
	for _, m := range messages {
		msgs = append(msgs, Msg{Role: m.Role, Content: m.Content})
	}

	totalTokens := 0
	ctx := context.Background() //制造一个空的上下文（不能被取消，无截止时间，没有值，永远不会出差错），后续可以传给工具调用，让它们支持超时和取消请求

	for round := 0; round < maxToolRounds; round++ {
		reply, tokens, err := askWithTools(msgs)
		totalTokens += tokens
		if err != nil {
			return "", totalTokens, err
		}

		// 把模型这一轮说的话（哪怕是"我要调工具"）也记进历史
		msgs = append(msgs, reply) //没调用工具所以直接把模型的回答放进历史里

		// 没要求调工具 ，这就是最终答案，收工
		if len(reply.ToolCalls) == 0 {
			if reply.Content == "" {
				return "", totalTokens, fmt.Errorf("模型返回了空内容")
			}
			return reply.Content, totalTokens, nil
		}

		// 模型要求调工具：逐个执行，把结果作为 role=tool 塞回去
		for _, call := range reply.ToolCalls {
			result, execErr := ExecTool(ctx, userID, call.Function.Name, call.Function.Arguments)
			if execErr != nil {
				// 关键设计：工具出错不要直接让整个请求失败，
				// 而是把错误信息当成"工具结果"喂回去，让模型自己向用户解释。
				// 这样一次工具超时，用户拿到的是"暂时查不到"，而不是 500。
				// 用 json.Marshal 而不是手拼字符串：错误信息里如果带引号或换行，
				// 手拼会拼出非法 JSON，模型那边直接解析失败
				b, _ := json.Marshal(map[string]string{"error": execErr.Error()})
				result = string(b)
			}
			//由于模型要调用工具，所以要对工具序列化进行存储。当不需要调用工具时，ToolCalls为空数组，模型直接返回结果。工具执行后，结果会被放回Msg的Content中，并带上ToolCallID以对应调用。
			msgs = append(msgs, Msg{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    result,
			})
		}
	}

	// 跑满轮次还没得出结论，给个体面的兜底，别把空白丢给用户
	return "这个问题我需要查的东西有点多，能说得再具体一点吗？", totalTokens, nil
}

// askWithTools 真正发请求的那一步
func askWithTools(messages []Msg) (Msg, int, error) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return Msg{}, 0, fmt.Errorf("环境变量 DEEPSEEK_API_KEY 没设置")
	}

	body, err := json.Marshal(toolRequest{
		Model:      deepseekModel,
		Messages:   messages,
		Tools:      ToolSchemas(),
		ToolChoice: "auto", // 让模型自己决定要不要用工具
		Stream:     false,
	})
	//发出了一个请求
	// 告诉模型：你可以用这些工具，自己决定要不要用。
	// 然后模型会返回一个消息，里面可能包含工具调用的请求（ToolCalls），也可能只是直接回答问题。
	if err != nil {
		return Msg{}, 0, err
	}

	req, err := http.NewRequest("POST", deepseekBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Msg{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Msg{}, 0, fmt.Errorf("请求 DeepSeek 失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Msg{}, 0, err
	}

	var parsed toolResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Msg{}, 0, fmt.Errorf("解析返回失败，原始内容: %s", string(raw))
	}
	if parsed.Error != nil {
		return Msg{}, 0, fmt.Errorf("DeepSeek 报错: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return Msg{}, 0, fmt.Errorf("DeepSeek 没有返回答案，原始内容: %s", string(raw))
	}

	return parsed.Choices[0].Message, parsed.Usage.TotalTokens, nil
}

//普通对话时用户直接输入的就是文本，
// 输入本身就是字符串，
// 当整个请求被外层json.Marshal时，
// 字符串会被自动加上引号，变成合法的JSON字符串。
// 工具调用时，返回的是结构化数据,模型看不懂go的结构体。所以要转换成JSON字符串。
