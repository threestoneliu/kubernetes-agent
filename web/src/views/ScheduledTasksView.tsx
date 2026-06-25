import React from 'react'
import {
  listScheduledTasks, listSessions, createScheduledTask,
  deleteScheduledTask, updateScheduledTask, runScheduledTask,
  type ScheduledTask, type Session,
} from '../api'
import { ConfirmModal } from '../components/ConfirmModal'
import { useToast } from '../components/ToastProvider'

export function ScheduledTasksView() {
  const { show } = useToast()
  const [tasks, setTasks] = React.useState<ScheduledTask[]>([])
  const [sessions, setSessions] = React.useState<Session[]>([])
  const [deleteId, setDeleteId] = React.useState<string | null>(null)
  const [createOpen, setCreateOpen] = React.useState(false)
  const [loading, setLoading] = React.useState(true)
  const [runningIds, setRunningIds] = React.useState<Set<string>>(new Set())

  async function refresh() {
    try {
      const [tRes, sRes] = await Promise.all([listScheduledTasks(), listSessions()])
      setTasks(tRes)
      setSessions(sRes.sessions)
    } catch (err) {
      show(formatError(err))
    } finally {
      setLoading(false)
    }
  }

  React.useEffect(() => { void refresh() }, [show])

  async function handleToggle(task: ScheduledTask) {
    try {
      await updateScheduledTask(task.id, { enabled: !task.enabled })
      await refresh()
    } catch (err) {
      show(formatError(err))
    }
  }

  async function handleRun(task: ScheduledTask) {
    setRunningIds((s) => new Set([...s, task.id]))
    try {
      await runScheduledTask(task.id)
      show('任务已触发')
      await refresh()
    } catch (err) {
      show(formatError(err))
    } finally {
      setRunningIds((s) => { const n = new Set(s); n.delete(task.id); return n })
    }
  }

  async function handleDelete(id: string) {
    setDeleteId(null)
    try {
      await deleteScheduledTask(id)
      await refresh()
    } catch (err) {
      show(formatError(err))
    }
  }

  function relativeTime(epochSecs: number | undefined): string {
    if (!epochSecs) return '从未'
    const diff = Math.floor(Date.now() / 1000) - epochSecs
    if (diff < 60) return '刚刚'
    if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`
    if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`
    if (diff < 86400 * 30) return `${Math.floor(diff / 86400)} 天前`
    return new Date(epochSecs * 1000).toLocaleDateString()
  }

  const deleteTask = deleteId ? tasks.find((t) => t.id === deleteId) : null

  return (
    <div style={{ padding: 16 }}>
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 16, justifyContent: 'space-between' }}>
        <h2 style={{ margin: 0 }}>定时任务</h2>
        <button className="primary" onClick={() => setCreateOpen(true)}>
          + 新建任务
        </button>
      </div>

      {loading ? (
        <p className="muted">加载中…</p>
      ) : tasks.length === 0 ? (
        <p className="muted">暂无定时任务</p>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid var(--border)', textAlign: 'left' }}>
              <th style={{ padding: '8px 4px' }}>名称</th>
              <th style={{ padding: '8px 4px', maxWidth: 200 }}>指令</th>
              <th style={{ padding: '8px 4px' }}>会话</th>
              <th style={{ padding: '8px 4px' }}>类型</th>
              <th style={{ padding: '8px 4px' }}>最近运行</th>
              <th style={{ padding: '8px 4px' }}>运行次数</th>
              <th style={{ padding: '8px 4px' }}>状态</th>
              <th style={{ padding: '8px 4px' }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {tasks.map((task) => {
              const sess = sessions.find((s) => s.id === task.session_id)
              return (
                <tr key={task.id} style={{ borderBottom: '1px solid var(--border, #eee)' }}>
                  <td style={{ padding: '8px 4px' }}>
                    <span style={{ fontWeight: 500 }}>{task.name}</span>
                  </td>
                  <td className="muted" style={{ padding: '8px 4px', fontSize: 12, maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={task.prompt}>
                    {task.prompt}
                  </td>
                  <td className="muted" style={{ padding: '8px 4px', fontSize: 12 }}>
                    {sess?.title ?? task.session_id.slice(0, 8)}
                  </td>
                  <td className="muted" style={{ padding: '8px 4px', fontSize: 12 }}>
                    {task.cron_expr ? `CRON: ${task.cron_expr}` : task.once_at ? '一次性' : '-'}
                  </td>
                  <td className="muted" style={{ padding: '8px 4px', fontSize: 12 }}>
                    {relativeTime(task.last_run)}
                  </td>
                  <td className="muted" style={{ padding: '8px 4px', fontSize: 12 }}>
                    {task.run_count}
                  </td>
                  <td style={{ padding: '8px 4px' }}>
                    <label style={{ display: 'flex', alignItems: 'center', gap: 4, cursor: 'pointer' }}>
                      <input
                        type="checkbox"
                        checked={task.enabled}
                        onChange={() => handleToggle(task)}
                      />
                      <span style={{ fontSize: 12, color: task.enabled ? 'var(--accent)' : 'var(--muted)' }}>
                        {task.enabled ? '启用' : '禁用'}
                      </span>
                    </label>
                  </td>
                  <td style={{ padding: '8px 4px' }}>
                    <div style={{ display: 'flex', gap: 4 }}>
                      <button
                        onClick={() => handleRun(task)}
                        disabled={runningIds.has(task.id)}
                        style={{ fontSize: 12, padding: '2px 8px' }}
                      >
                        {runningIds.has(task.id) ? '运行中…' : '▶ 立即运行'}
                      </button>
                      <button
                        onClick={() => setDeleteId(task.id)}
                        style={{ fontSize: 12, padding: '2px 8px', color: '#d32f2f' }}
                      >
                        🗑
                      </button>
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      )}

      {createOpen && (
        <CreateTaskModal
          sessions={sessions}
          onClose={() => setCreateOpen(false)}
          onCreated={() => { setCreateOpen(false); void refresh() }}
          show={show}
        />
      )}

      {deleteTask && (
        <ConfirmModal
          title="删除任务"
          message={<span>确认删除 <strong>{deleteTask.name}</strong>？</span>}
          confirmLabel="确认删除"
          onConfirm={() => handleDelete(deleteTask.id)}
          onCancel={() => setDeleteId(null)}
          danger
        />
      )}
    </div>
  )
}

function CreateTaskModal({
  sessions,
  onClose,
  onCreated,
  show,
}: {
  sessions: Session[]
  onClose: () => void
  onCreated: () => void
  show: (msg: string) => void
}) {
  const [name, setName] = React.useState('')
  const [prompt, setPrompt] = React.useState('')
  const [cronExpr, setCronExpr] = React.useState('')
  const [sessionId, setSessionId] = React.useState('')
  const [saving, setSaving] = React.useState(false)

  // Default to first session if only one
  React.useEffect(() => {
    if (!sessionId && sessions.length > 0) setSessionId(sessions[0].id)
  }, [sessions, sessionId])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim() || !prompt.trim() || !cronExpr.trim() || !sessionId) return
    setSaving(true)
    try {
      await createScheduledTask({ name: name.trim(), prompt: prompt.trim(), cron_expr: cronExpr.trim(), session_id: sessionId, created_by: 'user' })
      onCreated()
    } catch (err) {
      show(formatError(err))
      setSaving(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ minWidth: 420 }}>
        <h3 style={{ margin: '0 0 16px' }}>新建定时任务</h3>
        <form onSubmit={handleSubmit}>
          <label style={{ display: 'block', marginBottom: 12 }}>
            <span className="muted" style={{ fontSize: 12 }}>任务名称</span>
            <input
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="例如：每5分钟健康检查"
              style={{ width: '100%', marginTop: 4 }}
            />
          </label>
          <label style={{ display: 'block', marginBottom: 12 }}>
            <span className="muted" style={{ fontSize: 12 }}>定时指令 <span style={{ fontWeight: 'normal', color: '#d32f2f' }}>*</span></span>
            <textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="例如：列出 production 命名空间下所有 Pod 的健康状态"
              rows={3}
              style={{ width: '100%', marginTop: 4, resize: 'vertical' }}
            />
          </label>
          <label style={{ display: 'block', marginBottom: 12 }}>
            <span className="muted" style={{ fontSize: 12 }}>CRON 表达式 <a href="https://crontab.guru/" target="_blank" rel="noopener" style={{ fontWeight: 'normal' }}>参考</a></span>
            <input
              value={cronExpr}
              onChange={(e) => setCronExpr(e.target.value)}
              placeholder="*/5 * * * * (每5分钟)"
              style={{ width: '100%', marginTop: 4 }}
            />
          </label>
          <label style={{ display: 'block', marginBottom: 16 }}>
            <span className="muted" style={{ fontSize: 12 }}>所属会话</span>
            <select
              value={sessionId}
              onChange={(e) => setSessionId(e.target.value)}
              style={{ width: '100%', marginTop: 4 }}
            >
              {sessions.map((s) => (
                <option key={s.id} value={s.id}>{s.title}</option>
              ))}
            </select>
          </label>
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            <button type="button" onClick={onClose}>取消</button>
            <button
              type="submit"
              disabled={saving || !name.trim() || !prompt.trim() || !cronExpr.trim() || !sessionId}
              className="primary"
            >
              {saving ? '创建中…' : '创建'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function formatError(err: unknown): string {
  if (err && typeof err === 'object' && 'message' in err) return String((err as { message: unknown }).message)
  return String(err)
}
