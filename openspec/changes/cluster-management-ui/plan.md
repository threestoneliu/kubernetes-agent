# Cluster Management UI Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development
> to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "新建集群" toolbar button that opens a modal dialog for cluster creation, replacing the current inline form at the top of ClusterView.

**Architecture:** No architecture changes — this is a pure UI refactor. The modal reuses existing form fields (name + kubeconfig) and API call (createCluster). State management via React useState.

**Tech Stack:** React/TypeScript, existing ConfirmModal component, existing API functions.

---

## Task 1: Refactor ClusterView to use modal form

**Files:**
- Modify: `web/src/views/ClusterView.tsx` (entire file)

- [ ] **Step 1: Add showModal state**

Add `const [showModal, setShowModal] = React.useState(false)` after the existing `submitting` state line.

- [ ] **Step 2: Replace toolbar**

Replace the existing `<div className="card">` containing the `<form>` with:

```tsx
<div className="toolbar">
  <strong>已配置的集群</strong>
  <span className="muted">{clusters.length} 个</span>
  <button onClick={() => void refresh()} disabled={loading}>刷新</button>
  <button onClick={() => setShowModal(true)}>新建集群</button>
</div>
```

- [ ] **Step 3: Remove embedded form**

Delete the entire `<div className="card">` block (the form with name/kubeconfig inputs).

- [ ] **Step 4: Add Modal**

After the toolbar div, add the Modal:

```tsx
{showModal && (
  <div
    className="modal-overlay"
    role="dialog"
    aria-modal="true"
    onClick={(e) => { if (e.target === e.currentTarget) setShowModal(false) }}
  >
    <div className="modal">
      <h2>新建集群</h2>
      <form onSubmit={(e) => {
        e.preventDefault()
        if (!name.trim() || !kubeconfig.trim()) return
        setSubmitting(true)
        createCluster({ name: name.trim(), kubeconfig })
          .then(() => { setShowModal(false); setName(''); setKubeconfig(''); void refresh() })
          .catch((err) => { show(formatError(err)) })
          .finally(() => setSubmitting(false))
      }}>
        <label>
          名称
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="例如: dev" disabled={submitting} />
        </label>
        <label>
          kubeconfig (YAML)
          <textarea value={kubeconfig} onChange={(e) => setKubeconfig(e.target.value)} placeholder="apiVersion: v1\nkind: Config\n..." disabled={submitting} />
        </label>
        <div className="modal-actions">
          <button type="button" onClick={() => setShowModal(false)} disabled={submitting}>取消</button>
          <button type="submit" className="primary" disabled={submitting || !name.trim() || !kubeconfig.trim()}>
            {submitting ? '提交中…' : '添加'}
          </button>
        </div>
      </form>
    </div>
  </div>
)}
```

- [ ] **Step 5: Verify build**

Run: `cd web && pnpm build`
Expected: Build succeeds with no errors

- [ ] **Step 6: Commit**

```bash
git add web/src/views/ClusterView.tsx
git commit -m "feat(cluster-ui): add modal form for new cluster creation"
```
