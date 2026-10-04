package agent

import (
	"backend/database"
	"backend/models"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// 定义是内部程序看的,而toolschemas是给模型看的
// ToolSpec 一个工具的定义
type ToolSpec struct {
	Name        string                 // 工具名，模型靠它来调用
	Description string                 // 用自然语言告诉模型"什么时候该用我"，写得越准模型越不容易选错
	Parameters  map[string]interface{} // 参数格式，用 JSON Schema 描述		AI界的普通话
	Handler     func(ctx context.Context, userID uint, args map[string]any) (string, error)
}

// toolRegistry 工具注册表：工具定义
var toolRegistry = map[string]ToolSpec{}

// init() 函数在包初始化时执行，注册工具，比main函数早执行
func init() {
	toolRegistry["search_goods"] = ToolSpec{
		Name:        "search_goods",
		Description: "按关键词搜索在售的二手商品，返回标题、价格、分类。用户问某样东西有没有人卖、大概多少钱时用这个。",
		Parameters: map[string]interface{}{
			"type": "object", //告诉AI这是一个对象
			"properties": map[string]interface{}{
				"keyword": map[string]interface{}{
					"type":        "string",
					"description": "搜索关键词，例如 自行车、教材、台灯",
				},
			},
			"required": []string{"keyword"},
		},
		Handler: searchGoods,
	}

	toolRegistry["get_my_profile"] = ToolSpec{
		Name:        "get_my_profile",
		Description: "查询当前登录用户的等级、经验值和连续签到天数。用户问自己的等级、还差多少升级时用这个。",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: getMyProfile,
	}
}

//handler帮助ai获取数据库的内容

// 工具实现
// searchGoods 搜索在售商品
func searchGoods(ctx context.Context, userID uint, args map[string]any) (string, error) {
	keyword, _ := args["keyword"].(string)
	keyword = strings.TrimSpace(keyword) //去空格
	if keyword == "" {
		return "", fmt.Errorf("keyword 不能为空")
	}

	var list []models.Goods
	err := database.DB.WithContext(ctx).
		//WithContext使取消等信号可以传到数据库层面
		// 只能查已过审的商品，未过审的不该出现在助手的视野里
		Where("status = ? AND (title LIKE ? OR description LIKE ?)", "approved", "%"+keyword+"%", "%"+keyword+"%").
		// 字段白名单：只取这几列，别把整个商品对象塞给模型（省 token，也防泄露）
		Select("id", "title", "price", "category", "created_at").
		Order("created_at DESC").
		Limit(5).
		Find(&list).Error //WithContext将上下文传给数据库操作，支持超时和取消请求
	if err != nil {
		return "", err
	}

	if len(list) == 0 {
		// 没结果也要明确告诉模型"确实没有"，而不是返回空数组让它自己瞎猜
		return fmt.Sprintf(`{"count":0,"message":"目前没有找到与「%s」相关的在售商品"}`, keyword), nil
	}

	data, _ := json.Marshal(list) //把结果转成 JSON 字符串，返回给模型,结构为[]byte
	return string(data), nil
}

// getMyProfile 查询当前用户等级信息
func getMyProfile(ctx context.Context, userID uint, args map[string]any) (string, error) {
	var u models.User
	// userID 来自函数参数（JWT 里的），不是模型传的 —— 这是防越权的关键
	if err := database.DB.WithContext(ctx).
		Select("id", "level", "exp", "sign_in_streak").
		First(&u, userID).Error; err != nil {
		return "", err
	}

	// 升级规则跟 services/exp.go 保持一致：等级 = 经验/100 + 1
	gap := u.Level*100 - u.Exp
	if gap < 0 {
		gap = 0
	}

	data, _ := json.Marshal(map[string]interface{}{
		"level":          u.Level,
		"exp":            u.Exp,
		"exp_to_upgrade": gap,
		"sign_in_streak": u.SignInStreak,
	})
	return string(data), nil
}

//给模型和执行用的接口

// ToolSchemas 把注册表转成模型能看懂的 tools 数组，OpenAI 协议里具体的工具细节必须放在function键里面
func ToolSchemas() []map[string]interface{} {
	list := make([]map[string]interface{}, 0, len(toolRegistry))
	for _, t := range toolRegistry {
		list = append(list, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			},
		})
	}
	return list
}

// ExecTool 执行一个工具调用。
// name 是模型给的工具名，argsJSON 是模型给的参数（JSON 字符串）。
func ExecTool(ctx context.Context, userID uint, name string, argsJSON string) (string, error) {
	spec, ok := toolRegistry[name]
	if !ok {
		return "", fmt.Errorf("没有叫 %s 的工具,可用的有:search_goods、get_my_profile", name)
	}

	var args map[string]any
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("工具参数不是合法 JSON: %w", err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}

	// 保险丝：万一模型自作主张传了 user_id，直接丢掉，永远用 JWT 里的那个
	delete(args, "user_id")

	return spec.Handler(ctx, userID, args)
}
