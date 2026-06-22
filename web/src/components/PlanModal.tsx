import type { PendingPlan } from '../state'
import { RiskBadge } from './RiskBadge'
import { Markdown } from './Markdown'

interface DiffItem {
  verb?: string
  resource?: string
  namespace?: string
  name?: string
  kind?: string
  [key: string]: unknown
}

function DiffCard({ diff }: { diff: DiffItem }) {
  const action = diff.verb?.toUpperCase() ?? 'UPDATE'
  const kind = diff.kind ?? diff.resource ?? 'Unknown'
  const name = diff.name ?? 'unknown'
  const ns = diff.namespace ?? 'default'

  // Extract key changes from the diff for summary
  const changes: string[] = []
  for (const [key, val] of Object.entries(diff)) {
    if (['verb', 'resource', 'namespace', 'name', 'kind', 'cluster_id'].includes(key)) continue
    if (val !== undefined && val !== null) {
      changes.push(`${key}: ${JSON.stringify(val)}`)
    }
  }
  const summary = changes.length > 0 ? changes.slice(0, 3).join(', ') : null

  // Build YAML preview from the diff item itself
  const yamlLines: string[] = []
  if (diff.kind || diff.resource) {
    yamlLines.push(`kind: ${diff.kind ?? diff.resource}`)
  }
  if (diff.name) {
    yamlLines.push(`metadata:`)
    yamlLines.push(`  name: ${diff.name}`)
    if (diff.namespace && diff.namespace !== 'default') {
      yamlLines.push(`  namespace: ${diff.namespace}`)
    }
  }
  for (const [key, val] of Object.entries(diff)) {
    if (['verb', 'resource', 'namespace', 'name', 'kind', 'cluster_id'].includes(key)) continue
    if (val !== undefined && val !== null) {
      if (typeof val === 'object') {
        yamlLines.push(`${key}:`)
        for (const [k2, v2] of Object.entries(val as Record<string, unknown>)) {
          yamlLines.push(`  ${k2}: ${JSON.stringify(v2)}`)
        }
      } else {
        yamlLines.push(`${key}: ${JSON.stringify(val)}`)
      }
    }
  }

  const actionColor = action === 'CREATE' ? 'green' : action === 'DELETE' ? 'red' : 'blue'

  return (
    <div style={{ border: '1px solid var(--border)', borderRadius: 6, padding: '10px 12px', marginBottom: 8 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
        <span style={{
          color: actionColor,
          fontWeight: 600,
          fontSize: 12,
          background: actionColor + '20',
          padding: '2px 6px',
          borderRadius: 4,
        }}>
          {action}
        </span>
        <span style={{ fontWeight: 500 }}>{kind}</span>
        <span className="muted">/</span>
        <span>{ns}/{name}</span>
      </div>
      {summary && (
        <div className="muted" style={{ fontSize: 13, marginBottom: 6 }}>
          {summary}
        </div>
      )}
      {yamlLines.length > 0 && (
        <details>
          <summary style={{ cursor: 'pointer', fontSize: 13, color: 'var(--accent)' }}>
            查看 YAML
          </summary>
          <pre style={{
            background: 'var(--bg-secondary)',
            padding: 8,
            borderRadius: 4,
            fontSize: 12,
            overflow: 'auto',
            marginTop: 6,
          }}>
            {yamlLines.join('\n')}
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
              <DiffCard key={i} diff={diff as DiffItem} />
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
