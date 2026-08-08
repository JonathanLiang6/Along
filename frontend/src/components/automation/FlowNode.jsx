import { memo } from 'react'
import { Handle, Position } from '@xyflow/react'
import { getNodeTypeInfo, NODE_COLOR_CLASSES, CONDITIONAL_TYPES, OUTPUT_VAR_TYPES } from './flowNodeTypes'

/**
 * React Flow 自定义节点 — 工作流节点卡片。
 * 每个节点：左=输入句柄；右=成功出边句柄(success)；
 * condition/repeat 底部还有失败出边句柄(failure)。
 */
function FlowNode({ data, selected }) {
  const step = data.step || {}
  const info = getNodeTypeInfo(step.step_type)
  const Icon = info.icon
  const color = NODE_COLOR_CLASSES[info.color] || NODE_COLOR_CLASSES.blue
  const isConditional = CONDITIONAL_TYPES.includes(step.step_type)
  const running = data.running
  const success = data.success

  return (
    <div
      className={`
        relative px-3 py-2 rounded-xl bg-surface/95 border backdrop-blur-sm
        shadow-md transition-all w-44
        ${running ? `border-2 ${color.border} ${color.ring} ring-2 animate-pulse` : 'border-border'}
        ${selected ? `${color.border} ring-2 ${color.ring}` : ''}
        ${success === true ? 'border-green-400 ring-2 ring-green-400/30' : ''}
        ${success === false ? 'border-red-400 ring-2 ring-red-400/30' : ''}
      `}
    >
      {/* 输入句柄 */}
      <Handle
        type="target"
        position={Position.Left}
        className="!w-3 !h-3 !bg-border-strong !border-2 !border-surface"
        style={{ top: '50%' }}
      />

      <div className="flex items-center gap-2">
        <span className={`shrink-0 p-1.5 rounded-lg ${color.bg} ${color.text}`}>
          <Icon className="w-3.5 h-3.5" />
        </span>
        <div className="min-w-0 flex-1">
          <div className="text-xs font-medium text-text truncate leading-tight">
            {step.name || info.label}
          </div>
          <div className="flex items-center gap-1 mt-0.5">
            <span className={`text-[10px] ${color.text}`}>{info.label}</span>
            {OUTPUT_VAR_TYPES.includes(step.step_type) && step.output_var && (
              <span className="text-[10px] text-text-subtle font-mono truncate">
                → {step.output_var}
              </span>
            )}
          </div>
        </div>
      </div>

      {/* 成功出边句柄 */}
      <Handle
        type="source"
        position={Position.Right}
        id="success"
        className="!w-3 !h-3 !bg-green-500 !border-2 !border-surface"
        style={{ top: '50%' }}
        title="成功 →"
      />

      {/* 条件/循环节点的失败出边句柄 */}
      {isConditional && (
        <Handle
          type="source"
          position={Position.Bottom}
          id="failure"
          className="!w-3 !h-3 !bg-red-500 !border-2 !border-surface"
          style={{ right: '12%', left: 'auto' }}
          title="失败 →"
        />
      )}

      {/* 句柄标签 */}
      {isConditional && (
        <div className="absolute -bottom-4 left-0 right-0 flex justify-between px-1 text-[9px] leading-none text-text-subtle pointer-events-none">
          <span>成功</span>
          <span>失败</span>
        </div>
      )}
    </div>
  )
}

export default memo(FlowNode)
