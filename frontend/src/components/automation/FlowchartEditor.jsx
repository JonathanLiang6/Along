import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  ReactFlow, Background, BackgroundVariant, Controls, MiniMap, ReactFlowProvider,
  useNodesState, useEdgesState, addEdge, applyNodeChanges, applyEdgeChanges,
  MarkerType, useReactFlow,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { LayoutGrid, Trash2, Copy, MousePointerClick, ChevronLeft, ChevronRight } from 'lucide-react'
import FlowNode from './FlowNode'
import { useEvents } from '../../hooks/useEvents'
import NodePalette from './NodePalette'
import NodeConfigPanel from './NodeConfigPanel'
import {
  CONDITIONAL_TYPES, NODE_CONFIG_DEFAULTS, NODE_DEFAULT_NAME,
} from './flowNodeTypes'

const nodeTypes = { flowNode: FlowNode }

const LAYER_X = 230
const LAYER_Y = 92

// 简单分层自动布局：从无入边节点（入口）开始按深度分层，同层纵向排布
function autoLayoutPositions(steps) {
  const byIndex = new Map(steps.map((s) => [s.step_index, s]))
  const sorted = [...steps].sort((a, b) => a.step_index - b.step_index)
  const layer = new Map()
  const inSet = new Map()
  sorted.forEach((s) => {
    layer.set(s.step_index, 0)
    inSet.set(s.step_index, new Set())
  })
  sorted.forEach((s) => {
    const targets = [s.next_on_success, s.next_on_failure].filter(
      (t) => t > 0 && byIndex.has(t)
    )
    targets.forEach((t) => inSet.get(t).add(s.step_index))
  })
  const queue = sorted
    .filter((s) => inSet.get(s.step_index).size === 0)
    .map((s) => s.step_index)
  if (queue.length === 0 && sorted.length > 0) queue.push(sorted[0].step_index)
  const visited = new Set()
  while (queue.length) {
    const cur = queue.shift()
    if (visited.has(cur)) continue
    visited.add(cur)
    const s = byIndex.get(cur)
    const curLayer = layer.get(cur)
    const targets = [s.next_on_success, s.next_on_failure].filter(
      (t) => t > 0 && byIndex.has(t)
    )
    targets.forEach((t) => {
      layer.set(t, Math.max(layer.get(t), curLayer + 1))
      if (!visited.has(t)) queue.push(t)
    })
  }
  const byLayer = new Map()
  sorted.forEach((s) => {
    const l = layer.get(s.step_index) || 0
    if (!byLayer.has(l)) byLayer.set(l, [])
    byLayer.get(l).push(s)
  })
  const pos = {}
  byLayer.forEach((nodesInLayer, l) => {
    nodesInLayer.forEach((s, i) => {
      pos[s.step_index] = { x: l * LAYER_X, y: i * LAYER_Y }
    })
  })
  return pos
}

// steps → React Flow nodes/edges（pos 全为 0 时自动布局）
function stepsToFlow(steps) {
  const arr = steps || []
  const allZero = arr.every((s) => !s.pos_x && !s.pos_y)
  const autoPos = allZero && arr.length > 0 ? autoLayoutPositions(arr) : null

  const nodes = arr.map((s) => {
    const p = autoPos ? autoPos[s.step_index] : { x: s.pos_x || 0, y: s.pos_y || 0 }
    return {
      id: String(s.step_index),
      type: 'flowNode',
      position: { x: p.x, y: p.y },
      data: { step: { ...s }, running: false },
    }
  })
  const edges = []
  arr.forEach((s) => {
    // -2 表示「重试本步」→ 画布上用自环边表达
    if (s.next_on_success > 0 || s.next_on_success === -2) {
      edges.push({
        id: `e${s.step_index}-${s.next_on_success}-s`,
        source: String(s.step_index),
        target: String(s.next_on_success === -2 ? s.step_index : s.next_on_success),
        sourceHandle: 'success',
        markerEnd: { type: MarkerType.ArrowClosed, width: 15, height: 15 },
      })
    }
    if (
      CONDITIONAL_TYPES.includes(s.step_type) &&
      (s.next_on_failure > 0 || s.next_on_failure === -2)
    ) {
      edges.push({
        id: `e${s.step_index}-${s.next_on_failure}-f`,
        source: String(s.step_index),
        target: String(s.next_on_failure === -2 ? s.step_index : s.next_on_failure),
        sourceHandle: 'failure',
        markerEnd: { type: MarkerType.ArrowClosed, width: 15, height: 15 },
        style: { stroke: '#f87171' },
      })
    }
  })
  return { nodes, edges }
}

