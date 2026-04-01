import { useState, useEffect, useCallback } from 'react'
import { Dialog } from '@base-ui/react/dialog'
import { Drawer } from '@base-ui/react/drawer'
import { Input } from '@base-ui/react/input'
import { Select } from '@base-ui/react/select'
import { Switch } from '@base-ui/react/switch'
import { api, type Callback, type CallbackInput, type CallbackVarInfo, type Task } from '../api/client'
import styles from './CallbackPanel.module.css'

const ALL_EVENTS = [
  { value: 'success', label: '成功', cls: 'evSuccess' },
  { value: 'failed', label: '失败', cls: 'evFailed' },
  { value: 'cancelled', label: '取消', cls: 'evCancelled' },
] as const

export default function CallbackPanel() {
  const [list, setList] = useState<Callback[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(false)
  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<Callback | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Callback | null>(null)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const [cbs, ts] = await Promise.all([api.listCallbacks(), api.listTasks()])
      setList(cbs)
      setTasks(ts)
    } catch { /* ignore */ }
    finally { setLoading(false) }
  }, [])

  useEffect(() => { fetchData() }, [fetchData])

  const handleToggle = async (cb: Callback) => {
    try {
      await api.updateCallback(cb.id, {
        name: cb.name, url: cb.url, events: cb.events,
        headers: cb.headers, body_template: cb.body_template,
        match_mode: cb.match_mode,
        task_ids: cb.task_ids, enabled: !cb.enabled,
      })
      fetchData()
    } catch { /* ignore */ }
  }

  const confirmDelete = async () => {
    if (!deleteTarget) return
    try {
      await api.deleteCallback(deleteTarget.id)
      fetchData()
    } catch { /* ignore */ }
    finally { setDeleteTarget(null) }
  }

  const openCreate = () => { setEditing(null); setShowForm(true) }
  const openEdit = (cb: Callback) => { setEditing(cb); setShowForm(true) }

  const taskLabel = (id: number) => {
    const t = tasks.find(t => t.id === id)
    return t ? (t.label || t.name) : `#${id}`
  }

  const eventBadge = (ev: string) => {
    const cls = ev === 'success' ? styles.evSuccess
      : ev === 'failed' ? styles.evFailed
      : styles.evCancelled
    const label = ev === 'success' ? '成功' : ev === 'failed' ? '失败' : '取消'
    return <span key={ev} className={`${styles.eventTag} ${cls}`}>{label}</span>
  }

  return (
    <div>
      <div className={styles.panelHeader}>
        <h2 className={styles.panelTitle}>回调管理</h2>
        <div className={styles.actions}>
          <button className={styles.refreshBtn} onClick={fetchData} disabled={loading}>
            {loading ? '刷新中...' : '刷新'}
          </button>
          <button className={styles.createBtn} onClick={openCreate}>新建回调</button>
        </div>
      </div>

      <div className={styles.tableWrap}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>名称</th>
              <th>URL</th>
              <th>触发事件</th>
              <th>匹配范围</th>
              <th>启用</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {list.map(cb => (
              <tr key={cb.id}>
                <td className={styles.nameCell}>{cb.name}</td>
                <td className={styles.urlCell} title={cb.url}>{cb.url}</td>
                <td>
                  <div className={styles.eventTags}>
                    {cb.events.map(e => eventBadge(e))}
                  </div>
                </td>
                <td>
                  {cb.match_mode === 'all' ? (
                    <span className={styles.matchAll}>全部任务</span>
                  ) : (
                    <span className={styles.matchSelected} title={cb.task_ids.map(id => taskLabel(id)).join(', ')}>
                      {cb.task_ids.length} 个任务
                    </span>
                  )}
                </td>
                <td>
                  <Switch.Root className={styles.switch} checked={cb.enabled}
                    onCheckedChange={() => handleToggle(cb)}>
                    <Switch.Thumb className={styles.switchThumb} />
                  </Switch.Root>
                </td>
                <td className={styles.actionCell}>
                  <button className={styles.actionBtn} onClick={() => openEdit(cb)}>编辑</button>
                  <button className={styles.actionBtnDanger} onClick={() => setDeleteTarget(cb)}>删除</button>
                </td>
              </tr>
            ))}
            {list.length === 0 && !loading && (
              <tr><td colSpan={6} className={styles.empty}>暂无回调配置</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {showForm && (
        <CallbackFormDrawer
          callback={editing}
          tasks={tasks}
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
              确定删除回调「{deleteTarget?.name}」吗？
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

// ---- Form Drawer ----

function CallbackFormDrawer({ callback, tasks, onClose, onSaved }: {
  callback: Callback | null
  tasks: Task[]
  onClose: () => void
  onSaved: () => void
}) {
  const isEdit = !!callback

  const [form, setForm] = useState<CallbackInput>(() => {
    if (callback) return {
      name: callback.name,
      url: callback.url,
      events: callback.events,
      headers: callback.headers,
      body_template: callback.body_template,
      match_mode: callback.match_mode,
      task_ids: callback.task_ids,
      enabled: callback.enabled,
    }
    return {
      name: '', url: '', events: ['success', 'failed', 'cancelled'],
      headers: {}, body_template: '', match_mode: 'all', task_ids: [], enabled: true,
    }
  })

  const [headersText, setHeadersText] = useState(() => {
    const h = callback?.headers ?? {}
    return Object.entries(h).map(([k, v]) => `${k}: ${v}`).join('\n')
  })
  const [cbVars, setCbVars] = useState<CallbackVarInfo[]>([])
  const [showVarHelp, setShowVarHelp] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => { api.listCallbackVars().then(setCbVars).catch(() => {}) }, [])

  const set = (patch: Partial<CallbackInput>) => setForm(prev => ({ ...prev, ...patch }))

  const toggleEvent = (ev: string) => {
    const events = form.events.includes(ev)
      ? form.events.filter(e => e !== ev)
      : [...form.events, ev]
    set({ events })
  }

  const toggleTask = (taskId: number) => {
    const ids = form.task_ids.includes(taskId)
      ? form.task_ids.filter(id => id !== taskId)
      : [...form.task_ids, taskId]
    set({ task_ids: ids })
  }

  const parseHeaders = (text: string): Record<string, string> => {
    const headers: Record<string, string> = {}
    for (const line of text.split('\n').filter(Boolean)) {
      const idx = line.indexOf(':')
      if (idx > 0) {
        headers[line.slice(0, idx).trim()] = line.slice(idx + 1).trim()
      }
    }
    return headers
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (!form.name.trim()) { setError('名称不能为空'); return }
    if (!form.url.trim()) { setError('URL 不能为空'); return }
    if (form.events.length === 0) { setError('至少选择一个触发事件'); return }
    if (form.match_mode === 'selected' && form.task_ids.length === 0) {
      setError('请至少选择一个任务'); return
    }

    const data: CallbackInput = {
      ...form,
      headers: parseHeaders(headersText),
    }

    setSubmitting(true)
    try {
      if (isEdit && callback) {
        await api.updateCallback(callback.id, data)
      } else {
        await api.createCallback(data)
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
                  {isEdit ? '编辑回调' : '新建回调'}
                </Drawer.Title>
                <Drawer.Close className={styles.drawerClose}>×</Drawer.Close>
              </div>

              {error && <div className={styles.formError}>{error}</div>}

              <form onSubmit={handleSubmit} className={styles.form}>
                <label className={styles.formLabel}>
                  名称
                  <Input className={styles.formInput} value={form.name}
                    onChange={e => set({ name: e.target.value })} required
                    placeholder="输入回调名称" />
                </label>

                <label className={styles.formLabel}>
                  回调 URL
                  <Input className={styles.formInput} value={form.url}
                    onChange={e => set({ url: e.target.value })} required
                    placeholder="输入回调地址，如 Webhook URL" />
                </label>

                <div className={styles.formLabel}>
                  触发事件
                  <div className={styles.eventCheckboxes}>
                    {ALL_EVENTS.map(ev => (
                      <label key={ev.value} className={styles.eventCheckLabel}>
                        <input type="checkbox"
                          checked={form.events.includes(ev.value)}
                          onChange={() => toggleEvent(ev.value)} />
                        <span className={`${styles.eventTag} ${styles[ev.cls]}`}>{ev.label}</span>
                      </label>
                    ))}
                  </div>
                </div>

                <div className={styles.formLabel}>
                  匹配范围
                  <Select.Root value={form.match_mode}
                    onValueChange={(val) => set({ match_mode: val as 'all' | 'selected' })}>
                    <Select.Trigger className={styles.selectTrigger}>
                      <Select.Value />
                      <Select.Icon className={styles.selectIcon}><ChevronIcon /></Select.Icon>
                    </Select.Trigger>
                    <Select.Portal>
                      <Select.Positioner className={styles.selectPositioner} sideOffset={4}>
                        <Select.Popup className={styles.selectPopup}>
                          <Select.List>
                            <Select.Item value="all" className={styles.selectItem}>
                              <Select.ItemText>全部任务</Select.ItemText>
                              <Select.ItemIndicator className={styles.selectIndicator}><CheckIcon /></Select.ItemIndicator>
                            </Select.Item>
                            <Select.Item value="selected" className={styles.selectItem}>
                              <Select.ItemText>指定任务</Select.ItemText>
                              <Select.ItemIndicator className={styles.selectIndicator}><CheckIcon /></Select.ItemIndicator>
                            </Select.Item>
                          </Select.List>
                        </Select.Popup>
                      </Select.Positioner>
                    </Select.Portal>
                  </Select.Root>
                </div>

                {form.match_mode === 'selected' && (
                  <div className={styles.taskPickerSection}>
                    <span className={styles.taskPickerHint}>选择要匹配的任务：</span>
                    <div className={styles.taskPicker}>
                      {tasks.map(t => (
                        <label key={t.id} className={`${styles.taskPickerItem} ${form.task_ids.includes(t.id) ? styles.taskPickerItemActive : ''}`}>
                          <input type="checkbox"
                            checked={form.task_ids.includes(t.id)}
                            onChange={() => toggleTask(t.id)} />
                          <span>{t.label || t.name}</span>
                          <span className={styles.taskPickerKind}>{t.kind}</span>
                        </label>
                      ))}
                      {tasks.length === 0 && <span className={styles.taskPickerEmpty}>暂无可选任务</span>}
                    </div>
                  </div>
                )}

                <label className={styles.formLabel}>
                  自定义 Headers（可选）
                  <textarea className={styles.formTextarea} value={headersText}
                    onChange={e => setHeadersText(e.target.value)} rows={2}
                    placeholder="每行一个，格式 Key: Value" />
                  <span className={styles.hint}>如 Authorization: Bearer token</span>
                </label>

                <div className={styles.formLabel}>
                  <div className={styles.bodyTitleRow}>
                    <span>请求体模板（可选）</span>
                    <button type="button" className={styles.helpToggle}
                      onClick={() => setShowVarHelp(!showVarHelp)}>
                      {showVarHelp ? '收起' : '可用变量'}
                    </button>
                  </div>
                  {showVarHelp && cbVars.length > 0 && (
                    <div className={styles.varHelp}>
                      {cbVars.map(v => (
                        <div key={v.name} className={styles.varHelpRow}>
                          <code className={styles.varCode}>${'{' + v.name + '}'}</code>
                          <span className={styles.varLabel}>{v.label}</span>
                        </div>
                      ))}
                      <span className={styles.hint}>
                        也支持函数: {'${dateFormat(yyyyMMdd, -1d)}'}, {'${uuid()}'} 等
                      </span>
                    </div>
                  )}
                  <textarea className={styles.formTextarea} value={form.body_template}
                    onChange={e => set({ body_template: e.target.value })} rows={6}
                    placeholder={'留空使用默认 JSON body，自定义示例：\n{\n  "msg_type": "text",\n  "content": {\n    "text": "[${status}] ${task_label}(${task_kind}) 耗时${duration_ms}ms"\n  }\n}'} />
                  <span className={styles.hint}>留空则发送包含所有字段的默认 JSON</span>
                </div>

                <div className={styles.formSwitchRow}>
                  <span className={styles.formSwitchLabel}>启用</span>
                  <Switch.Root className={styles.switch} checked={form.enabled}
                    onCheckedChange={(checked) => set({ enabled: checked })}>
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
