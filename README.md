# Along

基于 Go + Wails + React 的多 Agent 智能助手桌面应用。通过 **Orchestrator（主 Agent）** 统一编排 10 个专业子 Agent 协作，提供对话、规划、记忆、调研、自动化等能力。

## 核心能力

- **智能对话**：自然语言交互，支持流式响应，上下文感知
- **Agent 编排**：LLM 驱动的主 Agent 自动分析意图 → 生成执行计划 → 调度子 Agent 协作完成复杂任务
- **记忆系统**：5 层长期记忆（L1 个人画像 → L5 日常喜好），自动提取与去重
- **计划管理**：目标拆解、里程碑追踪、进度可视化、打卡记录
- **联网调研**：多引擎搜索（DuckDuckGo + Bing），AI 自动总结生成结构化报告
- **反思复盘**：周期性成长分析、关系回顾、项目总结
- **工具调用**：文件读写、目录浏览、Git 操作、浏览器打开
- **自动化任务**：Cron 定时调度 + 可视化工作流编排，支持 10 种任务类型

## 架构

```
用户消息
    │
    ▼
┌──────────────────────────────┐
│      Orchestrator（主Agent）   │
│                              │
│ 1. ContextProvider注入上下文   │
│    (Memory / Plan / History)  │
│ 2. LLM 分析意图 → 生成计划     │
│ 3. LLM 不可用时关键词路由兜底   │
│ 4. Pipeline 按计划执行        │
└──────────┬───────────────────┘
           │
    ┌──────┼──────┬──────────┐
    ▼      ▼      ▼          ▼
┌─────┐┌─────┐┌─────┐  ┌──────────┐
│Web  ││Summ ││File │  │Emotion   │
│Agent││Agent││Agent│  │Agent     │
└─────┘└─────┘└─────┘  └──────────┘
    ... 共 10 个子 Agent

自动化 = Scheduler (cron) → 构造 prompt → Orchestrator
                                            │
                                    ┌───────┴───────┐
                                    │   Pipeline    │
                                    │ (统一执行引擎) │
                                    └───────────────┘
```

### 新架构 vs 旧架构

| 方面 | 旧架构 | 新架构 |
|------|--------|--------|
| 消息路由 | 关键词匹配 → 选**一个** Agent | LLM 分析意图 → 动态**编排多个** Agent |
| 聊天 & 自动化 | 两套独立系统，互不相通 | 共用同一个 Pipeline 执行引擎 |
| 自动化执行器 | 10 种独立 Executor（~1800行） | 统一为 Scheduler → Orchestrator |
| Agent 协作 | 硬编码直接引用 | Orchestrator 统一调度 |
| 上下文注入 | 各 Agent 各自查询（或不查） | ContextProvider 自动注入 |

### 10 个子 Agent

| Agent | 职责 | 触发场景 |
|-------|------|---------|
| **Planner** | 目标分解、里程碑规划 | 制定计划、拆解任务 |
| **Web** | 联网搜索、网页抓取 | 搜索信息、查资料 |
| **TechAnalysis** | AI 技术概念深度分析 | 了解技术原理 |
| **Research** | 多轮搜索 + 交叉验证 | 深度调研、文献综述 |
| **Summarize** | 信息去重分类提炼 | 整理内容、生成摘要 |
| **FileGeneration** | 按模板生成 Markdown | 生成报告、保存文档 |
| **Tool** | 文件/目录/Git/浏览器 | 文件操作、Git 查询 |
| **Reflection** | 周期性复盘分析 | 回顾总结、反思成长 |
| **Memory** | 记忆提取与查询 | 记住偏好、回忆事实 |
| **Emotion** | 日常聊天与兜底 | 闲聊、情绪支持 |

## 技术栈