// React Flow nodes/edges → steps（把边换算成 next_on_success / next_on_failure；
// 自环边还原为 -2 重试语义）
function flowToSteps(nodesArr, edgesArr) {
  return nodesArr
    .filter((n) => n.data && n.data.step)
    .map((n) => {
      const s = { ...n.data.step }
      s.pos_x = Math.round(n.position.x)
      s.pos_y = Math.round(n.position.y)
      const succ = edgesArr.find(
        (e) => e.source === n.id && (e.sourceHandle === 'success' || !e.sourceHandle)
      )
      const fail = edgesArr.find((e) => e.source === n.id && e.sourceHandle === 'failure')
      s.next_on_success = succ ? (succ.source === succ.target ? -2 : Number(succ.target)) : -1
      s.next_on_failure = fail ? (fail.source === fail.target ? -2 : Number(fail.target)) : -1
      return s
    })
    .sort((a, b) => a.step_index - b.step_index)
}

function FlowchartEditorInner({ steps, onChange, taskId }) {
  const [nodes, setNodes] = useNodesState([])
  const [edges, setEdges] = useEdgesState([])
  const [selectedId, setSelectedId] = useState(null)
  const [leftCollapsed, setLeftCollapsed] = useState(false)
  const [rightCollapsed, setRightCollapsed] = useState(false)
  const { screenToFlowPosition, fitView } = useReactFlow()

  const nodesRef = useRef(nodes)
  const edgesRef = useRef(edges)
  useEffect(() => { nodesRef.current = nodes }, [nodes])
  useEffect(() => { edgesRef.current = edges }, [edges])

  // 父级 steps 变化（加载/外部改动）时重建画布。
  // 不做选中重置——用户编辑时父级 steps 也会同步更新，避免输入过程中丢选中。
  const prevStepsRef = useRef()
  useEffect(() => {
    if (prevStepsRef.current === steps) return
    prevStepsRef.current = steps
    const { nodes: ns, edges: es } = stepsToFlow(steps)
    nodesRef.current = ns
    edgesRef.current = es
    setNodes(ns)
    setEdges(es)
    setTimeout(() => fitView && fitView({ padding: 0.15 }), 60)
  }, [steps, setNodes, setEdges, fitView])

  // 运行高亮：监听后端 automation:step 事件（经共享事件总线）
  useEvents('automation:step', (payload) => {
    if (!payload) return
    if (taskId && payload.task_id !== taskId) return
    const sid = String(payload.step_index)
    setRunningStep(sid)
    setNodes((prev) =>
      prev.map((n) => ({
        ...n,
        data: { ...n.data, running: n.id === sid },
      }))
    )
  })

  // 执行结束清除高亮
  useEvents('automation:execution', (payload) => {
    if (taskId && payload && payload.task_id === taskId) {
      setRunningStep(null)
      setNodes((prev) => prev.map((n) => ({ ...n, data: { ...n.data, running: false } })))
    }
  })

  // 空画布自动补一个「开始」节点
  useEffect(() => {
    if (nodesRef.current.length === 0) {
      addNode('start', { x: 0, y: 0 })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const commit = (nextNodes, nextEdges) => {
    nodesRef.current = nextNodes
    edgesRef.current = nextEdges
    setNodes(nextNodes)
    setEdges(nextEdges)
    onChange(flowToSteps(nextNodes, nextEdges))
  }

  const pushCurrent = () => {
    onChange(flowToSteps(nodesRef.current, edgesRef.current))
  }

  // 添加节点
  const addNode = (type, pos) => {
    const idxs = nodesRef.current.map((n) => Number(n.data.step.step_index))
    const nextIndex = idxs.length > 0 ? Math.max(...idxs) + 1 : 0
    const step = {
      step_index: nextIndex,
      step_type: type,
      name: NODE_DEFAULT_NAME[type] || '节点',
      config: { ...(NODE_CONFIG_DEFAULTS[type] || {}) },
      output_var: '',
      next_on_success: -1,
      next_on_failure: -1,
      pos_x: pos ? Math.round(pos.x) : 0,
      pos_y: pos ? Math.round(pos.y) : 0,
    }
    const node = {
      id: String(nextIndex),
      type: 'flowNode',
      position: { x: step.pos_x, y: step.pos_y },
      data: { step, running: false },
    }
    commit([...nodesRef.current, node], edgesRef.current)
    setSelectedId(String(nextIndex))
  }

  const handleNodesChange = (changes) => {
    const nextNodes = applyNodeChanges(changes, nodesRef.current)
    const hasRemove = changes.some((c) => c.type === 'remove')
    // 选中状态同步
    const sel = changes.find((c) => c.type === 'select' && c.selected)
    if (sel) setSelectedId(sel.id)
    const desel = changes.find((c) => c.type === 'select' && c.selected === false)
    if (desel && selectedId === desel.id) setSelectedId(null)

    if (hasRemove) {
      const removed = changes.filter((c) => c.type === 'remove').map((c) => c.id)
      const nextEdges = edgesRef.current.filter(
        (e) => !removed.includes(e.source) && !removed.includes(e.target)
      )
      if (removed.includes(String(selectedId))) setSelectedId(null)
      commit(nextNodes, nextEdges)
    } else {
      nodesRef.current = nextNodes
      setNodes(nextNodes)
    }
  }

  const handleEdgesChange = (changes) => {
    const nextEdges = applyEdgeChanges(changes, edgesRef.current)
    if (changes.some((c) => c.type === 'remove')) {
      commit(nodesRef.current, nextEdges)
    } else {
      edgesRef.current = nextEdges
      setEdges(nextEdges)
    }
  }

  const handleConnect = (params) => {
    const newEdge = { ...params, markerEnd: { type: MarkerType.ArrowClosed, width: 15, height: 15 } }
    if (params.sourceHandle === 'failure') {
      newEdge.style = { stroke: '#f87171' }
    }
    commit(nodesRef.current, addEdge(newEdge, edgesRef.current))
  }

  const isValidConnection = useCallback((conn) => {
    if (!conn.source || !conn.target) return false
    const s = nodesRef.current.find((n) => n.id === conn.source)
    if (!s) return false
    if (conn.sourceHandle === 'failure' && !CONDITIONAL_TYPES.includes(s.data.step.step_type)) {
      return false
    }
    // 每个句柄最多一条出边（自环=重试允许，但仅限 success 句柄出度 1 / failure 句柄出度 1）
    const exists = edgesRef.current.find(
      (e) => e.source === conn.source && e.sourceHandle === conn.sourceHandle
    )
    return !exists
  }, [])

  const onNodeDragStop = () => pushCurrent()

  const onDragOver = (e) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
  }

  const onDrop = (e) => {
    e.preventDefault()
    const type = e.dataTransfer.getData('application/along-node')
    if (!type) return
    addNode(type, screenToFlowPosition({ x: e.clientX, y: e.clientY }))
  }

  const deleteSelected = () => {
    if (!selectedId) return
    const id = selectedId
    commit(
      nodesRef.current.filter((n) => n.id !== id),
      edgesRef.current.filter((e) => e.source !== id && e.target !== id)
    )
    setSelectedId(null)
  }

  const duplicateSelected = () => {
    const n = nodesRef.current.find((x) => x.id === selectedId)
    if (!n) return
    const idxs = nodesRef.current.map((x) => Number(x.data.step.step_index))
    const nextIndex = Math.max(...idxs) + 1
    const orig = n.data.step
    const step = {
      ...orig,
      step_index: nextIndex,
      name: `${orig.name || '节点'} (副本)`,
      pos_x: (orig.pos_x || 0) + 40,
      pos_y: (orig.pos_y || 0) + 40,
      next_on_success: -1,
      next_on_failure: -1,
    }
    const node = { ...n, id: String(nextIndex), position: { x: step.pos_x, y: step.pos_y }, data: { step, running: false } }
    commit([...nodesRef.current, node], edgesRef.current)
    setSelectedId(String(nextIndex))
  }

  const doAutoLayout = () => {
    const stepsArr = flowToSteps(nodesRef.current, edgesRef.current)
    const pos = autoLayoutPositions(stepsArr)
    const nextNodes = nodesRef.current.map((n) => ({
      ...n,
      position: pos[Number(n.id)] || n.position,
    }))
    commit(nextNodes, edgesRef.current)
  }

  const updateSelected = (patch) => {
    if (!selectedId) return
    commit(
      nodesRef.current.map((n) =>
        n.id === selectedId
          ? { ...n, data: { ...n.data, step: { ...n.data.step, ...patch } } }
          : n
      ),
      edgesRef.current
    )
  }

  const updateSelectedConfig = (patch) => {
    if (!selectedId) return
    commit(
      nodesRef.current.map((n) =>
        n.id === selectedId
          ? {
              ...n,
              data: {
                ...n.data,
                step: { ...n.data.step, config: { ...n.data.step.config, ...patch } },
              },
            }
          : n
      ),
      edgesRef.current
    )
  }

  const selectedStep = selectedId ? nodes.find((n) => n.id === selectedId)?.data?.step : null
  const availableVars = useMemo(
    () =>
      nodes
        .filter((n) => n.id !== selectedId && n.data && n.data.step.output_var)
        .map((n) => ({ var: n.data.step.output_var, label: n.data.step.name || n.data.step.step_type })),
    [nodes, selectedId]
  )

  return (
    <div className="flex h-full bg-bg">
      {/* 左侧节点面板（可收起，收起后画布占据更大区域） */}
      {leftCollapsed ? (
        <div className="w-9 shrink-0 border-r border-border bg-surface-subtle/40 flex flex-col items-center pt-2">
          <button
            onClick={() => setLeftCollapsed(false)}
            title="展开节点库"
            className="p-1.5 text-text-muted hover:text-text rounded-lg transition-colors"
          >
            <ChevronRight className="w-4 h-4" />
          </button>
        </div>
      ) : (
        <NodePalette onAdd={(type) => addNode(type, { x: 80, y: 80 })} onToggle={() => setLeftCollapsed(true)} />
      )}

      {/* 中间画布 */}
      <div className="flex-1 relative min-w-0">
        {/* 顶部工具栏 */}
        <div className="absolute top-2 left-1/2 -translate-x-1/2 z-10 flex items-center gap-1 bg-surface/90 backdrop-blur border border-border rounded-xl px-2 py-1.5 shadow-lg">
          <button
            onClick={doAutoLayout}
            title="自动布局"
            className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs text-text-muted hover:text-text hover:bg-surface-subtle rounded-lg transition-colors"
          >
            <LayoutGrid className="w-3.5 h-3.5" />
            自动布局
          </button>
          <button
            onClick={duplicateSelected}
            disabled={!selectedId}
            title="复制节点"
            className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs text-text-muted hover:text-text hover:bg-surface-subtle rounded-lg transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <Copy className="w-3.5 h-3.5" />
            复制
          </button>
          <button
            onClick={deleteSelected}
            disabled={!selectedId}
            title="删除节点"
            className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs text-text-muted hover:text-danger-400 hover:bg-danger-500/10 rounded-lg transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <Trash2 className="w-3.5 h-3.5" />
            删除
          </button>
        </div>

        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          onNodesChange={handleNodesChange}
          onEdgesChange={handleEdgesChange}
          onConnect={handleConnect}
          isValidConnection={isValidConnection}
          onNodeDragStop={onNodeDragStop}
          onDrop={onDrop}
          onDragOver={onDragOver}
          onPaneClick={() => setSelectedId(null)}
          fitView
          fitViewOptions={{ padding: 0.15 }}
          proOptions={{ hideAttribution: true }}
          defaultEdgeOptions={{ markerEnd: { type: MarkerType.ArrowClosed, width: 15, height: 15 } }}
          minZoom={0.2}
          maxZoom={2}
        >
          <Background variant={BackgroundVariant.Dots} gap={22} size={1.2} />
          <Controls position="bottom-left" />
          <MiniMap
            pannable
            zoomable
            position="bottom-right"
            className="!bg-surface-subtle/80"
            maskColor="rgba(0,0,0,0.4)"
            nodeColor={(n) => '#22d3ee'}
          />
        </ReactFlow>

        {/* 空态提示 */}
        {nodes.length === 0 && (
          <div className="absolute inset-0 flex flex-col items-center justify-center text-text-subtle pointer-events-none">
            <MousePointerClick className="w-8 h-8 mb-2 opacity-50" />
            <p className="text-sm">从左侧拖拽节点到此处，或双击节点添加</p>
            <p className="text-xs mt-1">从一个「开始」节点出发搭建流程</p>
          </div>
        )}
      </div>

      {/* 右侧配置面板（可收起，收起后画布占据更大区域） */}
      {rightCollapsed ? (
        <div className="w-9 shrink-0 border-l border-border bg-surface-subtle/40 flex flex-col items-center pt-2">
          <button
            onClick={() => setRightCollapsed(false)}
            title="展开配置面板"
            className="p-1.5 text-text-muted hover:text-text rounded-lg transition-colors"
          >
            <ChevronLeft className="w-4 h-4" />
          </button>
        </div>
      ) : (
        <div className="w-72 shrink-0 border-l border-border bg-surface-subtle/30">
          <NodeConfigPanel
            step={selectedStep}
            onChange={updateSelected}
            onConfigChange={updateSelectedConfig}
            availableVars={availableVars}
            onDelete={deleteSelected}
            onDuplicate={duplicateSelected}
            onToggle={() => setRightCollapsed(true)}
          />
        </div>
      )}
    </div>
  )
}

export default function FlowchartEditor(props) {
  return (
    <ReactFlowProvider>
      <FlowchartEditorInner {...props} />
    </ReactFlowProvider>
  )
}
