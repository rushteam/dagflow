import { useState, useEffect, useCallback } from 'react'
import { Dialog } from '@base-ui/react/dialog'
import { Input } from '@base-ui/react/input'
import { api, type APITokenInfo } from '../api/client'
import styles from './TokenPanel.module.css'

const EXPIRE_OPTIONS: { value: string; label: string }[] = [
  { value: '', label: '永不过期' },
  { value: '30d', label: '30 天' },
  { value: '90d', label: '90 天' },
  { value: '365d', label: '365 天' },
]

export default function TokenPanel() {
  const [list, setList] = useState<APITokenInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [showCreate, setShowCreate] = useState(false)
  const [revealedSecret, setRevealedSecret] = useState<string | null>(null)
  const [revokeTarget, setRevokeTarget] = useState<APITokenInfo | null>(null)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      setList(await api.listAPITokens())
    } catch {
      setList([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const formatTime = (iso?: string) => {
    if (!iso) return '-'
    return new Date(iso).toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  const confirmRevoke = async () => {
    if (!revokeTarget) return
    try {
      await api.revokeAPIToken(revokeTarget.id)
      fetchData()
    } catch {
      /* ignore */
    } finally {
      setRevokeTarget(null)
    }
  }

  return (
    <div>
      <div className={styles.panelHeader}>
        <div>
          <h2 className={styles.panelTitle}>API 令牌</h2>
          <p className={styles.hint}>
            用于程序调用 <code>Authorization: Bearer tk_…</code>（与浏览器登录的 JWT 不同）。创建后明文只显示一次，请立即复制保存。完整说明见{' '}
            <a href="/docs" target="_blank" rel="noreferrer">/docs</a>。
          </p>
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.refreshBtn} onClick={fetchData} disabled={loading}>
            {loading ? '刷新中...' : '刷新'}
          </button>
          <button type="button" className={styles.createBtn} onClick={() => setShowCreate(true)}>
            新建令牌
          </button>
        </div>
      </div>

      <div className={styles.tableWrap}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>名称</th>
              <th>前缀</th>
              <th>创建者</th>
              <th>过期时间</th>
              <th>最后使用</th>
              <th>状态</th>
              <th>创建时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {list.map((t) => (
              <tr key={t.id}>
                <td>{t.name}</td>
                <td className={styles.mono}>{t.prefix}…</td>
                <td>{t.creator_name ?? `#${t.created_by}`}</td>
                <td>{formatTime(t.expires_at)}</td>
                <td>{formatTime(t.last_used_at)}</td>
                <td>
                  {t.enabled ? (
                    <span className={styles.badgeOk}>有效</span>
                  ) : (
                    <span className={styles.badgeOff}>已撤销</span>
                  )}
                </td>
                <td>{formatTime(t.created_at)}</td>
                <td className={styles.actionCell}>
                  {t.enabled && (
                    <button type="button" className={styles.actionBtnDanger} onClick={() => setRevokeTarget(t)}>
                      撤销
                    </button>
                  )}
                </td>
              </tr>
            ))}
            {list.length === 0 && !loading && (
              <tr>
                <td colSpan={8} className={styles.empty}>
                  暂无 API 令牌，点击「新建令牌」创建
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {showCreate && (
        <CreateTokenDialog
          onClose={() => setShowCreate(false)}
          onCreated={(raw) => {
            setShowCreate(false)
            setRevealedSecret(raw)
            fetchData()
          }}
        />
      )}

      {revealedSecret && (
        <RevealSecretDialog secret={revealedSecret} onClose={() => setRevealedSecret(null)} />
      )}

      <Dialog.Root open={!!revokeTarget} onOpenChange={(open) => { if (!open) setRevokeTarget(null) }}>
        <Dialog.Portal>
          <Dialog.Backdrop className={styles.backdrop} />
          <Dialog.Popup className={styles.formDialog}>
            <Dialog.Title className={styles.dialogTitle}>确认撤销</Dialog.Title>
            <Dialog.Description className={styles.hint}>
              撤销后 <strong>{revokeTarget?.name}</strong> 将无法再用于 API 鉴权，不可恢复。
            </Dialog.Description>
            <div className={styles.formActions}>
              <Dialog.Close className={styles.cancelBtn}>取消</Dialog.Close>
              <button type="button" className={styles.submitBtn} onClick={confirmRevoke}>
                撤销
              </button>
            </div>
          </Dialog.Popup>
        </Dialog.Portal>
      </Dialog.Root>
    </div>
  )
}

function CreateTokenDialog({
  onClose,
  onCreated,
}: {
  onClose: () => void
  onCreated: (raw: string) => void
}) {
  const [name, setName] = useState('')
  const [expiresIn, setExpiresIn] = useState<string>('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (!name.trim()) {
      setError('请输入名称')
      return
    }
    setSubmitting(true)
    try {
      const res = await api.createAPIToken(name.trim(), expiresIn)
      onCreated(res.token)
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog.Root open onOpenChange={(open) => { if (!open) onClose() }}>
      <Dialog.Portal>
        <Dialog.Backdrop className={styles.backdrop} />
        <Dialog.Popup className={styles.formDialog}>
          <Dialog.Title className={styles.dialogTitle}>新建 API 令牌</Dialog.Title>
          {error && <div className={styles.formError}>{error}</div>}
          <form onSubmit={handleSubmit}>
            <label className={styles.formLabel}>
              名称
              <Input
                className={styles.formInput}
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="如 CI 流水线、监控脚本"
                required
              />
            </label>
            <label className={styles.formLabel}>
              过期时间
              <select
                className={styles.formSelect}
                value={expiresIn}
                onChange={(e) => setExpiresIn(e.target.value)}
              >
                {EXPIRE_OPTIONS.map((opt) => (
                  <option key={opt.label} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
            </label>
            <div className={styles.formActions}>
              <button type="button" className={styles.cancelBtn} onClick={onClose}>
                取消
              </button>
              <button type="submit" className={styles.submitBtn} disabled={submitting}>
                {submitting ? '创建中...' : '创建'}
              </button>
            </div>
          </form>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

function RevealSecretDialog({ secret, onClose }: { secret: string; onClose: () => void }) {
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(secret)
    } catch {
      /* ignore */
    }
  }

  return (
    <Dialog.Root open onOpenChange={(open) => { if (!open) onClose() }}>
      <Dialog.Portal>
        <Dialog.Backdrop className={styles.backdrop} />
        <Dialog.Popup className={styles.wideDialog}>
          <Dialog.Title className={styles.dialogTitle}>请保存您的令牌</Dialog.Title>
          <p className={styles.secretWarn}>此内容仅显示一次，关闭后无法再次查看完整令牌。</p>
          <div className={styles.secretBox}>{secret}</div>
          <div className={styles.formActions}>
            <button type="button" className={styles.copyBtn} onClick={copy}>
              复制到剪贴板
            </button>
            <Dialog.Close className={styles.submitBtn}>我已保存</Dialog.Close>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
