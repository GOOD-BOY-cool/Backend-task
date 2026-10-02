package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// 这一层只干一件事：把对话历史发给 DeepSeek，把回答拿回来。
// DeepSeek 的接口地址与模型名。
const (
	deepseekBaseURL = "https://api.deepseek.com"
	deepseekModel   = "deepseek-v4-flash"
)

// Message 一条对话消息。字段名必须叫 role / content，
// 因为这是 DeepSeek（以及所有 OpenAI 兼容接口）规定好的格式。
type Message struct {
	Role    string `json:"role"`    //system=设定角色 / user=用户说的 / assistant=AI说的
	Content string `json:"content"` //具体内容
}

// chatRequest 发给 DeepSeek 的请求体
type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"` //false=一次性返回完整答案，true=一个字一个字吐
}

// chatResponse DeepSeek 返回的内容很多，所以只挑需要的字段解析，
// 其余的 Go 会自动忽略。这样做的好处：DeepSeek 后续加字段不会影响我们，解析器也不会报错。
type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"` //AI 的回答在这里
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"` //这次花了多少 token，用来计费和限流
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Ask 调用 DeepSeek。
// 参数 messages 是完整对话历史（含 system 设定和之前几轮）；
// 返回 AI 回答内容、消耗的 token 数、可能出错的错误。
func Ask(messages []Message) (string, int, error) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return "", 0, fmt.Errorf("环境变量 DEEPSEEK_API_KEY 没设置")
	}

	//把请求体转成 JSON
	body, err := json.Marshal(chatRequest{
		Model:    deepseekModel,
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		return "", 0, err
	}

	//构造 HTTP 请求
	req, err := http.NewRequest("POST", deepseekBaseURL+"/chat/completions", bytes.NewReader(body)) //[]byte到io.Reader的转换让内存里的东西无缝接入所有流式处理的API。/chat/completions是现在流行AI接口的标准路径，DeepSeek也遵循这个标准
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	//发出去时超时必须设，否则网络卡住会把整个服务拖死
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req) //建立TCP连接，发送HTTP请求，等待响应，把响应头读到内存里，返回一个http.Response对象
	if err != nil {
		return "", 0, fmt.Errorf("请求 DeepSeek 失败: %w", err)
	}
	defer resp.Body.Close() //response.Body是一个io.ReadCloser，必须在用完后关闭，否则会泄漏资源。defer保证函数退出时关闭  io.Readcloser能读取数据且用完必须关

	//读回原始字节。出错时把它打印出来，排查问题全靠这一步
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}

	//解析成结构体
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", 0, fmt.Errorf("解析返回失败，原始内容: %s", string(raw)) //原始字节转换为人类可读的字符串
	}

	//三种失败情况逐个挡掉，别让脏数据流到下一步
	if parsed.Error != nil {
		return "", 0, fmt.Errorf("DeepSeek 返回错误: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", 0, fmt.Errorf("DeepSeek 没有返回答案，原始内容: %s", string(raw))
	}

	//成功：掏出回答正文
	return parsed.Choices[0].Message.Content, parsed.Usage.TotalTokens, nil
	//因为choices是一个数组(里面有很多返回的请况)，取第一个元素的Message字段的Content作为回答正文返回，同时返回消耗的总token数和nil表示没有错误
}