| 层 | 技术 |
|----|------|
| 桌面壳 | [Wails v2](https://wails.io/) |
| 后端 | Go 1.25+ |
| 前端 | React 18 + Vite 5 + Tailwind CSS 3 |
| 数据库 | SQLite（WAL 模式） |
| AI 提供商 | DeepSeek / 智谱 GLM-4 / 通义千问 |
| 定时调度 | robfig/cron v3 |
| 系统托盘 | energye/systray |

## 快速开始

### 前置条件

- **Go** 1.25+（[下载](https://go.dev/dl/)）
- **Node.js** 18+（[下载](https://nodejs.org/)）
- **Wails CLI**：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### 开发模式

```bash
cd frontend && npm install && cd ..
wails dev
```

### 构建

```bash
wails build
# 输出在 build/bin/
```

## 项目结构

```
Along/
├── main.go                          # Wails 应用入口
├── app.go                           # App 主体，84 个前端 API 方法
├── tray.go                          # Windows 系统托盘
├── wails.json                       # Wails 配置
│
├── frontend/                        # React 前端
│   └── src/
│       ├── App.jsx                  # 根组件（5 个 Tab 导航）
│       ├── pages/
│       │   ├── CompanionPage.jsx    # 聊天主页
│       │   ├── PlanPage.jsx         # 计划管理
│       │   ├── AutomationPage.jsx   # 自动化任务 + 流程编辑器
│       │   ├── MemoryPage.jsx       # 记忆浏览
│       │   ├── SettingsPage.jsx     # 设置面板
│       │   ├── UsPage.jsx           # 关系页面
│       │   └── OnboardingPage.jsx   # 首次引导
│       ├── components/chat/         # 聊天组件
│       └── hooks/useChat.js         # 聊天 Hook（流式响应）
│
├── internal/
│   ├── ai/                          # AI 客户端层
│   │   ├── deepseek.go              # 多 Provider 客户端（流式 SSE）
│   │   └── prompt.go               # System Prompt 构建
│   │
│   ├── orchestrator/                # ★ 主 Agent 编排器
│   │   ├── orchestrator.go          # 核心编排逻辑
│   │   ├── planner.go               # LLM 计划生成
│   │   ├── keyword_router.go        # 关键词兜底路由
│   │   └── context.go               # 上下文自动注入
│   │
│   ├── pipeline/                    # ★ 统一执行引擎
│   │   ├── plan.go                  # Plan / Step 数据结构
│   │   ├── runner.go                # 流式步骤执行器
│   │   └── condition.go             # 增强条件表达式求值
│   │
│   ├── scheduler/                   # ★ 定时调度器
│   │   ├── scheduler.go             # Cron 调度 + 任务执行
│   │   └── system_tasks.go          # backup / cleanup 系统任务
│   │
│   ├── agents/                      # 子 Agent 实现
│   │   ├── agent.go                 # Agent 接口 + Capability 声明
│   │   ├── manager.go               # Agent 注册与路由
│   │   ├── web_agent.go             # 搜索（DuckDuckGo + Bing）
│   │   ├── emotion_agent.go         # 情感陪伴
│   │   ├── planner_agent.go         # 计划管理
│   │   └── ...（共 11 个文件）
│   │
│   ├── core/companion_core.go       # 核心协调器
│   ├── services/                    # 服务层（Settings/Memory/Conversation/Plan/Automation）
│   ├── models/models.go             # 数据模型
│   └── db/db.go                     # SQLite Schema + 迁移
│
├── docs/superpowers/specs/          # 设计文档
├── .env.example                     # 环境变量模板
└── LICENSE                          # MIT
```

## 配置

首次启动后在设置页面配置 AI API Key：

| Provider | API 地址 | 模型 |
|----------|---------|------|
| DeepSeek（默认） | `api.deepseek.com` | `deepseek-chat` |
| 智谱 GLM | `open.bigmodel.cn` | `glm-4-flash` |
| 通义千问 | `dashscope.aliyuncs.com` | `qwen-turbo` |

所有数据本地存储，不上传云端：
- `companion.db` — SQLite 数据库
- `conversations/` — JSON 对话备份
- `research_docs/` — 生成的调研文档

### 安全

- API Key 使用 AES-256-GCM 加密存储
- 加密密钥基于机器名和用户名派生

## 页面功能

| 页面 | 功能 |
|------|------|
| **伙伴** | 聊天主界面、今日观察、主动建议 |
| **计划** | 目标列表、里程碑管理、打卡记录、进度追踪 |
| **自动化** | 任务创建/编辑/启停、工作流可视化编排、执行记录 |
| **记忆** | L1-L5 分类管理、记忆搜索、手动编辑 |
| **我们** | 成长轨迹、共同回忆、复盘报告 |
| **设置** | API Key、Provider、主题、开机启动、托盘 |

## 快捷键

| 快捷键 | 功能 |
|--------|------|
| `Ctrl + K` | 全局搜索（记忆 + 对话 + 高光） |
| `Ctrl + Enter` | 发送消息（多行输入时） |
| `Esc` | 关闭弹窗 / 返回 |
| `Ctrl + ,` | 打开设置 |

## License

MIT License — 详见 [LICENSE](LICENSE)
