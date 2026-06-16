import React from 'react'
import { ApiCallError, Policy, listPolicies, setPolicyEnabled, updatePolicy } from '../api'
import { useToast } from '../components/ToastProvider'

type Effect = 'allow' | 'confirm' | 'deny' | string

function EffectBadge({ effect }: { effect: Effect }) {
  const known = ['allow', 'confirm', 'deny'].includes(effect) ? effect : ''
  return (
    <span className={`badge ${known}`}>
      {effect || '(unknown)'}
    </span>
  )
}

export function PolicyView() {
  const { show } = useToast()
  const [policies, setPolicies] = React.useState<Policy[]>([])
  const [loading, setLoading] = React.useState(false)
  const [editing, setEditing] = React.useState<string | null>(null)
  const [draft, setDraft] = React.useState('')
  const [saving, setSaving] = React.useState(false)

  const refresh = React.useCallback(async () => {
    setLoading(true)
    try {
      const res = await listPolicies()
      setPolicies(res.policies)
    } catch (err) {
      show(formatError(err))
    } finally {
      setLoading(false)
    }
  }, [show])

  React.useEffect(() => {
    void refresh()
  }, [refresh])

  async function toggle(p: Policy, enabled: boolean) {
    try {
      await setPolicyEnabled(p.id, enabled)
      await refresh()
    } catch (err) {
      show(formatError(err))
    }
  }

  function startEdit(p: Policy) {
    setEditing(p.id)
    setDraft(p.yaml)
  }

  function cancelEdit() {
    setEditing(null)
    setDraft('')
  }

  async function saveEdit(p: Policy) {
    setSaving(true)
    try {
      await updatePolicy(p.id, draft)
      setEditing(null)
      setDraft('')
      await refresh()
    } catch (err) {
      show(formatError(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <h2 style={{ margin: 0 }}>策略编辑</h2>
      <div className="toolbar">
        <span className="muted">{policies.length} 条策略</span>
        <button onClick={() => void refresh()} disabled={loading}>刷新</button>
      </div>
      <div className="list">
        {policies.map((p) => (
          <div key={p.id} style={{ padding: '12px 0', borderBottom: '1px solid var(--border-soft)' }}>
            <div className="row">
              <div style={{ flex: 1 }}>
                <strong>{p.name}</strong>{' '}
                <span className="muted" style={{ fontSize: 12 }}>{p.id}</span>
              </div>
              <EffectBadge effect={extractEffect(p.yaml)} />
              <label style={{ display: 'flex', alignItems: 'center', gap: 6, marginLeft: 8 }}>
                <input
                  type="checkbox"
                  checked={p.enabled}
                  onChange={(e) => void toggle(p, e.target.checked)}
                />
                {p.enabled ? '启用' : '已停用'}
              </label>
              {editing === p.id ? (
                <>
                  <button onClick={cancelEdit} disabled={saving}>取消</button>
                  <button
                    className="primary"
                    onClick={() => void saveEdit(p)}
                    disabled={saving || !draft.trim()}
                  >
                    {saving ? '保存中…' : '保存'}
                  </button>
                </>
              ) : (
                <button onClick={() => startEdit(p)}>编辑 YAML</button>
              )}
            </div>
            {editing === p.id ? (
              <textarea
                className="policy-yaml"
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                disabled={saving}
                style={{ width: '100%', marginTop: 8 }}
              />
            ) : (
              <details style={{ marginTop: 6 }}>
                <summary className="muted">查看 YAML</summary>
                <pre>{p.yaml}</pre>
              </details>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

function extractEffect(yaml: string): string {
  // Cheap best-effort parse: pull the first "effect:" line out of the YAML
  // so the badge stays accurate without dragging in a full YAML parser.
  for (const line of yaml.split('\n')) {
    const m = line.match(/^\s*effect:\s*(\S+)\s*$/)
    if (m) return m[1]
  }
  return ''
}

function formatError(err: unknown): string {
  if (err instanceof ApiCallError) return `${err.code}: ${err.message}`
  if (err instanceof Error) return err.message
  return String(err)
}