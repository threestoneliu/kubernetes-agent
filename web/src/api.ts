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
  created_at: number
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

export function updatePolicy(id: string, yaml: string): Promise<Policy> {
  return request('PUT', `/api/policies/${encodeURIComponent(id)}`, { yaml })
}

export async function setPolicyEnabled(id: string, enabled: boolean): Promise<void> {
  await request<void>('PATCH', `/api/policies/${encodeURIComponent(id)}/enabled`, { enabled })
}

// --- sessions ---

export function listSessions(): Promise<{ sessions: Session[] }> {
  return request('GET', '/api/sessions')
}

export function createSession(input: { title: string; cluster_id?: string }): Promise<Session> {
  return request('POST', '/api/sessions', input)
}

export function getSession(id: string): Promise<Session> {
  return request('GET', `/api/sessions/${encodeURIComponent(id)}`)
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