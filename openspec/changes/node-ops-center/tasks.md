## 1. SSH Engine (internal/ssh/)

- [ ] 1.1 Create `internal/ssh/client.go` — SSH client struct with connection, auth config, timeout settings
- [ ] 1.2 Create `internal/ssh/pool.go` — connection pool with per-node connection reuse, concurrency control (default 10), and queueing
- [ ] 1.3 Create `internal/ssh/exec.go` — command execution with stdout/stderr capture, timeout support, structured result type (exit code, stdout, stderr, duration)
- [ ] 1.4 Implement command mapping for each op type: sysctl (`sysctl -w k=v`), service_restart (`systemctl <action> <name>`), file_write (`cat > path << EOF`), shell (direct), reboot (`shutdown -r +delay`)

## 2. Data Model & Storage (internal/store/)

- [ ] 2.1 Create `internal/store/nodes.go` — `nodes` table schema, CRUD for Node (id, name, address, port, labels JSON, auth JSON encrypted, source, created_at, updated_at)
- [ ] 2.2 Create `internal/store/tasks.go` — `node_tasks` table schema, CRUD for NodeTask (id, name, op_type, target_labels JSON, target_nodes JSON, params JSON, exec_mode, created_at, created_by)
- [ ] 2.3 Create `internal/store/runs.go` — `node_runs` table schema (id, task_id, triggered_by, status, started_at, completed_at, results JSON), audit log with 100-entry auto-rotation
- [ ] 2.4 Implement `ListNodesByLabels` — filter nodes by label selectors with AND logic per filter, OR within values array
- [ ] 2.5 Implement K8s node sync — `SyncK8sNodes` function that fetches nodes via kubectl, imports hostname/IP/labels, marks source="k8s"

## 3. API Handlers (internal/server/)

- [ ] 3.1 Create `internal/server/handler_nodes.go` — REST handlers: `GET /api/nodes`, `POST /api/nodes`, `PUT /api/nodes/{id}`, `DELETE /api/nodes/{id}`, `POST /api/nodes/sync` (trigger K8s sync)
- [ ] 3.2 Create `internal/server/handler_tasks.go` — REST handlers: `GET /api/tasks`, `POST /api/tasks`, `PUT /api/tasks/{id}`, `DELETE /api/tasks/{id}`
- [ ] 3.3 Create `internal/server/handler_runs.go` — REST handlers: `POST /api/tasks/{id}/run` (execute task), `GET /api/runs` (list history), `GET /api/runs/{id}` (get result detail)
- [ ] 3.4 Wire handlers into router in `cmd/server/main.go`

## 4. Backend Encryption (internal/crypto/)

- [ ] 4.1 Reuse existing AES-256-GCM encryption from kubeconfig storage for SSH credential encryption
- [ ] 4.2 Ensure `EncryptCredential` / `DecryptCredential` helpers are available and used for SSH key and password storage

## 5. Frontend API (web/src/)

- [ ] 5.1 Add to `web/src/api.ts`: `getNodes`, `createNode`, `updateNode`, `deleteNode`, `syncNodes`, `getTasks`, `createTask`, `updateTask`, `deleteTask`, `runTask`, `getRuns`, `getRun`
- [ ] 5.2 Add TypeScript types for Node, NodeTask, NodeRun, NodeResult matching backend data model

## 6. Frontend UI — NodeOpsView (web/src/views/NodeOpsView.tsx)

- [ ] 6.1 Create `NodeOpsView.tsx` with tab navigation: Nodes / Tasks / Results / Settings
- [ ] 6.2 Implement Nodes tab: node list with source badge (k8s/manual), label filter sidebar, node cards (hostname, IP, online/offline indicator, tags), Add Node button opening form modal
- [ ] 6.3 Implement Add/Edit Node modal: form with address, port, SSH auth (key textarea or password), label editor (tag input), connectivity test button
- [ ] 6.4 Implement Tasks tab: task list with name, type badge, target summary; Create Task button opening form
- [ ] 6.5 Implement Task form: name, op_type selector (sysctl/file_write/service_restart/shell/reboot), dynamic params panel per type, target selector (label filter builder + manual node picker), exec_mode radio (parallel/sequential)
- [ ] 6.6 Implement Results tab: aggregated summary (success/fail counts, progress bar), expandable run details per node with structured summary and raw stdout/stderr
- [ ] 6.7 Implement danger op confirmation modal: shows node list, operation summary, impact analysis; requires user to type CONFIRM; only shown for reboot and disk operations

## 7. Integration & Testing

- [ ] 7.1 `make build` succeeds with all new files
- [ ] 7.2 Test: add a manual node with SSH auth, verify connectivity indicator updates
- [ ] 7.3 Test: create a sysctl task, execute against one node, verify result summary and raw output
- [ ] 7.4 Test: create a reboot task, verify confirmation modal appears and requires CONFIRM input
- [ ] 7.5 Test: execute task on multiple nodes in parallel, verify aggregate counts are correct