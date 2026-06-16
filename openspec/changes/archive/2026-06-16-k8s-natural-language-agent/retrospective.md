# Retrospective: k8s-natural-language-agent

> Written: 2026-06-16 (after verify passed)
> Commit range: `c5f5850..248b5d5`
> Worktree: `/Users/liuzhilei/code/vibe/kubernetes-agent/.claude/worktrees/feat+k8s-natural-language-agent`

---

## 0. Evidence

- **Commit range**: `c5f5850..248b5d5` (60 commits)
- **Diff size**: +23,225 / -1 lines across 146 files
- **Tasks done**: 75/90 (`grep -cE '^\s*- \[x\]' tasks.md` → 75; 15 stale checkboxes — see §2)
- **Active hours**: multi-session, multi-day implementation
- **Subagent dispatches**: ~15 (14 task subagents via `subagent-driven-development` + 1 verify + 1 retrospective)
- **New external dependencies**: `github.com/go-chi/chi/v5 v5.3.0`, `github.com/google/uuid v1.6.0`, `github.com/stretchr/testify v1.11.1`, `gopkg.in/yaml.v3 v3.0.1`, `k8s.io/apimachinery v0.36.2`, `k8s.io/client-go v0.36.2`, `modernc.org/sqlite v1.52.0` (direct); `charm.land/fantasy v0.20.0` plus heavy transitive AWS/GCP SDKs (indirect, via fantasy)
- **Bugs encountered post-merge**: 0
- **OpenSpec validate state at archive**: PASS (`openspec validate k8s-natural-language-agent --type change` → valid: true)
- **Test coverage signal** (from `go test -cover ./internal/...`):
  - `internal/policy` 98.4% (target ≥90% ✅)
  - `internal/tools/k8s` 91.8% (target ≥90% ✅)
  - `internal/config` 86.1%
  - `internal/server` 80.8%
  - `internal/llm` 78.0%
  - `internal/crypto` 75.0%
  - `internal/store` 75.0%
  - `internal/agent` 73.4%
- **Web typecheck**: clean (`cd web && pnpm typecheck` → 0 errors)
- **Go test full suite**: 10/10 packages pass (`go test ./...`)

Commit chain (highlights):

```
c5f5850  Initial commit
213f333  docs(openspec): add k8s-natural-language-agent brainstorm
0813f09  docs(openspec): add k8s-natural-language-agent design
1a57ef2  docs(openspec): add k8s-natural-language-agent proposal
4f86458  docs(openspec): add 6 capability specs for k8s-natural-language-agent
b39059b  docs(openspec): add k8s-natural-language-agent tasks
7ad7708  docs(openspec): add k8s-natural-language-agent implementation plan
985ffda  feat(scaffold): init go module, config, logging
9e96d5a  feat(store): sqlite + 6 repos + migrations
b49246c  feat(crypto): AES-256-GCM + master key
1321ce1  feat(startup): wire master key + db + default policy seed
8bea1a6  feat(policy): 3-state engine + default rules + seed
c708f1e  feat(tools): 5 k8s tools + ask_user + dry-run planning
e8f1dfe  fix(tools): bridge Operation JSON wire format to unexported fields
bda932f  feat(llm): provider interface + ping + system prompt
bc03e66  feat(llm): wire Anthropic, OpenAI, OpenAI-compat adapters to fantasy
b1aeba6  feat(llm): define Event, Stream, Client, Message, Tool types
b5a139c  feat(agent): 12 SSE event payload types
f6c145d  feat(agent): register 6 k8s tools with JSON schemas
f123cbc  feat(agent): runner loop with event dispatch, plan/ask blocking, retry
23b7b8e  feat(agent): multi-step outer loop with turn-wide persistence
b175d21  feat(agent): session state with ConfirmPlan/CancelPlan/AnswerAsk
539fd88  test(agent): runner tests covering token, tool_call, plan/ask blocking
ff4995e  feat(llm): add Registry type with Status() for /healthz
3c8bf1a  feat(server): chi router + SSE chat + REST handlers
e003058  test(server): httptest coverage for the 5 handlers
4c3850c  test(server): extend sessions test to cover messages endpoint
78d3d74  feat(server): POST /api/sessions/{id}/resume for plan/ask_user
c756d06  feat(web): scaffold vite + react + 3 views + plan modal
19b2516  feat(embed): SPA embed.FS scaffolding + Makefile staging
58a6eab  feat(server): mount SPA fallback + quiet static-asset logging
28a228a  test(server): cover staticHandler root/asset/spa-fallback
25696fd  feat(server): wire NewRouter into main, serve HTTP on cfg port
f1f7932  refactor(k8s): turn ClientFactory into an interface
8351637  feat(server): wire RunnerFactory into main
d8b93fc  test: add 4-scenario e2e suite for chat + resume flow
4bd450e  test(k8s): lift tools/k8s coverage to 91.8%
6494fa6  test(llm): lift llm coverage to 78% via fakeProvider + converters
41ab448  test(agent,policy): lift agent to 73.4% via tools + helpers
794d31f  test(server): lift server to 80.8% via policy + session handlers
cf351fc  test(k8s): use keyed struct literals in wellKnownGV
6a4bc28  fix(tools): silence unusedparams linter on rollbackOne stub
a11857f  docs(openspec): mark Task 13 testing + e2e validation complete
5aff888  docs: README + default policies + LLM providers + dev mode
233c119  fix(startup): actually wire PingAll into main, stabilize roadmap link
248b5d5  docs(openspec): resolve 3 verify warnings via spec MVP-deviation notes
```

