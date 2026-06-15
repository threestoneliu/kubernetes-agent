// UIState is the top-level "what is the chat view doing right now"
// discriminator. Exactly one of these is active at a time and each
// variant carries the data the renderer needs to draw its blocking UI.

export type Risk = 'low' | 'medium' | 'high'

export type PendingPlan = {
  planId: string
  summary: string
  risk: Risk
  // Raw structured diff the modal surfaces before confirm.
  diffs: Array<{
    verb?: string
    resource?: string
    namespace?: string
    name?: string
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