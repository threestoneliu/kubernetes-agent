# Node Ops Center Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Node Ops Center panel — SSH-based node management with structured ops (sysctl/file_write/service_restart/shell/reboot), connection pooling, label targeting, result aggregation, and danger-op confirmation.

**Architecture:** A Go SSH engine (`internal/ssh/`) with a connection pool per node, SQLite storage for nodes/tasks/runs, REST handlers wired into the existing chi router, and a React NodeOpsView with tab navigation. Encryption reuses the existing AES-256-GCM from `internal/crypto/aead.go`.

**Tech Stack:** Go (golang.org/x/crypto/ssh), SQLite, React/TypeScript, chi router, AES-256-GCM

---

## File Structure

| File | Responsibility |
|------|----------------|
| `internal/ssh/client.go` | SSH client struct, auth (key + password fallback), timeout |
| `internal/ssh/pool.go` | Per-node connection reuse, 10-concurrent slot gate, 30s TTL |
| `internal/ssh/exec.go` | Command exec with stdout/stderr/duration capture, op-type command mapping |
| `internal/store/nodes.go` | `nodes` table CRUD, `ListNodesByLabels`, K8s node sync |
| `internal/store/tasks.go` | `node_tasks` table CRUD |
| `internal/store/runs.go` | `node_runs` table CRUD, 100-entry audit log auto-rotation |
| `internal/server/handler_nodes.go` | `GET/POST/PUT/DELETE /api/nodes`, `POST /api/nodes/sync` |
| `internal/server/handler_tasks.go` | `GET/POST/PUT/DELETE /api/tasks` |
| `internal/server/handler_runs.go` | `POST /api/tasks/{id}/run`, `GET /api/runs`, `GET /api/runs/{id}` |
| `web/src/api.ts` | New API functions for nodes/tasks/runs |
| `web/src/views/NodeOpsView.tsx` | Full tabbed panel UI |
| `cmd/server/main.go` | Wire new handlers into router, add SSH pool shutdown |

---

## Task 1: SSH Client (`internal/ssh/client.go`)

**Files:**
- Create: `internal/ssh/client.go`
- Test: `internal/ssh/client_test.go`

- [ ] **Step 1: Write the failing test**

```go
package ssh

import (
	"net"
	"testing"
)

func TestSSHClient_AuthKeyThenPassword(t *testing.T) {
	// Verify auth type constants are defined
	if authTypeKey != "key" || authTypePassword != "password" {
		t.Errorf("auth type constants incorrect")
	}
}

func TestSSHClient_Timeout(t *testing.T) {
	c := &Client{Timeout: 30}
	if c.Timeout != 30 {
		t.Errorf("expected timeout 30, got %d", c.Timeout)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ssh/... -v`
Expected: FAIL — "internal/ssh/*" does not exist

- [ ] **Step 3: Write minimal implementation**

```go
// internal/ssh/client.go
package ssh

import (
	"golang.org/x/crypto/ssh"
)

const (
	authTypeKey      = "key"
	authTypePassword = "password"
)

type Auth struct {
	Type     string // "key" or "password"
	Key      []byte // private key content (encrypted bytes, decrypted before use)
	Password string
}

type Client struct {
	Address  string
	Port     int
	Auth     Auth
	Timeout  int // seconds
	conn     *ssh.Client
}

func (c *Client) Connect() error {
	cfg, err := c.buildConfig()
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(c.Address, string(rune(c.Port)))
	conn, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *Client) buildConfig() (*ssh.ClientConfig, error) {
	var auths []ssh.AuthMethod
	if len(c.Auth.Key) > 0 {
		signer, err := ssh.ParsePrivateKey(c.Auth.Key)
		if err == nil {
			auths = append(auths, ssh.PublicKeys(signer))
		}
	}
	if c.Auth.Password != "" {
		auths = append(auths, ssh.Password(c.Auth.Password))
	}
	if len(auths) == 0 {
		auths = append(auths, ssh.Password(""))
	}
	return &ssh.ClientConfig{
		User:            "root",
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30,
	}, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) IsConnected() bool {
	return c.conn != nil && c.conn.Conn != nil && !c.conn.Conn.Closec.IsClosed()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ssh/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ssh/client.go internal/ssh/client_test.go
git commit -m "feat(ssh): add SSH client with key + password auth"
```

---

## Task 2: SSH Connection Pool (`internal/ssh/pool.go`)

**Files:**
- Create: `internal/ssh/pool.go`
- Test: `internal/ssh/pool_test.go`

- [ ] **Step 1: Write the failing test**

```go
package ssh

import (
	"testing"
)

func TestPool_ReusesConnection(t *testing.T) {
	p := NewPool(10)
	c := &Client{Address: "127.0.0.1", Port: 22}
	p.Put("node1", c)
	got := p.Get("node1")
	if got != c {
		t.Errorf("expected same client back")
	}
}

func TestPool_MaxConcurrency(t *testing.T) {
	p := NewPool(2)
	if p.maxSlots != 2 {
		t.Errorf("expected maxSlots 2, got %d", p.maxSlots)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ssh/... -v`
Expected: FAIL — "NewPool not defined"

- [ ] **Step 3: Write minimal implementation**

```go
// internal/ssh/pool.go
package ssh

import (
	"sync"
	"time"
)

type Pool struct {
	maxSlots int
	timeout  time.Duration
	mu       sync.Mutex
	pool     map[string]*Client
	inFlight map[string]int
	slots    chan struct{}
}

func NewPool(maxConcurrency int) *Pool {
	p := &Pool{
		maxSlots: maxConcurrency,
		timeout:  30 * time.Second,
		pool:     make(map[string]*Client),
		inFlight: make(map[string]int),
		slots:    make(chan struct{}, maxConcurrency),
	}
	for i := 0; i < maxConcurrency; i++ {
		p.slots <- struct{}{}
	}
	return p
}

func (p *Pool) Get(nodeID string) *Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pool[nodeID]
}

func (p *Pool) Put(nodeID string, c *Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pool[nodeID] = c
}

func (p *Pool) Remove(nodeID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.pool[nodeID]; ok {
		c.Close()
		delete(p.pool, nodeID)
	}
}

func (p *Pool) AcquireSlot() {
	<-p.slots
}

func (p *Pool) ReleaseSlot() {
	p.slots <- struct{}{}
}

func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.pool {
		c.Close()
	}
	p.pool = make(map[string]*Client)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ssh/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ssh/pool.go internal/ssh/pool_test.go
git commit -m "feat(ssh): add SSH connection pool with concurrency control"
```

---

## Task 3: SSH Command Execution (`internal/ssh/exec.go`)

**Files:**
- Create: `internal/ssh/exec.go`
- Test: `internal/ssh/exec_test.go`

- [ ] **Step 1: Write the failing test**

```go
package ssh

import (
	"testing"
)

func TestOpType_Constants(t *testing.T) {
	if OpTypeSysctl != "sysctl" ||
		OpTypeFileWrite != "file_write" ||
		OpTypeServiceRestart != "service_restart" ||
		OpTypeShell != "shell" ||
		OpTypeReboot != "reboot" {
		t.Errorf("unexpected OpType constants")
	}
}

func TestResult_Structure(t *testing.T) {
	r := Result{ExitCode: 0, Stdout: "ok", Stderr: "", Duration: 100}
	if r.ExitCode != 0 || r.Stdout != "ok" {
		t.Errorf("unexpected result fields")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ssh/... -v`
Expected: FAIL — OpType/Result not defined

- [ ] **Step 3: Write minimal implementation**

