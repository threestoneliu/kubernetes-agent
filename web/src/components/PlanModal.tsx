import type { PendingPlan } from '../state'
import { RiskBadge } from './RiskBadge'
import { Markdown } from './Markdown'

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
          <details open>
            <summary>{plan.diffs.length} 个变更</summary>
            <pre>{JSON.stringify(plan.diffs, null, 2)}</pre>
          </details>
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