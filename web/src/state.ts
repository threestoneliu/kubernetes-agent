// UIState is the top-level "what is the chat view doing right now"
// discriminator. Exactly one of these is active at a time and each
// variant carries the data the renderer needs to draw its blocking UI.

export type Risk = 'low' | 'medium' | 'high'

export type PendingPlan = {
  planId: string
  summary: string
  risk: Risk
  // Diffs returned by plan_write: each entry has action/resource/name/namespace
  // and optional before/after manifests.
  diffs: Array<{
    action: string
    resource: string
    name: string
    namespace: string
    before?: Record<string, unknown>
    after?: Record<string, unknown>
    risk?: string
    summary?: string
    [key: string]: unknown
  }>
}

export type PendingAsk = {
  question: string
  options: string[]
  multi: boolean
}

export type UIState =
  | { kind: 'idle' }
  | { kind: 'streaming' }
  | { kind: 'plan_awaiting'; plan: PendingPlan }
  | { kind: 'ask_user'; ask: PendingAsk }
  | { kind: 'error'; message: string }

export const idle: UIState = { kind: 'idle' }