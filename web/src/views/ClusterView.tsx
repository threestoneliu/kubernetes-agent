import React from 'react'
import { ApiCallError, Cluster, createCluster, deleteCluster, listClusters } from '../api'
import { useToast } from '../components/ToastProvider'

export function ClusterView() {
  const { show } = useToast()
  const [clusters, setClusters] = React.useState<Cluster[]>([])
  const [loading, setLoading] = React.useState(false)
  const [name, setName] = React.useState('')
  const [kubeconfig, setKubeconfig] = React.useState('')
  const [submitting, setSubmitting] = React.useState(false)
  const [pendingDelete, setPendingDelete] = React.useState<string | null>(null)
  const [showModal, setShowModal] = React.useState(false)

  const refresh = React.useCallback(async () => {
    setLoading(true)
    try {
      const res = await listClusters()
      setClusters(res.clusters)
    } catch (err) {
      show(formatError(err))
    } finally {
      setLoading(false)
    }
  }, [show])

  React.useEffect(() => {
    void refresh()
  }, [refresh])

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim() || !kubeconfig.trim()) return
    setSubmitting(true)
    try {
      await createCluster({ name: name.trim(), kubeconfig })
      setName('')
      setKubeconfig('')
      await refresh()
    } catch (err) {
      show(formatError(err))
    } finally {
      setSubmitting(false)
    }
  }

  function handleModalSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim() || !kubeconfig.trim()) return
    setSubmitting(true)
    createCluster({ name: name.trim(), kubeconfig })
      .then(() => { setShowModal(false); setName(''); setKubeconfig('') })
      .catch((err) => show(formatError(err)))
      .finally(() => setSubmitting(false))
  }

  async function remove(id: string) {
    setPendingDelete(id)
    try {
      await deleteCluster(id)
      await refresh()
    } catch (err) {
      show(formatError(err))
    } finally {
      setPendingDelete(null)
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <h2 style={{ margin: 0 }}>集群管理</h2>
      <div className="toolbar">
        <strong>已配置的集群</strong>
        <span className="muted">{clusters.length} 个</span>
        <button onClick={() => void refresh()} disabled={loading}>刷新</button>
        <button onClick={() => setShowModal(true)}>新建集群</button>
      </div>
      {showModal && (
        <div
          className="modal-overlay"
          role="dialog"
          aria-modal="true"
          onClick={(e) => { if (e.target === e.currentTarget) setShowModal(false) }}
        >
          <div className="modal">
            <h2>新建集群</h2>
            <form className="form-grid" onSubmit={handleModalSubmit}>
              <label>
                名称
                <input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="例如: dev"
                  disabled={submitting}
                />
              </label>
              <label>
                kubeconfig (YAML)
                <textarea
                  value={kubeconfig}
                  onChange={(e) => setKubeconfig(e.target.value)}
                  placeholder="apiVersion: v1\nkind: Config\n..."
                  disabled={submitting}
                />
              </label>
              <div className="modal-actions">
                <button type="button" className="cancel" onClick={() => setShowModal(false)} disabled={submitting}>取消</button>
                <button type="submit" className="primary" disabled={submitting || !name.trim() || !kubeconfig.trim()}>
                  {submitting ? '提交中…' : '添加'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
      <div className="list">
        {clusters.map((c) => (
          <div className="row" key={c.id}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div>
                <strong>{c.name}</strong>{' '}
                <span className="muted" style={{ fontSize: 12 }}>{c.id}</span>
              </div>
              <div className="muted" style={{ wordBreak: 'break-all' }}>{c.server || '(无 server)'}</div>
              <div className="muted">user: {c.user || '-'}</div>
            </div>
            <button
              className="danger"
              onClick={() => void remove(c.id)}
              disabled={pendingDelete === c.id}
            >
              {pendingDelete === c.id ? '删除中…' : '删除'}
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}

function formatError(err: unknown): string {
  if (err instanceof ApiCallError) return `${err.code}: ${err.message}`
  if (err instanceof Error) return err.message
  return String(err)
}
