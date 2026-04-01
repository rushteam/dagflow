import { useState, useEffect, useCallback, useMemo } from 'react'
import { Dialog } from '@base-ui/react/dialog'
import { Drawer } from '@base-ui/react/drawer'
import { Input } from '@base-ui/react/input'
import { Select } from '@base-ui/react/select'
import { Switch } from '@base-ui/react/switch'
import { api, type Task, type TaskInput, type TaskVariable, type KindInfo, type TaskRun, type ChildRun } from '../api/client'
import styles from './TaskPanel.module.css'

interface TaskPanelProps {
  onNavigateToRunLog?: () => void
}

export default function TaskPanel({ onNavigateToRunLog }: TaskPanelProps) {
  const [tasks, setTasks] = useState<Task[]>([])
  const [kinds, setKinds] = useState<KindInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<Task | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Task | null>(null)
  const [runsTarget, setRunsTarget] = useState<Task | null>(null)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const [t, k] = await Promise.all([api.listTasks(), api.listKinds()])
      setTasks(t)
      setKinds(k)
    } catch { /* ignore */ }
    finally { setLoading(false) }
  }, [])

  useEffect(() => { fetchData() }, [fetchData])

  const handleToggle = async (t: Task) => {
    try {
      await api.updateTask(t.id, {
        name: t.name, label: t.label, kind: t.kind,
        payload: t.payload, variables: t.variables ?? [], enabled: !t.enabled,
      })
      fetchData()
    } catch { /* ignore */ }
  }

  const confirmDelete = async () => {
    if (!deleteTarget) return
    try {
      await api.deleteTask(deleteTarget.id)
      fetchData()
    } catch { /* ignore */ }
    finally { setDeleteTarget(null) }
  }

  const [runningId, setRunningId] = useState<number | null>(null)
  const [varsTarget, setVarsTarget] = useState<Task | null>(null)

  const doRun = async (t: Task, vars?: Record<string, string>) => {
    if (runningId === t.id) return
    setRunningId(t.id)
    const minWait = new Promise(r => setTimeout(r, 1000))
    try {
      await Promise.all([api.runTask(t.id, vars), minWait])
    } catch { /* ignore */ }
    finally { setRunningId(null) }
  }

  const handleRun = (t: Task) => {
    if (runningId === t.id) return
    const hasVars = t.variables && t.variables.length > 0
    if (hasVars) {
      setVarsTarget(t)
    } else {
      doRun(t)
    }
  }

  const openCreate = () => { setEditing(null); setShowForm(true) }
  const openEdit = (task: Task) => { setEditing(task); setShowForm(true) }

  const kindLabel = (name: string) => kinds.find(k => k.name === name)?.label ?? name

  return (
    <div>
      <div className={styles.panelHeader}>
        <h2 className={styles.panelTitle}>任务管理</h2>
        <div className={styles.actions}>
          <button className={styles.refreshBtn} onClick={fetchData} disabled={loading}>
            {loading ? '刷新中...' : '刷新'}
          </button>
          <button className={styles.createBtn} onClick={openCreate}>新建任务</button>
        </div>
      </div>

      <div className={styles.tableWrap}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>名称</th>
              <th>标签</th>
              <th>类型</th>
              <th>Payload</th>
              <th>启用</th>
              <th>创建时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {tasks.map(t => (
              <tr key={t.id}>
                <td className={styles.nameCell}>{t.name}</td>
                <td>{t.label || '-'}</td>
                <td><span className={styles.kindBadge}>{kindLabel(t.kind)}</span></td>
                <td className={styles.payloadCell} title={JSON.stringify(t.payload)}>
                  {JSON.stringify(t.payload)}
                </td>
                <td>
                  <Switch.Root
                    className={styles.switch}
                    checked={t.enabled}
                    onCheckedChange={() => handleToggle(t)}
                  >
                    <Switch.Thumb className={styles.switchThumb} />
                  </Switch.Root>
                </td>
                <td>{formatTime(t.created_at)}</td>
                <td className={styles.actionCell}>
                  <button className={styles.actionBtnPrimary} disabled={runningId === t.id}
                    onClick={() => handleRun(t)}>
                    {runningId === t.id ? '执行中...' : '执行'}
                  </button>
                  <button className={styles.actionBtn} onClick={() => setRunsTarget(t)}>日志</button>
                  <button className={styles.actionBtn} onClick={() => openEdit(t)}>编辑</button>
                  <button className={styles.actionBtnDanger} onClick={() => setDeleteTarget(t)}>删除</button>
                </td>
              </tr>
            ))}
            {tasks.length === 0 && !loading && (
              <tr><td colSpan={7} className={styles.empty}>暂无任务数据</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {showForm && (
        <TaskFormDialog
          kinds={kinds}
          task={editing}
          onClose={() => setShowForm(false)}
          onSaved={() => { setShowForm(false); fetchData() }}
        />
      )}

      <Dialog.Root open={!!deleteTarget} onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}>
        <Dialog.Portal>
          <Dialog.Backdrop className={styles.backdrop} />
          <Dialog.Popup className={styles.confirmDialog}>
            <Dialog.Title className={styles.dialogTitle}>确认删除</Dialog.Title>
            <Dialog.Description className={styles.confirmDesc}>
              确定删除任务「{deleteTarget?.name}」吗？关联的调度也将无法执行。
            </Dialog.Description>
            <div className={styles.formActions}>
              <Dialog.Close className={styles.cancelBtn}>取消</Dialog.Close>
              <button className={styles.dangerBtn} onClick={confirmDelete}>删除</button>
            </div>
          </Dialog.Popup>
        </Dialog.Portal>
      </Dialog.Root>

      {runsTarget && (
        <TaskRunsDialog task={runsTarget} onClose={() => setRunsTarget(null)}
          onNavigateToRunLog={onNavigateToRunLog} />
      )}

      {varsTarget && (
        <VarsRunDialog task={varsTarget}
          running={runningId === varsTarget.id}
          onRun={(vars) => { doRun(varsTarget, vars); setVarsTarget(null) }}
          onClose={() => setVarsTarget(null)} />
      )}
    </div>
  )
}

