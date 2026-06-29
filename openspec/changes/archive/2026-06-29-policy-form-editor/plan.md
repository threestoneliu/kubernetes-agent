# Policy Form Editor Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Replace raw YAML editing with a structured form + YAML side-by-side editor for policy creation and editing in the web UI.

**Architecture:** `PolicyFormModal` is the single new component. It manages all form state internally (no external state lib). YAML serialization/deserialization lives as module-level helpers in the same file. `TagInput` is a reusable sub-component. The `PolicyView` list only holds the policy list and delegates all editing to `PolicyFormModal`.

**Tech Stack:** React (hooks), plain CSS (no Tailwind), `js-yaml` for YAML parsing, no external state management.

---

## Task 1: TagInput Component

**Files:**
- Create: `web/src/components/TagInput.tsx`
- Test: manual in browser

- [ ] **Step 1: Create TagInput skeleton**

```tsx
// web/src/components/TagInput.tsx
import React from 'react'

interface TagInputProps {
  label: string
  tags: string[]
  onChange: (tags: string[]) => void
  placeholder?: string
}

export function TagInput({ label, tags, onChange, placeholder = '' }: TagInputProps) {
  const [input, setInput] = React.useState('')

  function addTag() {
    const trimmed = input.trim()
    if (!trimmed) return
    if (tags.includes(trimmed)) { setInput(''); return }
    onChange([...tags, trimmed])
    setInput('')
  }

  function removeTag(index: number) {
    onChange(tags.filter((_, i) => i !== index))
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') { e.preventDefault(); addTag() }
  }

  return (
    <label style={{ display: 'block', marginBottom: 12 }}>
      <span className="muted" style={{ fontSize: 12 }}>{label}</span>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginTop: 4 }}>
        {tags.map((tag, i) => (
          <span key={i} style={{ display: 'flex', alignItems: 'center', gap: 2, background: 'var(--accent)', color: '#fff', borderRadius: 4, padding: '2px 6px', fontSize: 12 }}>
            {tag}
            <button type="button" onClick={() => removeTag(i)} style={{ background: 'none', border: 'none', color: 'inherit', cursor: 'pointer', padding: 0, lineHeight: 1 }}>×</button>
          </span>
        ))}
        <input
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={tags.length === 0 ? placeholder : ''}
          style={{ border: 'none', outline: 'none', background: 'transparent', fontSize: 12, minWidth: 80 }}
        />
      </div>
    </label>
  )
}
```

- [ ] **Step 2: Import and use in PolicyFormModal (stub placeholder first)**

After writing `PolicyFormModal`, add `import { TagInput } from '../components/TagInput'` and use it for namespace and kind fields.

---

## Task 2: YAML Serialization Helpers

**Files:**
- Modify: `web/src/views/PolicyView.tsx` (add module-level helpers at top)
- Test: manual in browser (create/edit flow)

- [ ] **Step 1: Add js-yaml import and PolicyForm interface**

Add at top of `PolicyView.tsx`:

```typescript
import yaml from 'js-yaml'

type Effect = 'allow' | 'confirm' | 'deny'

interface PolicyForm {
  name: string
  effect: Effect
  action: { apply: boolean; delete: boolean; scale: boolean }
  namespace: string[]
  kind: string[]
  unsafeFields: string
}
```

- [ ] **Step 2: Add default empty form constant**

```typescript
const EMPTY_FORM: PolicyForm = {
  name: '',
  effect: 'deny',
  action: { apply: true, delete: false, scale: false },
  namespace: [],
  kind: [],
  unsafeFields: '',
}
```

- [ ] **Step 3: Add serializeFormToYaml**

```typescript
function serializeFormToYaml(form: PolicyForm): string {
  const match: Record<string, any> = {}
  const actions = []
  if (form.action.apply) actions.push('apply')
  if (form.action.delete) actions.push('delete')
  if (form.action.scale) actions.push('scale')
  if (actions.length > 0) match.action = actions
  if (form.namespace.length > 0) match.namespace = form.namespace
  if (form.kind.length > 0) match.kind = form.kind
  const unsafeFields = form.unsafeFields.trim()
  if (unsafeFields) {
    try { match.unsafeFields = yaml.load(unsafeFields) } catch { /* ignore */ }
  }

  const rule: Record<string, any> = {
    name: form.name,
    effect: form.effect,
    match,
  }

  return yaml.dump(rule, { sortKeys: false, lineWidth: -1 }).trim()
}
```

- [ ] **Step 4: Add parseYamlToForm**

```typescript
function parseYamlToForm(yamlText: string): PolicyForm | null {
  try {
    const doc = yaml.load(yamlText) as Record<string, any>
    if (!doc || typeof doc !== 'object') return null
    const match: Record<string, any> = doc.match || {}
    const actions = match.action || []
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
```

- [ ] **Step 5: Verify serialize round-trip by console logging** (manual test only — not a unit test)

---

## Task 3: PolicyFormModal Component

**Files:**
- Modify: `web/src/views/PolicyView.tsx` — replace `CreatePolicyModal` with `PolicyFormModal`
- Test: manual in browser

- [ ] **Step 1: Write PolicyFormModal function signature and layout shell**

```typescript
function PolicyFormModal({
  policy,
  onClose,
  onDone,
  show,
}: {
  policy: Policy | null
  onClose: () => void
  onDone: () => void
  show: (msg: string) => void
}) {
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
```

