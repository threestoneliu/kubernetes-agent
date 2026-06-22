# plan-execution-ux Implementation Plan

**Goal:** 验证 plan-execution-ux 两个优化点：(1) Modal 确认后直接执行无需 chat 中转，(2) DiffCard 展示人类可读摘要

**Architecture:** 前端 PlanModal 组件 + backend `summarizeOne()` 生成摘要，SSE 事件驱动执行流程

**Tech Stack:** Go (backend), React/TypeScript (frontend), SQLite (persistence)

---

## Task 1: Backend Summary 生成验证

- [ ] **Step 1:** 验证 `plan_write.go` 中 `diff.Summary = summarizeOne(*diff)` 在 `dryRun` 后正确赋值
- [ ] **Step 2:** 验证 `summarizeOne()` 对 apply+无 before=CREATE 场景返回"创建 Kind ns/name"
- [ ] **Step 3:** 验证 `summarizeOne()` 对 apply+有 before=UPDATE 场景返回"更新 Kind ns/name: 变更字段"
- [ ] **Step 4:** 验证 `summarizeOne()` 对 delete 返回"删除 Kind ns/name"
- [ ] **Step 5:** 验证 `summarizeOne()` 对 scale 返回"调整 Kind ns/name replicas: X → Y"

**验证命令:**
```bash
go test ./internal/tools/k8s/... -run TestSummarize -v
```

---

## Task 2: 前端 DiffCard 展示验证

- [ ] **Step 1:** 验证 `PlanModal.tsx` 中 `DiffCard` 使用 `diff.summary ?? null` 而非 `summarizeChange(diff.before, diff.after)`
- [ ] **Step 2:** 验证 `state.ts` 的 `PendingPlan.diffs` 类型包含 `summary?: string` 字段
- [ ] **Step 3:** 验证 action 颜色映射 CREATE=绿/UPDATE=蓝/DELETE=红/SCALE=黄
- [ ] **Step 4:** 验证 YAML 使用 `<details><summary>` 折叠结构
- [ ] **Step 5:** 验证 `toYAML()` 跳过 creationTimestamp/managedFields 等字段

**验证命令:**
```bash
cd web && pnpm build
```

---

## Task 3: Modal 确认流程端到端验证

- [ ] **Step 1:** 验证 `prompt.go` step 3 为"Modal 确认后，直接调 k8s_execute_plan，不需要在 chat 里再次确认"
- [ ] **Step 2:** 验证 `Session.ResetPlan()` 在 `WaitPlan()` 前被调用（`agent/tools.go` plan_write handler）
- [ ] **Step 3:** 验证取消路径 `CancelPlan` 正确重置 plan channel

**验证命令:**
```bash
go test ./internal/agent/... -v
```

---

## Task 4: 端到端集成测试

- [ ] **Step 1:** 启动 dev server，`kubectl create deployment nginx --image=nginx` 场景验证 DiffCard 显示"创建 Deployment default/nginx"
- [ ] **Step 2:** `kubectl scale deployment nginx --replicas=3` 场景验证 DiffCard 显示"调整 Deployment default/nginx replicas: 1 → 3"
- [ ] **Step 3:** `kubectl delete deployment nginx` 场景验证 DiffCard 显示"删除 Deployment default/nginx"
- [ ] **Step 4:** 验证 Modal 点"确认执行"后直接执行，无 chat 输入"yes"提示

---

**Commit after each task group:**
```bash
git add -A && git commit -m "test: verify plan-execution-ux [task N]"
```