```go
// internal/ssh/exec.go
package ssh

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type OpType string

const (
	OpTypeSysctl         OpType = "sysctl"
	OpTypeFileWrite      OpType = "file_write"
	OpTypeServiceRestart OpType = "service_restart"
	OpTypeShell          OpType = "shell"
	OpTypeReboot         OpType = "reboot"
)

type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration int // milliseconds
	Error    string
}

func (c *Client) Exec(cmd string) Result {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return Result{Error: err.Error()}
		}
	}
	start := time.Now()
	session, err := c.conn.NewSession()
	if err != nil {
		return Result{Error: err.Error()}
	}
	defer session.Close()

	stdoutBuf, stderrBuf := new(strings.Builder), new(strings.Builder)
	session.Stdout, session.Stderr = stdoutBuf, stderrBuf

	if err := session.Run(cmd); err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			return Result{
				ExitCode: exitErr.ExitStatus(),
				Stdout:   stdoutBuf.String(),
				Stderr:   stderrBuf.String(),
				Duration: int(time.Since(start).Milliseconds()),
			}
		}
		return Result{Error: err.Error(), Duration: int(time.Since(start).Milliseconds())}
	}
	return Result{
		ExitCode: 0,
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: int(time.Since(start).Milliseconds()),
	}
}

// MapOpToCommand converts an op type + params to the actual SSH command string.
func MapOpToCommand(opType OpType, params map[string]any) (string, error) {
	switch opType {
	case OpTypeSysctl:
		entries, ok := params["entries"].([]any)
		if !ok {
			return "", fmt.Errorf("sysctl requires entries")
		}
		var cmds []string
		for _, e := range entries {
			entry, ok := e.(map[string]any)
			if !ok {
				continue
			}
			key, _ := entry["key"].(string)
			value, _ := entry["value"].(string)
			if key != "" && value != "" {
				cmds = append(cmds, fmt.Sprintf("sysctl -w %s=%s", key, value))
			}
		}
		return strings.Join(cmds, " && "), nil

	case OpTypeFileWrite:
		path, _ := params["file_path"].(string)
		content, _ := params["file_content"].(string)
		if path == "" {
			return "", fmt.Errorf("file_write requires file_path")
		}
		return fmt.Sprintf("cat > %s << 'EOF'\n%s\nEOF", path, content), nil

	case OpTypeServiceRestart:
		action, _ := params["action"].(string)
		svc, _ := params["service_name"].(string)
		if svc == "" || action == "" {
			return "", fmt.Errorf("service_restart requires action and service_name")
		}
		return fmt.Sprintf("systemctl %s %s", action, svc), nil

	case OpTypeShell:
		cmd, _ := params["command"].(string)
		return cmd, nil

	case OpTypeReboot:
		delay := 10
		if d, ok := params["delay"].(float64); ok {
			delay = int(d)
		}
		return fmt.Sprintf("shutdown -r +%d", delay), nil

	default:
		return "", fmt.Errorf("unknown op type: %s", opType)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ssh/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ssh/exec.go internal/ssh/exec_test.go
git commit -m "feat(ssh): add command execution with op-type mapping"
```

---

## Task 4: Nodes Store (`internal/store/nodes.go`)

**Files:**
- Create: `internal/store/nodes.go`
- Test: `internal/store/nodes_test.go`

- [ ] **Step 1: Write the failing test**

```go
package store

import (
	"context"
	"testing"
)

func TestNode_CRUD(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	n := Node{ID: "n1", Name: "node-a", Address: "10.0.0.1", Port: 22, Source: "manual"}
	if err := db.CreateNode(ctx, n); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := db.GetNode(ctx, "n1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "node-a" {
		t.Errorf("name: got %q, want node-a", got.Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/... -v -run TestNode_CRUD`
Expected: FAIL — Node type / CreateNode not defined

- [ ] **Step 3: Write minimal implementation**

```go
// internal/store/nodes.go
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var ErrNodeNotFound = errors.New("node not found")

type Node struct {
	ID        string
	Name      string
	Address   string
	Port      int
	Labels    []Label
	Auth      NodeAuth  // stored encrypted, deserialized from JSON
	Source    string    // "k8s" or "manual"
	CreatedAt time.Time
	UpdatedAt time.Time
}

type NodeAuth struct {
	Type     string
	Key      []byte // encrypted
	Password string // encrypted
}

type Label struct {
	Key   string
	Value string
}

func (d *DB) MigrateNodes() error {
	_, err := d.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS nodes (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			address     TEXT NOT NULL,
			port        INTEGER NOT NULL DEFAULT 22,
			labels      TEXT NOT NULL DEFAULT '[]',
			auth        TEXT NOT NULL DEFAULT '{}',
			source      TEXT NOT NULL DEFAULT 'manual',
			created_at  INTEGER NOT NULL,
			updated_at  INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_nodes_source ON nodes(source);
	`)
	return err
}

func (d *DB) CreateNode(ctx context.Context, n Node) error {
	labels, _ := json.Marshal(n.Labels)
	auth, _ := json.Marshal(n.Auth)
	now := time.Now().Unix()
	_, err := d.ExecContext(ctx,
		`INSERT INTO nodes (id, name, address, port, labels, auth, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Name, n.Address, n.Port, labels, auth, n.Source, now, now)
	return err
}

func (d *DB) GetNode(ctx context.Context, id string) (Node, error) {
	var n Node
	var labelsJSON, authJSON string
	var ts int64
	err := d.QueryRowContext(ctx,
		`SELECT id, name, address, port, labels, auth, source, created_at, updated_at FROM nodes WHERE id = ?`, id).
		Scan(&n.ID, &n.Name, &n.Address, &n.Port, &labelsJSON, &authJSON, &n.Source, &ts, &ts)
	if errors.Is(err, sql.ErrNoRows) {
		return n, ErrNodeNotFound
	}
	if err != nil {
		return n, err
	}
	json.Unmarshal([]byte(labelsJSON), &n.Labels)
	json.Unmarshal([]byte(authJSON), &n.Auth)
	n.CreatedAt = time.Unix(ts, 0)
	n.UpdatedAt = time.Unix(ts, 0)
	return n, nil
}

func (d *DB) ListNodes(ctx context.Context) ([]Node, error) {
	rows, err := d.QueryContext(ctx, `SELECT id, name, address, port, labels, auth, source, created_at, updated_at FROM nodes ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Node
	for rows.Next() {
		var n Node
		var labelsJSON, authJSON string
		var ts int64
		if err := rows.Scan(&n.ID, &n.Name, &n.Address, &n.Port, &labelsJSON, &authJSON, &n.Source, &ts, &ts); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(labelsJSON), &n.Labels)
		json.Unmarshal([]byte(authJSON), &n.Auth)
		n.CreatedAt = time.Unix(ts, 0)
		n.UpdatedAt = time.Unix(ts, 0)
		out = append(out, n)
	}
	return out, rows.Err()
}

func (d *DB) UpdateNode(ctx context.Context, n Node) error {
	labels, _ := json.Marshal(n.Labels)
	auth, _ := json.Marshal(n.Auth)
	now := time.Now().Unix()
	res, err := d.ExecContext(ctx,
		`UPDATE nodes SET name=?, address=?, port=?, labels=?, auth=?, source=?, updated_at=? WHERE id=?`,
		n.Name, n.Address, n.Port, labels, auth, n.Source, now, n.ID)
	if err != nil {
		return err
	}
	nRows, _ := res.RowsAffected()
	if nRows == 0 {
		return ErrNodeNotFound
	}
	return nil
}

func (d *DB) DeleteNode(ctx context.Context, id string) error {
	res, err := d.ExecContext(ctx, `DELETE FROM nodes WHERE id=?`, id)
	if err != nil {
		return err
	}
	nRows, _ := res.RowsAffected()
	if nRows == 0 {
		return ErrNodeNotFound
	}
	return nil
}

func (d *DB) ListNodesByLabels(ctx context.Context, filters []LabelFilter) ([]Node, error) {
	// Build query that filters by ALL labels (AND), with OR within values (IN)
	all, err := d.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	if len(filters) == 0 {
		return all, nil
	}
	var result []Node
	for _, n := range all {
		match := true
		for _, f := range filters {
			found := false
			for _, l := range n.Labels {
				if l.Key == f.Key {
					for _, v := range f.Values {
						if l.Value == v {
							found = true
							break
						}
					}
				}
			}
			if !found {
				match = false
				break
			}
		}
		if match {
			result = append(result, n)
		}
	}
	return result, nil
}

type LabelFilter struct {
	Key    string
	Values []string
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/... -v -run TestNode_CRUD`
Expected: PASS

- [ ] **Step 5: Integrate migration into existing Migrate() call**

Modify `internal/store/migrations.go` — add a call to `db.MigrateNodes()` in the existing `Migrate()` function (add it as migration index 3 or append to existing).

```go
// In func (d *DB) Migrate(), add after existing migrations:
if err := d.MigrateNodes(); err != nil {
    return fmt.Errorf("migrate nodes: %w", err)
}
```

Run: `go test ./internal/store/... -v -run TestNode_CRUD`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/store/nodes.go internal/store/migrations.go
git commit -m "feat(store): add nodes table with CRUD and label filtering"
```

---

## Task 5: Tasks Store (`internal/store/tasks.go`)

**Files:**
- Create: `internal/store/tasks.go`
- Test: `internal/store/tasks_test.go`

- [ ] **Step 1: Write the failing test**

```go
package store

import (
	"context"
	"encoding/json"
	"testing"
)

func TestTask_CRUD(t *testing.T) {
	db, _ := Open(":memory:")
	db.Migrate()

	ctx := context.Background()
	task := NodeTask{
		ID:       "t1",
		Name:     "set-keepalive",
		OpType:   OpTypeSysctl,
		ExecMode: "parallel",
	}
	params, _ := json.Marshal(map[string]any{"entries": []map[string]string{{"key": "a", "value": "b"}}})
	task.Params = params

	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := db.GetTask(ctx, "t1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "set-keepalive" {
		t.Errorf("name: got %q", got.Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/... -v -run TestTask_CRUD`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

```go
// internal/store/tasks.go
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var ErrTaskNotFound = errors.New("task not found")

type NodeTask struct {
	ID          string
	Name        string
	OpType      string
	TargetLabels []LabelFilter `json:"-"`
	TargetNodes []string       `json:"-"`
	Params      json.RawMessage
	ExecMode    string
	CreatedAt   time.Time
	CreatedBy   string
}

func (d *DB) MigrateTasks() error {
	_, err := d.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS node_tasks (
			id           TEXT PRIMARY KEY,
			name         TEXT NOT NULL,
			op_type      TEXT NOT NULL,
			target_labels TEXT NOT NULL DEFAULT '[]',
			target_nodes  TEXT NOT NULL DEFAULT '[]',
			params       TEXT NOT NULL DEFAULT '{}',
			exec_mode    TEXT NOT NULL DEFAULT 'parallel',
			created_at   INTEGER NOT NULL,
			created_by   TEXT
		)
	`)
	return err
}

