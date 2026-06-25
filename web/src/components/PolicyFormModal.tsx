import React from 'react'
import * as yaml from 'js-yaml'
import { createPolicy, Policy, updatePolicy } from '../api'
import { TagInput } from './TagInput'

type Effect = 'allow' | 'confirm' | 'deny'

interface PolicyForm {
  name: string
  effect: Effect
  action: { apply: boolean; delete: boolean; scale: boolean }
  namespace: string[]
  kind: string[]
  unsafeFields: string
}

const EMPTY_FORM: PolicyForm = {
  name: '',
  effect: 'deny',
  action: { apply: true, delete: false, scale: false },
  namespace: [],
  kind: [],
  unsafeFields: '',
}

function serializeFormToYaml(form: PolicyForm): string {
  const actions: string[] = []
  if (form.action.apply) actions.push('apply')
  if (form.action.delete) actions.push('delete')
  if (form.action.scale) actions.push('scale')

  const match: Record<string, any> = {}
  if (actions.length > 0) match.action = actions
  if (form.namespace.length > 0) match.namespace = form.namespace
  if (form.kind.length > 0) match.kind = form.kind
  if (form.unsafeFields.trim()) {
    try { match.unsafeFields = yaml.load(form.unsafeFields) } catch { /* ignore */ }
  }

  const rule: Record<string, any> = {
    name: form.name,
    effect: form.effect,
    match,
  }

  return yaml.dump(rule, { sortKeys: false, lineWidth: -1 }).trim()
}

function parseYamlToForm(yamlText: string): PolicyForm | null {
  try {
    const doc = yaml.load(yamlText) as Record<string, any>
    if (!doc || typeof doc !== 'object') return null
    const match: Record<string, any> = doc.match || {}
    const actions: string[] = Array.isArray(match.action) ? match.action : []
    return {
      name: typeof doc.name === 'string' ? doc.name : '',
      effect: (['allow', 'confirm', 'deny'].includes(doc.effect)) ? doc.effect : 'deny',
      action: {
        apply: actions.includes('apply'),
        delete: actions.includes('delete'),
        scale: actions.includes('scale'),
      },
      namespace: Array.isArray(match.namespace) ? match.namespace.filter((v: any) => typeof v === 'string') : [],
      kind: Array.isArray(match.kind) ? match.kind.filter((v: any) => typeof v === 'string') : [],
      unsafeFields: match.unsafeFields ? yaml.dump(match.unsafeFields) : '',
    }
  } catch {
    return null
  }
}

interface PolicyFormModalProps {
  policy: Policy | null
  onClose: () => void
  onDone: () => void
  show: (msg: string) => void
}