// ---- Form Dialog ----

function TaskFormDialog({ kinds, task, onClose, onSaved }: {
  kinds: KindInfo[]
  task: Task | null
  onClose: () => void
  onSaved: () => void
}) {
  const isEdit = !!task
  const [allTasks, setAllTasks] = useState<Task[]>([])
  const [form, setForm] = useState<TaskInput>(() => {
    if (task) return {
      name: task.name,
      label: task.label,
      kind: task.kind,
      payload: task.payload,
      variables: task.variables ?? [],
      enabled: task.enabled,
    }
    return {
      name: '', label: '', kind: kinds[0]?.name ?? 'shell',
      payload: {}, variables: [], enabled: true,
    }
  })

  useEffect(() => { api.listTasks().then(setAllTasks).catch(() => {}) }, [])

  // shell 专用状态
  const [commandsText, setCommandsText] = useState(() => {
    if (task?.kind === 'shell' && Array.isArray((task.payload as { commands?: string[] }).commands)) {
      return (task.payload as { commands: string[] }).commands.join('\n')
    }
    return ''
  })

  // http 专用状态
  const initHttp = (): HttpFields => {
    if (task?.kind === 'http') {
      const p = task.payload as Record<string, unknown>
      return {
        url: (p.url as string) ?? '',
        method: (p.method as string) ?? 'GET',
        headers: p.headers ? Object.entries(p.headers as Record<string, string>).map(([k, v]) => `${k}: ${v}`).join('\n') : '',
        body: (p.body as string) ?? '',
        timeout: (p.timeout as number) ?? 30,
      }
    }
    return { url: '', method: 'GET', headers: '', body: '', timeout: 30 }
  }
  const [httpFields, setHttpFields] = useState<HttpFields>(initHttp)
  const setHttp = (patch: Partial<HttpFields>) => setHttpFields(prev => ({ ...prev, ...patch }))

  // dag 专用状态
  const initDagNodes = (): DagNodeField[] => {
    if (task?.kind === 'dag') {
      const p = task.payload as { nodes?: DagNodeField[]; strategy?: string }
      return p.nodes ?? []
    }
    return []
  }
  const [dagNodes, setDagNodes] = useState<DagNodeField[]>(initDagNodes)
  const [dagStrategy, setDagStrategy] = useState<string>(() => {
    if (task?.kind === 'dag') return ((task.payload as Record<string, unknown>).strategy as string) || 'fail_fast'
    return 'fail_fast'
  })

  // 通用 fallback 状态
  const [rawPayload, setRawPayload] = useState(() => {
    if (task && task.kind !== 'shell' && task.kind !== 'http' && task.kind !== 'dag') return JSON.stringify(task.payload, null, 2)
    return '{}'
  })
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const set = (patch: Partial<TaskInput>) => setForm(prev => ({ ...prev, ...patch }))

  const currentKind = form.kind
  const kindHint = kinds.find(k => k.name === currentKind)?.payload_hint

  const handleKindChange = (newKind: string) => {
    set({ kind: newKind })
    if (newKind === 'shell') {
      setCommandsText('')
    } else if (newKind === 'http') {
      setHttpFields({ url: '', method: 'GET', headers: '', body: '', timeout: 30 })
    } else if (newKind === 'dag') {
      setDagNodes([])
      setDagStrategy('fail_fast')
    } else {
      const hint = kinds.find(k => k.name === newKind)?.payload_hint
      setRawPayload(hint ?? '{}')
    }
  }

  const buildPayload = (): Record<string, unknown> | null => {
    if (currentKind === 'shell') {
      const commands = commandsText.split('\n').map(l => l.trim()).filter(Boolean)
      if (commands.length === 0) { setError('请至少输入一条命令'); return null }
      return { commands }
    }
    if (currentKind === 'http') {
      if (!httpFields.url.trim()) { setError('URL 不能为空'); return null }
      const headers: Record<string, string> = {}
      for (const line of httpFields.headers.split('\n').filter(Boolean)) {
        const idx = line.indexOf(':')
        if (idx <= 0) { setError(`Header 格式错误: "${line}"，应为 Key: Value`); return null }
        headers[line.slice(0, idx).trim()] = line.slice(idx + 1).trim()
      }
      return {
        url: httpFields.url.trim(),
        method: httpFields.method || 'GET',
        headers,
        body: httpFields.body,
        timeout: httpFields.timeout || 30,
      }
    }
    if (currentKind === 'dag') {
      if (dagNodes.length === 0) { setError('DAG 至少需要一个节点'); return null }
      for (const n of dagNodes) {
        if (!n.name.trim()) { setError('节点名不能为空'); return null }
        if (!n.task_id) { setError(`节点「${n.name}」未选择任务`); return null }
      }
      return { nodes: dagNodes, strategy: dagStrategy }
    }
    try { return JSON.parse(rawPayload) } catch {
      setError('Payload JSON 格式无效'); return null
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    const payload = buildPayload()
    if (!payload) return

    const data: TaskInput = { ...form, payload }

    setSubmitting(true)
    try {
      if (isEdit && task) {
        await api.updateTask(task.id, data)
      } else {
        await api.createTask(data)
      }
      onSaved()
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Drawer.Root open onOpenChange={(open) => { if (!open) onClose() }} swipeDirection="right">
      <Drawer.Portal>
        <Drawer.Backdrop className={styles.drawerBackdrop} />
        <Drawer.Viewport className={styles.drawerViewport}>
          <Drawer.Popup className={styles.drawerPopup}>
            <Drawer.Content className={styles.drawerContent}>
              <div className={styles.drawerHeader}>
                <Drawer.Title className={styles.drawerTitle}>
                  {isEdit ? '编辑任务' : '新建任务'}
                </Drawer.Title>
                <Drawer.Close className={styles.drawerClose}>×</Drawer.Close>
              </div>

              {error && <div className={styles.formError}>{error}</div>}

              <form onSubmit={handleSubmit} className={styles.form}>
                <label className={styles.formLabel}>
                  名称（唯一标识）
                  <Input className={styles.formInput} value={form.name}
                    onChange={(e) => set({ name: e.target.value })} required
                    placeholder="输入任务名称（英文标识）" />
                </label>

                <label className={styles.formLabel}>
                  标签（显示名）
                  <Input className={styles.formInput} value={form.label}
                    onChange={(e) => set({ label: e.target.value })}
                    placeholder="输入显示名称（中文）" />
                </label>

                <div className={styles.formLabel}>
                  类型（Kind）
                  <Select.Root value={form.kind} onValueChange={(val) => handleKindChange(val as string)}>
                    <Select.Trigger className={styles.selectTrigger}>
                      <Select.Value placeholder="选择类型" />
                      <Select.Icon className={styles.selectIcon}><ChevronIcon /></Select.Icon>
                    </Select.Trigger>
                    <Select.Portal>
                      <Select.Positioner className={styles.selectPositioner} sideOffset={4}>
                        <Select.Popup className={styles.selectPopup}>
                          <Select.List>
                            {kinds.map(k => (
                              <Select.Item key={k.name} value={k.name} className={styles.selectItem}>
                                <Select.ItemText>{k.label}（{k.name}）</Select.ItemText>
                                <Select.ItemIndicator className={styles.selectIndicator}>
                                  <CheckIcon />
                                </Select.ItemIndicator>
                              </Select.Item>
                            ))}
                          </Select.List>
                        </Select.Popup>
                      </Select.Positioner>
                    </Select.Portal>
                  </Select.Root>
                </div>

                {currentKind === 'shell' ? (
                  <label className={styles.formLabel}>
                    命令列表
                    <textarea className={styles.formTextarea} value={commandsText}
                      onChange={e => setCommandsText(e.target.value)} rows={5}
                      placeholder={"echo 'hello world'\nls -la"} />
                    <span className={styles.commandsHelp}>每行一条 shell 命令，按顺序执行</span>
                  </label>
                ) : currentKind === 'http' ? (
                  <HttpPayloadForm fields={httpFields} onChange={setHttp} />
                ) : currentKind === 'dag' ? (
                  <DagPayloadForm
                    nodes={dagNodes}
                    strategy={dagStrategy}
                    tasks={allTasks.filter(t => t.id !== task?.id)}
                    onNodesChange={setDagNodes}
                    onStrategyChange={setDagStrategy}
                  />
                ) : (
                  <label className={styles.formLabel}>
                    Payload（JSON）
                    <textarea className={styles.formTextarea} value={rawPayload}
                      onChange={e => setRawPayload(e.target.value)} rows={5} />
                    {kindHint && <span className={styles.hint}>示例：{kindHint}</span>}
                  </label>
                )}

                <VariablesEditor
                  variables={form.variables}
                  onChange={(variables) => set({ variables })}
                />

                <div className={styles.formSwitchRow}>
                  <span className={styles.formSwitchLabel}>启用</span>
                  <Switch.Root
                    className={styles.switch}
                    checked={form.enabled}
                    onCheckedChange={(checked) => set({ enabled: checked })}
                  >
                    <Switch.Thumb className={styles.switchThumb} />
                  </Switch.Root>
                </div>

                <div className={styles.drawerFooter}>
                  <button type="button" className={styles.cancelBtn} onClick={onClose}>取消</button>
                  <button type="submit" className={styles.submitBtn} disabled={submitting}>
                    {submitting ? '提交中...' : isEdit ? '保存' : '创建'}
                  </button>
                </div>
              </form>
            </Drawer.Content>
          </Drawer.Popup>
        </Drawer.Viewport>
      </Drawer.Portal>
    </Drawer.Root>
  )
}

// ---- Runs Dialog ----

function TaskRunsDialog({ task, onClose, onNavigateToRunLog }: {
  task: Task; onClose: () => void; onNavigateToRunLog?: () => void
}) {
  const [allRuns, setAllRuns] = useState<TaskRun[]>([])
  const [loading, setLoading] = useState(true)

  const fetchRuns = useCallback(async () => {
    setLoading(true)
    try { setAllRuns(await api.listTaskRuns(task.id)) } catch { /* ignore */ }
    finally { setLoading(false) }
  }, [task.id])

  useEffect(() => { fetchRuns() }, [fetchRuns])

  const runs = allRuns.slice(0, 10)
  const hasMore = allRuns.length > 10

  const statusBadge = (s: string) => {
    const cls = s === 'success' ? styles.statusSuccess
      : s === 'failed' ? styles.statusFailed
      : s === 'cancelled' ? styles.statusCancelled
      : styles.statusRunning
    return <span className={`${styles.statusBadge} ${cls}`}>{s}</span>
  }

  const handleCancel = async (runId: number) => {
    try {
      await api.cancelTaskRun(runId)
      setTimeout(fetchRuns, 500)
    } catch { /* ignore */ }
  }

  const triggerLabel = (r: TaskRun) => {
    if (r.trigger_type === 'schedule') return `调度 #${r.trigger_id ?? ''}`
    if (r.trigger_type === 'dag') return 'DAG'
    return '手动'
  }

  const handleViewMore = () => {
    onClose()
    onNavigateToRunLog?.()
  }

  return (
    <Dialog.Root open onOpenChange={(open) => { if (!open) onClose() }}>
      <Dialog.Portal>
        <Dialog.Backdrop className={styles.backdrop} />
        <Dialog.Popup className={styles.runsDialog}>
          <Dialog.Title className={styles.dialogTitle}>
            运行日志 — {task.label || task.name}
          </Dialog.Title>

          <div className={styles.runsToolbar}>
            <span className={styles.runsCount}>最近 {runs.length} 条{hasMore ? `（共 ${allRuns.length} 条）` : ''}</span>
            <button className={styles.refreshBtn} onClick={fetchRuns} disabled={loading}>
              {loading ? '刷新中...' : '刷新'}
            </button>
          </div>

          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>ID</th>
                  <th>触发方式</th>
                  <th>状态</th>
                  <th>开始时间</th>
                  <th>耗时</th>
                  <th>错误信息</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {runs.map(r => (
                  <RunRow key={r.id} run={r} isDag={task.kind === 'dag'}
                    statusBadge={statusBadge} triggerLabel={triggerLabel}
                    onCancel={handleCancel} />
                ))}
                {runs.length === 0 && !loading && (
                  <tr><td colSpan={7} className={styles.empty}>暂无运行记录</td></tr>
                )}
              </tbody>
            </table>
          </div>

          <div className={styles.formActions}>
            {hasMore && onNavigateToRunLog && (
              <button type="button" className={styles.viewMoreBtn} onClick={handleViewMore}>
                查看更多 →
              </button>
            )}
            <Dialog.Close className={styles.cancelBtn}>关闭</Dialog.Close>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}

function ChevronIcon() {
  return (
    <svg width="8" height="12" viewBox="0 0 8 12" fill="none" stroke="currentColor" strokeWidth="1.5">
      <path d="M0.5 4.5L4 1.5L7.5 4.5" />
      <path d="M0.5 7.5L4 10.5L7.5 7.5" />
    </svg>
  )
}

function CheckIcon() {
  return (
    <svg fill="currentColor" width="10" height="10" viewBox="0 0 10 10">
      <path d="M9.1603 1.12218C9.50684 1.34873 9.60427 1.81354 9.37792 2.16038L5.13603 8.66012C5.01614 8.8438 4.82192 8.96576 4.60451 8.99384C4.3871 9.02194 4.1683 8.95335 4.00574 8.80615L1.24664 6.30769C0.939709 6.02975 0.916013 5.55541 1.19372 5.24822C1.47142 4.94102 1.94536 4.91731 2.2523 5.19524L4.36085 7.10461L8.12299 1.33999C8.34934 0.993152 8.81376 0.895638 9.1603 1.12218Z" />
    </svg>
  )
}

// ---- Run Row (with DAG child expand) ----

export function RunRow({ run, isDag, statusBadge, triggerLabel, onCancel }: {
  run: TaskRun; isDag: boolean;
  statusBadge: (s: string) => React.ReactNode;
  triggerLabel: (r: TaskRun) => string;
  onCancel: (runId: number) => void;
}) {
  const [expanded, setExpanded] = useState(false)
  const [children, setChildren] = useState<ChildRun[]>([])
  const [loadingChildren, setLoadingChildren] = useState(false)

  const toggleExpand = async () => {
    if (expanded) { setExpanded(false); return }
    setLoadingChildren(true)
    try { setChildren(await api.listChildRuns(run.id)) } catch { /* ignore */ }
    finally { setLoadingChildren(false); setExpanded(true) }
  }

  return (
    <>
      <tr>
        <td>
          {isDag && (
            <button className={styles.expandBtn} onClick={toggleExpand}>
              {expanded ? '▾' : '▸'}
            </button>
          )}
          {run.id}
        </td>
        <td>{triggerLabel(run)}</td>
        <td>{statusBadge(run.status)}</td>
        <td>{formatTime(run.started_at)}</td>
        <td>{run.duration_ms != null ? `${run.duration_ms}ms` : '-'}</td>
        <td className={styles.errorCell} title={run.error_msg ?? ''}>
          {run.error_msg || '-'}
        </td>
        <td>
          {run.status === 'running' && (
            <button className={styles.cancelRunBtn} onClick={() => onCancel(run.id)}>
              停止
            </button>
          )}
        </td>
      </tr>
      {expanded && (
        loadingChildren ? (
          <tr><td colSpan={7} className={styles.childLoading}>加载子任务...</td></tr>
        ) : children.length > 0 ? (
          children.map(c => (
            <tr key={c.id} className={styles.childRow}>
              <td className={styles.childIndent}>↳ {c.id}</td>
              <td>{c.task_name}</td>
              <td>{statusBadge(c.status)}</td>
              <td>{formatTime(c.started_at)}</td>
              <td>{c.duration_ms != null ? `${c.duration_ms}ms` : '-'}</td>
              <td className={styles.errorCell} title={c.error_msg ?? ''}>
                {c.error_msg || '-'}
              </td>
              <td></td>
            </tr>
          ))
        ) : (
          <tr><td colSpan={7} className={styles.childLoading}>无子任务记录</td></tr>
        )
      )}
    </>
  )
}

// ---- DAG Visual Editor ----

interface DagNodeField {
  name: string
  task_id: number
  depends_on: string[]
}

const NODE_W = 150, NODE_H = 56, GAP_X = 40, GAP_Y = 80, PAD = 24

interface LayoutNode { name: string; taskId: number; x: number; y: number }
interface LayoutArrow { from: string; to: string; x1: number; y1: number; x2: number; y2: number }
interface LayoutResult { nodes: LayoutNode[]; arrows: LayoutArrow[]; width: number; height: number }

function computeDAGLayout(nodes: DagNodeField[]): LayoutResult {
  if (nodes.length === 0) return { nodes: [], arrows: [], width: 300, height: 100 }

  const nameMap = new Map(nodes.map(n => [n.name, n]))
  const inDeg = new Map(nodes.map(n => [n.name, 0]))
  const children = new Map<string, string[]>()

  for (const n of nodes) {
    for (const dep of n.depends_on) {
      if (nameMap.has(dep)) {
        inDeg.set(n.name, (inDeg.get(n.name) ?? 0) + 1)
        children.set(dep, [...(children.get(dep) ?? []), n.name])
      }
    }
  }

  const layers: string[][] = []
  const remaining = new Map(inDeg)
  const placed = new Set<string>()

  while (placed.size < nodes.length) {
    const layer = [...remaining.entries()].filter(([, d]) => d === 0).map(([n]) => n)
    if (layer.length === 0) {
      for (const n of nodes) if (!placed.has(n.name)) layer.push(n.name)
      layers.push(layer)
      break
    }
    for (const name of layer) {
      remaining.delete(name)
      placed.add(name)
      for (const child of children.get(name) ?? []) {
        remaining.set(child, (remaining.get(child) ?? 1) - 1)
      }
    }
    layers.push(layer)
  }

  const maxPerLayer = Math.max(...layers.map(l => l.length))
  const canvasW = Math.max(300, maxPerLayer * (NODE_W + GAP_X) - GAP_X + PAD * 2)
  const canvasH = Math.max(100, layers.length * (NODE_H + GAP_Y) - GAP_Y + PAD * 2)

  const layoutNodes: LayoutNode[] = []
  const nodePos = new Map<string, { x: number; y: number }>()

  for (let li = 0; li < layers.length; li++) {
    const layer = layers[li]
    const layerW = layer.length * (NODE_W + GAP_X) - GAP_X
    const startX = (canvasW - layerW) / 2
    for (let ni = 0; ni < layer.length; ni++) {
      const name = layer[ni]
      const node = nameMap.get(name)!
      const x = startX + ni * (NODE_W + GAP_X)
      const y = PAD + li * (NODE_H + GAP_Y)
      layoutNodes.push({ name, taskId: node.task_id, x, y })
      nodePos.set(name, { x, y })
    }
  }

  const arrows: LayoutArrow[] = []
  for (const n of nodes) {
    const to = nodePos.get(n.name)
    if (!to) continue
    for (const dep of n.depends_on) {
      const from = nodePos.get(dep)
      if (!from) continue
      arrows.push({
        from: dep, to: n.name,
        x1: from.x + NODE_W / 2, y1: from.y + NODE_H,
        x2: to.x + NODE_W / 2, y2: to.y,
      })
    }
  }

  return { nodes: layoutNodes, arrows, width: canvasW, height: canvasH }
}

function wouldCreateCycle(nodes: DagNodeField[], from: string, to: string): boolean {
  const adj = new Map<string, string[]>()
  for (const n of nodes) {
    for (const dep of n.depends_on) adj.set(dep, [...(adj.get(dep) ?? []), n.name])
  }
  adj.set(from, [...(adj.get(from) ?? []), to])

  const visited = new Set<string>()
  const stack = [to]
  while (stack.length) {
    const cur = stack.pop()!
    if (cur === from) return true
    if (visited.has(cur)) continue
    visited.add(cur)
    for (const next of adj.get(cur) ?? []) stack.push(next)
  }
  return false
}

function DagPayloadForm({ nodes, strategy, tasks, onNodesChange, onStrategyChange }: {
  nodes: DagNodeField[]
  strategy: string
  tasks: Task[]
  onNodesChange: (nodes: DagNodeField[]) => void
  onStrategyChange: (s: string) => void
}) {
  const [connectFrom, setConnectFrom] = useState<string | null>(null)
  const [selectedNode, setSelectedNode] = useState<string | null>(null)
  const [panelMode, setPanelMode] = useState<'add' | 'edit' | null>(null)
  const [formName, setFormName] = useState('')
  const [formTaskId, setFormTaskId] = useState<number>(0)
  const [cycleWarning, setCycleWarning] = useState<string | null>(null)

  const layout = useMemo(() => computeDAGLayout(nodes), [nodes])

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { setConnectFrom(null); setCycleWarning(null) }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  useEffect(() => {
    if (cycleWarning) {
      const t = setTimeout(() => setCycleWarning(null), 2500)
      return () => clearTimeout(t)
    }
  }, [cycleWarning])

  const taskLabel = (tid: number) => {
    const t = tasks.find(t => t.id === tid)
    return t ? (t.label || t.name) : `#${tid}`
  }
  const taskKind = (tid: number) => tasks.find(t => t.id === tid)?.kind ?? ''

  const openAddPanel = () => {
    setSelectedNode(null)
    setFormName('')
    setFormTaskId(0)
    setPanelMode('add')
  }

  const openEditPanel = (name: string) => {
    const node = nodes.find(n => n.name === name)
    if (!node) return
    setSelectedNode(name)
    setFormName(node.name)
    setFormTaskId(node.task_id)
    setPanelMode('edit')
  }

  const closePanel = () => {
    setPanelMode(null)
    setSelectedNode(null)
  }

  const tryConnect = (from: string, to: string) => {
    const node = nodes.find(n => n.name === to)
    if (!node || node.depends_on.includes(from)) return
    if (wouldCreateCycle(nodes, from, to)) {
      setCycleWarning(`${from} → ${to} 会产生循环依赖`)
      return
    }
    onNodesChange(nodes.map(n =>
      n.name === to ? { ...n, depends_on: [...n.depends_on, from] } : n
    ))
  }

  const handleNodeClick = (name: string) => {
    if (connectFrom === '__pending__') {
      setConnectFrom(name)
      setCycleWarning(null)
      return
    }
    if (connectFrom) {
      if (connectFrom !== name) tryConnect(connectFrom, name)
      setConnectFrom(null)
    } else {
      if (selectedNode === name) closePanel()
      else openEditPanel(name)
    }
  }

  const handleStartConnect = (name: string, e: React.MouseEvent) => {
    e.stopPropagation()
    setConnectFrom(name)
    setPanelMode(null)
    setSelectedNode(null)
    setCycleWarning(null)
  }

  const removeNode = (name: string) => {
    onNodesChange(
      nodes.filter(n => n.name !== name)
        .map(n => ({ ...n, depends_on: n.depends_on.filter(d => d !== name) }))
    )
    if (selectedNode === name) closePanel()
  }

  const removeArrow = (from: string, to: string) => {
    onNodesChange(nodes.map(n =>
      n.name === to ? { ...n, depends_on: n.depends_on.filter(d => d !== from) } : n
    ))
  }

  const handleSubmit = () => {
    if (!formName.trim() || !formTaskId) return
    if (panelMode === 'add') {
      if (nodes.some(n => n.name === formName.trim())) return
      onNodesChange([...nodes, { name: formName.trim(), task_id: formTaskId, depends_on: [] }])
      setFormName('')
      setFormTaskId(0)
    } else if (panelMode === 'edit' && selectedNode) {
      const renamed = formName.trim() !== selectedNode
      const nameConflict = renamed && nodes.some(n => n.name === formName.trim())
      if (nameConflict) return
      onNodesChange(nodes.map(n => {
        if (n.name === selectedNode) return { ...n, name: formName.trim(), task_id: formTaskId }
        if (renamed) return { ...n, depends_on: n.depends_on.map(d => d === selectedNode ? formName.trim() : d) }
        return n
      }))
      if (renamed) setSelectedNode(formName.trim())
    }
  }

  const editingNode = panelMode === 'edit' && selectedNode ? nodes.find(n => n.name === selectedNode) : null
  const nameConflict = formName.trim()
    && (panelMode === 'add'
      ? nodes.some(n => n.name === formName.trim())
      : panelMode === 'edit' && selectedNode && formName.trim() !== selectedNode && nodes.some(n => n.name === formName.trim()))
  const showSidebar = panelMode !== null

  return (
    <div className={styles.dagEditor}>
      <div className={styles.dagToolbar}>
        <button type="button"
          className={`${styles.dagToolBtn} ${connectFrom ? styles.dagToolBtnActive : ''}`}
          onClick={() => { setConnectFrom(connectFrom ? null : '__pending__'); setCycleWarning(null) }}>
          {connectFrom ? '点击目标节点...' : '连接节点'}
        </button>
        <button type="button"
          className={`${styles.dagToolBtn} ${panelMode === 'add' ? styles.dagToolBtnActive : ''}`}
          onClick={() => panelMode === 'add' ? closePanel() : openAddPanel()}>
          + 添加节点
        </button>
        <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 6 }}>
          <span className={styles.dagToolLabel}>失败策略</span>
          <Select.Root value={strategy} onValueChange={(v) => { if (v) onStrategyChange(v) }}>
            <Select.Trigger className={styles.dagStrategyTrigger}>
              <Select.Value />
              <Select.Icon className={styles.selectIcon}><ChevronIcon /></Select.Icon>
            </Select.Trigger>
            <Select.Portal>
              <Select.Positioner className={styles.selectPositioner} sideOffset={4}>
                <Select.Popup className={styles.selectPopup}>
                  <Select.List>
                    <Select.Item value="fail_fast" className={styles.selectItem}>
                      <Select.ItemText>快速失败</Select.ItemText>
                      <Select.ItemIndicator className={styles.selectIndicator}><CheckIcon /></Select.ItemIndicator>
                    </Select.Item>
                    <Select.Item value="continue_on_error" className={styles.selectItem}>
                      <Select.ItemText>继续执行</Select.ItemText>
                      <Select.ItemIndicator className={styles.selectIndicator}><CheckIcon /></Select.ItemIndicator>
                    </Select.Item>
                  </Select.List>
                </Select.Popup>
              </Select.Positioner>
            </Select.Portal>
          </Select.Root>
        </div>
      </div>

      <div className={styles.dagBody}>
        <div className={styles.dagCanvasWrap}>
          {cycleWarning && (
            <div className={styles.dagCycleWarn}>{cycleWarning}</div>
          )}
          {connectFrom && connectFrom !== '__pending__' && (
            <div className={styles.dagHint}>从「{connectFrom}」→ 点击目标节点（Esc 取消）</div>
          )}
          {connectFrom === '__pending__' && (
            <div className={styles.dagHint}>点击起始节点</div>
          )}

          <div className={styles.dagCanvas}
            style={{ minHeight: Math.max(120, layout.height) }}
            onClick={() => { setConnectFrom(null); closePanel(); setCycleWarning(null) }}>

            {nodes.length === 0 && (
              <div className={styles.dagEmpty}>点击「添加节点」开始构建 DAG</div>
            )}

            <svg className={styles.dagSvg} width="100%" height={Math.max(120, layout.height)}>
              <defs>
                <marker id="dag-arrow" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
                  <polygon points="0 0, 8 3, 0 6" fill="#999" />
                </marker>
                <marker id="dag-arrow-hl" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
                  <polygon points="0 0, 8 3, 0 6" fill="#cf1322" />
                </marker>
              </defs>
              {layout.arrows.map(a => {
                const midY = (a.y1 + a.y2) / 2
                return (
                  <g key={`${a.from}-${a.to}`} className={styles.dagArrowGroup}
                    onClick={e => { e.stopPropagation(); removeArrow(a.from, a.to) }}>
                    <path d={`M${a.x1},${a.y1} C${a.x1},${midY} ${a.x2},${midY} ${a.x2},${a.y2}`}
                      className={styles.dagArrowHit} />
                    <path d={`M${a.x1},${a.y1} C${a.x1},${midY} ${a.x2},${midY} ${a.x2},${a.y2}`}
                      className={styles.dagArrow} markerEnd="url(#dag-arrow)" />
                  </g>
                )
              })}
            </svg>

            {layout.nodes.map(n => (
              <div key={n.name}
                className={`${styles.dagNodeCard} ${
                  selectedNode === n.name ? styles.dagNodeSelected : ''
                } ${connectFrom && connectFrom !== '__pending__' && connectFrom !== n.name ? styles.dagNodeConnectTarget : ''
                } ${connectFrom === n.name ? styles.dagNodeConnectSource : ''}`}
                style={{ left: n.x, top: n.y, width: NODE_W, height: NODE_H }}
                onClick={e => { e.stopPropagation(); handleNodeClick(n.name) }}>
                <div className={styles.dagNodeCardName}>{n.name}</div>
                <div className={styles.dagNodeCardTask}>
                  {n.taskId ? taskLabel(n.taskId) : '未选择任务'}
                  {taskKind(n.taskId) && <span className={styles.dagNodeCardKind}>{taskKind(n.taskId)}</span>}
                </div>
                <button type="button" className={styles.dagNodeCardDel}
                  onClick={e => { e.stopPropagation(); removeNode(n.name) }}>×</button>
                <button type="button" className={styles.dagNodeCardPort}
                  title="从此节点拉线连接"
                  onClick={e => handleStartConnect(n.name, e)}>●</button>
              </div>
            ))}
          </div>
        </div>

        {showSidebar && (
          <div className={styles.dagSidebar} onClick={e => e.stopPropagation()}>
            <div className={styles.dagSideSection}>
              <div className={styles.dagSideTitle}>
                {panelMode === 'add' ? '添加节点' : '节点属性'}
                {panelMode === 'edit' && (
                  <button type="button" className={styles.dagSideDelBtn}
                    onClick={() => removeNode(selectedNode!)}>删除节点</button>
                )}
              </div>

              <label className={styles.formLabel}>
                节点名
                <Input className={styles.formInput} value={formName}
                  onChange={e => setFormName(e.target.value)} placeholder="输入步骤名称"
                  readOnly={panelMode === 'edit'} />
                {nameConflict && <span className={styles.dagFieldErr}>名称已存在</span>}
              </label>

              <div className={styles.formLabel}>
                关联任务
                <Select.Root value={String(formTaskId || '')}
                  onValueChange={v => {
                    setFormTaskId(Number(v))
                    if (panelMode === 'edit' && selectedNode) {
                      onNodesChange(nodes.map(n => n.name === selectedNode ? { ...n, task_id: Number(v) } : n))
                    }
                  }}>
                  <Select.Trigger className={styles.selectTrigger}>
                    <Select.Value placeholder="选择任务" />
                    <Select.Icon className={styles.selectIcon}><ChevronIcon /></Select.Icon>
                  </Select.Trigger>
                  <Select.Portal>
                    <Select.Positioner className={styles.selectPositioner} sideOffset={4}>
                      <Select.Popup className={styles.selectPopup}>
                        <Select.List>
                          {tasks.map(t => (
                            <Select.Item key={t.id} value={String(t.id)} className={styles.selectItem}>
                              <Select.ItemText>{t.label || t.name}（{t.kind}）</Select.ItemText>
                              <Select.ItemIndicator className={styles.selectIndicator}><CheckIcon /></Select.ItemIndicator>
                            </Select.Item>
                          ))}
                        </Select.List>
                      </Select.Popup>
                    </Select.Positioner>
                  </Select.Portal>
                </Select.Root>
              </div>

              {panelMode === 'add' && (
                <button type="button" className={styles.dagAddBtn}
                  onClick={handleSubmit}
                  disabled={!formName.trim() || !formTaskId || !!nameConflict}>
                  + 添加
                </button>
              )}

              {editingNode && editingNode.depends_on.length > 0 && (
                <div className={styles.dagPropRow} style={{ flexDirection: 'column', alignItems: 'flex-start' }}>
                  <span className={styles.dagPropLabel}>上游依赖</span>
                  <div className={styles.dagDetailDeps}>
                    {editingNode.depends_on.map(d => (
                      <span key={d} className={styles.dagDepTag}>
                        {d}
                        <button type="button" onClick={() => removeArrow(d, editingNode.name)}>×</button>
                      </span>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

// ---- Variables Editor (task form) ----

function VariablesEditor({ variables, onChange }: {
  variables: TaskVariable[]
  onChange: (vars: TaskVariable[]) => void
}) {
  const addVar = () => onChange([...variables, { key: '', default_value: '' }])

  const updateVar = (idx: number, patch: Partial<TaskVariable>) => {
    const next = [...variables]
    next[idx] = { ...next[idx], ...patch }
    onChange(next)
  }

  const removeVar = (idx: number) => onChange(variables.filter((_, i) => i !== idx))

  return (
    <div className={styles.varsSection}>
      <div className={styles.varsSectionHeader}>
        <span className={styles.formLabel} style={{ marginBottom: 0 }}>变量定义</span>
        <span className={styles.varsHint}>在 Payload 中使用 {'${KEY}'} 引用</span>
      </div>
      {variables.map((v, idx) => (
        <div key={idx} className={styles.varsRow}>
          <Input className={styles.varsInput} value={v.key}
            onChange={e => updateVar(idx, { key: e.target.value })}
            placeholder="变量名 (KEY)" />
          <Input className={styles.varsInput} value={v.default_value}
            onChange={e => updateVar(idx, { default_value: e.target.value })}
            placeholder="默认值" />
          <button type="button" className={styles.varsRemove} onClick={() => removeVar(idx)}>×</button>
        </div>
      ))}
      <button type="button" className={styles.varsAdd} onClick={addVar}>+ 添加变量</button>
    </div>
  )
}

// ---- Variables Run Dialog (prompt before execution) ----

function VarsRunDialog({ task, running, onRun, onClose }: {
  task: Task
  running: boolean
  onRun: (vars: Record<string, string>) => void
  onClose: () => void
}) {
  const [values, setValues] = useState<Record<string, string>>(() => {
    const init: Record<string, string> = {}
    for (const v of task.variables ?? []) init[v.key] = v.default_value
    return init
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onRun(values)
  }

  return (
    <Dialog.Root open onOpenChange={(open) => { if (!open) onClose() }}>
      <Dialog.Portal>
        <Dialog.Backdrop className={styles.backdrop} />
        <Dialog.Popup className={styles.formDialog}>
          <Dialog.Title className={styles.dialogTitle}>
            填写变量 - {task.label || task.name}
          </Dialog.Title>
          <form onSubmit={handleSubmit} className={styles.form}>
            {(task.variables ?? []).map(v => (
              <label key={v.key} className={styles.formLabel}>
                <span className={styles.varsRunKey}>${'{' + v.key + '}'}</span>
                <Input className={styles.formInput}
                  value={values[v.key] ?? ''}
                  onChange={e => setValues(prev => ({ ...prev, [v.key]: e.target.value }))}
                  placeholder={v.default_value || '请输入值'} />
              </label>
            ))}
            <div className={styles.formActions}>
              <button type="button" className={styles.cancelBtn} onClick={onClose}>取消</button>
              <button type="submit" className={styles.submitBtn} disabled={running}>
                {running ? '执行中...' : '执行'}
              </button>
            </div>
          </form>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

// ---- HTTP Payload Form ----

interface HttpFields {
  url: string
  method: string
  headers: string
  body: string
  timeout: number
}

const HTTP_METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'] as const

function HttpPayloadForm({ fields, onChange }: {
  fields: HttpFields
  onChange: (patch: Partial<HttpFields>) => void
}) {
  return (
    <div className={styles.httpForm}>
      <label className={styles.formLabel}>
        URL
        <Input className={styles.formInput} value={fields.url}
          onChange={e => onChange({ url: e.target.value })}
          placeholder="https://api.example.com/endpoint" required />
        <span className={styles.commandsHelp}>支持 {'${VAR}'} 环境变量替换</span>
      </label>

      <div className={styles.httpRow}>
        <div className={styles.formLabel} style={{ flex: 1 }}>
          Method
          <Select.Root value={fields.method} onValueChange={(val) => onChange({ method: val as string })}>
            <Select.Trigger className={styles.selectTrigger}>
              <Select.Value />
              <Select.Icon className={styles.selectIcon}><ChevronIcon /></Select.Icon>
            </Select.Trigger>
            <Select.Portal>
              <Select.Positioner className={styles.selectPositioner} sideOffset={4}>
                <Select.Popup className={styles.selectPopup}>
                  <Select.List>
                    {HTTP_METHODS.map(m => (
                      <Select.Item key={m} value={m} className={styles.selectItem}>
                        <Select.ItemText>{m}</Select.ItemText>
                        <Select.ItemIndicator className={styles.selectIndicator}>
                          <CheckIcon />
                        </Select.ItemIndicator>
                      </Select.Item>
                    ))}
                  </Select.List>
                </Select.Popup>
              </Select.Positioner>
            </Select.Portal>
          </Select.Root>
        </div>

        <label className={styles.formLabel} style={{ flex: 0, minWidth: 100 }}>
          超时（秒）
          <Input className={styles.formInput} type="number" min={1} max={300}
            value={String(fields.timeout)}
            onChange={e => onChange({ timeout: parseInt(e.target.value) || 30 })} />
        </label>
      </div>

      <label className={styles.formLabel}>
        Headers
        <textarea className={styles.formTextarea} value={fields.headers}
          onChange={e => onChange({ headers: e.target.value })} rows={3}
          placeholder={"Content-Type: application/json\nAuthorization: Bearer ${TOKEN}"} />
        <span className={styles.commandsHelp}>每行一个 Header，格式 Key: Value</span>
      </label>

      <label className={styles.formLabel}>
        Body
        <textarea className={styles.formTextarea} value={fields.body}
          onChange={e => onChange({ body: e.target.value })} rows={4}
          placeholder={'{"key": "value"}'} />
      </label>
    </div>
  )
}