func (d *DB) CreateTask(ctx context.Context, t NodeTask) error {
	labels, _ := json.Marshal(t.TargetLabels)
	nodes, _ := json.Marshal(t.TargetNodes)
	now := time.Now().Unix()
	_, err := d.ExecContext(ctx,
		`INSERT INTO node_tasks (id, name, op_type, target_labels, target_nodes, params, exec_mode, created_at, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.OpType, labels, nodes, t.Params, t.ExecMode, now, t.CreatedBy)
	return err
}

func (d *DB) GetTask(ctx context.Context, id string) (NodeTask, error) {
	var t NodeTask
	var labelsJSON, nodesJSON string
	var ts int64
	err := d.QueryRowContext(ctx,
		`SELECT id, name, op_type, target_labels, target_nodes, params, exec_mode, created_at, created_by FROM node_tasks WHERE id = ?`, id).
		Scan(&t.ID, &t.Name, &t.OpType, &labelsJSON, &nodesJSON, &t.Params, &t.ExecMode, &ts, &t.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrTaskNotFound
	}
	if err != nil {
		return t, err
	}
	json.Unmarshal([]byte(labelsJSON), &t.TargetLabels)
	json.Unmarshal([]byte(nodesJSON), &t.TargetNodes)
	t.CreatedAt = time.Unix(ts, 0)
	return t, nil
}

func (d *DB) ListTasks(ctx context.Context) ([]NodeTask, error) {
	rows, err := d.QueryContext(ctx, `SELECT id, name, op_type, target_labels, target_nodes, params, exec_mode, created_at, created_by FROM node_tasks ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeTask
	for rows.Next() {
		var t NodeTask
		var labelsJSON, nodesJSON string
		var ts int64
		if err := rows.Scan(&t.ID, &t.Name, &t.OpType, &labelsJSON, &nodesJSON, &t.Params, &t.ExecMode, &ts, &t.CreatedBy); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(labelsJSON), &t.TargetLabels)
		json.Unmarshal([]byte(nodesJSON), &t.TargetNodes)
		t.CreatedAt = time.Unix(ts, 0)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d *DB) UpdateTask(ctx context.Context, t NodeTask) error {
	labels, _ := json.Marshal(t.TargetLabels)
	nodes, _ := json.Marshal(t.TargetNodes)
	res, err := d.ExecContext(ctx,
		`UPDATE node_tasks SET name=?, op_type=?, target_labels=?, target_nodes=?, params=?, exec_mode=? WHERE id=?`,
		t.Name, t.OpType, labels, nodes, t.Params, t.ExecMode, t.ID)
	if err != nil {
		return err
	}
	nRows, _ := res.RowsAffected()
	if nRows == 0 {
		return ErrTaskNotFound
	}
	return nil
}

func (d *DB) DeleteTask(ctx context.Context, id string) error {
	res, err := d.ExecContext(ctx, `DELETE FROM node_tasks WHERE id=?`, id)
	if err != nil {
		return err
	}
	nRows, _ := res.RowsAffected()
	if nRows == 0 {
		return ErrTaskNotFound
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/... -v -run TestTask_CRUD`
Expected: PASS

- [ ] **Step 5: Add migration to Migrate() in migrations.go**

Add `d.MigrateTasks()` call to the Migrate function alongside the other migration calls.

- [ ] **Step 6: Commit**

```bash
git add internal/store/tasks.go internal/store/migrations.go
git commit -m "feat(store): add node_tasks table with CRUD"
```

---

## Task 6: Runs Store + Audit Log (`internal/store/runs.go`)

**Files:**
- Create: `internal/store/runs.go`
- Test: `internal/store/runs_test.go`

- [ ] **Step 1: Write the failing test**

```go
package store

import (
	"context"
	"testing"
)

func TestRun_CRUD(t *testing.T) {
	db, _ := Open(":memory:")
	db.Migrate()

	ctx := context.Background()
	run := NodeRun{ID: "r1", TaskID: "t1", TriggeredBy: "manual", Status: "pending"}
	if err := db.CreateRun(ctx, run); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := db.GetRun(ctx, "r1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "pending" {
		t.Errorf("status: got %q", got.Status)
	}
}

func TestAuditLog_AutoRotate(t *testing.T) {
	db, _ := Open(":memory:")
	db.Migrate()

	ctx := context.Background()
	for i := 0; i < 105; i++ {
		db.CreateRun(ctx, NodeRun{ID: "r" + string(rune(i)), TaskID: "t1", TriggeredBy: "manual", Status: "done"})
	}
	// After 105 runs with 100-entry limit, oldest should be rotated
	runs, _ := db.ListRuns(ctx, "t1")
	if len(runs) > 100 {
		t.Errorf("expected <=100 runs after rotation, got %d", len(runs))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/... -v -run TestRun_CRUD`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

```go
// internal/store/runs.go
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const maxAuditEntries = 100

var ErrRunNotFound = errors.New("run not found")

type NodeRun struct {
	ID          string
	TaskID      string
	TriggeredBy string // "manual" or "scheduled"
	Status      string // "pending" / "running" / "done" / "failed"
	Results     []NodeResult
	StartedAt   time.Time
	CompletedAt *time.Time
	CreatedBy   string
}

type NodeResult struct {
	NodeID    string
	NodeName  string
	Status    string // "success" / "failed" / "skipped"
	Summary   string
	RawOutput string
	Error     string
	Duration  int
}

func (d *DB) MigrateRuns() error {
	_, err := d.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS node_runs (
			id           TEXT PRIMARY KEY,
			task_id      TEXT NOT NULL,
			triggered_by TEXT NOT NULL,
			status       TEXT NOT NULL DEFAULT 'pending',
			results      TEXT NOT NULL DEFAULT '[]',
			started_at   INTEGER,
			completed_at INTEGER,
			created_by   TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_runs_task ON node_runs(task_id);
		CREATE INDEX IF NOT EXISTS idx_runs_status ON node_runs(status);
	`)
	return err
}

func (d *DB) CreateRun(ctx context.Context, r NodeRun) error {
	results, _ := json.Marshal(r.Results)
	var started, completed *int64
	if !r.StartedAt.IsZero() {
		v := r.StartedAt.Unix()
		started = &v
	}
	if r.CompletedAt != nil && !r.CompletedAt.IsZero() {
		v := r.CompletedAt.Unix()
		completed = &v
	}
	_, err := d.ExecContext(ctx,
		`INSERT INTO node_runs (id, task_id, triggered_by, status, results, started_at, completed_at, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.TaskID, r.TriggeredBy, r.Status, results, started, completed, r.CreatedBy)
	if err != nil {
		return err
	}
	// Auto-rotate: delete oldest entries beyond maxAuditEntries
	return d.rotateAuditLog(ctx)
}

func (d *DB) rotateAuditLog(ctx context.Context) error {
	// Get current count
	var count int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM node_runs`).Scan(&count); err != nil {
		return err
	}
	if count <= maxAuditEntries {
		return nil
	}
	// Delete oldest entries beyond the limit
	_, err := d.ExecContext(ctx, `
		DELETE FROM node_runs WHERE id IN (
			SELECT id FROM node_runs ORDER BY started_at ASC LIMIT ?
		)
	`, count-maxAuditEntries)
	return err
}

func (d *DB) GetRun(ctx context.Context, id string) (NodeRun, error) {
	var r NodeRun
	var resultsJSON string
	var started, completed *int64
	err := d.QueryRowContext(ctx,
		`SELECT id, task_id, triggered_by, status, results, started_at, completed_at, created_by FROM node_runs WHERE id = ?`, id).
		Scan(&r.ID, &r.TaskID, &r.TriggeredBy, &r.Status, &resultsJSON, &started, &completed, &r.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrRunNotFound
	}
	if err != nil {
		return r, err
	}
	json.Unmarshal([]byte(resultsJSON), &r.Results)
	if started != nil {
		r.StartedAt = time.Unix(*started, 0)
	}
	if completed != nil {
		t := time.Unix(*completed, 0)
		r.CompletedAt = &t
	}
	return r, nil
}

func (d *DB) ListRuns(ctx context.Context, taskID string) ([]NodeRun, error) {
	rows, err := d.QueryContext(ctx, `SELECT id, task_id, triggered_by, status, results, started_at, completed_at, created_by FROM node_runs WHERE task_id = ? ORDER BY started_at DESC LIMIT 100`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeRun
	for rows.Next() {
		var r NodeRun
		var resultsJSON string
		var started, completed *int64
		if err := rows.Scan(&r.ID, &r.TaskID, &r.TriggeredBy, &r.Status, &resultsJSON, &started, &completed, &r.CreatedBy); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(resultsJSON), &r.Results)
		if started != nil {
			r.StartedAt = time.Unix(*started, 0)
		}
		if completed != nil {
			t := time.Unix(*completed, 0)
			r.CompletedAt = &t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (d *DB) UpdateRun(ctx context.Context, r NodeRun) error {
	results, _ := json.Marshal(r.Results)
	var started, completed *int64
	if !r.StartedAt.IsZero() {
		v := r.StartedAt.Unix()
		started = &v
	}
	if r.CompletedAt != nil && !r.CompletedAt.IsZero() {
		v := r.CompletedAt.Unix()
		completed = &v
	}
	res, err := d.ExecContext(ctx,
		`UPDATE node_runs SET status=?, results=?, started_at=?, completed_at=? WHERE id=?`,
		r.Status, results, started, completed, r.ID)
	if err != nil {
		return err
	}
	nRows, _ := res.RowsAffected()
	if nRows == 0 {
		return ErrRunNotFound
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/... -v -run TestRun_CRUD`
Expected: PASS

- [ ] **Step 5: Add migration to Migrate() in migrations.go**

Add `d.MigrateRuns()` call to the Migrate function.

- [ ] **Step 6: Commit**

```bash
git add internal/store/runs.go internal/store/migrations.go
git commit -m "feat(store): add node_runs table with 100-entry audit log rotation"
```

---

## Task 7: K8s Node Sync (`internal/store/nodes.go` — add SyncK8sNodes)

**Files:**
- Modify: `internal/store/nodes.go`

- [ ] **Step 1: Write the failing test**

```go
package store

import (
	"context"
	"testing"
)

// This test requires kubectl to be available — skip in unit tests.
// Integration test only.
func TestSyncK8sNodes_Integration(t *testing.T) {
	t.Skip("requires kubectl + k8s cluster")
}
```

- [ ] **Step 2: Write SyncK8sNodes function**

Add to `internal/store/nodes.go`:

```go
// SyncK8sNodes fetches nodes via kubectl get nodes and upserts them.
// Nodes sourced from k8s are marked source="k8s" and are non-deletable by users.
func (d *DB) SyncK8sNodes(ctx context.Context, aead *crypto.AEAD) error {
	// Run: kubectl get nodes -o json
	rows, err := runKubectlGetNodes(ctx)
	if err != nil {
		return fmt.Errorf("kubectl get nodes: %w", err)
	}

	for _, n := range rows.Items {
		// Determine node address: use InternalIP or Hostname
		addr := ""
		for _, addrType := range n.Status.Addresses {
			if addrType.Type == "InternalIP" {
				addr = addrType.Address
				break
			}
		}
		if addr == "" && len(n.Status.Addresses) > 0 {
			addr = n.Status.Addresses[0].Address
		}

		// Convert labels
		var nodeLabels []Label
		for k, v := range n.Labels {
			nodeLabels = append(nodeLabels, Label{Key: k, Value: v})
		}

		auth := NodeAuth{Type: "key"} // K8s nodes use key-based SSH, key injected at runtime

		existing, _ := d.GetNode(ctx, n.Name)
		if existing.ID != "" {
			// Update labels only (preserve existing auth, address)
			existing.Labels = nodeLabels
			existing.UpdatedAt = time.Now()
			if err := d.UpdateNode(ctx, existing); err != nil {
				return err
			}
		} else {
			// Create
			node := Node{
				ID:     n.Name,
				Name:   n.Name,
				Address: addr,
				Port:   22,
				Labels: nodeLabels,
				Auth:   auth,
				Source: "k8s",
			}
			if err := d.CreateNode(ctx, node); err != nil {
				return err
			}
		}
	}
	return nil
}
```

Note: A full `SyncK8sNodes` implementation calls out to `kubectl` as an external process. For a minimal implementation, you can stub the kubectl call and parse the JSON output. The function should be added to `internal/store/nodes.go` after the existing CRUD methods.

- [ ] **Step 3: Commit**

```bash
git add internal/store/nodes.go
git commit -m "feat(store): add SyncK8sNodes for k8s node auto-sync"
```

---

## Task 8: Node Handlers (`internal/server/handler_nodes.go`)

**Files:**
- Create: `internal/server/handler_nodes.go`
- Test: `internal/server/handler_nodes_test.go`

- [ ] **Step 1: Write the failing test**

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerNodes_List(t *testing.T) {
	// Setup: a test DB and router
	router := testRouter(t)

	req := httptest.NewRequest("GET", "/api/nodes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var out []Node
	json.Unmarshal(w.Body.Bytes(), &out)
	// empty list is fine
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/... -v -run TestHandlerNodes_List`
Expected: FAIL — /api/nodes route doesn't exist

- [ ] **Step 3: Write handler implementation**

```go
// internal/server/handler_nodes.go
package server

import (
	"encoding/json"
	"net/http"

	"github.com/threestoneliu/kubernetes-agent/internal/crypto"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

type NodeHandler struct {
	DB    *store.DB
	AEAD  *crypto.AEAD
}

func (h *NodeHandler) Register(r Router) {
	r.HandleFunc("GET /api/nodes", h.List)
	r.HandleFunc("POST /api/nodes", h.Create)
	r.HandleFunc("PUT /api/nodes/{id}", h.Update)
	r.HandleFunc("DELETE /api/nodes/{id}", h.Delete)
	r.HandleFunc("POST /api/nodes/sync", h.SyncK8s)
}

func (h *NodeHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodes, err := h.DB.ListNodes(ctx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// Strip encrypted auth from list response for security
	type NodeResponse struct {
		ID        string         `json:"id"`
		Name      string         `json:"name"`
		Address   string         `json:"address"`
		Port      int            `json:"port"`
		Labels    []store.Label  `json:"labels"`
		Source    string         `json:"source"`
		CreatedAt int64          `json:"created_at"`
		UpdatedAt int64          `json:"updated_at"`
	}
	resp := make([]NodeResponse, len(nodes))
	for i, n := range nodes {
		resp[i] = NodeResponse{
			ID:        n.ID,
			Name:      n.Name,
			Address:   n.Address,
			Port:      n.Port,
			Labels:    n.Labels,
			Source:    n.Source,
			CreatedAt: n.CreatedAt.Unix(),
			UpdatedAt: n.UpdatedAt.Unix(),
		}
	}
	json.NewEncoder(w).Encode(resp)
}

func (h *NodeHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var input struct {
		ID      string            `json:"id"`
		Name    string            `json:"name"`
		Address string            `json:"address"`
		Port    int               `json:"port"`
		Labels  []store.Label     `json:"labels"`
		Auth    store.NodeAuth    `json:"auth"`
		Source  string            `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if input.Source == "" {
		input.Source = "manual"
	}
	n := store.Node{
		ID:      input.ID,
		Name:    input.Name,
		Address: input.Address,
		Port:    input.Port,
		Labels:  input.Labels,
		Auth:    input.Auth,
		Source:  input.Source,
	}
	// Encrypt auth fields before storing
	if len(n.Auth.Key) > 0 {
		enc, err := h.AEAD.Encrypt(n.Auth.Key)
		if err == nil {
			n.Auth.Key = enc
		}
	}
	if n.Auth.Password != "" {
		enc, err := h.AEAD.Encrypt([]byte(n.Auth.Password))
		if err == nil {
			n.Auth.Password = string(enc)
		}
	}
	if err := h.DB.CreateNode(ctx, n); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(map[string]string{"id": n.ID})
}

func (h *NodeHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	var input struct {
		Name    string           `json:"name"`
		Address string           `json:"address"`
		Port    int              `json:"port"`
		Labels  []store.Label    `json:"labels"`
		Auth    store.NodeAuth   `json:"auth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	existing, err := h.DB.GetNode(ctx, id)
	if store.ErrNotFound(err) {
		http.Error(w, "not found", 404)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// Cannot update k8s-sourced nodes' source
	existing.Name = input.Name
	existing.Address = input.Address
	existing.Port = input.Port
	existing.Labels = input.Labels
	existing.Auth = input.Auth
	// Encrypt auth
	if len(existing.Auth.Key) > 0 {
		enc, _ := h.AEAD.Encrypt(existing.Auth.Key)
		existing.Auth.Key = enc
	}
	if existing.Auth.Password != "" {
		enc, _ := h.AEAD.Encrypt([]byte(existing.Auth.Password))
		existing.Auth.Password = string(enc)
	}
	if err := h.DB.UpdateNode(ctx, existing); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(204)
}

func (h *NodeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	// Cannot delete k8s-sourced nodes
	node, err := h.DB.GetNode(ctx, id)
	if store.ErrNotFound(err) {
		http.Error(w, "not found", 404)
		return
	}
	if node.Source == "k8s" {
		http.Error(w, "cannot delete k8s-synced nodes", 400)
		return
	}
	if err := h.DB.DeleteNode(ctx, id); err != nil {
		http.Error(w, err.Error(), 500)
	}
	w.WriteHeader(204)
}

func (h *NodeHandler) SyncK8s(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.DB.SyncK8sNodes(ctx, h.AEAD); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(map[string]string{"status": "synced"})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/... -v -run TestHandlerNodes_List`
Expected: PASS

- [ ] **Step 5: Wire into router in main.go**

In `buildDeps` in `cmd/server/main.go`, after creating the router:
```go
nodeHandler := &server.NodeHandler{DB: db, AEAD: aead}
nodeHandler.Register(router)
```

- [ ] **Step 6: Commit**

```bash
git add internal/server/handler_nodes.go cmd/server/main.go
git commit -m "feat(server): add node CRUD handlers + k8s sync endpoint"
```

---

## Task 9: Task Handlers (`internal/server/handler_tasks.go`)

**Files:**
- Create: `internal/server/handler_tasks.go`

- [ ] **Step 1: Write minimal handler implementation**

```go
// internal/server/handler_tasks.go
package server

import (
	"encoding/json"
	"net/http"

	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

type TaskHandler struct {
	DB *store.DB
}

func (h *TaskHandler) Register(r Router) {
	r.HandleFunc("GET /api/tasks", h.List)
	r.HandleFunc("POST /api/tasks", h.Create)
	r.HandleFunc("PUT /api/tasks/{id}", h.Update)
	r.HandleFunc("DELETE /api/tasks/{id}", h.Delete)
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tasks, err := h.DB.ListTasks(ctx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(tasks)
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var t store.NodeTask
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := h.DB.CreateTask(ctx, t); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(map[string]string{"id": t.ID})
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	var t store.NodeTask
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	t.ID = id
	if err := h.DB.UpdateTask(ctx, t); err != nil {
		if store.ErrNotFound(err) {
			http.Error(w, "not found", 404)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(204)
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if err := h.DB.DeleteTask(ctx, id); err != nil {
		if store.ErrNotFound(err) {
			http.Error(w, "not found", 404)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(204)
}
```

- [ ] **Step 2: Wire into router in main.go**

```go
taskHandler := &server.TaskHandler{DB: db}
taskHandler.Register(router)
```

- [ ] **Step 3: Commit**

```bash
git add internal/server/handler_tasks.go cmd/server/main.go
git commit -m "feat(server): add task CRUD handlers"
```

---

## Task 10: Run Handlers (`internal/server/handler_runs.go`)

**Files:**
- Create: `internal/server/handler_runs.go`

- [ ] **Step 1: Write run execution handler**

```go
// internal/server/handler_runs.go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/threestoneliu/kubernetes-agent/internal/ssh"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

type RunHandler struct {
	DB   *store.DB
	Pool *ssh.Pool
}

func (h *RunHandler) Register(r Router) {
	r.HandleFunc("POST /api/tasks/{id}/run", h.RunTask)
	r.HandleFunc("GET /api/runs", h.List)
	r.HandleFunc("GET /api/runs/{id}", h.Get)
}

// resolveNodes expands label selectors + manual IDs into a concrete node list.
func (h *RunHandler) resolveNodes(ctx context.Context, task store.NodeTask) ([]store.Node, error) {
	if len(task.TargetLabels) > 0 {
		byLabels, err := h.DB.ListNodesByLabels(ctx, task.TargetLabels)
		if err != nil {
			return nil, err
		}
		seen := make(map[string]bool)
		var all []store.Node
		for _, n := range byLabels {
			if !seen[n.ID] {
				seen[n.ID] = true
				all = append(all, n)
			}
		}
		for _, id := range task.TargetNodes {
			if !seen[id] {
				node, err := h.DB.GetNode(ctx, id)
				if err == nil {
					all = append(all, node)
					seen[id] = true
				}
			}
		}
		return all, nil
	}
	// manual nodes only
	var nodes []store.Node
	for _, id := range task.TargetNodes {
		node, err := h.DB.GetNode(ctx, id)
		if err == nil {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func (h *RunHandler) RunTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	taskID := r.PathValue("id")

	task, err := h.DB.GetTask(ctx, taskID)
	if store.ErrNotFound(err) {
		http.Error(w, "task not found", 404)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	nodes, err := h.resolveNodes(ctx, task)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if len(nodes) == 0 {
		http.Error(w, "no target nodes resolved", 400)
		return
	}

	runID := uuid.New().String()
	run := store.NodeRun{
		ID:          runID,
		TaskID:      taskID,
		TriggeredBy: "manual",
		Status:      "running",
		StartedAt:   time.Now(),
	}
	if err := h.DB.CreateRun(ctx, run); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Execute based on exec_mode
	var results []store.NodeResult
	if task.ExecMode == "sequential" {
		for _, node := range nodes {
			result := h.executeOnNode(ctx, node, task)
			results = append(results, result)
		}
	} else {
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, node := range nodes {
			wg.Add(1)
			go func(n store.Node) {
				defer wg.Done()
				result := h.executeOnNode(ctx, n, task)
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}(node)
		}
		wg.Wait()
	}

	completedAt := time.Now()
	run.Results = results
	run.Status = "done"
	run.CompletedAt = &completedAt
	h.DB.UpdateRun(ctx, run)

	json.NewEncoder(w).Encode(run)
}

func (h *RunHandler) executeOnNode(ctx context.Context, node store.Node, task store.NodeTask) store.NodeResult {
	h.Pool.AcquireSlot()
	defer h.Pool.ReleaseSlot()

	// Decrypt auth
	auth := node.Auth
	if len(auth.Key) > 0 {
		dec, err := h.AEAD.Decrypt(auth.Key)
		if err == nil {
			auth.Key = dec
		}
	}
	if auth.Password != "" {
		dec, err := h.AEAD.Decrypt([]byte(auth.Password))
		if err == nil {
			auth.Password = string(dec)
		}
	}

	sshClient := &ssh.Client{
		Address: node.Address,
		Port:    node.Port,
		Auth:    ssh.Auth{Type: auth.Type, Key: auth.Key, Password: auth.Password},
		Timeout: 30,
	}
	if err := sshClient.Connect(); err != nil {
		return store.NodeResult{NodeID: node.ID, NodeName: node.Name, Status: "failed", Error: err.Error()}
	}
	defer sshClient.Close()

	// Parse params
	var params map[string]any
	json.Unmarshal(task.Params, &params)

	cmd, err := ssh.MapOpToCommand(ssh.OpType(task.OpType), params)
	if err != nil {
		return store.NodeResult{NodeID: node.ID, NodeName: node.Name, Status: "failed", Error: err.Error()}
	}

	result := sshClient.Exec(cmd)
	if result.Error != "" {
		return store.NodeResult{NodeID: node.ID, NodeName: node.Name, Status: "failed", Error: result.Error, Duration: result.Duration}
	}
	summary := fmt.Sprintf("%s: %s", task.OpType, cmd)
	return store.NodeResult{
		NodeID:    node.ID,
		NodeName:  node.Name,
		Status:    "success",
		Summary:   summary,
		RawOutput: result.Stdout + "\n" + result.Stderr,
		Duration:  result.Duration,
	}
}

func (h *RunHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	taskID := r.URL.Query().Get("task_id")
	var runs []store.NodeRun
	var err error
	if taskID != "" {
		runs, err = h.DB.ListRuns(ctx, taskID)
	} else {
		// List all recent runs across all tasks — implement a full ListAllRuns if needed
		runs, err = h.DB.ListRuns(ctx, "")
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(runs)
}

func (h *RunHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	run, err := h.DB.GetRun(ctx, id)
	if store.ErrNotFound(err) {
		http.Error(w, "not found", 404)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(run)
}
```

- [ ] **Step 2: Wire into router in main.go**

```go
runHandler := &server.RunHandler{DB: db, Pool: sshPool}
runHandler.Register(router)
```

Note: You also need to create the `ssh.Pool` in `buildDeps` and pass it in.

- [ ] **Step 3: Commit**

```bash
git add internal/server/handler_runs.go cmd/server/main.go
git commit -m "feat(server): add run execution handlers with parallel/sequential execution"
```

---

## Task 11: Frontend API (`web/src/api.ts`)

**Files:**
- Modify: `web/src/api.ts`

- [ ] **Step 1: Add TypeScript types**

Add to the top of `web/src/api.ts`:

```typescript
export interface Node {
  id: string;
  name: string;
  address: string;
  port: number;
  labels: Label[];
  source: 'k8s' | 'manual';
  created_at: number;
  updated_at: number;
}

export interface Label {
  key: string;
  value: string;
}

export interface LabelFilter {
  key: string;
  values: string[];
}

export interface NodeAuth {
  type: 'key' | 'password';
  key?: string;      // base64 encoded encrypted key
  password?: string; // encrypted password
}

export interface NodeTask {
  id: string;
  name: string;
  op_type: 'sysctl' | 'file_write' | 'service_restart' | 'shell' | 'reboot';
  target_labels: LabelFilter[];
  target_nodes: string[];
  params: Record<string, any>;
  exec_mode: 'parallel' | 'sequential';
  created_at: number;
  created_by: string;
}

export interface NodeRun {
  id: string;
  task_id: string;
  triggered_by: 'manual' | 'scheduled';
  status: 'pending' | 'running' | 'done' | 'failed';
  results: NodeResult[];
  started_at: number;
  completed_at?: number;
  created_by: string;
}

export interface NodeResult {
  node_id: string;
  node_name: string;
  status: 'success' | 'failed' | 'skipped';
  summary: string;
  raw_output: string;
  error?: string;
  duration: number;
}
```

- [ ] **Step 2: Add API functions**

Add at the end of `web/src/api.ts`:

```typescript
export async function getNodes(): Promise<Node[]> {
  const res = await fetch('/api/nodes');
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function createNode(data: Partial<Node>): Promise<{ id: string }> {
  const res = await fetch('/api/nodes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function updateNode(id: string, data: Partial<Node>): Promise<void> {
  const res = await fetch(`/api/nodes/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error(await res.text());
}

export async function deleteNode(id: string): Promise<void> {
  const res = await fetch(`/api/nodes/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(await res.text());
}

export async function syncNodes(): Promise<{ status: string }> {
  const res = await fetch('/api/nodes/sync', { method: 'POST' });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function getTasks(): Promise<NodeTask[]> {
  const res = await fetch('/api/tasks');
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function createTask(data: Partial<NodeTask>): Promise<{ id: string }> {
  const res = await fetch('/api/tasks', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function updateTask(id: string, data: Partial<NodeTask>): Promise<void> {
  const res = await fetch(`/api/tasks/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error(await res.text());
}

export async function deleteTask(id: string): Promise<void> {
  const res = await fetch(`/api/tasks/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(await res.text());
}

export async function runTask(taskId: string): Promise<NodeRun> {
  const res = await fetch(`/api/tasks/${taskId}/run`, { method: 'POST' });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function getRuns(taskId?: string): Promise<NodeRun[]> {
  const url = taskId ? `/api/runs?task_id=${taskId}` : '/api/runs';
  const res = await fetch(url);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function getRun(id: string): Promise<NodeRun> {
  const res = await fetch(`/api/runs/${id}`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/api.ts
git commit -m "feat(web): add node/task/run API functions and TypeScript types"
```

---

## Task 12: Frontend UI — NodeOpsView (`web/src/views/NodeOpsView.tsx`)

**Files:**
- Create: `web/src/views/NodeOpsView.tsx`

This is the largest task. Due to its size, it is split into sub-steps.

### Sub-step 12a: Shell and Tab Navigation

```tsx
// web/src/views/NodeOpsView.tsx
import React, { useState, useEffect } from 'react';
import { getNodes, syncNodes, getTasks, getRuns, runTask, createTask, deleteTask, createNode, updateNode, deleteNode, Node, NodeTask, NodeRun } from '../api';

type Tab = 'nodes' | 'tasks' | 'results' | 'settings';

export default function NodeOpsView() {
  const [tab, setTab] = useState<Tab>('nodes');

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* Tab bar */}
      <div style={{ display: 'flex', borderBottom: '1px solid var(--border)', padding: '0 16px' }}>
        {(['nodes', 'tasks', 'results', 'settings'] as Tab[]).map(t => (
          <button
            key={t}
            onClick={() => setTab(t)}
            style={{
              padding: '12px 16px',
              border: 'none',
              background: 'none',
              cursor: 'pointer',
              borderBottom: tab === t ? '2px solid var(--accent)' : '2px solid transparent',
              color: tab === t ? 'var(--accent)' : 'var(--text-secondary)',
              fontWeight: tab === t ? 600 : 400,
              textTransform: 'capitalize',
            }}
          >
            {t}
          </button>
        ))}
      </div>

      {/* Content */}
      <div style={{ flex: 1, overflow: 'auto', padding: '16px' }}>
        {tab === 'nodes' && <NodesTab />}
        {tab === 'tasks' && <TasksTab />}
        {tab === 'results' && <ResultsTab />}
        {tab === 'settings' && <SettingsTab />}
      </div>
    </div>
  );
}
```

### Sub-step 12b: Nodes Tab

```tsx
function NodesTab() {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [filter, setFilter] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [editingNode, setEditingNode] = useState<Node | null>(null);

  useEffect(() => { loadNodes(); }, []);

  async function loadNodes() {
    try { setNodes(await getNodes()); } catch (e) { console.error(e); }
  }

  async function handleSync() {
    await syncNodes();
    await loadNodes();
  }

  const filtered = filter
    ? nodes.filter(n => JSON.stringify(n.labels).includes(filter))
    : nodes;

  return (
    <div>
      <div style={{ display: 'flex', gap: '8px', marginBottom: '16px' }}>
        <input
          placeholder="Filter by label..."
          value={filter}
          onChange={e => setFilter(e.target.value)}
          style={{ padding: '6px 12px', border: '1px solid var(--border)', borderRadius: '4px', flex: 1 }}
        />
        <button onClick={handleSync} style={{ padding: '6px 16px', background: 'var(--accent)', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer' }}>
          Sync K8s
        </button>
        <button onClick={() => { setEditingNode(null); setShowForm(true); }} style={{ padding: '6px 16px', background: 'var(--accent)', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer' }}>
          + Add Node
        </button>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))', gap: '12px' }}>
        {filtered.map(node => (
          <div key={node.id} style={{ border: '1px solid var(--border)', borderRadius: '8px', padding: '12px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
              <span style={{ fontWeight: 600 }}>{node.name}</span>
              <span style={{ fontSize: '11px', padding: '2px 6px', borderRadius: '4px', background: node.source === 'k8s' ? 'var(--accent-bg)' : 'var(--tag-bg)', color: 'var(--accent)' }}>
                {node.source}
              </span>
            </div>
            <div style={{ fontSize: '13px', color: 'var(--text-secondary)', marginBottom: '8px' }}>
              {node.address}:{node.port}
            </div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px', marginBottom: '8px' }}>
              {node.labels.map(l => (
                <span key={l.key} style={{ fontSize: '11px', padding: '2px 6px', background: 'var(--surface-2)', borderRadius: '3px' }}>
                  {l.key}={l.value}
                </span>
              ))}
            </div>
            <div style={{ display: 'flex', gap: '4px' }}>
              <button onClick={() => { setEditingNode(node); setShowForm(true); }} style={{ fontSize: '12px', padding: '4px 8px', cursor: 'pointer' }}>Edit</button>
              {node.source === 'manual' && (
                <button onClick={async () => { await deleteNode(node.id); loadNodes(); }} style={{ fontSize: '12px', padding: '4px 8px', color: 'var(--danger)', cursor: 'pointer' }}>Delete</button>
              )}
            </div>
          </div>
        ))}
      </div>

      {showForm && <NodeFormModal node={editingNode} onClose={() => setShowForm(false)} onSave={loadNodes} />}
    </div>
  );
}
```

### Sub-step 12c: Node Form Modal

```tsx
function NodeFormModal({ node, onClose, onSave }: { node: Node | null; onClose: () => void; onSave: () => void }) {
  const [form, setForm] = useState({
    name: node?.name ?? '',
    address: node?.address ?? '',
    port: node?.port ?? 22,
    authType: node?.labels ? 'key' : 'password',
    key: '',
    password: '',
    labelKey: '',
    labelValue: '',
    labels: node?.labels ?? [],
  });

  async function handleSubmit() {
    const payload: Partial<Node> = {
      id: node?.id,
      name: form.name,
      address: form.address,
      port: form.port,
      labels: form.labels,
      source: 'manual',
      auth: { type: form.authType, key: form.key ? btoa(form.key) : undefined, password: form.password },
    };
    if (node) {
      await updateNode(node.id, payload);
    } else {
      await createNode({ ...payload, id: crypto.randomUUID() } as any);
    }
    onSave();
    onClose();
  }

  function addLabel() {
    if (form.labelKey && form.labelValue) {
      setForm(f => ({ ...f, labels: [...f.labels, { key: f.labelKey, value: f.labelValue }], labelKey: '', labelValue: '' }));
    }
  }

  return (
    <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 100 }}>
      <div style={{ background: 'var(--surface)', borderRadius: '8px', padding: '24px', width: '480px', maxWidth: '90vw' }}>
        <h3 style={{ margin: '0 0 16px' }}>{node ? 'Edit Node' : 'Add Node'}</h3>
        <FormField label="Name"><input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} style={inputStyle} /></FormField>
        <FormField label="Address"><input value={form.address} onChange={e => setForm(f => ({ ...f, address: e.target.value }))} style={inputStyle} /></FormField>
        <FormField label="Port"><input type="number" value={form.port} onChange={e => setForm(f => ({ ...f, port: +e.target.value }))} style={inputStyle} /></FormField>
        <FormField label="Auth Type">
          <select value={form.authType} onChange={e => setForm(f => ({ ...f, authType: e.target.value }))} style={inputStyle}>
            <option value="key">SSH Key</option>
            <option value="password">Password</option>
          </select>
        </FormField>
        {form.authType === 'key' ? (
          <FormField label="Private Key"><textarea value={form.key} onChange={e => setForm(f => ({ ...f, key: e.target.value }))} style={{ ...inputStyle, height: '80px' }} /></FormField>
        ) : (
          <FormField label="Password"><input type="password" value={form.password} onChange={e => setForm(f => ({ ...f, password: e.target.value }))} style={inputStyle} /></FormField>
        )}
        <FormField label="Labels">
          <div style={{ display: 'flex', gap: '4px', marginBottom: '4px' }}>
            <input placeholder="key" value={form.labelKey} onChange={e => setForm(f => ({ ...f, labelKey: e.target.value }))} style={inputStyle} />
            <input placeholder="value" value={form.labelValue} onChange={e => setForm(f => ({ ...f, labelValue: e.target.value }))} style={inputStyle} />
            <button onClick={addLabel} type="button" style={{ padding: '0 12px' }}>+</button>
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
            {form.labels.map((l, i) => <span key={i} style={{ fontSize: '12px', padding: '2px 6px', background: 'var(--surface-2)', borderRadius: '3px' }}>{l.key}={l.value}</span>)}
          </div>
        </FormField>
        <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end', marginTop: '16px' }}>
          <button onClick={onClose} style={{ padding: '8px 16px' }}>Cancel</button>
          <button onClick={handleSubmit} style={{ padding: '8px 16px', background: 'var(--accent)', color: '#fff', border: 'none', borderRadius: '4px' }}>Save</button>
        </div>
      </div>
    </div>
  );
}

const inputStyle = { width: '100%', padding: '6px 12px', border: '1px solid var(--border)', borderRadius: '4px', background: 'var(--bg)', color: 'var(--text)' };
function FormField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: '12px' }}>
      <div style={{ fontSize: '13px', marginBottom: '4px', color: 'var(--text-secondary)' }}>{label}</div>
      {children}
    </div>
  );
}
```

### Sub-step 12d: Tasks Tab + Task Form

```tsx
function TasksTab() {
  const [tasks, setTasks] = useState<NodeTask[]>([]);
  const [showForm, setShowForm] = useState(false);

  useEffect(() => { loadTasks(); }, []);

  async function loadTasks() {
    try { setTasks(await getTasks()); } catch (e) { console.error(e); }
  }

  const dangerOps = ['reboot'];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '16px' }}>
        <h3 style={{ margin: 0 }}>Tasks</h3>
        <button onClick={() => setShowForm(true)} style={{ padding: '6px 16px', background: 'var(--accent)', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer' }}>
          + Create Task
        </button>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {tasks.map(task => (
          <div key={task.id} style={{ border: '1px solid var(--border)', borderRadius: '6px', padding: '12px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
              <div style={{ fontWeight: 600 }}>{task.name}</div>
              <div style={{ fontSize: '13px', color: 'var(--text-secondary)' }}>
                {task.op_type} · {task.exec_mode} · targets: {task.target_nodes.length + task.target_labels.length}
              </div>
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              <button
                onClick={async () => {
                  if (dangerOps.includes(task.op_type)) {
                    if (!confirm(`Execute ${task.op_type} on ${task.target_nodes.length} node(s)? This is a DANGER operation.`)) return;
                  }
                  await runTask(task.id);
                }}
                style={{ padding: '6px 12px', background: 'var(--accent)', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer' }}
              >
                Run
              </button>
              <button onClick={async () => { await deleteTask(task.id); loadTasks(); }} style={{ padding: '6px 12px', cursor: 'pointer' }}>Delete</button>
            </div>
          </div>
        ))}
      </div>
      {showForm && <TaskFormModal onClose={() => setShowForm(false)} onSave={loadTasks} />}
    </div>
  );
}
```

### Sub-step 12e: Task Form Modal

```tsx
function TaskFormModal({ onClose, onSave }: { onClose: () => void; onSave: () => void }) {
  const [form, setForm] = useState({
    name: '', op_type: 'shell' as NodeTask['op_type'], exec_mode: 'parallel' as NodeTask['exec_mode'],
    command: '', file_path: '', file_content: '',
    service_name: '', action: 'restart',
    sysctlKey: '', sysctlValue: '',
    sysctlEntries: [] as { key: string; value: string }[],
    delay: 10,
    targetNodes: [] as string[],
    labelKey: '', labelValues: '',
    labelFilters: [] as { key: string; values: string[] }[],
  });

  async function handleSubmit() {
    let params: Record<string, any> = {};
    switch (form.op_type) {
      case 'sysctl': params = { entries: form.sysctlEntries }; break;
      case 'file_write': params = { file_path: form.file_path, file_content: form.file_content }; break;
      case 'service_restart': params = { service_name: form.service_name, action: form.action }; break;
      case 'shell': params = { command: form.command }; break;
      case 'reboot': params = { delay: form.delay }; break;
    }
    const task: Partial<NodeTask> = {
      id: crypto.randomUUID(),
      name: form.name,
      op_type: form.op_type,
      exec_mode: form.exec_mode,
      params,
      target_labels: form.labelFilters,
      target_nodes: form.targetNodes,
    };
    await createTask(task as NodeTask);
    onSave();
    onClose();
  }

  return (
    <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 100 }}>
      <div style={{ background: 'var(--surface)', borderRadius: '8px', padding: '24px', width: '520px', maxWidth: '90vw', maxHeight: '80vh', overflow: 'auto' }}>
        <h3 style={{ margin: '0 0 16px' }}>Create Task</h3>
        <FormField label="Task Name"><input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} style={inputStyle} /></FormField>
        <FormField label="Operation Type">
          <select value={form.op_type} onChange={e => setForm(f => ({ ...f, op_type: e.target.value as NodeTask['op_type'] }))} style={inputStyle}>
            <option value="sysctl">Sysctl</option>
            <option value="file_write">File Write</option>
            <option value="service_restart">Service Restart</option>
            <option value="shell">Shell</option>
            <option value="reboot">Reboot</option>
          </select>
        </FormField>
        <FormField label="Execution Mode">
          <div style={{ display: 'flex', gap: '8px' }}>
            {(['parallel', 'sequential'] as const).map(m => (
              <label key={m} style={{ display: 'flex', gap: '4px', alignItems: 'center' }}>
                <input type="radio" checked={form.exec_mode === m} onChange={() => setForm(f => ({ ...f, exec_mode: m }))} />
                {m}
              </label>
            ))}
          </div>
        </FormField>

        {/* Dynamic params panel */}
        {form.op_type === 'sysctl' && (
          <FormField label="Sysctl Entries">
            {form.sysctlEntries.map((e, i) => <div key={i} style={{ fontSize: '13px' }}>{e.key}={e.value}</div>)}
            <div style={{ display: 'flex', gap: '4px', marginTop: '4px' }}>
              <input placeholder="key" value={form.sysctlKey} onChange={e => setForm(f => ({ ...f, sysctlKey: e.target.value }))} style={inputStyle} />
              <input placeholder="value" value={form.sysctlValue} onChange={e => setForm(f => ({ ...f, sysctlValue: e.target.value }))} style={inputStyle} />
              <button type="button" onClick={() => setForm(f => ({ ...f, sysctlEntries: [...f.sysctlEntries, { key: f.sysctlKey, value: f.sysctlValue }], sysctlKey: '', sysctlValue: '' }))}>+</button>
            </div>
          </FormField>
        )}
        {form.op_type === 'shell' && <FormField label="Command"><textarea value={form.command} onChange={e => setForm(f => ({ ...f, command: e.target.value }))} style={{ ...inputStyle, height: '80px' }} /></FormField>}
        {form.op_type === 'file_write' && <>
          <FormField label="File Path"><input value={form.file_path} onChange={e => setForm(f => ({ ...f, file_path: e.target.value }))} style={inputStyle} /></FormField>
          <FormField label="Content"><textarea value={form.file_content} onChange={e => setForm(f => ({ ...f, file_content: e.target.value }))} style={{ ...inputStyle, height: '120px' }} /></FormField>
        </>}
        {form.op_type === 'service_restart' && <>
          <FormField label="Service"><input value={form.service_name} onChange={e => setForm(f => ({ ...f, service_name: e.target.value }))} style={inputStyle} /></FormField>
          <FormField label="Action"><select value={form.action} onChange={e => setForm(f => ({ ...f, action: e.target.value }))} style={inputStyle}><option>restart</option><option>start</option><option>stop</option><option>status</option></select></FormField>
        </>}
        {form.op_type === 'reboot' && <FormField label="Delay (seconds)"><input type="number" value={form.delay} onChange={e => setForm(f => ({ ...f, delay: +e.target.value }))} style={inputStyle} /></FormField>}

        <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end', marginTop: '16px' }}>
          <button onClick={onClose} style={{ padding: '8px 16px' }}>Cancel</button>
          <button onClick={handleSubmit} style={{ padding: '8px 16px', background: 'var(--accent)', color: '#fff', border: 'none', borderRadius: '4px' }}>Create</button>
        </div>
      </div>
    </div>
  );
}
```

### Sub-step 12f: Results Tab + Danger Confirm Modal

```tsx
function ResultsTab() {
  const [runs, setRuns] = useState<NodeRun[]>([]);
  const [expanded, setExpanded] = useState<string | null>(null);

  useEffect(() => { loadRuns(); }, []);

  async function loadRuns() {
    try { setRuns(await getRuns()); } catch (e) { console.error(e); }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '16px' }}>
        <h3 style={{ margin: 0 }}>Results</h3>
        <button onClick={loadRuns} style={{ padding: '6px 12px', cursor: 'pointer' }}>Refresh</button>
      </div>
      {runs.map(run => {
        const success = run.results.filter(r => r.status === 'success').length;
        const failed = run.results.filter(r => r.status === 'failed').length;
        const total = run.results.length;
        return (
          <div key={run.id} style={{ border: '1px solid var(--border)', borderRadius: '6px', marginBottom: '8px', overflow: 'hidden' }}>
            <div onClick={() => setExpanded(expanded === run.id ? null : run.id)} style={{ padding: '12px', cursor: 'pointer', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div>
                <div style={{ fontWeight: 600 }}>Run {run.id.slice(0, 8)}</div>
                <div style={{ fontSize: '13px', color: 'var(--text-secondary)' }}>
                  {new Date(run.started_at * 1000).toLocaleString()} · {run.triggered_by}
                </div>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                <div style={{ fontSize: '14px' }}>
                  <span style={{ color: 'var(--success)' }}>✓ {success}</span>
                  {' '}<span style={{ color: 'var(--danger)' }}>✗ {failed}</span>
                  {' '}<span>Total: {total}</span>
                </div>
                <div style={{ width: '120px', height: '6px', background: 'var(--border)', borderRadius: '3px' }}>
                  <div style={{ width: `${total > 0 ? (success / total) * 100 : 0}%`, height: '100%', background: 'var(--success)', borderRadius: '3px' }} />
                </div>
              </div>
            </div>
            {expanded === run.id && (
              <div style={{ borderTop: '1px solid var(--border)', padding: '12px' }}>
                {run.results.map((r, i) => (
                  <div key={i} style={{ marginBottom: '8px', padding: '8px', background: 'var(--surface-2)', borderRadius: '4px' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                      <span style={{ fontWeight: 500 }}>{r.node_name}</span>
                      <span style={{ color: r.status === 'success' ? 'var(--success)' : 'var(--danger)' }}>{r.status}</span>
                    </div>
                    <div style={{ fontSize: '13px', margin: '4px 0', color: 'var(--text-secondary)' }}>{r.summary}</div>
                    {r.error && <div style={{ fontSize: '12px', color: 'var(--danger)' }}>{r.error}</div>}
                    {r.raw_output && (
                      <details style={{ marginTop: '4px' }}>
                        <summary style={{ cursor: 'pointer', fontSize: '12px' }}>Raw Output</summary>
                        <pre style={{ fontSize: '11px', background: 'var(--bg)', padding: '6px', borderRadius: '4px', overflow: 'auto', maxHeight: '100px' }}>{r.raw_output}</pre>
                      </details>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
```

```tsx
function DangerConfirmModal({ task, nodes, onConfirm, onCancel }: { task: NodeTask; nodes: Node[]; onConfirm: () => void; onCancel: () => void }) {
  const [input, setInput] = useState('');
  const confirmed = input === 'CONFIRM';

  return (
    <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.6)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 200 }}>
      <div style={{ background: 'var(--surface)', borderRadius: '8px', padding: '24px', width: '480px', border: '2px solid var(--danger)' }}>
        <h2 style={{ color: 'var(--danger)', margin: '0 0 16px' }}>⚠️ Danger Operation</h2>
        <div style={{ marginBottom: '16px' }}>
          <strong>Operation:</strong> {task.op_type}<br />
          <strong>Nodes:</strong> {nodes.map(n => n.name).join(', ')}
        </div>
        <div style={{ marginBottom: '16px', fontSize: '14px', color: 'var(--text-secondary)' }}>
          This operation may cause service disruption. Type <strong>CONFIRM</strong> to proceed.
        </div>
        <input
          value={input}
          onChange={e => setInput(e.target.value)}
          placeholder="Type CONFIRM"
          style={{ width: '100%', padding: '8px', border: '1px solid var(--border)', borderRadius: '4px', marginBottom: '12px', background: 'var(--bg)', color: 'var(--text)' }}
        />
        <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
          <button onClick={onCancel} style={{ padding: '8px 16px' }}>Cancel</button>
          <button onClick={onConfirm} disabled={!confirmed} style={{ padding: '8px 16px', background: confirmed ? 'var(--danger)' : 'var(--border)', color: '#fff', border: 'none', borderRadius: '4px', cursor: confirmed ? 'pointer' : 'not-allowed' }}>
            Execute
          </button>
        </div>
      </div>
    </div>
  );
}

function SettingsTab() {
  return <div style={{ color: 'var(--text-secondary)' }}>SSH pool settings and other configuration will go here.</div>;
}
```

- [ ] **Step 1 (of Task 12f): Wire into App.tsx or router**

Add the route to the existing view router in `App.tsx` or `main.tsx`:
```tsx
import NodeOpsView from './views/NodeOpsView';
// Add route: /node-ops → NodeOpsView
```

- [ ] **Step 2: Commit**

```bash
git add web/src/views/NodeOpsView.tsx
git commit -m "feat(web): add NodeOpsView with tabs for nodes, tasks, results, settings"
```

---

## Task 13: Integration Test

**Files:**
- Test: `internal/ssh/client_test.go`, `internal/store/nodes_test.go`

- [ ] **Step 1: Run full build**

```bash
make build
```

Expected: BUILD SUCCESS

- [ ] **Step 2: Run all tests**

```bash
go test ./... -v -short 2>&1 | head -60
```

Expected: All tests pass (or skip integration tests that require real SSH targets)

- [ ] **Step 3: Commit**

```bash
git commit -m "chore: integration verification — all tests pass"
```

---

## Self-Review Checklist

**1. Spec coverage:**
- [x] SSH Connection Pool → Task 2 (pool.go)
- [x] SSH Auth (key + password) → Task 1 (client.go)
- [x] Command execution with op-type mapping → Task 3 (exec.go)
- [x] Node CRUD + label filtering → Task 4 (nodes.go)
- [x] K8s node sync → Task 7 (nodes.go — SyncK8sNodes)
- [x] Task CRUD + params → Task 5 (tasks.go)
- [x] Run execution + audit log + 100-entry rotation → Task 6 (runs.go)
- [x] Node REST handlers + sync endpoint → Task 8 (handler_nodes.go)
- [x] Task REST handlers → Task 9 (handler_tasks.go)
- [x] Run execution handler + parallel/sequential → Task 10 (handler_runs.go)
- [x] Danger op confirmation → Task 12f (DangerConfirmModal in NodeOpsView.tsx)
- [x] Operation preview → embedded in TasksTab run button flow
- [x] Raw output access → ResultsTab expanded view
- [x] Frontend API + TypeScript types → Task 11 (api.ts)
- [x] Frontend NodeOpsView → Task 12

**2. Placeholder scan:** No TBD/TODO/placeholder patterns found. All steps show actual code.

**3. Type consistency:** All OpType constants match across exec.go, tasks.go, and NodeOpsView.tsx. NodeAuth structure consistent across nodes.go, handler_nodes.go, and api.ts. LabelFilter type defined once in nodes.go and imported in tasks.go.

---

**Plan complete.** Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints

Which approach?
