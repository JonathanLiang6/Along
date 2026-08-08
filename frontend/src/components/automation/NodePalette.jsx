import { GripVertical, ChevronLeft } from 'lucide-react'
import { FLOW_NODE_TYPES, NODE_COLOR_CLASSES } from './flowNodeTypes'

/**
 * 节点面板 — 从左侧拖拽节点类型到画布上创建节点。
 * 拖拽时在 dataTransfer 里写入类型 id，画布 drop 时读取并放置。
 */
export default function NodePalette({ onAdd, onToggle }) {
  return (
    <div className="w-40 shrink-0 border-r border-border bg-surface-subtle/40 flex flex-col">
      <div className="px-3 py-2.5 border-b border-border flex items-center justify-between gap-1">
        <div className="min-w-0">
          <p className="text-xs font-medium text-text">节点库</p>
          <p className="text-[10px] text-text-subtle mt-0.5 truncate">拖拽或双击添加</p>
        </div>
        {onToggle && (
          <button
            onClick={onToggle}
            title="收起节点库"
            className="shrink-0 p-1 text-text-muted hover:text-text hover:bg-surface-subtle rounded-lg transition-colors"
          >
            <ChevronLeft className="w-4 h-4" />
          </button>
        )}
      </div>
      <div className="flex-1 overflow-y-auto p-2 space-y-1.5">
        {FLOW_NODE_TYPES.map((t) => {
          const Icon = t.icon
          const color = NODE_COLOR_CLASSES[t.color] || NODE_COLOR_CLASSES.blue
          return (
            <div
              key={t.id}
              draggable
              onDragStart={(e) => {
                e.dataTransfer.setData('application/along-node', t.id)
                e.dataTransfer.effectAllowed = 'move'
              }}
              onDoubleClick={() => onAdd && onAdd(t.id)}
              title={t.desc}
              className="flex items-center gap-2 px-2 py-1.5 rounded-lg bg-bg border border-border hover:border-border-strong cursor-grab active:cursor-grabbing select-none transition-colors"
            >
              <GripVertical className="w-3 h-3 text-text-subtle shrink-0" />
              <span className={`p-1 rounded-md ${color.bg} ${color.text} shrink-0`}>
                <Icon className="w-3 h-3" />
              </span>
              <span className="text-xs text-text truncate">{t.label}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}
