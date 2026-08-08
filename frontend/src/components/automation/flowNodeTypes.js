import {
  Play, Bot, Search, FileText, GitBranch, Repeat, Braces, Clock, Globe,
  Microscope, CalendarDays, Brain, Cpu, Save, Bell,
} from 'lucide-react'

/**
 * 工作流节点类型定义 — 流程图设计器与旧版列表编辑器共用的权威定义。
 * 每种类型：图标 / 配色 / 默认名 / 配置字段 schema。
 * 配色沿用"不使用粉色"的约定。
 */
export const FLOW_NODE_TYPES = [
  { id: 'start', label: '开始', icon: Play, desc: '流程入口（自动创建）', color: 'emerald' },
  { id: 'agent', label: 'Agent 调用', icon: Bot, desc: '调用指定 Agent 执行', color: 'blue' },
  { id: 'search', label: '网络搜索', icon: Search, desc: '搜索互联网信息', color: 'cyan' },
  { id: 'summarize', label: '信息总结', icon: FileText, desc: '对前序内容进行 AI 总结', color: 'violet' },
  { id: 'condition', label: '条件判断', icon: GitBranch, desc: '根据条件走两条分支', color: 'amber' },
  { id: 'repeat', label: '循环重复', icon: Repeat, desc: '重复执行并回跳', color: 'orange' },
  { id: 'set_variable', label: '设置变量', icon: Braces, desc: '给变量赋一个值', color: 'indigo' },
  { id: 'delay', label: '延时等待', icon: Clock, desc: '等待指定秒数', color: 'teal' },
  { id: 'web_fetch', label: '网页抓取', icon: Globe, desc: '抓取网页正文', color: 'sky' },
  { id: 'research', label: '深度调研', icon: Microscope, desc: '多步联网深度调研', color: 'purple' },
  { id: 'reflection', label: '每周复盘', icon: CalendarDays, desc: '回顾某段时期的对话与任务', color: 'red' },
  { id: 'memory_recall', label: '记忆回忆', icon: Brain, desc: '从长期记忆检索相关内容', color: 'fuchsia' },
  { id: 'tech_analysis', label: '技术分析', icon: Cpu, desc: '对技术主题做分析解读', color: 'slate' },
  { id: 'save_file', label: '保存文件', icon: Save, desc: '保存内容到文件', color: 'green' },
  { id: 'notify', label: '发送通知', icon: Bell, desc: '推送消息提醒（弹窗+托盘）', color: 'rose' },
]

export const NODE_COLOR_CLASSES = {
  emerald: {
    text: 'text-emerald-400', bg: 'bg-emerald-400/10', border: 'border-emerald-400/40',
    ring: 'ring-emerald-400/40', dot: 'bg-emerald-400',
  },
  blue: {
    text: 'text-blue-400', bg: 'bg-blue-400/10', border: 'border-blue-400/40',
    ring: 'ring-blue-400/40', dot: 'bg-blue-400',
  },
  cyan: {
    text: 'text-cyan-400', bg: 'bg-cyan-400/10', border: 'border-cyan-400/40',
    ring: 'ring-cyan-400/40', dot: 'bg-cyan-400',
  },
  violet: {
    text: 'text-violet-400', bg: 'bg-violet-400/10', border: 'border-violet-400/40',
    ring: 'ring-violet-400/40', dot: 'bg-violet-400',
  },
  amber: {
    text: 'text-amber-400', bg: 'bg-amber-400/10', border: 'border-amber-400/40',
    ring: 'ring-amber-400/40', dot: 'bg-amber-400',
  },
  orange: {
    text: 'text-orange-400', bg: 'bg-orange-400/10', border: 'border-orange-400/40',
    ring: 'ring-orange-400/40', dot: 'bg-orange-400',
  },
  indigo: {
    text: 'text-indigo-400', bg: 'bg-indigo-400/10', border: 'border-indigo-400/40',
    ring: 'ring-indigo-400/40', dot: 'bg-indigo-400',
  },
  teal: {
    text: 'text-teal-400', bg: 'bg-teal-400/10', border: 'border-teal-400/40',
    ring: 'ring-teal-400/40', dot: 'bg-teal-400',
  },
  sky: {
    text: 'text-sky-400', bg: 'bg-sky-400/10', border: 'border-sky-400/40',
    ring: 'ring-sky-400/40', dot: 'bg-sky-400',
  },
  purple: {
    text: 'text-purple-400', bg: 'bg-purple-400/10', border: 'border-purple-400/40',
    ring: 'ring-purple-400/40', dot: 'bg-purple-400',
  },
  red: {
    text: 'text-red-400', bg: 'bg-red-400/10', border: 'border-red-400/40',
    ring: 'ring-red-400/40', dot: 'bg-red-400',
  },
  fuchsia: {
    text: 'text-fuchsia-400', bg: 'bg-fuchsia-400/10', border: 'border-fuchsia-400/40',
    ring: 'ring-fuchsia-400/40', dot: 'bg-fuchsia-400',
  },
  slate: {
    text: 'text-slate-300', bg: 'bg-slate-400/10', border: 'border-slate-400/40',
    ring: 'ring-slate-400/40', dot: 'bg-slate-400',
  },
  green: {
    text: 'text-green-400', bg: 'bg-green-400/10', border: 'border-green-400/40',
    ring: 'ring-green-400/40', dot: 'bg-green-400',
  },
  rose: {
    text: 'text-rose-400', bg: 'bg-rose-400/10', border: 'border-rose-400/40',
    ring: 'ring-rose-400/40', dot: 'bg-rose-400',
  },
}

