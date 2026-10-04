package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// 这一层只负责"怎么把流式响应读出来"，工具循环编排在 runner.go 里，两边解耦。

// DeltaFunc 流式回调：模型每吐出一小段文字就调一次。
// 返回非 nil 的 error 表示"这边不要了"（通常是客户端断开），读取会立刻中止。
type DeltaFunc func(text string) error

// streamRequest 流式请求体。跟非流式比，多了 stream 和 stream_options 两个字段。
type streamRequest struct {
	Model         string                   `json:"model"`
	Messages      []Msg                    `json:"messages"`
	Tools         []map[string]interface{} `json:"tools,omitempty"`
	ToolChoice    string                   `json:"tool_choice,omitempty"`
	Stream        bool                     `json:"stream"`
	StreamOptions map[string]bool          `json:"stream_options,omitempty"`
}

// streamChunk 流式响应里的每一块（对应一行 data:）。
// 跟非流式的区别：内容藏在 choices[0].delta 里，而不是 choices[0].message。
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	// usage 默认不返回，需要在请求里加 stream_options.include_usage 才有
	Usage *struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// streamChat 发一个流式请求，边读边回调。
//
// 返回拼装好的完整 Msg（流式是一块一块拼回来的，最后要还原成一条完整消息），
// 这样上层的工具循环就不需要区分"这次是不是流式来的"。
func streamChat(ctx context.Context, messages []Msg, withTools bool, onDelta DeltaFunc) (Msg, int, error) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return Msg{}, 0, fmt.Errorf("环境变量 DEEPSEEK_API_KEY 没设置")
	}

	reqBody := streamRequest{
		Model:    deepseekModel,
		Messages: messages,
		Stream:   true,
		StreamOptions: map[string]bool{
			"include_usage": true, // 不加这个就统计不到 token，钱花得不明不白
		},
	}
	if withTools {
		reqBody.Tools = ToolSchemas()
		reqBody.ToolChoice = "auto"
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Msg{}, 0, err
	}

	// 关键：用 WithContext，客户端一断开这里就读到 ctx 取消，连接立刻释放
	req, err := http.NewRequestWithContext(ctx, "POST", deepseekBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Msg{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream") // 关键：告诉 DeepSeek 我们要 SSE 流式响应

	// 这里刻意不设 Timeout！流式响应可能持续几十秒，
	// Timeout 会在正吐字的时候把连接掐断。超时交给 ctx 管。
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return Msg{}, 0, fmt.Errorf("请求 DeepSeek 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return Msg{}, 0, fmt.Errorf("DeepSeek 返回 HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var (
		content strings.Builder // 正文要一块一块拼起来，string.Builder 比 + 效率高
		calls   []ToolCall      // 工具调用同样是一块一块飘过来的
		byIndex = map[int]int{} // 流式用 index 区分第几个工具，要映射到切片的第几位
		tokens  int
	)

	scanner := bufio.NewScanner(resp.Body) //bufio.Scanner 是 Go 标准库里专门用来按行读的，SSE 流式响应就是一行一行发的
	// bufio.Scanner 默认单行上限 64K，模型的 arguments 可能很长，放大到 1M 保险
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	//Scan()读取下一段数据
	for scanner.Scan() {
		line := scanner.Text() //将读取到的字节切片转成字符串
		// SSE 规范：每一行都是"事件"，以 data: 开头的才是模型吐出的内容。
		// 其他行都是注释或者心跳，忽略掉。
		// 例如：
		// data: {"choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}
		// data: {"choices":[{"index":0,"delta":{"content":"你"},"finish_reason":null}]}
		// data: {"choices":[{"index":0,"delta":{"content":"好"},"finish_reason":null}]}
		// data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}
		// data: [DONE]

		// 只要 data: 开头的行。
		// 顺手滤掉空行和 ": xxx" 这类注释行（有些网关会发心跳注释）
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" { // 官方规定的结束标记
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return Msg{}, tokens, fmt.Errorf("解析流式片段失败: %w，原始内容: %s", err, payload)
		}
		if chunk.Error != nil {
			return Msg{}, tokens, fmt.Errorf("DeepSeek 报错: %s", chunk.Error.Message)
		}
		if chunk.Usage != nil {
			tokens = chunk.Usage.TotalTokens
		}
		// 最后一个片段只有 choices:[] 和 usage，没有正文，跳过
		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		if delta.Content != "" {
			content.WriteString(delta.Content)
			if onDelta != nil {
				if err := onDelta(delta.Content); err != nil {
					return Msg{}, tokens, err
				} //没关闭客户端就继续走
			} //AgentChatStream中的onDelta回调会把这段文字发给前端
		}

		// 工具调用是分块拼图的：先来 {index:0,id:"xxx"}，
		// 再来几十个 {index:0,function:{arguments:"{\\"key..."}}}，要按 index 累加
		for _, tc := range delta.ToolCalls {
			pos, ok := byIndex[tc.Index]
			if !ok {
				pos = len(calls)
				byIndex[tc.Index] = pos
				calls = append(calls, ToolCall{ID: tc.ID, Type: tc.Type})
			} //占座
			if tc.ID != "" {
				calls[pos].ID = tc.ID
			}
			if tc.Type != "" {
				calls[pos].Type = tc.Type
			} //填空,后续可能传来更完整的
			calls[pos].Function.Name += tc.Function.Name
			calls[pos].Function.Arguments += tc.Function.Arguments
		}
	}
	if err := scanner.Err(); err != nil {
		return Msg{}, tokens, fmt.Errorf("读取流失败: %w", err)
	}

	return Msg{
		Role:      "assistant",
		Content:   content.String(),
		ToolCalls: calls,
	}, tokens, nil
}

// AskStream 纯问答的流式版（不带工具）。
// 它是 llm.go 里 Ask 的流时对偶实现，用来做降级兜底。
func AskStream(ctx context.Context, messages []Message, onDelta DeltaFunc) (string, int, error) {
	msgs := make([]Msg, 0, len(messages))
	for _, m := range messages {
		msgs = append(msgs, Msg{Role: m.Role, Content: m.Content})
	}
	reply, tokens, err := streamChat(ctx, msgs, false, onDelta)
	if err != nil {
		return "", tokens, err
	}
	return reply.Content, tokens, nil
}

// askWithToolsStream 带工具的流式调用，供 runner.go 的工具循环使用
func askWithToolsStream(ctx context.Context, msgs []Msg, onDelta DeltaFunc) (Msg, int, error) {
	return streamChat(ctx, msgs, true, onDelta)
}
