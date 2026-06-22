import React from 'react'
import type { Session } from '../api'
import { exportSessionUrl, type SessionSort, type SessionOrder } from '../api'
import { ConfirmModal } from '../components/ConfirmModal'

interface Props {
  sessions: Session[]
  activeId: string | null
  streaming: boolean
  searchQ: string
  sort: SessionSort
  order: SessionOrder
  onSearch: (q: string) => void
  onSort: (sort: SessionSort, order: SessionOrder) => void
  onSelect: (id: string) => void
  onCreate: () => void
  onRename: (id: string, title: string) => void
  onDelete: (id: string) => void
  onBulkClear: () => void
  clusterNameById: (id: string) => string
  relativeTime: (epochSecs: number) => string
}

const SORT_OPTIONS: { value: string; label: string; sort: SessionSort; order: SessionOrder }[] = [
  { value: 'updated_at:desc', label: '更新时间↓', sort: 'updated_at', order: 'desc' },
  { value: 'updated_at:asc',  label: '更新时间↑', sort: 'updated_at', order: 'asc' },
  { value: 'created_at:desc', label: '创建时间↓', sort: 'created_at', order: 'desc' },
  { value: 'created_at:asc',  label: '创建时间↑', sort: 'created_at', order: 'asc' },
  { value: 'title:asc',       label: '标题 A→Z',  sort: 'title',      order: 'asc' },
  { value: 'title:desc',      label: '标题 Z→A',  sort: 'title',      order: 'desc' },
]

