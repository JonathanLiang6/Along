# Along

A multi-agent AI companion desktop application built with Go + Wails + React. Along helps developers with conversation, planning, memory management, web research, and task automation through coordinated specialist agents.

## Features

### Core Capabilities
- **Intelligent Conversation**: Natural language interaction with streaming response support
- **Multi-Agent Routing**: Automatic agent selection based on keyword matching and intent detection (9 specialist agents)
- **Memory System**: 5-tier long-term memory (L1-L5) for personal profile, relationships, events, projects, and preferences
- **Plan Management**: Goal decomposition, milestone tracking, progress visualization, and check-in journaling
- **Web Research**: Multi-engine search (DuckDuckGo, Bing) with AI-powered summarization
- **Reflection & Review**: Periodic growth analysis, relationship insights, and project retrospectives
- **Tool Integration**: File system operations, Git status/log, and browser interaction
- **Automation Engine**: Cron-based task scheduling with 10 task types, workflow orchestration, and dependency chaining

### UX
- **System Tray**: Minimize to tray, unread message counter, quick-access menu
- **Global Search**: Search across memories, conversations, and highlights (Ctrl+K)
- **Dark/Light Theme**: CSS variable-based theming
- **Onboarding Flow**: First-run guided setup
- **Data Export/Import**: Full data portability with JSON export
- **Auto-start**: Windows registry-based auto-start support

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Wails Desktop Shell                   │
├───────────────────────┬─────────────────────────────────┤
│     React Frontend     │         Go Backend              │
│                        │                                 │
│  Pages:                │  CompanionCore (orchestrator)   │
│  - Companion (chat)    │     │                           │
│  - Plan (goals)       │     ├── EmotionAgent             │
│  - Automation          │     ├── PlannerAgent             │
│  - Memory              │     ├── MemoryAgent              │
│  - Settings            │     ├── ResearchAgent            │
│                        │     ├── ReflectionAgent          │
│  Components:           │     ├── ToolAgent                │
│  - ChatInput           │     ├── WebAgent                 │
│  - ChatSidebar         │     ├── SummarizeAgent           │
│  - MessageBubble       │     ├── FileGenerationAgent      │
│  - CodeBlock           │     └── TechAnalysisAgent        │
│  - MoodCheckin         │                                 │
│  - TopicSuggestions    │  Automation Engine              │
│                        │     ├── Scheduler (cron)         │
│                        │     ├── 10 Task Executors        │
│                        │     └── Workflow Orchestrator    │
│                        │                                 │
│                        │  Services Layer                  │
│                        │     ├── SettingsService          │
│                        │     ├── MemoryService            │
│                        │     ├── ConversationService      │
│                        │     ├── PlanService              │
│                        │     └── AutomationService        │
│                        │                                 │
│                        │  Storage                        │
│                        │     ├── SQLite (primary)         │
│                        │     └── JSON files (backup)      │
└───────────────────────┴─────────────────────────────────┘
```

### Agent Routing
Messages are routed to the most appropriate agent based on keyword matching priority:

| Priority | Agent | Keywords |
|----------|-------|----------|
| 100 | Planner | 计划, 目标, 里程碑, 任务, 项目 |
| 95 | Web | 搜索, 查一下, 新闻, 最新 |
| 92 | TechAnalysis | 什么是, 解释, 分析, 原理, LLM, RAG |
| 90 | Research | 深度调研, 专题研究, 文献综述 |
| 88 | Summarize | 总结, 整理, 归纳, 梳理 |
| 87 | FileGeneration | 生成文档, 生成报告, 保存文档 |
| 85 | Tool | 读取文件, 写入文件, git状态 |
| 80 | Reflection | 复盘, 回顾, 总结, 反思 |
| 70 | Memory | 记得, 记住, 回忆, 之前 |
| 10 | Emotion | 开心, 难过, 焦虑, 想你 (fallback) |

### Automation Task Types
1. **agent_chat** - AI agent conversation
2. **web_search** - Web search with AI summarization
3. **report** - Periodic report generation
4. **backup** - Database backup
5. **reminder** - Scheduled notifications
6. **monitor** - System monitoring
7. **habit_checkin** - Habit tracking
8. **review** - Automated review/reflection
9. **cleanup** - Data maintenance
10. **workflow** - Multi-step customizable workflows

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Desktop Shell | [Wails v2](https://wails.io/) |
| Backend | Go 1.25+ |
| Frontend | React 18 + Vite 5 |
| Styling | Tailwind CSS 3 |
| Database | SQLite (via `go-sqlite3`) |
| AI Providers | DeepSeek / Zhipu GLM-4 / Qwen-Turbo |
| Scheduling | robfig/cron v3 |
| System Tray | energye/systray |

## Getting Started

### Prerequisites

- **Go** 1.25+ ([download](https://go.dev/dl/))
- **Node.js** 18+ ([download](https://nodejs.org/))
- **Wails CLI** ([install guide](https://wails.io/docs/gettingstarted/installation))

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### Development

```bash
# Install frontend dependencies
cd frontend && npm install && cd ..

# Start in development mode (hot reload)
wails dev
```

### Build

```bash
# Production build
wails build

