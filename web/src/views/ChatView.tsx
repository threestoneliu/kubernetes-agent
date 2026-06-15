import React from 'react'
import { openChatSse } from '../sse'
import { ApiCallError, Cluster, listClusters, resumeSession } from '../api'
import { useToast } from '../components/ToastProvider'
import { idle, PendingPlan, Risk, UIState } from '../state'
import { PlanModal } from '../components/PlanModal'
import { AskUserForm } from '../components/AskUserForm'

// Per-render message model. The wire format is more verbose; we fold it
// into a render-friendly shape here so the JSX stays simple.
type ToolRow = {
  id: string
  name: string
  input: unknown
  result?: { ok: true; output: unknown } | { ok: false; error: string }
}

type Msg =
  | { kind: 'user'; id: string; text: string }
  | { kind: 'assistant'; id: string; text: string; reasoning: string; tools: ToolRow[] }
  | { kind: 'system'; id: string; text: string }

function rid(): string {
  return Math.random().toString(36).slice(2, 10)
}

function isObject(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

function pickRisk(v: unknown): Risk {
  if (typeof v === 'string') {
    if (v === 'low' || v === 'medium' || v === 'high') return v
  }
  return 'medium'
}

export function ChatView() {
  const { show } = useToast()
  const [clusters, setClusters] = React.useState<Cluster[]>([])
  const [clusterId, setClusterId] = React.useState('')
  const [sessionId, setSessionId] = React.useState('')
  const [msgs, setMsgs] = React.useState<Msg[]>([])
  const [input, setInput] = React.useState('')
  const [ui, setUi] = React.useState<UIState>(idle)
  const [busy, setBusy] = React.useState(false)
  const closeRef = React.useRef<(() => void) | null>(null)
  const streamRef = React.useRef<HTMLDivElement | null>(null)

  // Initial load: cluster list. Failure is non-fatal — chat still works,
  // the user just has no cluster selector entries.
  React.useEffect(() => {
    listClusters()
      .then((res) => setClusters(res.clusters))
      .catch((err) => show(formatError(err)))
  }, [show])

  // Auto-scroll to the bottom on each new message. Cheap enough that we
  // do it unconditionally on every render — the volume is low.
  React.useEffect(() => {
    const el = streamRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [msgs, ui])

  function appendAssistant(): string {
    const id = rid()
    setMsgs((m) => [...m, { kind: 'assistant', id, text: '', reasoning: '', tools: [] }])
    return id
  }

  function patchAssistant(id: string, fn: (m: Extract<Msg, { kind: 'assistant' }>) => Extract<Msg, { kind: 'assistant' }>) {
    setMsgs((cur) =>
      cur.map((m) => (m.kind === 'assistant' && m.id === id ? fn(m) : m))
    )
  }

  function pushSystem(text: string) {
    setMsgs((m) => [...m, { kind: 'system', id: rid(), text }])
  }

  function canSend(): boolean {
    return (
      ui.kind === 'idle' &&
      input.trim().length > 0 &&
      !busy
    )
  }

  function send() {
    if (!canSend()) return
    const text = input.trim()
    setInput('')
    setUi({ kind: 'streaming' })

    setMsgs((m) => [...m, { kind: 'user', id: rid(), text }])

    const assistantId = appendAssistant()
    let lastEventId = ''
    let bufferedPlan: PendingPlan | null = null

    const close = openChatSse({
      body: { session_id: sessionId, message: text, cluster_id: clusterId },
      lastEventId: lastEventId || undefined,
      onEvent: (ev) => {
        lastEventId = ev.id
        const payload = ev.payload as Record<string, unknown> | null
        switch (ev.type) {
          case 'session_meta': {
            if (isObject(payload)) {
              const sid = typeof payload.session_id === 'string' ? payload.session_id : ''
              if (sid) setSessionId(sid)
              const cid = typeof payload.cluster_id === 'string' ? payload.cluster_id : ''
              if (cid) setClusterId(cid)
            }
            break
          }
          case 'reasoning': {
            const t = isObject(payload) && typeof payload.text === 'string' ? payload.text : ''
            patchAssistant(assistantId, (m) => ({ ...m, reasoning: m.reasoning + t }))
            break
          }
          case 'token': {
            const t = isObject(payload) && typeof payload.text === 'string' ? payload.text : ''
            patchAssistant(assistantId, (m) => ({ ...m, text: m.text + t }))
            break
          }
          case 'tool_call': {
            if (!isObject(payload)) break
            const id = typeof payload.id === 'string' ? payload.id : ''
            const name = typeof payload.name === 'string' ? payload.name : ''
            const input = payload.input
            patchAssistant(assistantId, (m) => ({
              ...m,
              tools: [...m.tools, { id, name, input }],
            }))
            break
          }
          case 'tool_result': {
            if (!isObject(payload)) break
            const id = typeof payload.id === 'string' ? payload.id : ''
            const errMsg = typeof payload.error === 'string' ? payload.error : ''
            const output = payload.output
            patchAssistant(assistantId, (m) => ({
              ...m,
              tools: m.tools.map((t) =>
                t.id === id
                  ? {
                      ...t,
                      result: errMsg
                        ? { ok: false, error: errMsg }
                        : { ok: true, output },
                    }
                  : t
              ),
            }))
            break
          }
          case 'plan_ready': {
            if (!isObject(payload)) break
            const planId = typeof payload.plan_id === 'string' ? payload.plan_id : ''
            const summary = typeof payload.summary === 'string' ? payload.summary : ''
            const diffs = Array.isArray(payload.diffs) ? (payload.diffs as PendingPlan['diffs']) : []
            // risk is sent on plan_awaiting_confirm; default to medium until then
            bufferedPlan = { planId, summary, risk: 'medium', diffs }
            break
          }
          case 'plan_awaiting_confirm': {
            if (!isObject(payload)) break
            const planId = typeof payload.plan_id === 'string' ? payload.plan_id : ''
            const risk = pickRisk(payload.risk)
            const plan: PendingPlan = bufferedPlan && bufferedPlan.planId === planId
              ? { ...bufferedPlan, risk }
              : { planId, summary: '', risk, diffs: [] }
            setUi({ kind: 'plan_awaiting', plan })
            break
          }
          case 'ask_user': {
            if (!isObject(payload)) break
            const question = typeof payload.question === 'string' ? payload.question : ''
            const options = Array.isArray(payload.options)
              ? payload.options.filter((o): o is string => typeof o === 'string')
              : []
            const multi = payload.multi_select === true
            const ask = { question, options, multi }
            setUi({ kind: 'ask_user', ask })
            break
          }
          case 'cluster_switch': {
            if (isObject(payload) && typeof payload.cluster_id === 'string') {
              setClusterId(payload.cluster_id)
            }
            break
          }
          case 'cancelled': {
            setUi(idle)
            break
          }
          case 'error': {
            const msg = isObject(payload) && typeof payload.message === 'string'
              ? payload.message
              : 'unknown error'
            show(`错误: ${msg}`)
            setUi({ kind: 'error', message: msg })
            break
          }
          case 'message_end': {
            // Only flip to idle if we weren't pushed into a blocking state
            // by plan_awaiting_confirm / ask_user during the same turn.
            setUi((cur) => (cur.kind === 'streaming' ? idle : cur))
            break
          }
        }
      },
      onError: (err) => {
        show(`SSE 错误: ${err.message}`)
        setUi(idle)
      },
      onClose: () => {
        setBusy(false)
        closeRef.current = null
        setUi((cur) => (cur.kind === 'streaming' ? idle : cur))
      },
    })

    closeRef.current = close
    setBusy(true)
  }

  function stop() {
    closeRef.current?.()
    closeRef.current = null
    setBusy(false)
    setUi(idle)
  }

  async function confirmPlan() {
    if (ui.kind !== 'plan_awaiting' || !sessionId) return
    setBusy(true)
    try {
      await resumeSession(sessionId, {
        kind: 'plan',
        plan_id: ui.plan.planId,
        approved: true,
      })
      // The backend confirms the agent loop will continue — surface a
      // system line so the user has feedback while we wait for the
      // follow-up SSE stream (which a future change wires up via
      // openChatSse with the same sessionId).
      pushSystem(`已确认执行 plan ${ui.plan.planId}`)
      setUi({ kind: 'streaming' })
    } catch (err) {
      show(formatError(err))
    } finally {
      setBusy(false)
    }
  }

  async function cancelPlan() {
    if (ui.kind !== 'plan_awaiting' || !sessionId) return
    setBusy(true)
    try {
      await resumeSession(sessionId, {
        kind: 'plan',
        plan_id: ui.plan.planId,
        approved: false,
      })
      pushSystem(`已取消 plan ${ui.plan.planId}`)
      setUi(idle)
    } catch (err) {
      show(formatError(err))
    } finally {
      setBusy(false)
    }
  }

  async function submitAsk(answer: string) {
    if (ui.kind !== 'ask_user' || !sessionId) return
    setBusy(true)
    try {
      await resumeSession(sessionId, { kind: 'ask_user', answer })
      pushSystem(`已回答: ${answer}`)
      setUi({ kind: 'streaming' })
    } catch (err) {
      show(formatError(err))
    } finally {
      setBusy(false)
    }
  }

  const inputDisabled = ui.kind !== 'idle'

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', minHeight: 0 }}>
      <div className="toolbar">
        <label style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
          集群:
          <select
            value={clusterId}
            onChange={(e) => setClusterId(e.target.value)}
            disabled={ui.kind !== 'idle'}
          >
            <option value="">(未选择)</option>
            {clusters.map((c) => (
              <option key={c.id} value={c.id}>{c.name}</option>
            ))}
          </select>
        </label>
        <span className="muted">
          会话: {sessionId ? sessionId.slice(0, 8) : '(新建)'}
        </span>
        <span className="muted" style={{ marginLeft: 'auto' }}>
          {ui.kind === 'streaming' && '生成中…'}
          {ui.kind === 'plan_awaiting' && '等待确认计划'}
          {ui.kind === 'ask_user' && '等待用户回答'}
          {ui.kind === 'error' && '错误'}
        </span>
      </div>

      <div ref={streamRef} className="chat-stream">
        {msgs.length === 0 && (
          <div className="muted" style={{ padding: 16 }}>
            向 Agent 描述一个 Kubernetes 任务,例如 "列出 production 命名空间下所有 Pod"。
          </div>
        )}
        {msgs.map((m) => (
          <Bubble key={m.id} msg={m} />
        ))}
      </div>

      <div className="composer">
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              send()
            }
          }}
          disabled={inputDisabled}
          placeholder="输入自然语言,Enter 发送"
        />
        {ui.kind === 'streaming' ? (
          <button onClick={stop} disabled={!busy} className="danger">停止</button>
        ) : (
          <button onClick={send} disabled={!canSend()} className="primary">发送</button>
        )}
      </div>

      {ui.kind === 'plan_awaiting' && (
        <PlanModal
          plan={ui.plan}
          onConfirm={() => void confirmPlan()}
          onCancel={() => void cancelPlan()}
          busy={busy}
        />
      )}
      {ui.kind === 'ask_user' && (
        <AskUserForm ask={ui.ask} onSubmit={(a) => void submitAsk(a)} busy={busy} />
      )}
    </div>
  )
}