- [ ] **Step 2: Implement left panel (form fields)**

Place inside the `return`:

```jsx
<div className="modal-overlay" onClick={onClose}>
  <div className="modal" onClick={e => e.stopPropagation()} style={{ minWidth: 720, maxWidth: 900 }}>
    <div style={{ display: 'flex', gap: 16 }}>
      {/* LEFT: Form (60%) */}
      <div style={{ flex: '0 0 60%' }}>
        <h3 style={{ margin: '0 0 16px' }}>{policy ? '编辑策略' : '新建策略'}</h3>
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
              placeholder={"spec.template.spec.containers[*].securityContext.privileged: true"}
              style={{ width: '100%', marginTop: 4, resize: 'vertical', fontFamily: 'monospace', fontSize: 12 }}
            />
          </label>

          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 16 }}>
            <button type="button" onClick={onClose} disabled={saving}>取消</button>
            <button
              type="submit"
              className="primary"
              disabled={saving || !form.name.trim() || (!form.action.apply && !form.action.delete && !form.action.scale)}
            >
              {saving ? '保存中…' : '保存'}
            </button>
          </div>
        </form>
      </div>

      {/* RIGHT: YAML (40%) */}
      <div style={{ flex: 1 }}>
        <h3 style={{ margin: '0 0 8px', fontSize: 14 }}>YAML</h3>
        <textarea
          value={yamlText}
          onChange={handleYamlChange}
          style={{
            width: '100%', height: '100%', minHeight: 300,
            fontFamily: 'monospace', fontSize: 12,
            border: yamlError ? '1px solid #d32f2f' : '1px solid var(--border)',
            borderRadius: 4, padding: 8, resize: 'vertical',
            background: yamlError ? '#fff8f8' : 'transparent',
          }}
        />
      </div>
    </div>
  </div>
</div>
```

- [ ] **Step 3: Add helper functions inside component**

```typescript
  function updateForm(patch: Partial<PolicyForm>) {
    const next = { ...form, ...patch }
    setForm(next)
    // Immediately sync to YAML
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
    if (!form.name.trim()) return
    if (yamlError) { show('YAML 格式错误，请修正后再保存'); return }
    setSaving(true)
    try {
      if (policy) {
        await updatePolicy(policy.id, yamlText)
      } else {
        await createPolicy(yamlText)
      }
      onDone()
    } catch (err) {
      show(formatError(err))
    } finally {
      setSaving(false)
    }
  }
```

---

## Task 4: PolicyView Integration

**Files:**
- Modify: `web/src/views/PolicyView.tsx`
- Test: manual in browser

- [ ] **Step 1: Remove `CreatePolicyModal`, `handleCreate`, `draft`, `yaml` state from PolicyView**

Delete: `CreatePolicyModal` function, `handleCreate`, `draft` state, `yaml` in `CreatePolicyModal`.

- [ ] **Step 2: Replace `showCreate` triggers with `PolicyFormModal(policy=null)`**

Toolbar button becomes:
```tsx
<button onClick={() => {/* open with null */}}>+ 新建策略</button>
```

- [ ] **Step 3: Replace inline edit buttons with `PolicyFormModal(policy=p)`**

In the list row, replace "编辑 YAML" + "删除" buttons with "编辑" button that opens the modal:
```tsx
<button onClick={() => {/* open with p */})}>编辑</button>
```

- [ ] **Step 4: Remove inline YAML textarea and "查看 YAML" `<details>` block from list rows**

- [ ] **Step 5: Add PolicyFormModal at bottom of render**

```tsx
{showCreateOrEdit && (
  <PolicyFormModal
    policy={editingPolicy}
    onClose={() => { setShowCreateOrEdit(false); setEditingPolicy(null) }}
    onDone={() => { setShowCreateOrEdit(false); setEditingPolicy(null); void refresh() }}
    show={show}
  />
)}
```

Where `showCreateOrEdit = showCreate || editingPolicy !== null`.

---

## Task 5: Install js-yaml

**Files:**
- Modify: `web/package.json`
- Test: `cd web && pnpm install && pnpm build`

- [ ] **Step 1: Install js-yaml**

Run: `cd web && pnpm add js-yaml && pnpm install`

- [ ] **Step 2: Verify build passes**

Run: `make build`

---

## Task 6: Manual Verification

**Files:** None (test only)

- [ ] **Step 1: 新建策略**
  1. Click "+ 新建策略"
  2. Form should be pre-filled with defaults
  3. Change name → right YAML updates immediately
  4. Type bad YAML in right panel → red border appears after 300ms
  5. Click save → policy appears in list

- [ ] **Step 2: 编辑策略**
  1. Click "编辑" on a policy row
  2. Form pre-filled from parsed YAML
  3. Change checkbox → YAML updates immediately
  4. Add namespace tag → YAML updates immediately
  5. Click save → policy updates in list

- [ ] **Step 3: YAML parse error guard**
  1. Break YAML manually → save button disabled (or shows error toast)

---

## Commit Points

After each task group completes successfully:
```bash
git add web/src/components/TagInput.tsx web/src/views/PolicyView.tsx web/package.json
git commit -m "feat(web): add TagInput component and PolicyFormModal"
```
