import { useState } from 'react'
import { Input } from '@base-ui/react/input'
import { api, type User } from '../api/client'
import styles from './Login.module.css'

interface Props {
  onLogin: (user: User, token: string) => void
}

export default function Login({ onLogin }: Props) {
  const [isRegister, setIsRegister] = useState(false)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [nickname, setNickname] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const res = isRegister
        ? await api.register(username, password, nickname)
        : await api.login(username, password)
      onLogin(res.user, res.token)
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className={styles.container}>
      <form className={styles.card} onSubmit={handleSubmit}>
        <h1 className={styles.title}>DagFlow</h1>
        <p className={styles.subtitle}>
          {isRegister ? '创建新账号' : '登录您的账号'}
        </p>

        {error && <div className={styles.error}>{error}</div>}

        <div className={styles.field}>
          <label className={styles.label}>用户名</label>
          <Input
            className={styles.input}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="请输入用户名"
            required
          />
        </div>

        {isRegister && (
          <div className={styles.field}>
            <label className={styles.label}>昵称</label>
            <Input
              className={styles.input}
              value={nickname}
              onChange={(e) => setNickname(e.target.value)}
              placeholder="请输入昵称（选填）"
            />
          </div>
        )}

        <div className={styles.field}>
          <label className={styles.label}>密码</label>
          <Input
            className={styles.input}
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="请输入密码"
            required
          />
        </div>

        <button className={styles.button} type="submit" disabled={loading}>
          {loading ? '处理中...' : isRegister ? '注册' : '登录'}
        </button>

        <button
          className={styles.link}
          type="button"
          onClick={() => {
            setIsRegister(!isRegister)
            setError('')
          }}
        >
          {isRegister ? '已有账号？去登录' : '没有账号？去注册'}
        </button>
      </form>
    </div>
  )
}
