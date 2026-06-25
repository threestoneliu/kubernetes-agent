import React from 'react'
import { ApiCallError, Policy, deletePolicy, listPolicies, setPolicyEnabled } from '../api'
import { ConfirmModal } from '../components/ConfirmModal'
import { PolicyFormModal } from '../components/PolicyFormModal'
import { useToast } from '../components/ToastProvider'

function EffectBadge({ effect }: { effect: string }) {
  const known = ['allow', 'confirm', 'deny'].includes(effect) ? effect : ''
  return (
    <span className={`badge ${known}`}>
      {effect || '(unknown)'}
    </span>
  )
}

function extractEffect(yamlStr: string): string {
  for (const line of yamlStr.split('\n')) {
    const m = line.match(/^\s*effect:\s*(\S+)\s*$/)
    if (m) return m[1]
  }
  return ''
}

export function PolicyView() {
  const { show } = useToast()
  const [policies, setPolicies] = React.useState<Policy[]>([])
  const [loading, setLoading] = React.useState(false)
  const [showForm, setShowForm] = React.useState(false)
  const [editingPolicy, setEditingPolicy] = React.useState<Policy | null>(null)
  const [deleteId, setDeleteId] = React.useState<string | null>(null)

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

  React.useEffect(() => { void refresh() }, [refresh])

  async function toggle(p: Policy, enabled: boolean) {
    try {
      await setPolicyEnabled(p.id, enabled)
      await refresh()
    } catch (err) {
      show(formatError(err))
    }
  }

  async function handleDelete(id: string) {
    setDeleteId(null)
    try {
      await deletePolicy(id)
      await refresh()
    } catch (err) {
      show(formatError(err))
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, height: '100%', overflow: 'hidden' }}>
      <h2 style={{ margin: 0 }}>策略编辑</h2>
      <div className="toolbar">
        <span className="muted">{policies.length} 条策略</span>
        <button onClick={() => void refresh()} disabled={loading}>刷新</button>
        <button onClick={() => { setEditingPolicy(null); setShowForm(true) }}>+ 新建策略</button>
      </div>
      <div className="list" style={{ flex: 1, overflowY: 'auto' }}>
        {policies.map((p) => (
          <div key={p.id} style={{ padding: '12px 0', borderBottom: '1px solid var(--border-soft)' }}>
            <div className="row" style={{ alignItems: 'center' }}>
              <div style={{ flex: 1, minWidth: 0 }}>
                <strong>{p.name}</strong>
                <span className="muted" style={{ fontSize: 12 }}> {p.id}</span>
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
              <button onClick={() => { setEditingPolicy(p); setShowForm(true) }} style={{ marginLeft: 8 }}>编辑</button>
              {['deny-delete-system-ns', 'deny-dangerous-kinds', 'deny-privileged', 'confirm-production'].includes(p.name)
                ? null
                : <button style={{ color: '#d32f2f', marginLeft: 4 }} onClick={() => setDeleteId(p.id)}>删除</button>
              }
            </div>
          </div>
        ))}
      </div>

      {showForm && (
        <PolicyFormModal
          policy={editingPolicy}
          onClose={() => setShowForm(false)}
          onDone={() => { setShowForm(false); setEditingPolicy(null); void refresh() }}
          show={show}
        />
      )}

      {deleteId && (
        <ConfirmModal
          title="删除策略"
          message={<span>确认删除策略 <strong>{policies.find(p => p.id === deleteId)?.name}</strong>？此操作不可撤销。</span>}
          confirmLabel="确认删除"
          onConfirm={() => void handleDelete(deleteId!)}
          onCancel={() => setDeleteId(null)}
          danger
        />
      )}
    </div>
  )
}

function formatError(err: unknown): string {
  if (err instanceof ApiCallError) return `${err.code}: ${err.message}`
  if (err instanceof Error) return err.message
  return String(err)
}
