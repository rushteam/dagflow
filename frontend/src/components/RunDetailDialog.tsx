import { useState, useEffect } from 'react'
import { Dialog } from '@base-ui/react/dialog'
import { api, type AllTaskRun, type ChildRun } from '../api/client'
import styles from './RunDetailDialog.module.css'

export function RunDetailDialog({ run: initialRun, onCancel, onClose }: {
  run: AllTaskRun
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

  const statusBadge = (s: string) => {
    const cls = s === 'success' ? styles.statusSuccess
      : s === 'failed' ? styles.statusFailed
      : s === 'cancelled' ? styles.statusCancelled
      : styles.statusRunning
    return <span className={`${styles.statusBadge} ${cls}`}>{s}</span>
  }

  const triggerLabel = (r: AllTaskRun) => {
    if (r.trigger_type === 'schedule') return `调度 #${r.trigger_id ?? ''}`
    if (r.trigger_type === 'dag') return 'DAG'
    return '手动'
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

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}
