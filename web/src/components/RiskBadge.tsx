import type { Risk } from '../state'

const RISK_META: Record<Risk, { emoji: string; label: string; color: string }> = {
  low: { emoji: '🟢', label: '低风险', color: '#16a34a' },
  medium: { emoji: '🟡', label: '中风险', color: '#ca8a04' },
  high: { emoji: '🔴', label: '高风险', color: '#dc2626' },
}

export function RiskBadge({ risk }: { risk: Risk }) {
  const meta = RISK_META[risk]
  return (
    <span style={{ color: meta.color, fontWeight: 600 }} aria-label={meta.label}>
      {meta.emoji} {meta.label}
    </span>
  )
}