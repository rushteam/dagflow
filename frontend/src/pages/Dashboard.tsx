import { useState, useEffect, useCallback } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { Dialog } from '@base-ui/react/dialog'
import { api, type User } from '../api/client'
import TaskPanel from './TaskPanel'
import SchedulePanel from './SchedulePanel'
import CallbackPanel from './CallbackPanel'
import RunLogPanel from './RunLogPanel'
import styles from './Dashboard.module.css'

const NAV_ITEMS = [
  { key: 'tasks', path: '/tasks', label: '任务管理' },
  { key: 'schedules', path: '/schedules', label: '调度管理' },
  { key: 'callbacks', path: '/callbacks', label: '回调管理' },
  { key: 'runlog', path: '/runlog', label: '运行日志' },
  { key: 'users', path: '/users', label: '用户管理' },
  { key: 'overview', path: '/overview', label: '系统概览' },
]

const PATH_TO_KEY = Object.fromEntries(NAV_ITEMS.map(n => [n.path, n.key]))
const DEFAULT_KEY = 'tasks'

interface Props {
  user: User
  onLogout: () => void
}

export default function Dashboard({ user, onLogout }: Props) {
  const location = useLocation()
  const navigate = useNavigate()
  const activeKey = PATH_TO_KEY[location.pathname] ?? DEFAULT_KEY

  useEffect(() => {
    if (!PATH_TO_KEY[location.pathname] && location.pathname !== '/login') {
      navigate('/tasks', { replace: true })
    }
  }, [location.pathname, navigate])

  const [users, setUsers] = useState<User[]>([])
  const [loadingUsers, setLoadingUsers] = useState(false)
  const navigateToRunLog = useCallback(() => navigate('/runlog'), [navigate])

  const fetchUsers = async () => {
    setLoadingUsers(true)
    try { setUsers(await api.listUsers()) } catch { /* ignore */ }
    finally { setLoadingUsers(false) }
  }

  useEffect(() => { fetchUsers() }, [])

  return (
    <div className={styles.layout}>
      {/* ---- Top Header ---- */}
      <header className={styles.header}>
        <div className={styles.headerLeft}>
          <h1 className={styles.logo}>Dash</h1>
          <span className={styles.badge}>DagFlow</span>
        </div>
        <div className={styles.headerRight}>
          <span className={styles.userInfo}>
            {user.nickname || user.username}
            <span className={styles.roleBadge}>{user.role}</span>
          </span>

          <Dialog.Root>
            <Dialog.Trigger className={styles.logoutBtn}>退出</Dialog.Trigger>
            <Dialog.Portal>
              <Dialog.Backdrop className={styles.backdrop} />
              <Dialog.Popup className={styles.dialog}>
                <Dialog.Title className={styles.dialogTitle}>确认退出</Dialog.Title>
                <Dialog.Description className={styles.dialogDesc}>
                  确定要退出登录吗？
                </Dialog.Description>
                <div className={styles.dialogActions}>
                  <Dialog.Close className={styles.cancelBtn}>取消</Dialog.Close>
                  <button className={styles.confirmBtn} onClick={onLogout}>确认退出</button>
                </div>
              </Dialog.Popup>
            </Dialog.Portal>
          </Dialog.Root>
        </div>
      </header>

      {/* ---- Body: Sidebar + Content ---- */}
      <div className={styles.body}>
        <aside className={styles.sidebar}>
          <nav className={styles.nav}>
            {NAV_ITEMS.map(item => (
              <button
                key={item.key}
                className={`${styles.navItem} ${activeKey === item.key ? styles.navItemActive : ''}`}
                onClick={() => navigate(item.path)}
              >
                {item.label}
              </button>
            ))}
          </nav>
        </aside>

        <main className={styles.main}>
          <div className={styles.content}>
            {activeKey === 'tasks' && <TaskPanel onNavigateToRunLog={navigateToRunLog} />}
            {activeKey === 'schedules' && <SchedulePanel onNavigateToRunLog={navigateToRunLog} />}
            {activeKey === 'callbacks' && <CallbackPanel />}
            {activeKey === 'runlog' && <RunLogPanel />}
            {activeKey === 'users' && <UsersPanel users={users} loading={loadingUsers} onRefresh={fetchUsers} />}
            {activeKey === 'overview' && <OverviewPanel users={users} />}
          </div>
        </main>
      </div>
    </div>
  )
}

function UsersPanel({ users, loading, onRefresh }: { users: User[]; loading: boolean; onRefresh: () => void }) {
  return (
    <>
      <div className={styles.panelHeader}>
        <h2 className={styles.panelTitle}>用户列表</h2>
        <button className={styles.refreshBtn} onClick={onRefresh} disabled={loading}>
          {loading ? '刷新中...' : '刷新'}
        </button>
      </div>
      <div className={styles.tableWrap}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>ID</th><th>用户名</th><th>昵称</th><th>角色</th><th>注册时间</th><th>最近登录</th>
            </tr>
          </thead>
          <tbody>
            {users.map(u => (
              <tr key={u.id}>
                <td>{u.id}</td>
                <td>{u.username}</td>
                <td>{u.nickname}</td>
                <td><span className={styles.roleBadge}>{u.role}</span></td>
                <td>{formatTime(u.created_at)}</td>
                <td>{u.last_login ? formatTime(u.last_login) : '-'}</td>
              </tr>
            ))}
            {users.length === 0 && !loading && (
              <tr><td colSpan={6} className={styles.empty}>暂无用户数据</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </>
  )
}

function OverviewPanel({ users }: { users: User[] }) {
  return (
    <>
      <div className={styles.panelHeader}>
        <h2 className={styles.panelTitle}>系统概览</h2>
      </div>
      <div className={styles.statsGrid}>
        <div className={styles.statCard}>
          <div className={styles.statValue}>{users.length}</div>
          <div className={styles.statLabel}>注册用户数</div>
        </div>
        <div className={styles.statCard}>
          <div className={styles.statValue}>{users.filter(u => u.last_login).length}</div>
          <div className={styles.statLabel}>活跃用户数</div>
        </div>
        <div className={styles.statCard}>
          <div className={styles.statValue}>{users.filter(u => u.role === 'admin').length}</div>
          <div className={styles.statLabel}>管理员数</div>
        </div>
      </div>
    </>
  )
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}
