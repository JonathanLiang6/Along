import { Info, X, Copy, Plus, ChevronRight } from 'lucide-react'
import {
  FLOW_NODE_TYPES, NODE_COLOR_CLASSES, NODE_CONFIG_FIELDS,
  NODE_CONFIG_DEFAULTS, NODE_DEFAULT_NAME, OUTPUT_VAR_TYPES,
} from './flowNodeTypes'

const inputCls =
  'w-full px-3 py-2 bg-bg border border-border rounded-lg text-sm focus:outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 transition-all'

/**
 * 节点配置面板 — 根据 NODE_CONFIG_FIELDS schema 渲染配置表单。
 * 支持字段类型：text / textarea / number / boolean / select / var_select。
 */
export default function NodeConfigPanel({ step, onChange, onConfigChange, availableVars, onDelete, onDuplicate, onToggle }) {
  if (!step) {
    return (
      <div className="flex flex-col h-full">
        <div className="px-3 py-2.5 border-b border-border flex items-center justify-between shrink-0">
          <p className="text-sm font-medium text-text">节点配置</p>
          {onToggle && (
            <button
              onClick={onToggle}
              title="收起配置面板"
              className="p-1.5 text-text-muted hover:text-text hover:bg-surface-subtle rounded-lg transition-colors"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          )}
        </div>
        <div className="flex-1 flex items-center justify-center p-4 text-sm text-text-subtle text-center leading-relaxed">
          点击画布中的节点<br />在这里进行详细配置
        </div>
      </div>
    )
  }
  const cfg = step.config || {}
  const fields = NODE_CONFIG_FIELDS[step.step_type] || []
  const info = FLOW_NODE_TYPES.find((t) => t.id === step.step_type)
  const color = NODE_COLOR_CLASSES[info?.color] || NODE_COLOR_CLASSES.blue
  const labelCls = 'block text-sm font-medium text-text mb-1.5'

  const insertVar = (fieldName, varName) => {
    const cur = cfg[fieldName] || ''
    onConfigChange({ [fieldName]: cur + `{{${varName}}}` })
  }

  const renderField = (field) => {
    const value = cfg[field.key]
    if (field.type === 'boolean') {
      const on = value === true || value === 'true'
      return (
        <div className="flex items-center gap-2 p-2.5 bg-surface-subtle rounded-lg">
          <button
            type="button"
            onClick={() => onConfigChange({ [field.key]: !on })}
            className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
              on ? 'bg-primary-500' : 'bg-border-strong'
            }`}
          >
            <span
              className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                on ? 'translate-x-6' : 'translate-x-1'
              }`}
            />
          </button>
          <label className="text-sm text-text">{field.label}</label>
        </div>
      )
    }
    if (field.type === 'select') {
      return (
        <select
          value={value ?? (field.options[0] && field.options[0].value) ?? ''}
          onChange={(e) => onConfigChange({ [field.key]: e.target.value })}
          className={inputCls}
        >
          {(field.options || []).map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      )
    }
    if (field.type === 'var_select') {
      return (
        <select
          value={value ?? ''}
          onChange={(e) => onConfigChange({ [field.key]: e.target.value })}
          className={inputCls}
        >
          <option value="">默认使用上一步输出...</option>
          {(availableVars || []).map((v) => (
            <option key={v.var} value={v.var}>
              {v.label} ({v.var})
            </option>
          ))}
        </select>
      )
    }
    if (field.type === 'textarea') {
      return (
        <textarea
          value={value ?? ''}
          onChange={(e) => onConfigChange({ [field.key]: e.target.value })}
          placeholder={field.placeholder || ''}
          rows={2}
          className={inputCls + ' resize-none font-mono'}
        />
      )
    }
    if (field.type === 'number') {
      return (
        <input
          type="number"
          value={value ?? ''}
          onChange={(e) => onConfigChange({ [field.key]: Number(e.target.value) })}
          placeholder={field.placeholder || ''}
          className={inputCls}
        />
      )
    }
    // text
    return (
      <input
        type="text"
        value={value ?? ''}
        onChange={(e) => onConfigChange({ [field.key]: e.target.value })}
        placeholder={field.placeholder || ''}
        className={inputCls}
      />
    )
  }

  return (
    <div className="flex flex-col h-full">
      <div className="px-3 py-2.5 border-b border-border flex items-center justify-between shrink-0">
        <div className="flex items-center gap-2 min-w-0">
          <span className={`p-1.5 rounded-lg ${color.bg} ${color.text}`}>
            {info && <info.icon className="w-4 h-4" />}
          </span>
          <div className="min-w-0">
            <p className="text-sm font-medium text-text truncate">节点配置</p>
            <p className="text-xs text-text-subtle truncate">
              {step.name || (info && info.label)}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <button
            onClick={onDuplicate}
            title="复制节点"
            className="p-1.5 text-text-muted hover:text-text hover:bg-surface-subtle rounded-lg transition-colors"
          >
            <Copy className="w-4 h-4" />
          </button>
          <button
            onClick={onDelete}
            title="删除节点"
            className="p-1.5 text-text-muted hover:text-danger-400 hover:bg-danger-500/10 rounded-lg transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
          {onToggle && (
            <button
              onClick={onToggle}
              title="收起配置面板"
              className="p-1.5 text-text-muted hover:text-text hover:bg-surface-subtle rounded-lg transition-colors"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-3 space-y-3">
        {/* 节点名称 */}
        <div>
          <label className={labelCls}>节点名称</label>
          <input
            type="text"
            value={step.name || ''}
            onChange={(e) => onChange({ name: e.target.value })}
            placeholder={NODE_DEFAULT_NAME[step.step_type] || '节点'}
            className={inputCls}
          />
        </div>

        {/* 节点类型切换 */}
        <div>
          <label className={labelCls}>节点类型</label>
          <div className="grid grid-cols-2 gap-1.5">
            {FLOW_NODE_TYPES.map((t) => {
              const Icon = t.icon
              const tc = NODE_COLOR_CLASSES[t.color] || NODE_COLOR_CLASSES.blue
              const active = step.step_type === t.id
              return (
                <button
                  key={t.id}
                  type="button"
                  onClick={() => {
                    onChange({ step_type: t.id })
                    onConfigChange(NODE_CONFIG_DEFAULTS[t.id] || {})
                  }}
                  className={`flex items-center gap-2 p-1.5 rounded-lg text-left text-sm transition-colors border ${
                    active ? `${tc.bg} ${tc.border}` : 'bg-bg border-border hover:border-border-strong'
                  }`}
                >
                  <Icon className={`w-4 h-4 ${tc.text}`} />
                  <div className="flex-1 min-w-0">
                    <div className="text-xs text-text truncate">{t.label}</div>
                  </div>
                </button>
              )
            })}
          </div>
        </div>

        {/* 输出变量名 */}
        {OUTPUT_VAR_TYPES.includes(step.step_type) && (
          <div>
            <label className={labelCls}>输出变量名</label>
            <input
              type="text"
              value={step.output_var || ''}
              onChange={(e) => onChange({ output_var: e.target.value })}
              placeholder="例如：search_result"
              className={inputCls + ' font-mono'}
            />
            <p className="text-xs text-text-subtle mt-1">后续节点可通过 {'{{变量名}}'} 引用</p>
          </div>
        )}

        {/* 类型配置字段 */}
        {fields.length > 0 ? (
          fields.map((field) => (
            <div key={field.key}>
              {field.varInsert ? (
                <div className="flex items-center justify-between mb-1.5">
                  <label className={labelCls.replace('mb-1.5', '')}>{field.label}</label>
                  <VarInsertButton
                    availableVars={availableVars}
                    onInsert={(v) => insertVar(field.key, v)}
                  />
                </div>
              ) : (
                field.type !== 'boolean' && <label className={labelCls}>{field.label}</label>
              )}
              {renderField(field)}
            </div>
          ))
        ) : (
          <div className="flex items-start gap-2 text-sm text-text-subtle bg-surface-subtle border border-border rounded-lg p-3">
            <Info className="w-4 h-4 flex-shrink-0 mt-0.5 text-primary-400" />
            <span>该节点无需额外配置，作为流程入口执行。</span>
          </div>
        )}

        {/* 连线提示 */}
        <div className="flex items-start gap-2 text-xs text-text-subtle bg-surface-subtle border border-border rounded-lg p-2.5">
          <Info className="w-3.5 h-3.5 flex-shrink-0 mt-0.5 text-primary-400" />
          <span>
            拖动节点右侧圆点连接到下一个节点。条件/循环节点下方还有「失败」分支。
          </span>
        </div>
      </div>
    </div>
  )
}

function VarInsertButton({ availableVars, onInsert }) {
  const vars = (availableVars || []).filter((v) => v.var)
  if (vars.length === 0) return null
  return (
    <div className="relative group">
      <button
        type="button"
        className="flex items-center gap-1 text-xs text-primary-400 hover:text-primary-300 px-1.5 py-0.5 rounded transition-colors"
        title="插入变量"
      >
        <Plus className="w-3 h-3" />
        插入变量
      </button>
      <div className="absolute right-0 top-full z-10 hidden group-hover:block pt-1">
        <div className="bg-surface border border-border rounded-lg shadow-xl py-1 max-h-48 overflow-y-auto min-w-36">
          {vars.map((v) => (
            <button
              key={v.var}
              onClick={() => onInsert(v.var)}
              className="block w-full text-left px-3 py-1.5 text-xs text-text hover:bg-surface-subtle transition-colors"
            >
              <span className="font-mono text-primary-400">{'{{' + v.var + '}}'}</span>
              <span className="text-text-subtle ml-1">{v.label}</span>
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
