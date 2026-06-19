<p align="center">
  <img src="web/ui/public/bots-nest2.jpeg" alt="Bots Nest" width="200" />
</p>

<h1 align="center">Bots Nest</h1>

<p align="center">
  <strong>企业微信 LLM 机器人平台</strong> —— 自托管、多机器人、可扩展的 AI 对话引擎
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go" />
  <img src="https://img.shields.io/badge/React-19-61DAFB?logo=react" />
  <img src="https://img.shields.io/badge/SQLite-WAL-003B57?logo=sqlite" />
  <img src="https://img.shields.io/badge/license-AGPL%20v3-blue" />
  <a href="./COMMERCIAL_LICENSE.md"><img src="https://img.shields.io/badge/commercial-license-green" /></a>
</p>

---

## 目录

- [项目介绍](#项目介绍)
- [商业授权](#商业授权)
- [核心功能](#核心功能)
- [技术栈](#技术栈)
- [快速开始](#快速开始)
- [配置说明](#配置说明)
- [后续计划](#后续计划)
- [联系我们](#联系我们)
- [License](#license)

---

## 项目介绍

**Bots Nest** 是一个自托管的**企业微信 LLM 机器人平台**，支持同时运行多个企业微信机器人实例。每个机器人可绑定不同的 LLM Provider 和模型，配备独立的技能引擎和会话管理，通过管理后台统一配置和监控。

团队成员可以直接在企业微信聊天中与 AI 对话、执行 Shell 命令、调用自定义工具，无需离开企业微信。

### 核心能力

| 能力 | 说明 |
|------|------|
| 多机器人 | 同时运行多个企业微信机器人，独立配置、独立运行 |
| 多 LLM Provider | 兼容 OpenAI API 格式，支持任意 LLM 提供商 |
| 技能引擎 | 每个机器人可配置多个技能，匹配触发注入 system prompt + tools |
| MCP 集成 | 接入 MCP（Model Context Protocol）工具服务，扩展 AI 能力 |
| Shell Agent | LLM 自动执行 Shell 命令，带白名单和超时安全控制 |
| 会话管理 | SQLite 本地存储，支持历史查看、过期、压缩和删除 |
| 管理后台 | React + Ant Design 构建的全功能 Web 管理界面 |

---

## 双授权模式

Bots Nest 采用 **AGPL v3 + 商业授权** 的双授权模式：

| 版本 | 许可证 | 费用 | 适用场景 |
|------|--------|------|----------|
| **开源版（社区版）** | [AGPL v3](./LICENSE) | 免费 | 个人学习、非商业使用、愿意开源修改的企业 |
| **商业版** | [商业授权](./COMMERCIAL_LICENSE.md) | 付费 | 企业内部私有化部署、不愿公开修改代码 |

**简单来说：**
- 如果你能接受修改代码后开源 → 用 AGPL 免费版
- 如果你需要保留代码私有 → 购买商业授权
- 商业授权包含优先技术支持和授权保障

👉 [查看商业授权详情](./COMMERCIAL_LICENSE.md)

---

## 核心功能

### 多机器人管理

- 支持同时运行**多个**企业微信机器人
- 每个机器人独立配置 WeCom Bot ID / Secret
- 动态启停单个机器人，不影响其他实例
- 自动重连：WebSocket 断开后自动恢复连接

### LLM Provider 公共配置

- 兼容 OpenAI API 格式的 LLM 提供商（OpenAI、Azure、Claude、DeepSeek、智谱、千问等）
- 公共配置，所有机器人共享
- 创建时自动探测可用模型列表
- 支持页面 CRUD 管理

### MCP 工具服务

- [Model Context Protocol](https://modelcontextprotocol.io) 标准集成
- 工具自动发现（调用 `/tools` 端点）
- 全局公共配置，所有机器人可用
- 工具名自动前缀隔离（`{mcpID}__{toolName}`）

### Skill 技能引擎

- 每个机器人独立配置技能组
- 技能通过关键词匹配自动触发
- 自定义 system prompt 和工具定义
- 热加载：修改技能即时生效

### Shell Agent

- LLM 驱动的 Shell 命令执行
- 命令白名单控制
- 交互式命令（vim、top 等）自动拦截
- 超时控制 + 输出长度截断

### 会话管理

- 按机器人隔离存储会话
- 私聊和群聊上下文分离
- 支持会话过期（软标记）和硬删除
- 自动摘要压缩（Token 超限时自动触发）

---

## 技术栈

| 层级 | 技术 | 说明 |
|------|------|------|
| **后端** | Go + Gin + GORM | HTTP 框架 + ORM |
| **数据库** | SQLite（WAL 模式） | 单文件数据库，零运维 |
| **LLM 接入** | OpenAI 兼容 API | 支持任意兼容 Provider |
| **企业微信接入** | WebSocket 长连接 | 无需公网 IP，JSON 明文通信 |
| **前端** | React 19 + TypeScript + Ant Design | 管理后台 UI |
| **构建** | Vite + Makefile + Docker | 单二进制部署，多阶段构建 |

---

## 快速开始

### 前置要求

- Go 1.26+
- Node.js 20+

### 1. 克隆项目

```bash
git clone https://github.com/hchw/bots-nest.git
cd bots-nest
```

### 2. 配置

复制 `config.yaml` 填入你的配置：

```yaml
# 数据库配置
database:
  driver: "sqlite"
  dsn: ".db/bots-nest.db?_journal_mode=WAL"

# LLM Providers（公共配置）
llm_providers:
  - name: "default"
    endpoint: "https://api.openai.com/v1"
    api_key: "sk-your-key-here"

# MCP 服务（公共配置）
mcps:
  - name: "example-mcp"
    endpoint: "http://localhost:9090"

# 机器人配置
bots:
  - name: "my-bot"
    wecom_bot_id: "your-wecom-bot-id"
    wecom_secret: "your-wecom-secret"
    llm_provider_id: "default"
    llm_model: "gpt-4o"
    llm_temperature: 0.7
    llm_max_tokens: 2048
    max_session_tokens: 4096
    skills:
      - name: "search"
        description: "搜索技能"
        system_prompt: "你是一个搜索助手"
        tools: "[]"
```

### 3. 运行

```bash
# 安装前端依赖
cd web/ui && npm install && cd ../..

# 开发模式（同时启动前后端）
make dev

# 构建生产版本
make build
./bots-nest
```

### 4. Docker 部署

```bash
docker build -t bots-nest .
docker run -p 8080:8080 -v ./config.yaml:/app/config.yaml bots-nest
```

访问 `http://localhost:8080` 进入管理后台。

---

## 配置说明

Bots Nest 采用 **YAML 启动加载 + API 运行时持久化**的配置策略：

1. 首次启动时从 `config.yaml` 读取数据写入 SQLite
2. 运行时所有配置变更通过管理后台 API 直接读写数据库
3. 页面 CRUD 操作即时生效，无需重启服务

### 企业微信接入

本项目使用企业微信**智能机器人长连接模式**（WebSocket API）：

- 无需公网 IP
- 无需消息加解密（JSON 明文传输）
- 仅需 **Bot ID + Secret** 两个凭证
- 文档：[企业微信智能机器人](https://developer.work.weixin.qq.com/document/path/101463)

### LLM 模型自动探测

创建或编辑 LLM Provider 时，系统自动调用 `GET /{endpoint}/models` 接口缓存可用模型列表。创建机器人时自动拉取作为下拉选项，也支持手动输入模型名。


## 后续计划

Bots Nest 将持续迭代，以下是我们规划中的核心功能：

### 账户与权限系统

- 用户注册/登录（JWT + Session 双模式）
- 多角色权限管理：管理员、操作员、只读用户
- API Key 管理与访问控制
- 操作审计日志

### 企业微信深度集成

- 支持企业微信 Webhook 回调模式（适合有公网 IP 的用户）
- 富媒体消息支持（图片、文件、语音、视频）
- 企业微信通讯录同步
- 应用消息主动推送

### 集群与高可用部署

- 多节点集群部署，支持水平扩展
- 分布式会话存储（PostgreSQL / MySQL）
- Redis 缓存 + 消息队列
- 负载均衡与健康检查
- 灾备与自动故障转移

### 定时任务与提醒

- 内置 Cron 调度器
- 定时消息推送（日报、周报、提醒）
- LLM 驱动的定时任务生成
- 日历与事件提醒集成

### 内置 LLM Provider 套餐购买

- 集成主流 LLM 厂商 API 代理
- 按量计费 / 包月套餐
- 用量统计与账单
- 额度预警与控制
- 一键开通，无需自行申请 API Key

### Skill 定制市场

- Skill 模板市场（社区贡献）
- 可视化 Skill 编辑器（拖拽式）
- Skill 版本管理与回滚
- 按机器人/按用户/按群组分配技能
- 技能运行日志与监控

### MCP 生态扩展

- MCP 注册中心与发现机制
- 内置常用 MCP 服务（数据库查询、代码执行、数据分析等）
- MCP 健康监控与熔断
- 自定义 MCP 快速开发 SDK

### 更多消息渠道

- 飞书机器人
- 钉钉机器人
- Discord Bot
- Telegram Bot
- Slack App
- 多渠道消息统一接入

### LLM 增强特性

- 流式回复支持
- 多模态模型接入（图片理解、文件分析）
- RAG（检索增强生成）
- 知识库管理
- 敏感词过滤与内容审核
- Prompt 模板管理

---

## 联系我们

<p align="center">
  <img src="web/ui/public/qr-1.jpg" alt="联系我们" width="200" />
</p>

<p align="center">
  扫码加入 QQ 群，获取更多信息与技术支持<br>
  或添加微信：<strong>hbl826396273</strong><br>
  或发送邮件：<strong>2012hchw@gmail.com</strong>
</p>

---

## License

Bots Nest 采用双授权模型：

- **开源版** — [GNU Affero General Public License v3](./LICENSE)（AGPL v3）
  - 自由使用、修改、分发，但修改后的代码必须开源
  - 通过网络提供服务时，必须提供完整的源代码
- **商业版** — [商业授权](./COMMERCIAL_LICENSE.md)
  - 允许企业内部私有化部署和二次开发
  - 无需公开任何修改代码
  - 包含优先技术支持

详见 [LICENSE](./LICENSE) 和 [COMMERCIAL_LICENSE.md](./COMMERCIAL_LICENSE.md)。
