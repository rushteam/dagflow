import { useState, useEffect, useCallback, useMemo } from 'react'
import { Dialog } from '@base-ui/react/dialog'
import { Input } from '@base-ui/react/input'
import { Select } from '@base-ui/react/select'
import { Switch } from '@base-ui/react/switch'
import { api, type Task, type Schedule, type ScheduleInput, type ScheduleLog, type VarOverride } from '../api/client'
import styles from './SchedulePanel.module.css'

export default function SchedulePanel() {
  const [schedules, setSchedules] = useState<Schedule[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(false)
  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<Schedule | null>(null)
  const [logsFor, setLogsFor] = useState<Schedule | null>(null)
  const [logs, setLogs] = useState<ScheduleLog[]>([])
  const [deleteTarget, setDeleteTarget] = useState<Schedule | null>(null)

  const taskMap = useMemo(() => {
    const m = new Map<number, Task>()
    tasks.forEach(t => m.set(t.id, t))
    return m
  }, [tasks])

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const [s, t] = await Promise.all([api.listSchedules(), api.listTasks()])
      setSchedules(s)
      setTasks(t)
    } catch { /* ignore */ }
    finally { setLoading(false) }
  }, [])

  useEffect(() => { fetchData() }, [fetchData])

  const handleToggle = async (sch: Schedule) => {
    try {
      await api.updateSchedule(sch.id, {
        name: sch.name,
        task_id: sch.task_id,
        schedule_type: sch.schedule_type,
        cron_expr: sch.cron_expr,
        run_at: sch.run_at,
        variable_overrides: sch.variable_overrides ?? [],
        enabled: !sch.enabled,
      })
      fetchData()
    } catch { /* ignore */ }
  }

  const handleTrigger = async (id: number) => {
    try {
      await api.triggerSchedule(id)
      setTimeout(fetchData, 1000)
    } catch { /* ignore */ }
  }

  const confirmDelete = async () => {
    if (!deleteTarget) return
    try {
      await api.deleteSchedule(deleteTarget.id)
      fetchData()
    } catch { /* ignore */ }
    finally { setDeleteTarget(null) }
  }

  const handleShowLogs = async (sch: Schedule) => {
    setLogsFor(sch)
    try {
      const l = await api.getScheduleLogs(sch.id)
      setLogs(l)
    } catch { setLogs([]) }
  }

  const openCreate = () => { setEditing(null); setShowForm(true) }
  const openEdit = (sch: Schedule) => { setEditing(sch); setShowForm(true) }

  const taskLabel = (taskId: number) => {
    const t = taskMap.get(taskId)
    return t ? (t.label || t.name) : `#${taskId}`
  }

  return (
    <div>
      <div className={styles.panelHeader}>
        <h2 className={styles.panelTitle}>调度管理</h2>
        <div className={styles.actions}>
          <button className={styles.refreshBtn} onClick={fetchData} disabled={loading}>
            {loading ? '刷新中...' : '刷新'}
          </button>
          <button className={styles.createBtn} onClick={openCreate}>新建调度</button>
        </div>
      </div>

      <div className={styles.tableWrap}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>名称</th>
              <th>任务</th>
              <th>类型</th>
              <th>表达式 / 执行时间</th>
              <th>状态</th>
              <th>启用</th>
              <th>上次执行</th>
              <th>下次执行</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {schedules.map((sch) => (
              <tr key={sch.id}>
                <td className={styles.nameCell}>{sch.name}</td>
                <td><code className={styles.code}>{taskLabel(sch.task_id)}</code></td>
                <td><span className={styles.typeBadge}>{sch.schedule_type}</span></td>
                <td className={styles.mono}>
                  {sch.schedule_type === 'cron' ? sch.cron_expr : sch.run_at ? formatTime(sch.run_at) : '-'}
                </td>
                <td><StatusBadge status={sch.status} /></td>
                <td>
                  <Switch.Root
                    className={styles.switch}
                    checked={sch.enabled}
                    onCheckedChange={() => handleToggle(sch)}
                  >
                    <Switch.Thumb className={styles.switchThumb} />
                  </Switch.Root>
                </td>
                <td>{sch.last_run_at ? formatTime(sch.last_run_at) : '-'}</td>
                <td>{sch.next_run_at ? formatTime(sch.next_run_at) : '-'}</td>
                <td className={styles.actionCell}>
                  <button className={styles.actionBtn} onClick={() => handleTrigger(sch.id)} title="立即执行">
                    执行
                  </button>
                  <button className={styles.actionBtn} onClick={() => handleShowLogs(sch)} title="查看日志">
                    日志
                  </button>
                  <button className={styles.actionBtn} onClick={() => openEdit(sch)} title="编辑">
                    编辑
                  </button>
                  <button className={styles.actionBtnDanger} onClick={() => setDeleteTarget(sch)} title="删除">
                    删除
                  </button>
                </td>
              </tr>
            ))}
            {schedules.length === 0 && !loading && (
              <tr><td colSpan={9} className={styles.empty}>暂无调度数据</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {showForm && (
        <ScheduleFormDialog
          tasks={tasks}
          schedule={editing}
          onClose={() => setShowForm(false)}
          onSaved={() => { setShowForm(false); fetchData() }}
        />
      )}

      {logsFor && (
        <LogsDialog
          schedule={logsFor}
          logs={logs}
          onClose={() => setLogsFor(null)}
        />
      )}

      <Dialog.Root open={!!deleteTarget} onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}>
        <Dialog.Portal>
          <Dialog.Backdrop className={styles.backdrop} />
          <Dialog.Popup className={styles.confirmDialog}>
            <Dialog.Title className={styles.dialogTitle}>确认删除</Dialog.Title>
            <Dialog.Description className={styles.confirmDesc}>
              确定删除调度「{deleteTarget?.name}」吗？此操作不可撤销。
            </Dialog.Description>
            <div className={styles.formActions}>
              <Dialog.Close className={styles.cancelBtn}>取消</Dialog.Close>
              <button className={styles.dangerBtn} onClick={confirmDelete}>删除</button>
            </div>
          </Dialog.Popup>
        </Dialog.Portal>
      </Dialog.Root>
    </div>
  )
}