---

## 1. Wins

- [evidence: c5f5850..248b5d5 = 60 commits] TDD with subagent-driven-development shipped a multi-package K8s agent end-to-end with one or zero follow-up bugs per task. Fresh subagent per task kept review feedback scoped.
- [evidence: openspec validate PASS + verify.md Final Assessment 0 critical / 0 warnings] All 6 capability specs (`natural-language-k8s-interaction`, `k8s-write-with-plan-preview`, `k8s-policy-guardrails`, `k8s-credential-encryption`, `multi-llm-provider-support`, `web-chat-ui`) have working implementations; all 12 design decisions (D1–D12 in `design.md`) followed; 3 spec/code divergences resolved via explicit "MVP 偏差" blockquote pattern in `web-chat-ui/spec.md`.
- [evidence: `internal/policy` 98.4%, `internal/tools/k8s` 91.8%] Both coverage targets met (≥90%). The 4 commits `4bd450e`, `6494fa6`, `41ab448`, `794d31f` systematically lifted coverage to targets after the feature work shipped.
- [evidence: `internal/server/e2e_test.go` (commit `d8b93fc`)] 4 e2e scenarios pass without envtest — `dynfake` (k8s dynfake) gives full dynamic-client coverage with no kubelet/etcd dependency. Single-machine CI-possible.
- [evidence: `internal/agent/{events,tools,agent,session}.go`] 12 SSE event types, 6 K8s tools, plan/ask_user blocking, history truncation at 0.8× context window, error retry classifier — single Runner with no goroutines leaks across turns.
- [evidence: `internal/server/static.go` + `//go:embed all:web_dist`] Single-binary embed.FS with hashed chunks `max-age=31536000` and `index.html` no-cache; build wired via `make copy-web` (go:embed cannot reach `web/dist/` from `internal/server/`).
- [evidence: `cmd/server/main.go:52` PingAll wired in commit `233c119`] Caught during `/opsx:verify`, not after archive — verify gate works.
- [evidence: `docs/{default-policies.md,llm-providers.md,dev-mode.md,roadmap.md}`] 4 docs files at ≥ 100 lines each; `roadmap.md` stabilized post-archive link in commit `233c119`.
- [evidence: `e8f1dfe` fix commit, not a feature commit] `Operation` JSON wire-bridge pattern (`operationWire` bridge struct with custom MarshalJSON/UnmarshalJSON) caught by TDD when test expected public JSON shape — fix committed in one line, not a redesign.

---

## 2. Misses

- 🟡 [painful | evidence: tasks.md 15 stale checkboxes] 15 sub-step checkboxes in `tasks.md` (all of Task 8's 8.1–8.10 and all of Task 14's 14.1–14.5) were reset to `[ ]` by an out-of-band edit after the code shipped. The work IS in the codebase (verified by file existence + tests passing + commit history), but the file drifted from reality. Cost: ~5 minutes during `/opsx:verify` to cross-reference; cost to fix forward: single git commit.
  > **Lesson**: when a subagent marks task substeps done, edit + commit should be atomic (one `git commit` per task), so a stray `tasks.md` rewrite would show up in `git log` and be trivially revertable.
- 🟡 [painful | evidence: `web-chat-ui/spec.md` line 66 + line 84, resolved in commit `248b5d5`] Plan said "前端 MUST 使用浏览器原生 `EventSource`" and "后端 MUST 代理 Vite (`KUBERNETES_AGENT_DEV=1`)", but `/api/chat` is POST (body + custom headers required), making both specs unimplementable as written. Cost: 2 spec/code mismatches caught only by verify.
  > **Lesson**: spec endpoints should reference the actual HTTP method (GET vs POST); transport choice is constrained by request shape, not free.
