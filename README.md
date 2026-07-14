# Along

一个面向开发者的多 Agent 智能助手，通过多个专业化 Agent 协作，提供对话、规划、记忆、反思等能力，帮助用户高效推进学习与项目。

## 核心能力

- **对话支持**：自然语言交互，理解上下文
- **规划管理**：目标拆解、任务跟踪、进度回顾
- **记忆系统**：长期记忆用户偏好、项目信息、关键事实（5层记忆体系）
- **信息整合**：搜索、调研、内容归纳
- **反思复盘**：周期性回顾成长与进展
- **工具调用**：文件系统、Git、浏览器等
- **自动化任务**：可视化任务调度、工作流编排

## 内部 Agent 架构

```
Companion Core
│
├── Emotion Agent      # 情绪识别与状态支持
├── Planner Agent      # 学习与项目规划
├── Research Agent     # 搜索与调研
├── Tool Agent         # 工具调用
├── Memory Agent       # 记忆管理
├── Reflection Agent   # 复盘分析
├── Web Agent          # 网页搜索
├── Summarize Agent    # 信息整合
└── File Generation Agent # 文档生成
```

## 技术栈

- **前端**：React + Tailwind CSS + Vite
- **后端**：Go
- **桌面壳**：Wails 2
- **数据库**：SQLite + JSON 文件

## 安装与运行

### 前置条件

- Go 1.21+
- Node.js 18+
- Wails CLI

### 开发模式

```bash
# 安装依赖
cd frontend && npm install

# 返回项目根目录，启动开发模式
wails dev
```

### 构建

```bash
# 构建前端
cd frontend && npm run build

# 返回项目根目录，构建桌面应用
wails build
```

## 项目结构

```
Along/
├── assets/           # 应用资源
├── docs/             # 设计文档
├── frontend/         # 前端代码
│   ├── src/
│   │   ├── components/  # 组件
│   │   ├── hooks/       # 自定义 Hooks
│   │   └── pages/       # 页面
│   └── ...
├── internal/         # 后端代码
│   ├── agents/       # Agent 实现
│   ├── ai/           # AI 接口
│   ├── automation/   # 自动化任务引擎
│   ├── core/         # 核心逻辑
│   ├── db/           # 数据库
│   ├── models/       # 数据模型
│   └── services/     # 服务层
├── app.go            # 应用入口
├── main.go           # 主函数
├── tray.go           # 系统托盘
├── go.mod
└── wails.json
```

## 配置

首次启动应用后，在设置页面配置 AI API Key：

- **推荐**：DeepSeek
- **备选**：智谱 GLM-4-Flash、通义千问 Qwen-Turbo

所有数据存储于用户本地，不进行云同步。

## 页面功能

- **伙伴**：聊天界面、今日观察、当前关注
- **项目**：目标列表、项目详情、风险分析
- **我们**：时间线、共同回忆、成长轨迹、复盘
- **记忆**：记忆分类管理（L1-L5）
- **自动化**：任务管理、工作流编排、执行记录

## 快捷键

| 快捷键 | 功能 |
|--------|------|
| Ctrl/Cmd + K | 全局搜索 |
| Ctrl/Cmd + Enter | 发送消息（多行输入时） |
| Esc | 关闭弹窗 / 返回上一页 |
| Ctrl/Cmd + , | 打开设置 |

## 许可证

MIT License