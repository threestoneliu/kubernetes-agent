// Typed wrappers around the backend REST surface. Each function returns
// the decoded payload and throws ApiError on non-2xx responses so callers
// can present a single error UI (toast).

export type ApiError = {
  status: number
  code: string
  message: string
  retryable: boolean
}

export class ApiCallError extends Error {
  readonly status: number
  readonly code: string
  readonly retryable: boolean
  constructor(e: ApiError) {
    super(e.message)
    this.name = 'ApiCallError'
    this.status = e.status
    this.code = e.code
    this.retryable = e.retryable
  }
}

// --- Domain types (mirror the Go wire structs). ---

export type ProviderStatus = { name: string; status: string; reason?: string }
export type Health = { ok: boolean; providers: ProviderStatus[] }

export type Cluster = {
  id: string
  name: string
  server: string
  user: string
  created_at: number
  updated_at: number
}

export type Policy = {
  id: string
  name: string
  yaml: string
  enabled: boolean
  created_at: number
  updated_at: number
}

export type Session = {
  id: string
  title: string
  cluster_id?: string | null
  created_at: number
  updated_at: number
}

export type Message = {
  id: string
  role: string
  content?: string | null
  tool_calls?: string | null
  tool_call_id?: string | null
  reasoning?: string | null
  source?: string | null
  created_at: number
}

export type ScheduledTask = {
  id: string
  name: string
  prompt: string
  cron_expr?: string | null
  once_at?: number | null
  session_id: string
  enabled: boolean
  cluster_id?: string | null
  created_by: string
  created_at: number
  next_run?: number | null
  last_run?: number | null
  run_count: number
}

// --- Helpers ---

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  let payload: BodyInit | undefined
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    payload = JSON.stringify(body)
  }
  const res = await fetch(path, { method, headers, body: payload })
  if (!res.ok) {
    let code = 'http_error'
    let message = res.statusText
    let retryable = false
    try {
      const txt = await res.text()
      if (txt) {
        const parsed = JSON.parse(txt) as { code?: string; message?: string; retryable?: boolean }
        if (parsed.code) code = parsed.code
        if (parsed.message) message = parsed.message
        if (typeof parsed.retryable === 'boolean') retryable = parsed.retryable
      }
    } catch {
      // body wasn't JSON; fall back to status text
    }
    throw new ApiCallError({ status: res.status, code, message, retryable })
  }
  // 204 — no body
  if (res.status === 204) {
    return undefined as T
  }
  return (await res.json()) as T
}

// --- healthz ---

export function getHealth(): Promise<Health> {
  return request<Health>('GET', '/healthz')
}

// --- clusters ---

export function listClusters(): Promise<{ clusters: Cluster[] }> {
  return request('GET', '/api/clusters')
}

export function createCluster(input: { name: string; kubeconfig: string }): Promise<Cluster> {
  return request('POST', '/api/clusters', input)
}

export async function deleteCluster(id: string): Promise<void> {
  await request<void>('DELETE', `/api/clusters/${encodeURIComponent(id)}`)
}

// --- policies ---

export function listPolicies(): Promise<{ policies: Policy[] }> {
  return request('GET', '/api/policies')
}

export function createPolicy(yaml: string): Promise<Policy> {
  return request('POST', '/api/policies', { yaml })
}

export function updatePolicy(id: string, yaml: string): Promise<Policy> {
  return request('PUT', `/api/policies/${encodeURIComponent(id)}`, { yaml })
}

export async function setPolicyEnabled(id: string, enabled: boolean): Promise<void> {
  await request<void>('PATCH', `/api/policies/${encodeURIComponent(id)}/enabled`, { enabled })
}

export async function deletePolicy(id: string): Promise<void> {
  await request<void>('DELETE', `/api/policies/${encodeURIComponent(id)}`)
}

// --- sessions ---

export type SessionSort = 'updated_at' | 'created_at' | 'title'
export type SessionOrder = 'asc' | 'desc'

export interface ListSessionsOpts {
  q?: string
  sort?: SessionSort
  order?: SessionOrder
  limit?: number
  offset?: number
}

export function listSessions(opts: ListSessionsOpts = {}): Promise<{ sessions: Session[] }> {
  const params = new URLSearchParams()
  if (opts.q) params.set('q', opts.q)
  if (opts.sort) params.set('sort', opts.sort)
  if (opts.order) params.set('order', opts.order)
  if (opts.limit != null) params.set('limit', String(opts.limit))
  if (opts.offset) params.set('offset', String(opts.offset))
  const q = params.toString()
  return request('GET', '/api/sessions' + (q ? '?' + q : ''))
}

export function createSession(input: { title: string; cluster_id?: string }): Promise<Session> {
  return request('POST', '/api/sessions', input)
}

export function getSession(id: string): Promise<Session> {
  return request('GET', `/api/sessions/${encodeURIComponent(id)}`)
}

export function renameSession(id: string, title: string): Promise<Session> {
  return request('PUT', `/api/sessions/${encodeURIComponent(id)}`, { title })
}

export async function deleteSession(id: string): Promise<{ deleted: number }> {
  return request('DELETE', `/api/sessions/${encodeURIComponent(id)}`)
}

export async function bulkDeleteSessions(): Promise<{ deleted: number }> {
  return request('DELETE', '/api/sessions')
}

export function exportSessionUrl(id: string, format: 'md' | 'json'): string {
  return `/api/sessions/${encodeURIComponent(id)}/export?format=${format}`
}

export function listMessages(id: string): Promise<{ messages: Message[] }> {
  return request('GET', `/api/sessions/${encodeURIComponent(id)}/messages`)
}

// --- resume (plan confirm / cancel / ask_user answer) ---

export type ResumeInput =
  | { kind: 'plan'; plan_id: string; approved: boolean }
  | { kind: 'ask_user'; answer: string }

export async function resumeSession(id: string, input: ResumeInput): Promise<void> {
  await request<void>('POST', `/api/sessions/${encodeURIComponent(id)}/resume`, input)
}

// --- scheduled tasks ---

export interface CreateScheduledTaskInput {
  name: string
  prompt: string
  cron_expr?: string
  once_at?: number
  session_id: string
  cluster_id?: string
  created_by: string
}

export function listScheduledTasks(sessionId?: string): Promise<ScheduledTask[]> {
  const params = sessionId ? `?session_id=${encodeURIComponent(sessionId)}` : ''
  return request<ScheduledTask[]>(`GET`, `/api/scheduled-tasks${params}`)
}

export function createScheduledTask(input: CreateScheduledTaskInput): Promise<ScheduledTask> {
  return request<ScheduledTask>('POST', '/api/scheduled-tasks', input)
}

export async function deleteScheduledTask(id: string): Promise<void> {
  await request<void>('DELETE', `/api/scheduled-tasks/${encodeURIComponent(id)}`)
}

export function updateScheduledTask(id: string, input: { enabled?: boolean; name?: string; cron_expr?: string }): Promise<ScheduledTask> {
  return request<ScheduledTask>('PATCH', `/api/scheduled-tasks/${encodeURIComponent(id)}`, input)
}

export async function runScheduledTask(id: string): Promise<void> {
  await request<void>('POST', `/api/scheduled-tasks/${encodeURIComponent(id)}/run`)
}