function Bubble({ msg }: { msg: Msg }) {
  if (msg.kind === 'user') {
    return (
      <div className="msg user">
        <div className="bubble user">{msg.text}</div>
      </div>
    )
  }
  if (msg.kind === 'system') {
    return (
      <div className="msg" style={{ alignSelf: 'center' }}>
        <span className="muted">— {msg.text} —</span>
      </div>
    )
  }
  // assistant
  return (
    <div className="msg assistant">
      {msg.reasoning && (
        <details style={{ marginBottom: 6 }}>
          <summary className="muted">思考过程</summary>
          <pre style={{ whiteSpace: 'pre-wrap' }}>{msg.reasoning}</pre>
        </details>
      )}
      {msg.text && <div className="bubble assistant">{msg.text}</div>}
      {msg.tools.map((t) => (
        <details
          key={t.id}
          className={`bubble ${t.result ? (t.result.ok ? 'tool-ok' : 'tool-err') : ''}`}
          style={{ marginTop: 6 }}
        >
          <summary>
            🔧 {t.name}
            {t.result && (
              <span className="muted" style={{ marginLeft: 8 }}>
                {t.result.ok ? '✓' : '✗'}
              </span>
            )}
          </summary>
          <div style={{ marginTop: 6 }}>
            <div className="muted">输入:</div>
            <pre>{formatJson(t.input)}</pre>
            {t.result && (
              <>
                <div className="muted">{t.result.ok ? '输出:' : '错误:'}</div>
                <pre>{t.result.ok ? formatJson(t.result.output) : t.result.error}</pre>
              </>
            )}
          </div>
        </details>
      ))}
    </div>
  )
}

function formatJson(v: unknown): string {
  if (typeof v === 'string') {
    // Try parse: many payloads are pre-serialised JSON strings.
    try {
      return JSON.stringify(JSON.parse(v), null, 2)
    } catch {
      return v
    }
  }
  try {
    return JSON.stringify(v, null, 2)
  } catch {
    return String(v)
  }
}

function formatError(err: unknown): string {
  if (err instanceof ApiCallError) return `${err.code}: ${err.message}`
  if (err instanceof Error) return err.message
  return String(err)
}