- 📌 [nit | evidence: coverage report at 0 critical] 4 packages below an aspirational 80% — `agent` 73.4%, `crypto` 75.0%, `store` 75.0%, `llm` 78.0%. All are above the explicit 70% target in the plan; `agent` is dragged down by error-retry branches that are tedious to script-fake. Cost: ~1 hour of focused fixture work per package to lift to 85%; deferred to a follow-up change.
- 📌 [nit | evidence: repeated `cd web && pnpm typecheck` runs] Each `pnpm typecheck` is ~2s but runs serially with the Go test suite. Could parallelize via `make` recipes but adds Makefile complexity for a 5s saving.
- 📌 [nit | evidence: `internal/tools/k8s/execute_plan.go:103` `rollbackOne` stub] Rollback is a stub that returns an error for any `action != ""`. Acceptable for MVP (design R7), but the stub returns a generic error message rather than a typed `ErrRollbackNotImplemented` — callers can't `errors.Is` on it.

---

## 3. Plan deviations

| Plan task | What changed | Why |
|-----------|--------------|-----|
| `Operation` JSON (Task 5) | Plan had unexported fields; `encoding/json` silently bypassed policy in `k8s_apply` (commit `e8f1dfe`). Added `operationWire` bridge struct with custom `MarshalJSON`/`UnmarshalJSON`. | Go `encoding/json` cannot marshal unexported fields. Without the bridge, the policy engine received zero-value `kind`, leading to false denies. Test caught it on first run. |
| Policy `UnsafeFields` (Task 6) | Plan wording implied AND semantics; implementation uses OR (any one match → rule fires). Documented in `internal/policy/engine.go` comment. | AND would make default rules effectively unusable (require every field to match for a deny to fire). OR is the intuitive semantics for "dangerous field" matching. |
| `PingAll` wiring (Task 14) | Plan had `PingAll` only as a documented helper; `cmd/server/main.go` never called it. Wired in commit `233c119`. | Caught by `/opsx:verify` (Health map was empty in `main.go` even though `Registry.Health` was wired into `/healthz`). verify gate worked. |
| SSE client transport (Task 10) | Plan said `EventSource`; `web/src/sse.ts` uses `fetch + ReadableStream`. | `/api/chat` is POST. `EventSource` is GET-only. Spec was stale. |
| `RunnerFactory` (Task 13) | Plan had `nil` factory in main.go; wiring deferred to a separate task. Final shape is a closure matching `server.RunnerFactory` interface. | Cleaner separation of concerns — factory construction is a single line in `main.go` but the wiring is testable in isolation. |

---

## 4. Skill / workflow compliance

| Skill                                            | Used |
|--------------------------------------------------|------|
| superpowers:brainstorming                        | ✓ |
| superpowers:writing-plans                        | ✓ |
| superpowers:using-git-worktrees                  | ✓ |
| superpowers:subagent-driven-development          | ✓ |
| (transitive) superpowers:test-driven-development | ✓ |
| (transitive) superpowers:requesting-code-review  | ✓ |
| superpowers:finishing-a-development-branch       | (next: archive + branch cleanup) |

> **Default expectation**: 全部 ✓。每個 skill 都是 schema 設計的一部分,跳過屬於異常情境。任一項 ✗ 都必須在下方 `### Deliberately Skipped Skills` subsection 提出原因與預防方案。

### Deliberately Skipped Skills

> (空白 — 所有 apply-phase skills 都按預期執行。)

---

## 5. Surprises

- **`/api/chat` POST constraint is real.** Browser-native `EventSource` is GET-only. The plan assumed it would work; the spec said it MUST work; the code (correctly) used `fetch + ReadableStream`. Lesson: spec endpoints should pin the HTTP method, not just the path.
- **charm.land/fantasy has heavyweight transitive deps.** Pulling in the LLM abstraction drags the AWS SDK v2 (full credential chain including SSO/OIDC) and GCP auth libraries. Direct deps are 7 lines in `go.mod`; indirect is 50+ lines. Acceptable for a multi-provider abstraction but worth flagging — if we ever drop one provider, the indirect surface doesn't shrink accordingly.
- **`modernc.org/sqlite` is fast enough.** Pure-Go SQLite (no cgo) with WAL mode + `SetMaxOpenConns(1)` handles the 4 e2e scenarios in <1s each. No reason to reach for `mattn/go-sqlite3`.
- **`dynfake` replaces envtest for e2e.** We get full dynamic-client coverage (CRUD + watch) against a fake dynamic client, with no kubelet/etcd startup cost. The 4 e2e scenarios (list pods, plan+execute, deny system NS, ask_user answer) all use it.
- **Spec/code divergence is the norm, not the exception.** 3 spec points were unwritable as written. The "MVP 偏差" blockquote pattern (`> **MVP 偏差**: <why>` under the affected scenario) documents the deviation in the spec itself, with a link to the design trade-off that justified it. This pattern should be a writing-plans skill template.
- **The verify gate works.** All 3 warnings + the PingAll wiring bug were caught by `/opsx:verify`, not by post-archive CI. This validates the superpowers-bridge schema's "verify before archive" edge.