function StatusBadge({ status }: { status: string }) {
  const cls = status === 'running' ? styles.statusRunning
    : status === 'failed' ? styles.statusFailed
    : status === 'completed' ? styles.statusCompleted
    : styles.statusIdle
  return <span className={cls}>{status}</span>
}

// ---- Form Dialog ----

function ScheduleFormDialog({ tasks, schedule, onClose, onSaved }: {
  tasks: Task[]
  schedule: Schedule | null
  onClose: () => void
  onSaved: () => void
}) {
  const isEdit = !!schedule
  const [form, setForm] = useState<ScheduleInput>(() => {
    if (schedule) return {
      name: schedule.name,
      task_id: schedule.task_id,
      schedule_type: schedule.schedule_type,
      cron_expr: schedule.cron_expr ?? '',
      run_at: schedule.run_at ? toLocalDatetime(schedule.run_at) : '',
      variable_overrides: schedule.variable_overrides ?? [],
      enabled: schedule.enabled,
    }
    return {
      name: '', task_id: tasks[0]?.id ?? 0, schedule_type: 'cron',
      cron_expr: '', run_at: '', variable_overrides: [], enabled: true,
    }
  })

  const selectedTask = tasks.find(t => t.id === form.task_id)
  const taskVars = selectedTask?.variables ?? []
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const set = (patch: Partial<ScheduleInput>) => setForm(prev => ({ ...prev, ...patch }))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    const data: ScheduleInput = {
      ...form,
      run_at: form.schedule_type === 'once' && form.run_at
        ? new Date(form.run_at).toISOString()
        : undefined,
    }

    setSubmitting(true)
    try {
      if (isEdit && schedule) {
        await api.updateSchedule(schedule.id, data)
      } else {
        await api.createSchedule(data)
      }
      onSaved()
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog.Root open onOpenChange={(open) => { if (!open) onClose() }}>
      <Dialog.Portal>
        <Dialog.Backdrop className={styles.backdrop} />
        <Dialog.Popup className={styles.formDialog}>
          <Dialog.Title className={styles.dialogTitle}>
            {isEdit ? '编辑调度' : '新建调度'}
          </Dialog.Title>

          {error && <div className={styles.formError}>{error}</div>}

          <form onSubmit={handleSubmit} className={styles.form}>
            <label className={styles.formLabel}>
              名称
              <Input className={styles.formInput} value={form.name}
                onChange={(e) => set({ name: e.target.value })} required />
            </label>

            <div className={styles.formLabel}>
              任务
              <Select.Root
                value={String(form.task_id)}
                onValueChange={(val) => {
                  if (!val) return
                  set({ task_id: Number(val), variable_overrides: [] })
                }}
              >
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

            <div className={styles.formLabel}>
              调度类型
              <Select.Root value={form.schedule_type} onValueChange={(val) => { if (val) set({ schedule_type: val as 'cron' | 'once' }) }}>
                <Select.Trigger className={styles.selectTrigger}>
                  <Select.Value placeholder="选择类型" />
                  <Select.Icon className={styles.selectIcon}><ChevronIcon /></Select.Icon>
                </Select.Trigger>
                <Select.Portal>
                  <Select.Positioner className={styles.selectPositioner} sideOffset={4}>
                    <Select.Popup className={styles.selectPopup}>
                      <Select.List>
                        <Select.Item value="cron" className={styles.selectItem}>
                          <Select.ItemText>周期性（Cron）</Select.ItemText>
                          <Select.ItemIndicator className={styles.selectIndicator}><CheckIcon /></Select.ItemIndicator>
                        </Select.Item>
                        <Select.Item value="once" className={styles.selectItem}>
                          <Select.ItemText>一次性</Select.ItemText>
                          <Select.ItemIndicator className={styles.selectIndicator}><CheckIcon /></Select.ItemIndicator>
                        </Select.Item>
                      </Select.List>
                    </Select.Popup>
                  </Select.Positioner>
                </Select.Portal>
              </Select.Root>
            </div>

            {form.schedule_type === 'cron' ? (
              <label className={styles.formLabel}>
                Cron 表达式
                <Input className={styles.formInput} value={form.cron_expr ?? ''}
                  onChange={(e) => set({ cron_expr: e.target.value })}
                  placeholder="*/5 * * * * (每5分钟)" required />
                <span className={styles.hint}>标准 5 段（分 时 日 月 周），或 6 段含秒</span>
              </label>
            ) : (
              <label className={styles.formLabel}>
                执行时间
                <Input className={styles.formInput} type="datetime-local"
                  value={form.run_at ?? ''}
                  onChange={(e) => set({ run_at: e.target.value })} required />
              </label>
            )}

            {taskVars.length > 0 && (
              <VarOverridesEditor
                taskVars={taskVars}
                overrides={form.variable_overrides}
                onChange={(variable_overrides) => set({ variable_overrides })}
              />
            )}

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

            <div className={styles.formActions}>
              <button type="button" className={styles.cancelBtn} onClick={onClose}>取消</button>
              <button type="submit" className={styles.submitBtn} disabled={submitting}>
                {submitting ? '提交中...' : isEdit ? '保存' : '创建'}
              </button>
            </div>
          </form>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

// ---- Variable Overrides Editor ----

function VarOverridesEditor({ taskVars, overrides, onChange }: {
  taskVars: { key: string; default_value: string }[]
  overrides: VarOverride[]
  onChange: (overrides: VarOverride[]) => void
}) {
  const getOverride = (key: string): VarOverride => {
    return overrides.find(o => o.key === key) ?? { key, type: 'fixed', value: '' }
  }

  const updateOverride = (key: string, patch: Partial<VarOverride>) => {
    const existing = overrides.find(o => o.key === key)
    if (existing) {
      onChange(overrides.map(o => o.key === key ? { ...o, ...patch } : o))
    } else {
      onChange([...overrides, { key, type: 'fixed', ...patch }])
    }
  }

  return (
    <div className={styles.overridesSection}>
      <span className={styles.overridesTitle}>变量覆盖</span>
      <span className={styles.overridesHint}>不配置则使用任务默认值</span>
      {taskVars.map(tv => {
        const ov = getOverride(tv.key)
        return (
          <div key={tv.key} className={styles.overrideRow}>
            <span className={styles.overrideKey}>${'{' + tv.key + '}'}</span>
            <Select.Root value={ov.type}
              onValueChange={(val) => {
                if (!val) return
                if (val === 'fixed') updateOverride(tv.key, { type: 'fixed', value: tv.default_value, format: undefined, offset: undefined })
                else updateOverride(tv.key, { type: 'date' as const, value: undefined, format: 'yyyyMMdd', offset: '0d' })
              }}>
              <Select.Trigger className={styles.overrideSelect}>
                <Select.Value />
                <Select.Icon className={styles.selectIcon}><ChevronIcon /></Select.Icon>
              </Select.Trigger>
              <Select.Portal>
                <Select.Positioner className={styles.selectPositioner} sideOffset={4}>
                  <Select.Popup className={styles.selectPopup}>
                    <Select.List>
                      <Select.Item value="fixed" className={styles.selectItem}>
                        <Select.ItemText>固定值</Select.ItemText>
                        <Select.ItemIndicator className={styles.selectIndicator}><CheckIcon /></Select.ItemIndicator>
                      </Select.Item>
                      <Select.Item value="date" className={styles.selectItem}>
                        <Select.ItemText>日期函数</Select.ItemText>
                        <Select.ItemIndicator className={styles.selectIndicator}><CheckIcon /></Select.ItemIndicator>
                      </Select.Item>
                    </Select.List>
                  </Select.Popup>
                </Select.Positioner>
              </Select.Portal>
            </Select.Root>
            {ov.type === 'fixed' ? (
              <Input className={styles.overrideInput}
                value={ov.value ?? ''}
                onChange={e => updateOverride(tv.key, { value: e.target.value })}
                placeholder={tv.default_value || '值'} />
            ) : (
              <>
                <Input className={styles.overrideInput}
                  value={ov.format ?? ''}
                  onChange={e => updateOverride(tv.key, { format: e.target.value })}
                  placeholder="yyyyMMdd" />
                <Input className={styles.overrideInputShort}
                  value={ov.offset ?? ''}
                  onChange={e => updateOverride(tv.key, { offset: e.target.value })}
                  placeholder="0d" />
              </>
            )}
          </div>
        )
      })}
    </div>
  )
}

// ---- Logs Dialog ----

function LogsDialog({ schedule, logs, onClose }: {
  schedule: Schedule
  logs: ScheduleLog[]
  onClose: () => void
}) {
  return (
    <Dialog.Root open onOpenChange={(open) => { if (!open) onClose() }}>
      <Dialog.Portal>
        <Dialog.Backdrop className={styles.backdrop} />
        <Dialog.Popup className={styles.logsDialog}>
          <Dialog.Title className={styles.dialogTitle}>
            执行日志 - {schedule.name}
          </Dialog.Title>

          <div className={styles.logsTableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>开始时间</th>
                  <th>状态</th>
                  <th>耗时</th>
                  <th>错误信息</th>
                </tr>
              </thead>
              <tbody>
                {logs.map(l => (
                  <tr key={l.id}>
                    <td>{formatTime(l.started_at)}</td>
                    <td><StatusBadge status={l.status} /></td>
                    <td>{l.duration_ms != null ? `${l.duration_ms}ms` : '-'}</td>
                    <td className={styles.errorCell}>{l.error_msg || '-'}</td>
                  </tr>
                ))}
                {logs.length === 0 && (
                  <tr><td colSpan={4} className={styles.empty}>暂无执行记录</td></tr>
                )}
              </tbody>
            </table>
          </div>

          <div className={styles.formActions}>
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
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

function toLocalDatetime(iso: string): string {
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
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
