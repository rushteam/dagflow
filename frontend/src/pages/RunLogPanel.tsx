import { useState, useEffect, useCallback, useRef } from 'react'
import { Dialog } from '@base-ui/react/dialog'
import { Input } from '@base-ui/react/input'
import { api, type AllTaskRun, type ChildRun, type PagedTaskRuns } from '../api/client'
import styles from './RunLogPanel.module.css'

const PAGE_SIZE = 15

export default function RunLogPanel() {
  const [data, setData] = useState<PagedTaskRuns>({ total: 0, page: 1, size: PAGE_SIZE, items: [] })
  const [loading, setLoading] = useState(false)
  const [filterName, setFilterName] = useState('')
  const [filterLabel, setFilterLabel] = useState('')
  const [filterRunID, setFilterRunID] = useState('')
  const [page, setPage] = useState(1)
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined)

  const fetchRuns = useCallback(async (p: number, name: string, label: string, runId: string) => {
    setLoading(true)
    try {
      const res = await api.listAllTaskRuns({
        page: p, size: PAGE_SIZE,
        task_name: name.trim() || undefined,
        task_label: label.trim() || undefined,
        run_id: runId.trim() || undefined,
      })
      setData(res)
    } catch { /* ignore */ }
    finally { setLoading(false) }
  }, [])

  useEffect(() => {
    fetchRuns(page, filterName, filterLabel, filterRunID)
  }, [page, fetchRuns])

  const handleFilterChange = useCallback(() => {
    clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => {
      setPage(1)
      fetchRuns(1, filterName, filterLabel, filterRunID)
    }, 300)
  }, [filterName, filterLabel, filterRunID, fetchRuns])

  useEffect(() => {
    handleFilterChange()
    return () => clearTimeout(debounceRef.current)
  }, [filterName, filterLabel, filterRunID])

  const handleClear = () => {
    setFilterName('')
    setFilterLabel('')
    setFilterRunID('')
    setPage(1)
    fetchRuns(1, '', '', '')
  }

  const totalPages = Math.max(1, Math.ceil(data.total / PAGE_SIZE))

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
      setTimeout(() => fetchRuns(page, filterName, filterLabel, filterRunID), 500)
    } catch { /* ignore */ }
  }

  const [detail, setDetail] = useState<AllTaskRun | null>(null)

  const triggerLabel = (r: AllTaskRun) => {
    if (r.trigger_type === 'schedule') return `调度 #${r.trigger_id ?? ''}`
    if (r.trigger_type === 'dag') return 'DAG'
    return '手动'
  }

  return (
    <div>
      <div className={styles.panelHeader}>
        <h2 className={styles.panelTitle}>运行日志</h2>
        <button className={styles.refreshBtn}
          onClick={() => fetchRuns(page, filterName, filterLabel, filterRunID)}
          disabled={loading}>
          {loading ? '刷新中...' : '刷新'}
        </button>
      </div>

      <div className={styles.filterBar}>
        <Input className={styles.filterInput} value={filterName}
          onChange={e => setFilterName(e.target.value)}
          placeholder="任务标识" />
        <Input className={styles.filterInput} value={filterLabel}
          onChange={e => setFilterLabel(e.target.value)}
          placeholder="任务名称" />
        <Input className={styles.filterInput} value={filterRunID}
          onChange={e => setFilterRunID(e.target.value)}
          placeholder="执行 ID" />
        {(filterName || filterLabel || filterRunID) && (
          <button className={styles.clearBtn} onClick={handleClear}>清除</button>
        )}
        <span className={styles.filterCount}>共 {data.total} 条</span>
      </div>

      <div className={styles.tableWrap}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>ID</th>
              <th>任务标识</th>
              <th>任务名称</th>
              <th>触发方式</th>
              <th>状态</th>
              <th>开始时间</th>
              <th>耗时</th>
              <th>错误信息</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {data.items.map(r => (
              <tr key={r.id} className={styles.clickableRow} onClick={() => setDetail(r)}>
                <td>{r.id}</td>
                <td className={styles.nameCell}>{r.task_name}</td>
                <td>{r.task_label || '-'}</td>
                <td>{triggerLabel(r)}</td>
                <td>{statusBadge(r.status)}</td>
                <td>{formatTime(r.started_at)}</td>
                <td>{r.duration_ms != null ? `${r.duration_ms}ms` : '-'}</td>
                <td className={styles.errorCell} title={r.error_msg ?? ''}>
                  {r.error_msg || '-'}
                </td>
                <td onClick={e => e.stopPropagation()}>
                  <div className={styles.actionCell}>
                    <button className={styles.detailBtn} onClick={() => setDetail(r)}>
                      详情
                    </button>
                    {r.status === 'running' && (
                      <button className={styles.cancelBtn} onClick={() => handleCancel(r.id)}>
                        停止
                      </button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
            {data.items.length === 0 && !loading && (
              <tr><td colSpan={9} className={styles.empty}>暂无运行记录</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && (
        <div className={styles.pagination}>
          <button className={styles.pageBtn} disabled={page <= 1}
            onClick={() => setPage(page - 1)}>
            ‹ 上一页
          </button>
          {buildPageNumbers(page, totalPages).map((p, i) =>
            p === '...' ? (
              <span key={`e${i}`} className={styles.pageEllipsis}>…</span>
            ) : (
              <button key={p}
                className={`${styles.pageBtn} ${p === page ? styles.pageBtnActive : ''}`}
                onClick={() => setPage(p as number)}>
                {p}
              </button>
            )
          )}
          <button className={styles.pageBtn} disabled={page >= totalPages}
            onClick={() => setPage(page + 1)}>
            下一页 ›
          </button>
        </div>
      )}

      {detail && (
        <RunDetailDialog
          run={detail}
          statusBadge={statusBadge}
          triggerLabel={triggerLabel}
          onCancel={handleCancel}
          onClose={() => setDetail(null)}
        />
      )}
    </div>
  )
}

// ---- Run Detail Dialog ----

function RunDetailDialog({ run: initialRun, statusBadge, triggerLabel, onCancel, onClose }: {
  run: AllTaskRun
  statusBadge: (s: string) => React.ReactNode
  triggerLabel: (r: AllTaskRun) => string
  onCancel: (runId: number) => void
  onClose: () => void
}) {
  const [run, setRun] = useState<AllTaskRun>(initialRun)
  const [loading, setLoading] = useState(true)
  const [children, setChildren] = useState<ChildRun[]>([])
  const [loadingChildren, setLoadingChildren] = useState(false)
  const isDag = run.task_kind === 'dag'

  useEffect(() => {
    api.getTaskRunDetail(initialRun.id)
      .then(detail => { setRun(detail); setLoading(false) })
      .catch(() => setLoading(false))
  }, [initialRun.id])

  useEffect(() => {
    if (!isDag) return
    setLoadingChildren(true)
    api.listChildRuns(run.id)
      .then(setChildren)
      .catch(() => setChildren([]))
      .finally(() => setLoadingChildren(false))
  }, [run.id, isDag])

  const handleCancel = () => {
    onCancel(run.id)
    setTimeout(onClose, 600)
  }

  return (
    <Dialog.Root open onOpenChange={(open) => { if (!open) onClose() }}>
      <Dialog.Portal>
        <Dialog.Backdrop className={styles.backdrop} />
        <Dialog.Popup className={styles.detailDialog}>
          <Dialog.Title className={styles.detailTitle}>
            运行详情 #{run.id}
          </Dialog.Title>

          <div className={styles.detailGrid}>
            <div className={styles.detailField}>
              <span className={styles.detailLabel}>任务标识</span>
              <span className={styles.detailValue}>{run.task_name}</span>
            </div>
            <div className={styles.detailField}>
              <span className={styles.detailLabel}>任务名称</span>
              <span className={styles.detailValue}>{run.task_label || '-'}</span>
            </div>
            <div className={styles.detailField}>
              <span className={styles.detailLabel}>任务类型</span>
              <span className={styles.detailValue}>
                <code className={styles.kindBadge}>{run.task_kind}</code>
              </span>
            </div>
            <div className={styles.detailField}>
              <span className={styles.detailLabel}>触发方式</span>
              <span className={styles.detailValue}>{triggerLabel(run)}</span>
            </div>
            <div className={styles.detailField}>
              <span className={styles.detailLabel}>状态</span>
              <span className={styles.detailValue}>{statusBadge(run.status)}</span>
            </div>
            <div className={styles.detailField}>
              <span className={styles.detailLabel}>开始时间</span>
              <span className={styles.detailValue}>{formatTime(run.started_at)}</span>
            </div>
            <div className={styles.detailField}>
              <span className={styles.detailLabel}>结束时间</span>
              <span className={styles.detailValue}>{run.finished_at ? formatTime(run.finished_at) : '-'}</span>
            </div>
            <div className={styles.detailField}>
              <span className={styles.detailLabel}>耗时</span>
              <span className={styles.detailValue}>{run.duration_ms != null ? `${run.duration_ms}ms` : '-'}</span>
            </div>
          </div>

          {run.output && (
            <div className={styles.detailOutputSection}>
              <span className={styles.detailSectionTitle}>执行输出</span>
              <pre className={styles.detailOutputPre}>{run.output}</pre>
            </div>
          )}

          {run.error_msg && (
            <div className={styles.detailErrorSection}>
              <span className={styles.detailLabel}>错误信息</span>
              <pre className={styles.detailErrorPre}>{run.error_msg}</pre>
            </div>
          )}

          {loading && !run.output && (
            <div className={styles.detailChildLoading}>加载详情...</div>
          )}

          {isDag && (
            <div className={styles.detailChildSection}>
              <span className={styles.detailSectionTitle}>DAG 子任务</span>
              {loadingChildren ? (
                <div className={styles.detailChildLoading}>加载中...</div>
              ) : children.length === 0 ? (
                <div className={styles.detailChildLoading}>无子任务记录</div>
              ) : (
                <div className={styles.detailChildTableWrap}>
                  <table className={styles.table}>
                    <thead>
                      <tr>
                        <th>ID</th>
                        <th>任务标识</th>
                        <th>任务名称</th>
                        <th>状态</th>
                        <th>开始时间</th>
                        <th>耗时</th>
                        <th>错误信息</th>
                      </tr>
                    </thead>
                    <tbody>
                      {children.map(c => (
                        <tr key={c.id}>
                          <td>{c.id}</td>
                          <td className={styles.nameCell}>{c.task_name}</td>
                          <td>{c.task_label || '-'}</td>
                          <td>{statusBadge(c.status)}</td>
                          <td>{formatTime(c.started_at)}</td>
                          <td>{c.duration_ms != null ? `${c.duration_ms}ms` : '-'}</td>
                          <td className={styles.errorCell} title={c.error_msg ?? ''}>
                            {c.error_msg || '-'}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}

          <div className={styles.detailActions}>
            {run.status === 'running' && (
              <button className={styles.cancelBtn} onClick={handleCancel}>
                停止任务
              </button>
            )}
            <Dialog.Close className={styles.closeBtn}>关闭</Dialog.Close>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

// ---- helpers ----

function buildPageNumbers(current: number, total: number): (number | '...')[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
  const pages: (number | '...')[] = [1]
  if (current > 3) pages.push('...')
  for (let i = Math.max(2, current - 1); i <= Math.min(total - 1, current + 1); i++) {
    pages.push(i)
  }
  if (current < total - 2) pages.push('...')
  pages.push(total)
  return pages
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}