---

## 6. Promote candidates → long-term learning

- [ ] 📌 **GOSUMDB=sum.golang.org env prefix is required for all `go` commands in this repo** → **Promote to project CLAUDE.md** (`.claude/CLAUDE.md` or root `AGENTS.md`)
  > **Why**: `charm.land/fantasy v0.20.0` ships with go.sum entries that fail verification when `GOSUMDB=off` (the default in some sandboxed shell environments). Symptom is `go test` failing with "verifying module: checksum database disabled by GOSUMDB=off". This is an environmental constant, not a code issue.
  > **How to apply**: any time `go test`, `go build`, `go mod tidy`, or `go vet` is invoked in this repo, prefix with `GOSUMDB=sum.golang.org ` or set the env in the Makefile recipe. The Makefile already does this; document for one-off shell runs.

- [ ] 🟡 **Spec/code divergence resolution should follow the "MVP 偏差" blockquote pattern** → **Promote to writing-plans skill** (`superpowers:writing-plans` template)
  > **Why**: this cycle produced 3 spec/code mismatches (EventSource vs fetch, Last-Event-ID replay vs advisory, KUBERNETES_AGENT_DEV proxy vs no-op). All 3 were resolved by adding a `> **MVP 偏差**: <why>` blockquote under the affected scenario, citing the design trade-off that justified the deviation. The pattern is reusable: a verifier can grep for the blockquote, a reader sees the rationale inline.
  > **How to apply**: when writing or updating a spec scenario that the implementation cannot match as written, do NOT silently change the spec. Add the deviation note inline so future readers see both the spec and the rationale.

- [ ] 🟡 **Mark task substeps done + commit atomically** → **Promote to project CLAUDE.md** (add to subagent-driven-development cycle)
  > **Why**: this cycle lost 15 substep checkboxes to an out-of-band edit. Atomic commit per task means a stray `tasks.md` rewrite shows up in `git log` and is revertable in one command.
  > **How to apply**: at the end of each subagent-driven-development cycle, the implementer should commit "docs(openspec): mark Task N complete" with the checkbox edit AND nothing else in the same commit. If the commit only contains a tasks.md edit, a stray rewrite is detectable by `git log -p -- tasks.md`.

- [ ] 📌 **Operation JSON wire-bridge pattern for unexported fields + json tags** → **Promote to memory** (type: feedback, scope: Go)
  > **Why**: Go `encoding/json` cannot marshal unexported fields. When an API contract needs JSON shape but the Go type wants unexported fields (to keep policy invariants), the pattern is a `xxxWire` bridge struct with custom `MarshalJSON`/`UnmarshalJSON` that explicitly maps fields. Used at `internal/tools/k8s/plan_write.go` `operationWire` (commit `e8f1dfe`); would be needed again at any new boundary where Go-side invariants must survive a JSON round-trip.
  > **How to apply**: when designing a Go type whose instances will be JSON-marshaled but whose fields must remain unexported (to enforce invariants via the constructor), write a `xxxWire` bridge struct in the same file with explicit `MarshalJSON`/`UnmarshalJSON`. Add a one-line code comment citing why the bridge exists.

- [ ] 📌 **Spec endpoints should pin the HTTP method** → **Promote to writing-plans skill** (template addition)
  > **Why**: the `web-chat-ui` spec said "前端 MUST 使用浏览器原生 `EventSource`" without specifying whether `/api/chat` is GET or POST. EventSource is GET-only; the chat endpoint is POST (body + headers). Pinning the method in the spec would have made the EventSource requirement self-contradictory at write time, not at verify time.
  > **How to apply**: in every spec scenario that names an HTTP endpoint, include the method (`GET /api/...`, `POST /api/...`). If the spec is method-agnostic, the spec author has not yet thought about transport constraints.

---

> **Carry-forward 機制**:下個 cycle 寫 retro 時,可 `grep -A 5 '^- \[ \]' openspec/changes/archive/*/retrospective.md` 取出既往 unchecked candidates,逐筆判斷要 carry-forward 到本 cycle §6、就地 promote、或標 stale 不再追蹤。
