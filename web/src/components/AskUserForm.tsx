import React from 'react'
import type { PendingAsk } from '../state'

export function AskUserForm({
  ask,
  onSubmit,
  busy,
}: {
  ask: PendingAsk
  onSubmit: (answer: string) => void
  busy: boolean
}) {
  const [text, setText] = React.useState('')
  const [selected, setSelected] = React.useState<Set<string>>(new Set())

  function toggle(opt: string) {
    const next = new Set(selected)
    if (next.has(opt)) {
      next.delete(opt)
    } else {
      if (!ask.multi) next.clear()
      next.add(opt)
    }
    setSelected(next)
  }

  function submitOptions() {
    const opts = Array.from(selected)
    if (opts.length === 0) return
    onSubmit(ask.multi ? opts.join(', ') : opts[0])
  }

  function submitText() {
    const t = text.trim()
    if (!t) return
    onSubmit(t)
  }

  const hasOptions = ask.options.length > 0

  return (
    <div className="modal-overlay" role="dialog" aria-modal="true">
      <div className="modal">
        <h2>{ask.multi ? '请选择(可多选)' : '请选择'}</h2>
        <p>{ask.question}</p>
        {hasOptions ? (
          <div className="form-grid" style={{ marginTop: 12 }}>
            {ask.options.map((opt) => (
              <label key={opt} style={{ flexDirection: 'row', alignItems: 'center', gap: 8 }}>
                <input
                  type={ask.multi ? 'checkbox' : 'radio'}
                  name="ask-opt"
                  checked={selected.has(opt)}
                  onChange={() => toggle(opt)}
                  disabled={busy}
                />
                <span>{opt}</span>
              </label>
            ))}
            <div className="modal-actions">
              <button onClick={submitOptions} disabled={busy || selected.size === 0} className="primary">
                {busy ? '提交中…' : '提交选项'}
              </button>
            </div>
          </div>
        ) : null}
        <details style={{ marginTop: 12 }}>
          <summary>其他回答…</summary>
          <div className="form-grid" style={{ marginTop: 8 }}>
            <textarea
              value={text}
              onChange={(e) => setText(e.target.value)}
              rows={3}
              disabled={busy}
              placeholder="自由输入"
            />
            <div className="modal-actions">
              <button onClick={submitText} disabled={busy || !text.trim()} className="primary">
                {busy ? '提交中…' : '提交'}
              </button>
            </div>
          </div>
        </details>
      </div>
    </div>
  )
}