export function SessionsPanel(props: Props) {
  const [deleteId, setDeleteId] = React.useState<string | null>(null)
  const [editing, setEditing] = React.useState<{ id: string; title: string } | null>(null)
  const [bulkOpen, setBulkOpen] = React.useState(false)

  const sortValue = `${props.sort}:${props.order}`
  const activeDeleteSession = deleteId
    ? props.sessions.find((s) => s.id === deleteId)
    : null

  return (
    <div className="sessions-panel">
      <div className="panel-title" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <span>会话列表</span>
        <select
          value={sortValue}
          onChange={(e) => {
            const opt = SORT_OPTIONS.find((o) => o.value === e.target.value)
            if (opt) props.onSort(opt.sort, opt.order)
          }}
          data-testid="session-sort"
          style={{ fontSize: 11, padding: '2px 6px' }}
        >
          {SORT_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
      </div>

      <ul className="session-list" data-testid="session-list">
        {props.sessions.length === 0 && (
          <li className="muted" style={{ padding: 12, textAlign: 'center' }}>
            {props.searchQ ? '无匹配会话' : '暂无历史会话'}
          </li>
        )}
        {props.sessions.map((s) => (
          <SessionRow
            key={s.id}
            session={s}
            isActive={s.id === props.activeId}
            streaming={props.streaming && s.id === props.activeId}
            editing={editing && editing.id === s.id ? editing : null}
            onSelect={() => props.onSelect(s.id)}
            onStartEdit={() => setEditing({ id: s.id, title: s.title })}
            onChangeEdit={(t) => setEditing({ id: s.id, title: t })}
            onCommitEdit={() => {
              if (editing && editing.title.trim()) {
                props.onRename(s.id, editing.title.trim())
              }
              setEditing(null)
            }}
            onCancelEdit={() => setEditing(null)}
            onDelete={() => setDeleteId(s.id)}
            clusterName={s.cluster_id ? props.clusterNameById(s.cluster_id) : ''}
            relativeTime={props.relativeTime(s.updated_at)}
          />
        ))}
      </ul>

      <div className="panel-footer">
        <button onClick={props.onCreate} className="primary" data-testid="new-session">
          + 新建会话
        </button>
        <button
          onClick={() => setBulkOpen(true)}
          disabled={props.streaming || props.sessions.length === 0}
          data-testid="bulk-clear"
        >
          🗑 清除全部
        </button>
      </div>

      {activeDeleteSession && (
        <ConfirmModal
          title="删除会话"
          message={
            <>
              确认删除 <strong>{activeDeleteSession.title}</strong>? 这不可恢复。
            </>
          }
          confirmLabel="确认删除"
          onConfirm={() => {
            const id = deleteId
            setDeleteId(null)
            if (id) props.onDelete(id)
          }}
          onCancel={() => setDeleteId(null)}
          danger
        />
      )}

      {bulkOpen && (
        <ConfirmModal
          title="清空全部会话"
          message={
            <>
              确认删除全部 <strong>{props.sessions.length}</strong> 个会话? 这不可恢复。
            </>
          }
          confirmLabel="确认清空"
          onConfirm={() => {
            setBulkOpen(false)
            props.onBulkClear()
          }}
          onCancel={() => setBulkOpen(false)}
          danger
        />
      )}
    </div>
  )
}

function SessionRow({
  session,
  isActive,
  streaming,
  editing,
  onSelect,
  onStartEdit,
  onChangeEdit,
  onCommitEdit,
  onCancelEdit,
  onDelete,
  clusterName,
  relativeTime,
}: {
  session: Session
  isActive: boolean
  streaming: boolean
  editing: { id: string; title: string } | null
  onSelect: () => void
  onStartEdit: () => void
  onChangeEdit: (t: string) => void
  onCommitEdit: () => void
  onCancelEdit: () => void
  onDelete: () => void
  clusterName: string
  relativeTime: string
}) {
  const [menuOpen, setMenuOpen] = React.useState(false)
  const menuRef = React.useRef<HTMLUListElement>(null)
  const menuOpenRef = React.useRef(false)

  menuOpenRef.current = menuOpen

  React.useEffect(() => {
    if (!menuOpen) return
    const handler = (e: MouseEvent) => {
      const btn = (e.target as Element)?.closest('[data-testid="session-menu-btn"]')
      if (btn) return
      if (menuRef.current?.contains(e.target as Node)) return
      setMenuOpen(false)
    }
    document.addEventListener('click', handler)
    return () => document.removeEventListener('click', handler)
  }, [menuOpen])

  return (
    <li
      className={`session-row ${isActive ? 'active' : ''}`}
      onClick={onSelect}
      data-testid="session-row"
    >
      <div className="session-dot" />
      <div className="row-main">
        {editing ? (
          <input
            autoFocus
            value={editing.title}
            onChange={(e) => onChangeEdit(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                onCommitEdit()
              } else if (e.key === 'Escape') {
                e.preventDefault()
                onCancelEdit()
              }
            }}
            onBlur={onCommitEdit}
            onClick={(e) => e.stopPropagation()}
            data-testid="session-rename-input"
          />
        ) : (
          <span className="title" onDoubleClick={(e) => { e.stopPropagation(); onStartEdit() }}>
            {session.title}
          </span>
        )}
        <span className="muted time-line">{relativeTime}</span>
      </div>
      <button
        className="row-menu-btn"
        onClick={(e) => { e.stopPropagation(); setMenuOpen((v) => !v) }}
        aria-label="会话菜单"
        data-testid="session-menu-btn"
        style={editing ? { opacity: 0.3, pointerEvents: 'none' } : {}}
      >
        ⋯
      </button>
      {menuOpen && (
        <ul ref={menuRef} className="row-menu" onClick={(e) => e.stopPropagation()}>
          <li onClick={() => { setMenuOpen(false); onStartEdit() }}>✏️ 重命名</li>
          <li>
            <a href={exportSessionUrl(session.id, 'md')} download={`session-${session.id.slice(0, 8)}.md`}>
              📄 导出 Markdown
            </a>
          </li>
          <li>
            <a href={exportSessionUrl(session.id, 'json')} download={`session-${session.id.slice(0, 8)}.json`}>
              🗂 导出 JSON
            </a>
          </li>
          <li
            className="danger"
            onClick={() => { if (streaming) return; setMenuOpen(false); onDelete() }}
            style={streaming ? { opacity: 0.4, pointerEvents: 'none' } : {}}
            title={streaming ? '请先停止当前会话' : undefined}
            data-testid="session-delete"
          >
            🗑 删除
          </li>
        </ul>
      )}
    </li>
  )
}
