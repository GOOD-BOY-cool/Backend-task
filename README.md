# 孤独市集 · 校园二手交易平台（后端）

面向校园场景的二手物品交易平台后端。支持商品发布、审核、收藏、举报、经验等级，
以及二期的**买卖撮合流程**（发起购买请求 → 卖家通知 → 同意/拒绝 → 线下交易地点 → 关闭售卖）。

---

## 技术栈

| 项目 | 选型 |
| --- | --- |
| 语言 | Go 1.27.1 |
| Web 框架 | Gin |
| ORM | GORM |
| 数据库 | MySQL |
| 鉴权 | JWT（`golang-jwt/jwt/v5`） |
| 密码存储 | bcrypt（`golang.org/x/crypto`） |
| AI 咨询 | DeepSeek API（SSE 流式，只读咨询） |

---

## 快速开始

```bash
# 1. 拉依赖
go mod download

# 2. 建库（库名见 config/config.go 的 DBName）
CREATE DATABASE campus_secondhand CHARACTER SET utf8mb4;

# 3. 按本机环境改 config/config.go（数据库连接、JWT 密钥）

# 4. 启动
go run main.go
```

服务默认监听 `:8080`。启动时会执行 `db.AutoMigrate`，自动建表 / 补字段，**不需要手写 SQL**。

表结构：`users`、`goods`、`favorites`、`reports`、`audit_logs`、`chat_sessions`、`chat_messages`、`sell_requests`。

---

## 目录结构

```
.
├── main.go            入口：连接数据库 → AutoMigrate → 注册路由 → 启动
├── config/            配置常量（数据库连接、JWT）
├── database/          全局 DB 句柄
├── middleware/        JWT 鉴权、管理员鉴权、CORS
├── models/            数据表模型
├── controllers/       HTTP 处理器
├── routes/            路由注册
├── services/          业务逻辑（经验/等级）
├── agent/             AI 咨询 Agent（DeepSeek + 工具调用 + SSE）
└── utils/             响应封装、密码工具
```

---

## 接口一览

所有接口前缀 `/api`，统一返回：

```json
{ "code": 200, "msg": "成功", "data": {} }
```

需要登录的接口在请求头带上：

```
Authorization: Bearer <token>
```

### 认证 `/api/auth`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/register` | 注册 |
| POST | `/login` | 登录，返回 token |
| GET | `/me` | 鉴权测试 |

### 商品 `/api/goods`

| 方法 | 路径 | 登录 | 说明 |
| --- | --- | :-: | --- |
| GET | `` | — | 列表，支持 `?keyword=&page=` |
| GET | `/ranked` | — | 排序总表（综合评分） |
| GET | `/:id` | — | 详情 |
| POST | `` | ✓ | 发布（有等级限制） |
| PUT | `/:id` | ✓ | 修改 |
| DELETE | `/:id` | ✓ | 下架 |
| POST | `/upload` | ✓ | 上传图片 |

### 收藏与举报 `/api/posts`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/:id/favorite` | 收藏 |
| DELETE | `/:id/favorite` | 取消收藏 |
| GET | `/my-favorite` | 我的收藏 |
| POST | `/:id/report` | 举报 |

### 管理员 `/api/admin`（需管理员角色）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/post/pending` | 待审核商品 |
| POST | `/posts/:id/audit` | 审核通过 / 驳回 |
| DELETE | `/posts/:id` | 删除商品 |
| GET | `/reports` | 举报列表 |
| POST | `/reports/:id/handle` | 处理举报 |

### 买卖流程 `/api/sell`（二期）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/request/:goods_id` | 买家发起购买请求 |
| GET | `/notice` | 卖家查看购买通知 |
| POST | `/notice/:request_id/reply` | 卖家同意 / 拒绝 |
| GET | `/result/:goods_id` | 买家查看结果与交易地点 |
| POST | `/close/:goods_id` | 卖家关闭售卖 |
| GET | `/mine` | 卖家查看自己商品的售卖概览 |
| GET | `/purchase` | 买家查看自己发出的购买请求 |

**流程**：买家发起请求 → 卖家在通知列表看到 → 同意（必须带线下交易地点）或拒绝 → 买家查看结果。

- `status` 取值：`pending` / `approved` / `rejected` / `closed`
- `position` 固定顺序 **[经度, 纬度]**，例如 `[120.123456, 30.123456]`
- 卖家关闭售卖后，该商品不再接受新的购买请求

### 签到与 AI 咨询

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/user/sign-in` | 每日签到（加经验） |
| POST | `/api/agent/chat` | AI 咨询（非流式） |
| POST | `/api/agent/chat/stream` | AI 咨询（SSE 流式） |
| GET | `/api/agent/sessions` | 会话列表 |

AI 咨询为**只读**助手：可以查询商品、用户等级等信息，不能代替用户下单或修改数据。

---

## 数据模型要点

- **User**：账号、密码哈希、QQ、邮箱、等级、经验、连续签到天数
- **Goods**：标题、描述、价格、图片、分类、`status`（管理员审核状态）、
  `sale_closed`（卖家是否关闭售卖）
- **SellRequest**：`request_id`（对外编号，形如 `req_xxxxxxxxxxxx`）、买家、商品、
  `status`、交易地点 `position`

> `Goods.status` 是**管理员审核状态**，`Goods.sale_closed` 是**卖家售卖状态**，
> 两者是正交的维度，不能互相替代。

---

## 注意事项

- 图片上传后通过 `/uploads/xxx` 静态路径访问，目录为 `./uploads`
- `config/config.go` 目前是**硬编码常量**，直接在文件里改；改完不要把真实密钥提交到仓库
- 建议补一个 `.gitignore`，至少忽略 `.env`、`uploads/`、`*.log`

---

## License

本项目仅用于学习交流。