export const getNodeTypeInfo = (type) =>
  FLOW_NODE_TYPES.find((t) => t.id === type) || FLOW_NODE_TYPES.find((t) => t.id === 'agent')

// 条件型节点：有 success / failure 两个出边句柄
export const CONDITIONAL_TYPES = ['condition', 'repeat']

// 支持「输出变量名」的节点：结果可被后续步骤通过 {{变量名}} 引用
export const OUTPUT_VAR_TYPES = [
  'agent', 'search', 'summarize', 'research', 'reflection',
  'memory_recall', 'tech_analysis', 'web_fetch',
]

export const AGENT_OPTIONS = [
  { value: 'web', label: 'Web Agent' },
  { value: 'planner', label: '计划 Agent' },
  { value: 'emotion', label: '情感 Agent' },
  { value: 'memory', label: '记忆 Agent' },
  { value: 'reflection', label: '反思 Agent' },
  { value: 'summarize', label: '总结 Agent' },
  { value: 'research', label: '调研 Agent' },
  { value: 'tech_analysis', label: '技术分析 Agent' },
  { value: 'file_generation', label: '文件生成 Agent' },
  { value: 'tool', label: '工具 Agent' },
]

export const NODE_DEFAULT_NAME = {
  start: '开始',
  agent: 'Agent 调用',
  search: '网络搜索',
  summarize: '信息总结',
  condition: '条件判断',
  repeat: '循环',
  set_variable: '设置变量',
  delay: '延时等待',
  web_fetch: '网页抓取',
  research: '深度调研',
  reflection: '每周复盘',
  memory_recall: '记忆回忆',
  tech_analysis: '技术分析',
  save_file: '保存文件',
  notify: '发送通知',
}

export const NODE_CONFIG_DEFAULTS = {
  start: {},
  agent: { agent_name: 'web', prompt: '' },
  search: { query: '', need_summary: true },
  summarize: { topic: '', content_var: '', summary_type: 'detailed' },
  condition: { source_var: '', operator: 'contains', compare_value: '' },
  repeat: { max_iterations: 5 },
  set_variable: { name: '', value: '' },
  delay: { seconds: 5 },
  web_fetch: { url: '' },
  research: { query: '' },
  reflection: { period: 'week' },
  memory_recall: { query: '' },
  tech_analysis: { topic: '' },
  save_file: { content_var: '', format: 'markdown', file_path: '', file_name: '' },
  notify: { content: '', level: 'normal' },
}

