import type { PendingPlan } from '../state'
import { RiskBadge } from './RiskBadge'
import { Markdown } from './Markdown'

interface DiffItem {
  action: string
  resource: string
  name: string
  namespace: string
  before?: Record<string, unknown>
  after?: Record<string, unknown>
  risk?: string
  [key: string]: unknown
}

// Extract a human-readable kind from a manifest (after or before).
function getKind(after?: Record<string, unknown>, before?: Record<string, unknown>): string {
  const from = after ?? before ?? {}
  return (from.kind as string) ?? from.resource ?? 'Unknown'
}

// Build a YAML string from a manifest for collapsible display.
function toYAML(obj: Record<string, unknown>): string {
  // Use a simple YAML-ish serialization — just show the top-level
  // fields that are most relevant for a K8s resource.
  const lines: string[] = []
  const skip = new Set(['creationTimestamp', 'generation', 'managedFields', 'ownerReferences', 'resourceVersion', 'uid', 'status'])

  function walk(o: unknown, indent: number): void {
    if (o === null || o === undefined) return
    if (typeof o !== 'object') {
      lines.push(' '.repeat(indent) + JSON.stringify(o))
      return
    }
    if (Array.isArray(o)) {
      for (const item of o) {
        lines.push(' '.repeat(indent) + '-')
        walk(item, indent + 2)
      }
      return
    }
    for (const [k, v] of Object.entries(o as Record<string, unknown>)) {
      if (skip.has(k)) continue
      if (typeof v === 'object' && v !== null) {
        lines.push(' '.repeat(indent) + k + ':')
        walk(v, indent + 2)
      } else {
        lines.push(' '.repeat(indent) + k + ': ' + JSON.stringify(v))
      }
    }
  }

  walk(obj, 0)
  return lines.join('\n')
}

// Extract a short summary of what changed between before and after.
function summarizeChange(before?: Record<string, unknown>, after?: Record<string, unknown>): string | null {
  if (!before && !after) return null
  const parts: string[] = []

  // Replicas
  const beforeReplicas = before?.spec as Record<string, unknown> | undefined
  const afterReplicas = after?.spec as Record<string, unknown> | undefined
  if (beforeReplicas?.replicas !== undefined || afterReplicas?.replicas !== undefined) {
    parts.push(`replicas: ${beforeReplicas?.replicas ?? '?'} → ${afterReplicas?.replicas ?? '?'}`)
  }

  // Image
  const beforeContainers = (before?.spec as Record<string, unknown>)?.template as Record<string, unknown>
  const afterContainers = (after?.spec as Record<string, unknown>)?.template as Record<string, unknown>
  const beforeImg = (beforeContainers?.spec as Record<string, unknown>)?.containers as Array<Record<string, unknown>> | undefined
  const afterImg = (afterContainers?.spec as Record<string, unknown>)?.containers as Array<Record<string, unknown>> | undefined
  const bImg = beforeImg?.[0]?.image as string | undefined
  const aImg = afterImg?.[0]?.image as string | undefined
  if (bImg !== undefined || aImg !== undefined) {
    parts.push(`image: ${bImg ?? '?'} → ${aImg ?? '?'}`)
  }

  // Labels
  const bLabels = before?.metadata as Record<string, unknown>
  const aLabels = after?.metadata as Record<string, unknown>
  if (JSON.stringify(bLabels?.labels) !== JSON.stringify(aLabels?.labels)) {
    parts.push('labels changed')
  }

  // Ports
  const bPorts = beforeImg?.[0]?.ports as Array<Record<string, unknown>> | undefined
  const aPorts = afterImg?.[0]?.ports as Array<Record<string, unknown>> | undefined
  if (JSON.stringify(bPorts) !== JSON.stringify(aPorts)) {
    parts.push('ports changed')
  }

  return parts.length > 0 ? parts.join(' | ') : null
}

function DiffCard({ diff }: { diff: DiffItem }) {
  const action = diff.action?.toUpperCase() ?? 'APPLY'
  const kind = getKind(diff.after, diff.before)
  const name = diff.name ?? 'unknown'
  const ns = diff.namespace ?? 'default'
  // Use backend-generated summary (e.g. "创建 Deployment default/nginx") from diff.summary
  const summary: string | null = diff.summary ?? null

  const yaml = diff.after ? toYAML(diff.after) : diff.before ? toYAML(diff.before) : ''

  const actionColor: Record<string, string> = {
    CREATE: '#22c55e',
    APPLY: '#3b82f6',
    UPDATE: '#3b82f6',
    DELETE: '#ef4444',
    SCALE: '#f59e0b',
  }
  const color = actionColor[action] ?? '#6b7280'

  return (
    <div style={{ border: '1px solid var(--border)', borderRadius: 6, padding: '10px 12px', marginBottom: 8 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: summary ? 4 : 0 }}>
        <span style={{
          color,
          fontWeight: 600,
          fontSize: 11,
          background: color + '20',
          padding: '2px 6px',
          borderRadius: 4,
          letterSpacing: '0.02em',
        }}>
          {action}
        </span>
        <span style={{ fontWeight: 600 }}>{kind}</span>
        <span className="muted">/</span>
        <span>{ns}/{name}</span>
      </div>
      {summary && (
        <div style={{
          fontSize: 13,
          color: 'var(--text-secondary)',
          marginBottom: yaml ? 6 : 0,
          fontFamily: 'monospace',
          background: 'var(--bg-secondary)',
          padding: '4px 8px',
          borderRadius: 4,
        }}>
          {summary}
        </div>
      )}
      {yaml && (
        <details>
          <summary style={{
            cursor: 'pointer',
            fontSize: 12,
            color: 'var(--accent)',
            userSelect: 'none',
          }}>
            查看完整 YAML
          </summary>
          <pre style={{
            background: 'var(--bg-secondary)',
            padding: 8,
            borderRadius: 4,
            fontSize: 11,
            overflow: 'auto',
            marginTop: 6,
            maxHeight: 300,
            fontFamily: 'monospace',
          }}>
            {yaml}
          </pre>
        </details>
      )}
    </div>
  )
}

export function PlanModal({
  plan,
  onConfirm,
  onCancel,
  busy,
}: {
  plan: PendingPlan
  onConfirm: () => void
  onCancel: () => void
  busy: boolean
}) {
  return (
    <div className="modal-overlay" role="dialog" aria-modal="true">
      <div className="modal">
        <h2>执行计划确认</h2>
        <div style={{ marginBottom: 12 }}>
          <RiskBadge risk={plan.risk} />
          <span className="muted" style={{ marginLeft: 8 }}>plan_id: {plan.planId}</span>
        </div>
        <div className="md">
          <Markdown source={plan.summary || '_(无摘要)_'} />
        </div>
        {plan.diffs.length > 0 && (
          <div style={{ marginTop: 12 }}>
            <div style={{ fontWeight: 500, marginBottom: 8 }}>{plan.diffs.length} 个变更</div>
            {plan.diffs.map((diff, i) => (
              <DiffCard key={i} diff={diff as unknown as DiffItem} />
            ))}
          </div>
        )}
        <div className="modal-actions">
          <button onClick={onCancel} disabled={busy}>取消</button>
          <button className="primary" onClick={onConfirm} disabled={busy}>
            {busy ? '执行中…' : '确认执行'}
          </button>
        </div>
      </div>
    </div>
  )
}
