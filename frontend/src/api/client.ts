export interface User {
  id: number
  username: string
  nickname: string
  role: string
  created_at: string
  last_login?: string
}

interface AuthResponse {
  token: string
  user: User
}

class ApiClient {
  private getHeaders(): Record<string, string> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    }
    const token = localStorage.getItem('token')
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }
    return headers
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const res = await fetch(path, {
      ...options,
      headers: { ...this.getHeaders(), ...(options.headers as Record<string, string>) },
    })

    if (!res.ok) {
      const body = await res.json().catch(() => ({ error: res.statusText }))
      throw new Error(body.error || `HTTP ${res.status}`)
    }

    if (res.status === 204) return undefined as T
    return res.json()
  }

  login(username: string, password: string): Promise<AuthResponse> {
    return this.request('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
  }

  register(username: string, password: string, nickname: string): Promise<AuthResponse> {
    return this.request('/api/v1/auth/register', {
      method: 'POST',
      body: JSON.stringify({ username, password, nickname }),
    })
  }

  me(): Promise<User> {
    return this.request('/api/v1/auth/me')
  }

  listUsers(): Promise<User[]> {
    return this.request('/api/v1/users')
  }

  // ---- kinds ----

  listKinds(): Promise<KindInfo[]> {
    return this.request('/api/v1/kinds')
  }

  // ---- tasks ----

  listTasks(): Promise<Task[]> {
    return this.request('/api/v1/tasks')
  }

  createTask(data: TaskInput): Promise<Task> {
    return this.request('/api/v1/tasks', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }

  updateTask(id: number, data: TaskInput): Promise<Task> {
    return this.request(`/api/v1/tasks/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  }

  deleteTask(id: number): Promise<void> {
    return this.request(`/api/v1/tasks/${id}`, { method: 'DELETE' })
  }

  runTask(id: number, variables?: Record<string, string>): Promise<{ message: string }> {
    return this.request(`/api/v1/tasks/${id}/run`, {
      method: 'POST',
      body: JSON.stringify({ variables }),
    })
  }

  listTaskRuns(id: number): Promise<TaskRun[]> {
    return this.request(`/api/v1/tasks/${id}/runs`)
  }

  listAllTaskRuns(params: {
    page?: number; size?: number;
    task_name?: string; task_label?: string; run_id?: string;
  } = {}): Promise<PagedTaskRuns> {
    const qs = new URLSearchParams()
    if (params.page) qs.set('page', String(params.page))
    if (params.size) qs.set('size', String(params.size))
    if (params.task_name) qs.set('task_name', params.task_name)
    if (params.task_label) qs.set('task_label', params.task_label)
    if (params.run_id) qs.set('run_id', params.run_id)
    const q = qs.toString()
    return this.request(`/api/v1/task-runs${q ? `?${q}` : ''}`)
  }

  getTaskRunDetail(runId: number): Promise<AllTaskRun> {
    return this.request(`/api/v1/task-runs/${runId}`)
  }

  cancelTaskRun(runId: number): Promise<{ message: string }> {
    return this.request(`/api/v1/task-runs/${runId}/cancel`, { method: 'POST' })
  }

  listChildRuns(runId: number): Promise<ChildRun[]> {
    return this.request(`/api/v1/task-runs/${runId}/children`)
  }

  // ---- schedules ----

  listSchedules(): Promise<Schedule[]> {
    return this.request('/api/v1/schedules')
  }

  createSchedule(data: ScheduleInput): Promise<Schedule> {
    return this.request('/api/v1/schedules', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }

  updateSchedule(id: number, data: ScheduleInput): Promise<Schedule> {
    return this.request(`/api/v1/schedules/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  }

  deleteSchedule(id: number): Promise<void> {
    return this.request(`/api/v1/schedules/${id}`, { method: 'DELETE' })
  }

  triggerSchedule(id: number): Promise<{ message: string }> {
    return this.request(`/api/v1/schedules/${id}/trigger`, { method: 'POST' })
  }

  // ---- callbacks ----

  listCallbacks(): Promise<Callback[]> {
    return this.request('/api/v1/callbacks')
  }

  createCallback(data: CallbackInput): Promise<Callback> {
    return this.request('/api/v1/callbacks', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }

  updateCallback(id: number, data: CallbackInput): Promise<Callback> {
    return this.request(`/api/v1/callbacks/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  }

  deleteCallback(id: number): Promise<void> {
    return this.request(`/api/v1/callbacks/${id}`, { method: 'DELETE' })
  }

  listCallbackVars(): Promise<CallbackVarInfo[]> {
    return this.request('/api/v1/callback-vars')
  }
}

export interface KindInfo {
  name: string
  label: string
  payload_hint: string
}

export interface TaskVariable {
  key: string
  default_value: string
}

export interface Task {
  id: number
  name: string
  label: string
  kind: string
  payload: Record<string, unknown>
  variables: TaskVariable[]
  enabled: boolean
  created_by?: number
  created_at: string
  updated_at: string
}

export interface TaskInput {
  name: string
  label: string
  kind: string
  payload: Record<string, unknown>
  variables: TaskVariable[]
  enabled: boolean
}

export interface VarOverride {
  key: string
  value: string
}

export interface Schedule {
  id: number
  name: string
  task_id: number
  schedule_type: 'cron' | 'once'
  cron_expr?: string
  run_at?: string
  variable_overrides: VarOverride[]
  enabled: boolean
  status: string
  last_run_at?: string
  next_run_at?: string
  created_at: string
}

export interface ScheduleInput {
  name: string
  task_id: number
  schedule_type: 'cron' | 'once'
  cron_expr?: string
  run_at?: string
  variable_overrides: VarOverride[]
  enabled: boolean
}

export interface TaskRun {
  id: number
  task_id: number
  trigger_type: 'manual' | 'schedule' | 'dag'
  trigger_id?: number
  triggered_by?: number
  parent_run_id?: number
  status: 'running' | 'success' | 'failed' | 'cancelled'
  started_at: string
  finished_at?: string
  duration_ms?: number
  error_msg?: string
  output?: string
}

export interface ChildRun extends AllTaskRun {
  parent_run_id?: number
}

export interface AllTaskRun extends TaskRun {
  task_name: string
  task_label: string
  task_kind: string
}

export interface PagedTaskRuns {
  total: number
  page: number
  size: number
  items: AllTaskRun[]
}

// ---- Callbacks ----

export interface Callback {
  id: number
  name: string
  url: string
  events: string[]
  headers: Record<string, string>
  body_template: string
  match_mode: 'all' | 'selected'
  task_ids: number[]
  enabled: boolean
  created_by?: number
  created_at: string
  updated_at: string
}

export interface CallbackInput {
  name: string
  url: string
  events: string[]
  headers: Record<string, string>
  body_template: string
  match_mode: 'all' | 'selected'
  task_ids: number[]
  enabled: boolean
}

export interface CallbackVarInfo {
  name: string
  label: string
  example: string
}

export const api = new ApiClient()