// 每种节点的配置字段 schema（NodeConfigPanel 据此渲染表单）
// 字段 type：text / textarea / number / boolean / select / var_select（来源变量下拉）
// varInsert: true 时在标签旁提供 {{变量}} 插入按钮
export const NODE_CONFIG_FIELDS = {
  start: [],
  agent: [
    {
      key: 'agent_name', label: '选择 Agent', type: 'select',
      options: AGENT_OPTIONS,
    },
    {
      key: 'prompt', label: '提示词', type: 'textarea',
      placeholder: '支持 {{date}} {{time}} {{变量名}}', varInsert: true,
    },
  ],
  search: [
    {
      key: 'query', label: '搜索关键词', type: 'text',
      placeholder: '支持 {{变量名}}', varInsert: true,
    },
    { key: 'need_summary', label: 'AI 总结搜索结果', type: 'boolean' },
  ],
  summarize: [
    {
      key: 'topic', label: '总结主题', type: 'text',
      placeholder: '支持 {{变量名}}，留空使用任务名', varInsert: true,
    },
    { key: 'content_var', label: '内容来源变量', type: 'var_select' },
    {
      key: 'summary_type', label: '总结类型', type: 'select',
      options: [
        { value: 'detailed', label: '详细总结' },
        { value: 'brief', label: '简要总结' },
      ],
    },
  ],
  condition: [
    { key: 'source_var', label: '数据来源变量', type: 'var_select' },
    {
      key: 'operator', label: '判断方式', type: 'select',
      options: [
        { value: 'contains', label: '包含 (contains)' },
        { value: 'not_contains', label: '不包含 (not_contains)' },
        { value: 'equals', label: '等于 (equals)' },
        { value: 'not_equals', label: '不等于 (not_equals)' },
        { value: 'gt', label: '大于 (>)' },
        { value: 'lt', label: '小于 (<)' },
        { value: 'is_empty', label: '为空 (is_empty)' },
        { value: 'not_empty', label: '非空 (not_empty)' },
      ],
    },
    {
      key: 'compare_value', label: '比较值', type: 'text',
      placeholder: '如：已完成 / 5 / 关键词', varInsert: true,
    },
  ],
  repeat: [
    { key: 'max_iterations', label: '最大循环次数', type: 'number', placeholder: '例如 5' },
  ],
  set_variable: [
    { key: 'name', label: '变量名', type: 'text', placeholder: '如：counter' },
    {
      key: 'value', label: '变量值', type: 'text',
      placeholder: '支持 {{变量名}} 与任意文本', varInsert: true,
    },
  ],
  delay: [
    { key: 'seconds', label: '等待秒数', type: 'number', placeholder: '1-3600' },
  ],
  web_fetch: [
    { key: 'url', label: '网页地址', type: 'text', placeholder: 'https://...' },
  ],
  research: [
    {
      key: 'query', label: '调研主题', type: 'textarea',
      placeholder: '需要深度调研的问题', varInsert: true,
    },
  ],
  reflection: [
    {
      key: 'period', label: '复盘周期', type: 'select',
      options: [
        { value: 'day', label: '每日' },
        { value: 'week', label: '每周' },
        { value: 'month', label: '每月' },
      ],
    },
  ],
  memory_recall: [
    {
      key: 'query', label: '回忆关键词', type: 'text',
      placeholder: '如：上周的计划', varInsert: true,
    },
  ],
  tech_analysis: [
    {
      key: 'topic', label: '分析主题', type: 'textarea',
      placeholder: '需要技术分析的内容', varInsert: true,
    },
  ],
  save_file: [
    { key: 'content_var', label: '内容来源变量', type: 'var_select' },
    {
      key: 'format', label: '文件格式', type: 'select',
      options: [
        { value: 'markdown', label: 'Markdown' },
        { value: 'text', label: '纯文本' },
        { value: 'json', label: 'JSON' },
      ],
    },
    {
      key: 'file_path', label: '保存路径', type: 'text',
      placeholder: '可选，默认文档目录', varInsert: true,
    },
    {
      key: 'file_name', label: '文件名', type: 'text',
      placeholder: '如：周报.md', varInsert: true,
    },
  ],
  notify: [
    {
      key: 'content', label: '通知内容', type: 'textarea',
      placeholder: '支持 {{变量名}}', varInsert: true,
    },
    {
      key: 'level', label: '重要程度', type: 'select',
      options: [
        { value: 'normal', label: '普通' },
        { value: 'important', label: '重要' },
        { value: 'urgent', label: '紧急' },
      ],
    },
  ],
}