export function PolicyFormModal({ policy, onClose, onDone, show }: PolicyFormModalProps) {
  const [form, setForm] = React.useState<PolicyForm>(() => {
    if (policy) {
      const parsed = parseYamlToForm(policy.yaml)
      return parsed ?? EMPTY_FORM
    }
    return EMPTY_FORM
  })
  const [yamlText, setYamlText] = React.useState(() =>
    policy ? policy.yaml : serializeFormToYaml(EMPTY_FORM)
  )
  const [yamlError, setYamlError] = React.useState(false)
  const [saving, setSaving] = React.useState(false)
  const yamlDebounce = React.useRef<ReturnType<typeof setTimeout> | null>(null)

  function updateForm(patch: Partial<PolicyForm>) {
    const next = { ...form, ...patch }
    setForm(next)
    setYamlText(serializeFormToYaml(next))
    setYamlError(false)
  }

  function handleYamlChange(e: React.ChangeEvent<HTMLTextAreaElement>) {
    const text = e.target.value
    setYamlText(text)
    if (yamlDebounce.current) clearTimeout(yamlDebounce.current)
    yamlDebounce.current = setTimeout(() => {
      const parsed = parseYamlToForm(text)
      if (parsed) {
        setForm(parsed)
        setYamlError(false)
      } else {
        setYamlError(true)
      }
    }, 300)
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.name.trim()) { show('规则名称不能为空'); return }
    if (yamlError) { show('YAML 格式错误，请修正后再保存'); return }
    const hasAction = form.action.apply || form.action.delete || form.action.scale
    if (!hasAction) { show('请至少选择一个操作类型'); return }
    setSaving(true)
    try {
      if (policy) {
        await updatePolicy(policy.id, yamlText)
      } else {
        await createPolicy(yamlText)
      }
      onDone()
    } catch (err) {
      show(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  const isEdit = policy !== null

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()} style={{ minWidth: 720, maxWidth: 900 }}>
        <div style={{ display: 'flex', gap: 16 }}>
          {/* LEFT: Form (60%) */}
          <div style={{ flex: '0 0 60%' }}>
            <h3 style={{ margin: '0 0 16px' }}>{isEdit ? '编辑策略' : '新建策略'}</h3>
            <form onSubmit={handleSubmit}>
              <label style={{ display: 'block', marginBottom: 12 }}>
                <span className="muted" style={{ fontSize: 12 }}>规则名称</span>
                <input
                  value={form.name}
                  onChange={e => updateForm({ name: e.target.value })}
                  placeholder="例如: deny-privileged"
                  style={{ width: '100%', marginTop: 4 }}
                />
              </label>

              <label style={{ display: 'block', marginBottom: 12 }}>
                <span className="muted" style={{ fontSize: 12 }}>效果</span>
                <select
                  value={form.effect}
                  onChange={e => updateForm({ effect: e.target.value as Effect })}
                  style={{ width: '100%', marginTop: 4 }}
                >
                  <option value="allow">allow — 直接放行</option>
                  <option value="confirm">confirm — 需用户确认</option>
                  <option value="deny">deny — 直接拒绝</option>
                </select>
              </label>

              <div style={{ marginBottom: 12 }}>
                <span className="muted" style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>操作类型</span>
                {(['apply', 'delete', 'scale'] as const).map(a => (
                  <label key={a} style={{ marginRight: 12 }}>
                    <input
                      type="checkbox"
                      checked={form.action[a]}
                      onChange={() => updateForm({ action: { ...form.action, [a]: !form.action[a] } })}
                    />
                    {a}
                  </label>
                ))}
              </div>

              <TagInput
                label="命名空间"
                tags={form.namespace}
                onChange={ns => updateForm({ namespace: ns })}
                placeholder="输入后回车添加"
              />

              <TagInput
                label="资源类型"
                tags={form.kind}
                onChange={k => updateForm({ kind: k })}
                placeholder="输入后回车添加"
              />

              <label style={{ display: 'block', marginBottom: 12 }}>
                <span className="muted" style={{ fontSize: 12 }}>危险字段 (JSONPath → 值)</span>
                <textarea
                  value={form.unsafeFields}
                  onChange={e => updateForm({ unsafeFields: e.target.value })}
                  rows={3}
                  placeholder={'spec.template.spec.containers[*].securityContext.privileged: true'}
                  style={{ width: '100%', marginTop: 4, resize: 'vertical', fontFamily: 'monospace', fontSize: 12 }}
                />
              </label>

              <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 16 }}>
                <button type="button" onClick={onClose} disabled={saving}>取消</button>
                <button
                  type="submit"
                  className="primary"
                  disabled={saving || !form.name.trim() || !(form.action.apply || form.action.delete || form.action.scale)}
                >
                  {saving ? '保存中…' : '保存'}
                </button>
              </div>
            </form>
          </div>

          {/* RIGHT: YAML (40%) */}
          <div style={{ flex: 1 }}>
            <h4 style={{ margin: '0 0 8px', fontSize: 14, fontWeight: 500 }}>YAML</h4>
            <textarea
              value={yamlText}
              onChange={handleYamlChange}
              style={{
                width: '100%',
                height: 'calc(100% - 20px)',
                minHeight: 300,
                fontFamily: 'monospace',
                fontSize: 12,
                border: yamlError ? '1px solid #d32f2f' : '1px solid var(--border)',
                borderRadius: 4,
                padding: 8,
                resize: 'vertical',
                background: yamlError ? '#fff8f8' : 'transparent',
                boxSizing: 'border-box',
              }}
            />
          </div>
        </div>
      </div>
    </div>
  )
}