# The output binary will be in build/bin/
```

## Project Structure

```
Along/
├── main.go                          # Application entry point
├── app.go                           # Core App struct, all frontend API methods
├── tray.go                          # System tray implementation
├── wails.json                       # Wails project configuration
├── go.mod / go.sum                  # Go module definition
│
├── assets/
│   └── icon.ico                     # Application icon
│
├── frontend/                        # React frontend
│   ├── package.json
│   ├── vite.config.js
│   ├── tailwind.config.js
│   └── src/
│       ├── main.jsx                 # React entry point
│       ├── App.jsx                  # Root component with tab navigation
│       ├── index.css                # Global styles & theming
│       ├── pages/
│       │   ├── CompanionPage.jsx    # Main chat interface
│       │   ├── PlanPage.jsx         # Goals & plans management
│       │   ├── AutomationPage.jsx   # Automation task management
│       │   ├── MemoryPage.jsx       # Memory browser
│       │   ├── SettingsPage.jsx     # Application settings
│       │   ├── OnboardingPage.jsx   # First-run onboarding
│       │   └── UsPage.jsx           # Relationship insights
│       ├── components/
│       │   ├── chat/
│       │   │   ├── ChatInput.jsx    # Message input component
│       │   │   ├── ChatSidebar.jsx  # Conversation list
│       │   │   ├── MessageBubble.jsx # Chat message display
│       │   │   ├── CodeBlock.jsx    # Syntax-highlighted code
│       │   │   └── WelcomeScreen.jsx # Empty state
│       │   ├── MoodCheckin.jsx      # Daily mood tracker
│       │   └── TopicSuggestions.jsx # Conversation starters
│       └── hooks/
│           └── useChat.js           # Chat logic with streaming
│
├── internal/
│   ├── ai/                          # AI client layer
│   │   ├── deepseek.go              # Multi-provider AI client (Stream/SSE)
│   │   └── prompt.go               # System prompt & agent prompt builder
│   │
│   ├── agents/                      # Agent implementations
│   │   ├── agent.go                 # Agent interface & common types
│   │   ├── manager.go               # Agent registration & routing
│   │   ├── emotion_agent.go         # Emotional support
│   │   ├── planner_agent.go         # Plan & goal management
│   │   ├── memory_agent.go          # Memory extraction & classification
│   │   ├── research_agent.go        # Deep research with multi-query
│   │   ├── reflection_agent.go      # Periodic review & reflection
│   │   ├── tool_agent.go            # File/Git/Browser operations
│   │   ├── web_agent.go             # Web search (DuckDuckGo + Bing)
│   │   ├── summarize_agent.go       # Information aggregation
│   │   ├── file_generation_agent.go # Markdown document generation
│   │   └── tech_analysis_agent.go   # AI technology analysis
│   │
│   ├── core/
│   │   └── companion_core.go        # Central orchestrator
│   │
│   ├── automation/                  # Automation engine
│   │   ├── engine.go                # Cron scheduler & task runner
│   │   ├── executor.go              # Executor registry (10 types)
│   │   ├── workflow.go              # Multi-step workflow executor
│   │   ├── web_search.go            # Web search task
│   │   ├── agent_chat.go            # Agent conversation task
│   │   ├── backup.go                # Database backup task
│   │   ├── cleanup.go               # Data cleanup task
│   │   ├── habit_checkin.go         # Habit tracking task
│   │   ├── monitor.go               # System monitoring task
│   │   ├── reminder.go              # Notification task
│   │   ├── report.go                # Report generation task
│   │   └── review.go                # Review/reflection task
│   │
│   ├── db/
│   │   └── db.go                    # SQLite schema & migrations
│   │
│   ├── models/
│   │   └── models.go                # Data model definitions
│   │
│   └── services/                    # Business logic layer
│       ├── settings.go              # Settings with AES encryption
│       ├── memory.go                # Memory CRUD & search
│       ├── conversation.go          # Conversation management
│       ├── plan.go                  # Goals, milestones, check-ins
│       └── automation_service.go    # Automation task CRUD
│
├── docs/
│   └── superpowers/specs/           # Design documents
│
├── LICENSE                          # MIT License
├── PRD.md                           # Product Requirements Document
└── .env.example                     # Environment config template
```

## Configuration

Configure the AI API key in the Settings page after first launch:

| Provider | API Endpoint | Model |
|----------|-------------|-------|
| DeepSeek (default) | `api.deepseek.com` | `deepseek-chat` |
| Zhipu GLM | `open.bigmodel.cn` | `glm-4-flash` |
| Qwen | `dashscope.aliyuncs.com` | `qwen-turbo` |

All data is stored locally in a `data/` directory alongside the executable, including:
- `companion.db` - SQLite database
- `conversations/` - JSON conversation backups
- `research_docs/` - Generated research documents

### Security
- API keys are encrypted at rest using AES-256-GCM
- Encryption key derived from machine hostname and username

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+K` | Global search |
| `Ctrl+Enter` | Send message (multi-line input) |
| `Esc` | Close dialog / go back |
| `Ctrl+,` | Open settings |

## License

MIT License - see [LICENSE](LICENSE) for details.
