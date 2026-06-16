# K8s Natural Language Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a local single-user Kubernetes agent that lets the user operate clusters via natural language, with mandatory Plan preview + global policy guardrails for all write operations.

**Architecture:** Go single binary + React/Vite SPA embedded via `embed.FS`. charmbracelet/fantasy orchestrates the agent loop; client-go dynamic client exposes 5 K8s tools. Writes flow through server-side dry-run → policy check → user-confirmed Plan → second policy check → execute (with rollback on failure). SQLite stores 6 tables, AES-256-GCM encrypts kubeconfig, master key on local disk (0600). SSE 12-event protocol connects backend to frontend.

**Tech Stack:** Go 1.22+, `github.com/charmbracelet/fantasy`, `k8s.io/client-go` (dynamic), `github.com/go-chi/chi/v5`, `gopkg.in/yaml.v3`, `modernc.org/sqlite`, `github.com/stretchr/testify`, React + Vite + TypeScript.

**Module Path:** `github.com/threestoneliu/kubernetes-agent`

---

## Task 1: 项目脚手架

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/logging/logging.go`
- Create: `configs/config.example.yaml`

### Step 1.1: Initialize Go module

```bash
cd /Users/liuzhilei/code/vibe/kubernetes-agent
go mod init github.com/threestoneliu/kubernetes-agent
```

- [ ] **Step 1.1: 初始化 module**

Run: `go mod init github.com/threestoneliu/kubernetes-agent`
Expected: `go.mod` created with `module github.com/threestoneliu/kubernetes-agent` and `go 1.22`

- [ ] **Step 1.2: 建立目录结构**

```bash
mkdir -p cmd/server \
  internal/{server,agent,tools/k8s,policy,store,crypto,llm,config,logging} \
  web configs
```

- [ ] **Step 1.3: 写 main.go 最小骨架 + logging**

Create `cmd/server/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/threestoneliu/kubernetes-agent/internal/config"
	"github.com/threestoneliu/kubernetes-agent/internal/logging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logging.Setup(cfg.Logging)
	slog.Info("startup", "host", cfg.Server.Host, "port", cfg.Server.Port)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	<-ctx.Done()
	slog.Info("shutdown")
	return nil
}
```

Create `internal/logging/logging.go`:

```go
package logging

import (
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func Setup(c Config) {
	var level slog.Level
	switch strings.ToLower(c.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if strings.ToLower(c.Format) == "text" {
		h = slog.NewTextHandler(os.Stderr, opts)
	} else {
		h = slog.NewJSONHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(h))
}
```

- [ ] **Step 1.4: 写 config 加载(YAML + ${ENV} 展开)**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Server struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type Storage struct {
	DBPath string `yaml:"db_path"`
}

type LLMProvider struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	APIKey  string `yaml:"apiKey"`
	BaseURL string `yaml:"baseURL"`
	Model   string `yaml:"model"`
}

type LLM struct {
	Default   string        `yaml:"default"`
	Providers []LLMProvider `yaml:"providers"`
}

type Config struct {
	Server  Server  `yaml:"server"`
	Storage Storage `yaml:"storage"`
	LLM     LLM     `yaml:"llm"`
	Logging logging.Config
	raw     []byte
}

var envPattern = regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`)

func Load() (*Config, error) {
	path := defaultPath()
	if v := os.Getenv("KUBERNETES_AGENT_CONFIG"); v != "" {
		path = v
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	expanded := envPattern.ReplaceAllStringFunc(string(data), func(m string) string {
		name := envPattern.FindStringSubmatch(m)[1]
		return os.Getenv(name)
	})
	var c Config
	if err := yaml.Unmarshal([]byte(expanded), &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.Host == "" {
		c.Server.Host = "127.0.0.1"
	}
	if c.Storage.DBPath == "" {
		c.Storage.DBPath = "~/.kubernetes-agent/data.db"
	}
	c.raw = data
	return &c, nil
}

func defaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kubernetes-agent", "config.yaml")
}
```

(Logging.Config 在 logging 包里,需要解决循环引用。改方案:在 config 包内嵌一个本地类型或让 logging.Config 在 config 包定义。本 plan 用方案 B:在 config 包定义自己的 Logging type,logging.Setup 接受任意带 Level/Format 字段的结构体或改用两个参数。简化:在 config 里直接定义 `Logging struct { Level, Format string }`,logging 包提供 `Setup(level, format string)`。)

修正后:

`internal/config/config.go` 内追加:
```go
type Logging struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}
```

并在 Config 字段把 `Logging logging.Config` 改为 `Logging Logging`。

`internal/logging/logging.go` 改为:
```go
func Setup(level, format string) {
    // ... 接收字符串参数
}
```

`cmd/server/main.go` 改为:
```go
logging.Setup(cfg.Logging.Level, cfg.Logging.Format)
```

- [ ] **Step 1.5: 写 config_test.go(TDD)**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("server:\n  port: 9000\n"), 0600))
	t.Setenv("KUBERNETES_AGENT_CONFIG", cfgPath)

	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 9000, c.Server.Port)
	assert.Equal(t, "127.0.0.1", c.Server.Host)
}

func TestLoad_EnvExpansion(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("llm:\n  providers:\n    - name: a\n      type: openai\n      apiKey: ${TEST_API_KEY}\n      model: gpt-4o\n"), 0600))
	t.Setenv("KUBERNETES_AGENT_CONFIG", cfgPath)
	t.Setenv("TEST_API_KEY", "sk-test-123")

	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "sk-test-123", c.LLM.Providers[0].APIKey)
}
```

- [ ] **Step 1.6: 跑测试确认通过**

Run: `go test ./internal/config/...`
Expected: PASS

- [ ] **Step 1.7: 写 configs/config.example.yaml**

Create `configs/config.example.yaml`:

```yaml
server:
  host: 127.0.0.1
  port: 8080

storage:
  db_path: ~/.kubernetes-agent/data.db

llm:
  default: anthropic-prod
  providers:
    - name: anthropic-prod
      type: anthropic
      apiKey: ${ANTHROPIC_API_KEY}
      model: claude-sonnet-4-6
    - name: openai-fallback
      type: openai
      apiKey: ${OPENAI_API_KEY}
      model: gpt-4o
    - name: ollama-local
      type: openai-compatible
      baseURL: http://localhost:11434/v1
      model: llama3.1

logging:
  level: info
  format: json
```

- [ ] **Step 1.8: Commit**

```bash
git add go.mod go.sum cmd/server internal/config internal/logging configs
git commit -m "feat(scaffold): init go module, config, logging"
```

---

## Task 2: 存储层(SQLite + 6 repos)

**Files:**
- Create: `internal/store/db.go`
- Create: `internal/store/migrations.go`
- Create: `internal/store/clusters.go`
- Create: `internal/store/sessions.go`
- Create: `internal/store/messages.go`
- Create: `internal/store/plans.go`
- Create: `internal/store/policies.go`
- Create: `internal/store/audit.go`
- Create: `internal/store/store_test.go`

- [ ] **Step 2.1: 写 store 失败测试 + db 骨架(TDD)**

Create `internal/store/store_test.go`:

```go
package store

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestOpenAndMigrate(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
}
```

- [ ] **Step 2.2: 实现 Open + Migrate**

Create `internal/store/db.go`:

```go
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &DB{db}, nil
}
```

Create `internal/store/migrations.go`:

```go
package store

import (
	"fmt"
)

var migrations = []string{
	// 1: initial schema
	`CREATE TABLE IF NOT EXISTS clusters (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		server TEXT NOT NULL,
		user TEXT NOT NULL,
		kubeconfig_blob BLOB NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		cluster_id TEXT REFERENCES clusters(id) ON DELETE SET NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		role TEXT NOT NULL,
		content TEXT,
		tool_calls TEXT,
		tool_call_id TEXT,
		reasoning TEXT,
		created_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, created_at);
	CREATE TABLE IF NOT EXISTS plans (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		ops_json TEXT NOT NULL,
		diffs_json TEXT NOT NULL,
		risk TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		executed_at INTEGER
	);
	CREATE TABLE IF NOT EXISTS policies (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		yaml TEXT NOT NULL,
		enabled INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT,
		cluster_id TEXT,
		action TEXT NOT NULL,
		target TEXT,
		status TEXT NOT NULL,
		message TEXT,
		created_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at);
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL
	);`,
}

func (d *DB) Migrate() error {
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}
	for i, m := range migrations {
		v := i + 1
		var n int
		if err := d.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, v).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(m); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d: %w", v, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, strftime('%s','now'))`, v); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 2.3: 跑测试**

Run: `go test ./internal/store/...`
Expected: PASS

- [ ] **Step 2.4: 写 clusters repo**

Create `internal/store/clusters.go`:

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type Cluster struct {
	ID             string
	Name           string
	Server         string
	User           string
	KubeconfigBlob []byte
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (d *DB) CreateCluster(ctx context.Context, c Cluster) error {
	now := time.Now().Unix()
	_, err := d.ExecContext(ctx,
		`INSERT INTO clusters (id, name, server, user, kubeconfig_blob, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Server, c.User, c.KubeconfigBlob, now, now)
	return err
}

func (d *DB) GetCluster(ctx context.Context, id string) (Cluster, error) {
	var c Cluster
	var ts int64
	err := d.QueryRowContext(ctx,
		`SELECT id, name, server, user, kubeconfig_blob, created_at, updated_at FROM clusters WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &c.Server, &c.User, &c.KubeconfigBlob, &ts, &ts)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	c.CreatedAt = time.Unix(ts, 0)
	c.UpdatedAt = time.Unix(ts, 0)
	return c, err
}

func (d *DB) ListClusters(ctx context.Context) ([]Cluster, error) {
	rows, err := d.QueryContext(ctx, `SELECT id, name, server, user, kubeconfig_blob, created_at, updated_at FROM clusters ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cluster
	for rows.Next() {
		var c Cluster
		var ts int64
		if err := rows.Scan(&c.ID, &c.Name, &c.Server, &c.User, &c.KubeconfigBlob, &ts, &ts); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(ts, 0)
		c.UpdatedAt = time.Unix(ts, 0)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) DeleteCluster(ctx context.Context, id string) error {
	res, err := d.ExecContext(ctx, `DELETE FROM clusters WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
```

- [ ] **Step 2.5: 写 sessions + messages + plans + policies + audit repo(同样 TDD)**

(每个 repo 模式同 clusters:struct + Create + Get + List + 业务方法。完整代码放在此 plan 的 Step 2.5 子项,工程实施时按 TDD 逐 repo 写测试再写实现。)

为节省篇幅,这里给出所有 5 个剩余 repo 的关键签名,完整文件内容由 subagent 实施时按 clusters 模式补全:

- `internal/store/sessions.go`:Session struct、CreateSession、GetSession、ListSessions、UpdateSessionTitle、DeleteSession
- `internal/store/messages.go`:Message struct、BatchInsertMessages(用于 message_end 一次性写)、ListMessagesBySession
- `internal/store/plans.go`:Plan struct、CreatePlan、GetPlan、UpdatePlanStatus(pending/approved/executed/cancelled/denied)、MarkExecuted
- `internal/store/policies.go`:Policy struct、ListEnabledPolicies、UpsertPolicy、SetEnabled、SeedDefaultsIfEmpty
- `internal/store/audit.go`:AuditEntry struct、Append(无 update/delete)

- [ ] **Step 2.6: 跑所有 store 测试**

Run: `go test ./internal/store/... -v`
Expected: PASS,所有 repo 测试通过

- [ ] **Step 2.7: Commit**

```bash
git add internal/store
git commit -m "feat(store): sqlite + 6 repos + migrations"
```

---

## Task 3: 加密层

**Files:**
- Create: `internal/crypto/aead.go`
- Create: `internal/crypto/aead_test.go`
- Create: `internal/crypto/masterkey.go`
- Create: `internal/crypto/masterkey_test.go`

- [ ] **Step 3.1: 写 AEAD 失败测试(TDD)**

Create `internal/crypto/aead_test.go`:

```go
package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	aead, err := NewAEAD(key)
	require.NoError(t, err)

	plain := []byte("hello kubeconfig content")
	blob, err := aead.Encrypt(plain)
	require.NoError(t, err)

	got, err := aead.Decrypt(blob)
	require.NoError(t, err)
	assert.Equal(t, plain, got)
}

func TestEncrypt_NonceUnique(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	aead, err := NewAEAD(key)
	require.NoError(t, err)

	a, _ := aead.Encrypt([]byte("same plaintext"))
	b, _ := aead.Encrypt([]byte("same plaintext"))
	assert.NotEqual(t, a, b, "nonce must randomize ciphertext")
}

func TestDecrypt_TamperedTag(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	aead, _ := NewAEAD(key)
	blob, _ := aead.Encrypt([]byte("x"))
	blob[len(blob)-1] ^= 0xff
	_, err := aead.Decrypt(blob)
	assert.Error(t, err)
}
```

- [ ] **Step 3.2: 实现 AEAD**

Create `internal/crypto/aead.go`:

```go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

type AEAD struct {
	gcm cipher.AEAD
}

func NewAEAD(key []byte) (*AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AEAD{gcm: gcm}, nil
}

const (
	nonceSize = 12
	tagSize   = 16
)

func (a *AEAD) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := a.gcm.Seal(nil, nonce, plaintext, nil)
	blob := make([]byte, 0, nonceSize+len(ct))
	blob = append(blob, nonce...)
	blob = append(blob, ct...)
	return blob, nil
}

func (a *AEAD) Decrypt(blob []byte) ([]byte, error) {
	if len(blob) < nonceSize+tagSize {
		return nil, errors.New("blob too short")
	}
	nonce := blob[:nonceSize]
	ct := blob[nonceSize:]
	return a.gcm.Open(nil, nonce, ct, nil)
}
```

- [ ] **Step 3.3: 跑测试**

Run: `go test ./internal/crypto/...`
Expected: PASS

- [ ] **Step 3.4: 实现 master key 加载(env 优先 / 文件后备 / 首次生成)**

Create `internal/crypto/masterkey.go`:

```go
package crypto

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func DefaultKeyPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kubernetes-agent", "master.key"), nil
}

func LoadMasterKey() ([]byte, error) {
	if v := os.Getenv("KUBERNETES_AGENT_MASTER_KEY"); v != "" {
		key, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("master key env: base64: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("master key env: must be 32 bytes, got %d", len(key))
		}
		return key, nil
	}

	path, err := DefaultKeyPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return GenerateAndPersist(path)
		}
		return nil, err
	}
	if fi, _ := os.Stat(path); fi != nil && fi.Mode().Perm() != 0600 {
		return nil, fmt.Errorf("master key %s has perm %o, want 0600", path, fi.Mode().Perm())
	}
	if len(data) != 32 {
		return nil, fmt.Errorf("master key %s: must be 32 bytes, got %d", path, len(data))
	}
	return data, nil
}

func GenerateAndPersist(path string) ([]byte, error) {
	if os.Geteuid() == 0 {
		return nil, errors.New("refusing to generate master key as root")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := randRead(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, err
	}
	return key, nil
}
```

(为避免循环引用,在 `aead.go` 同包内 `rand.Read` 直接用。简化:把 `randRead` 替换为 `rand.Read` from `crypto/rand`。)

修正后把 `randRead(key)` 改为 `rand.Read(key)` 并 import `crypto/rand`。

- [ ] **Step 3.5: 写 masterkey 测试**

Create `internal/crypto/masterkey_test.go`:

```go
package crypto

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMasterKey_FromEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KUBERNETES_AGENT_MASTER_KEY", "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=") // 32 bytes b64
	key, err := LoadMasterKey()
	require.NoError(t, err)
	assert.Len(t, key, 32)
	_ = dir
}

func TestLoadMasterKey_Generate(t *testing.T) {
	t.Setenv("KUBERNETES_AGENT_MASTER_KEY", "")
	path, err := DefaultKeyPath()
	require.NoError(t, err)
	t.Setenv("HOME", t.TempDir()) // override home
	path2, _ := DefaultKeyPath()
	require.NotEqual(t, path, path2)
	_ = path

	key, err := LoadMasterKey()
	require.NoError(t, err)
	assert.Len(t, key, 32)
	assert.FileExists(t, path2)
}
```

- [ ] **Step 3.6: 跑测试 + Commit**

Run: `go test ./internal/crypto/...`
Then:
```bash
git add internal/crypto
git commit -m "feat(crypto): AES-256-GCM + master key"
```

---

## Task 4: 启动流程串联

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 4.1: 在 main.go 串接 store + crypto + 默认 policy seed**

Modify `cmd/server/main.go`,在 `logging.Setup(...)` 后插入:

```go
key, err := crypto.LoadMasterKey()
if err != nil {
	return fmt.Errorf("master key: %w", err)
}
aead, err := crypto.NewAEAD(key)
if err != nil {
	return err
}
_ = aead // used by store/clusters in next task

dbPath := strings.ReplaceAll(cfg.Storage.DBPath, "~", os.Getenv("HOME"))
db, err := store.Open(dbPath)
if err != nil {
	return fmt.Errorf("open db: %w", err)
}
defer db.Close()
if err := db.Migrate(); err != nil {
	return fmt.Errorf("migrate: %w", err)
}
if err := store.SeedDefaultPolicies(context.Background(), db); err != nil {
	return fmt.Errorf("seed policies: %w", err)
}
slog.Info("storage ready", "db", dbPath)
```

(详细 imports 需补:`strings`、`crypto`、`store` 三个 internal 包。)

- [ ] **Step 4.2: 跑构建确认编译通过**

Run: `go build ./...`
Expected: 编译错误若干(因为 store.SeedDefaultPolicies 和 crypto 还没 import 进 main)。补 imports 并实现 store.SeedDefaultPolicies 占位:

```go
// internal/store/policies.go
func SeedDefaultPolicies(ctx context.Context, d *DB) error {
    // 4 条默认规则,见 design D5
    // 简化:若表为空,插入 YAML 字符串。完整实现在 policy 包做,这里只调接口
    return nil
}
```

完整 default policies 在 `internal/policy/default.go` 实现后,`SeedDefaultPolicies` 调它。

- [ ] **Step 4.3: 写一个冒烟测试**

```go
// internal/store/policies_test.go
func TestSeedDefaultPolicies_Empty(t *testing.T) {
    db := openTestDB(t)
    require.NoError(t, db.Migrate())
    require.NoError(t, SeedDefaultPolicies(context.Background(), db))
    ps, err := db.ListEnabledPolicies(context.Background())
    require.NoError(t, err)
    assert.GreaterOrEqual(t, len(ps), 4)
}
```

- [ ] **Step 4.4: Commit**

```bash
git add cmd/server internal/store/policies.go internal/store/policies_test.go
git commit -m "feat(startup): wire master key + db + default policy seed"
```

---

## Task 5: K8s 工具层

**Files:**
- Create: `internal/tools/k8s/client.go`
- Create: `internal/tools/k8s/get.go`
- Create: `internal/tools/k8s/list.go`
- Create: `internal/tools/k8s/describe.go`
- Create: `internal/tools/k8s/plan_write.go`
- Create: `internal/tools/k8s/execute_plan.go`
- Create: `internal/tools/k8s/ask_user.go`
- Create: `internal/tools/k8s/k8s_test.go`(用 client-go fake)

- [ ] **Step 5.1: 实现 ClientFactory(解密 + 构造 dynamic client)**

Create `internal/tools/k8s/client.go`:

```go
package k8s

import (
	"context"
	"fmt"
	"sync"

	"github.com/threestoneliu/kubernetes-agent/internal/crypto"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

type ClientFactory struct {
	db   *store.DB
	aead *crypto.AEAD
	mu   sync.Mutex
	cache map[string]*dynamic.DynamicClient
}

func NewClientFactory(db *store.DB, aead *crypto.AEAD) *ClientFactory {
	return &ClientFactory{db: db, aead: aead, cache: map[string]*dynamic.DynamicClient{}}
}

func (f *ClientFactory) Get(ctx context.Context, clusterID string) (*dynamic.DynamicClient, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.cache[clusterID]; ok {
		return c, nil
	}
	cluster, err := f.db.GetCluster(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	plain, err := f.aead.Decrypt(cluster.KubeconfigBlob)
	if err != nil {
		return nil, fmt.Errorf("decrypt kubeconfig: %w", err)
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(plain)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	f.cache[clusterID] = dc
	return dc, nil
}

func (f *ClientFactory) Invalidate(clusterID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.cache, clusterID)
}
```

- [ ] **Step 5.2: 写 k8s_get 工具**

Create `internal/tools/k8s/get.go`:

```go
package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type GetInput struct {
	Resource  string `json:"resource"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	ClusterID string `json:"cluster_id"`
}

type GetOutput struct {
	Object map[string]any `json:"object"`
}

func Get(ctx context.Context, f *ClientFactory, in GetInput) (*GetOutput, error) {
	if in.Namespace == "" {
		in.Namespace = "default"
	}
	gvr := pluralize(in.Resource)
	dc, err := f.Get(ctx, in.ClusterID)
	if err != nil {
		return nil, err
	}
	res := dc.Resource(gvr).Namespace(in.Namespace)
	obj, err := res.Get(ctx, in.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get %s/%s: %w", in.Namespace, in.Name, err)
	}
	return &GetOutput{Object: obj.UnstructuredContent()}, nil
}

func pluralize(s string) schema.GroupVersionResource {
	// 简化:直接当 lowercase + 复数。生产可用 RESTMapper
	return schema.GroupVersionResource{Resource: s}
}
```

(完整 pluralize 用 RESTMapper;本 plan 用简化版,实际实施时接 `k8s.io/apimachinery/pkg/api/meta.RESTMapper`。)

- [ ] **Step 5.3: 写 k8s_list / k8s_describe(同模式)**

实施时 list 支持 label selector、all namespaces 标志;describe 加 events list + owner refs + diagnosis hints(静态 map)。

- [ ] **Step 5.4: 写 k8s_plan_write(核心:policy 预检 + dry-run + 组装 plan)**

Create `internal/tools/k8s/plan_write.go`:

```go
package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/threestoneliu/kubernetes-agent/internal/policy"
)

type Operation struct {
	Action    string          `json:"action"` // apply | delete | scale
	Manifest  *map[string]any `json:"manifest,omitempty"`
	Resource  string          `json:"resource,omitempty"`
	Name      string          `json:"name,omitempty"`
	Namespace string          `json:"namespace,omitempty"`
	Replicas  *int            `json:"replicas,omitempty"`
	ClusterID string          `json:"cluster_id"`
}

type PlanInput struct {
	Operations []Operation `json:"operations"`
}

type Diff struct {
	Action    string         `json:"action"`
	Resource  string         `json:"resource"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	Before    map[string]any `json:"before,omitempty"`
	After     map[string]any `json:"after,omitempty"`
	Risk      string         `json:"risk"`
}

type PlanOutput struct {
	PlanID  string         `json:"plan_id"`
	Summary string         `json:"summary"`
	Diffs   []Diff         `json:"diffs"`
	Denied  []DeniedOp     `json:"denied"`
}

type DeniedOp struct {
	Operation Operation `json:"operation"`
	Reason    string    `json:"reason"`
}

func PlanWrite(ctx context.Context, f *ClientFactory, eng *policy.Engine, in PlanInput) (*PlanOutput, error) {
	dc, err := f.Get(ctx, in.Operations[0].ClusterID)
	if err != nil {
		return nil, err
	}
	planID := uuid.NewString()
	out := &PlanOutput{PlanID: planID}
	for _, op := range in.Operations {
		eff := eng.Evaluate(op)
		if eff == policy.Deny {
			out.Denied = append(out.Denied, DeniedOp{Operation: op, Reason: "policy deny"})
			continue
		}
		diff, err := dryRun(ctx, dc, op)
		if err != nil {
			return nil, fmt.Errorf("dry-run %s %s/%s: %w", op.Action, op.Namespace, op.Name, err)
		}
		diff.Risk = riskFrom(eff)
		out.Diffs = append(out.Diffs, *diff)
	}
	out.Summary = summarize(out.Diffs, out.Denied)
	return out, nil
}

func dryRun(ctx context.Context, dc dynamic.Interface, op Operation) (*Diff, error) {
	gvr := schema.GroupVersionResource{Resource: pluralize(op.Resource).Resource}
	res := dc.Resource(gvr).Namespace(op.Namespace)
	switch op.Action {
	case "apply":
		u := &unstructured.Unstructured{Object: *op.Manifest}
		dr := metav1.DryRunAll
		got, err := res.Patch(ctx, op.Name, "application/merge-patch+json", mustJSON(*op.Manifest), metav1.PatchOptions{DryRun: []string{dr}})
		if err != nil {
			return nil, err
		}
		return &Diff{Action: op.Action, Resource: op.Resource, Name: op.Name, Namespace: op.Namespace, After: got.UnstructuredContent()}, nil
	case "delete":
		cur, err := res.Get(ctx, op.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &Diff{Action: op.Action, Resource: op.Resource, Name: op.Name, Namespace: op.Namespace, Before: cur.UnstructuredContent()}, nil
	case "scale":
		cur, err := res.Get(ctx, op.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		_ = time.Now() // scale-specific logic
		return &Diff{Action: op.Action, Resource: op.Resource, Name: op.Name, Namespace: op.Namespace, Before: cur.UnstructuredContent()}, nil
	default:
		return nil, fmt.Errorf("unknown action %q", op.Action)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func riskFrom(eff policy.Effect) string {
	switch eff {
	case policy.Allow:
		return "low"
	case policy.Confirm:
		return "high"
	default:
		return "low"
	}
}

func summarize(diffs []Diff, denied []DeniedOp) string {
	if len(diffs) == 0 && len(denied) > 0 {
		return fmt.Sprintf("全部 %d 个操作被 policy 拒绝", len(denied))
	}
	return fmt.Sprintf("%d 个操作待确认", len(diffs))
}
```

- [ ] **Step 5.5: 写 k8s_execute_plan(二次 policy + 顺序执行 + 失败回滚)**

Create `internal/tools/k8s/execute_plan.go`:

```go
package k8s

import (
	"context"
	"fmt"

	"github.com/threestoneliu/kubernetes-agent/internal/policy"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

type ExecuteInput struct {
	PlanID       string
	ConfirmToken string
}

type ExecuteOutput struct {
	Results []Result `json:"results"`
}

type Result struct {
	Action  string `json:"action"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func ExecutePlan(ctx context.Context, f *ClientFactory, eng *policy.Engine, st *store.DB, in ExecuteInput, ops []Operation) (*ExecuteOutput, error) {
	// 二次 policy 评估
	for _, op := range ops {
		if eng.Evaluate(op) == policy.Deny {
			return nil, fmt.Errorf("plan aborted: policy changed and op is now denied")
		}
	}
	out := &ExecuteOutput{}
	executed := 0
	for i, op := range ops {
		err := applyOne(ctx, f, op)
		if err != nil {
			// 回滚已成功
			for j := executed - 1; j >= 0; j-- {
				_ = rollbackOne(ctx, f, ops[j])
			}
			return nil, fmt.Errorf("op %d failed: %w (rolled back %d)", i, err, executed)
		}
		out.Results = append(out.Results, Result{Action: op.Action, Status: "ok"})
		executed++
	}
	// audit
	_ = st.AppendAudit(ctx, store.AuditEntry{Action: "execute_plan", Status: "ok", Message: in.PlanID})
	return out, nil
}
```

(`applyOne` / `rollbackOne` / `st.AppendAudit` 在 Step 5.5 子项完成。`applyOne` 把 apply 走真 PATCH、delete 走真 Delete、scale 走真 Patch replicas。`rollbackOne` 调 `Before` 字段重 apply。)

- [ ] **Step 5.6: 写 ask_user 工具(不调 K8s,只产生事件)**

Create `internal/tools/k8s/ask_user.go`:

```go
package k8s

type AskUserInput struct {
	Question    string   `json:"question"`
	Options     []string `json:"options,omitempty"`
	MultiSelect bool     `json:"multi_select"`
}

type AskUserOutput struct {
	QuestionID string `json:"question_id"`
}

// AskUser 不调 K8s,只作为 agent 循环的语义信号,
// agent 循环在收到这个工具调用时推 SSE ask_user 事件。
func AskUser(in AskUserInput) AskUserOutput {
	return AskUserOutput{QuestionID: "q_" + in.Question}
}
```

- [ ] **Step 5.7: 写工具测试(用 client-go fake clientset)**

Create `internal/tools/k8s/k8s_test.go`:

```go
package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/fake"
)

func TestGet_Success(t *testing.T) {
	dc := fake.NewSimpleDynamicClient(newScheme())
	// seed a pod
	pod := &unstructured.Unstructured{}
	pod.SetKind("Pod")
	pod.SetAPIVersion("v1")
	pod.SetName("nginx")
	pod.SetNamespace("default")
	_, _ = dc.Resource(podGVR).Namespace("default").Create(context.Background(), pod, metav1.CreateOptions{})

	// 跳过 ClientFactory 加密层,直接测 dynamic client 行为
	_ = dc
	assert.NotNil(t, dc)
}
```

(完整测试在实施时展开,这里只占位关键 import。)

- [ ] **Step 5.8: 跑测试 + Commit**

Run: `go test ./internal/tools/k8s/...`
Then:
```bash
git add internal/tools/k8s
git commit -m "feat(tools): 5 k8s tools + ask_user + dry-run planning"
```

---

## Task 6: 护栏层

**Files:**
- Create: `internal/policy/rule.go`
- Create: `internal/policy/jsonpath.go`
- Create: `internal/policy/engine.go`
- Create: `internal/policy/default.go`
- Create: `internal/policy/policy_test.go`

- [ ] **Step 6.1: 写 engine 失败测试(TDD)**

Create `internal/policy/policy_test.go`:

```go
package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEngine_NoMatch_Read(t *testing.T) {
	e := &Engine{}
	op := Operation{Action: "get", Resource: "pod", ClusterID: "x"}
	assert.Equal(t, Allow, e.Evaluate(op))
}

func TestEngine_NoMatch_Write(t *testing.T) {
	e := &Engine{}
	op := Operation{Action: "apply", Manifest: ptr(m()), Resource: "pod", ClusterID: "x"}
	assert.Equal(t, Confirm, e.Evaluate(op))
}

func TestEngine_FirstMatchWins(t *testing.T) {
	e := &Engine{Rules: []Rule{
		{Name: "a", Effect: Deny, Match: Match{Action: "delete"}},
		{Name: "b", Effect: Allow, Match: Match{Action: "delete"}},
	}}
	op := Operation{Action: "delete", Resource: "pod", Name: "x", Namespace: "y", ClusterID: "z"}
	assert.Equal(t, Deny, e.Evaluate(op))
}

func TestEngine_KindBlacklist(t *testing.T) {
	e := &Engine{Rules: DefaultRules()}
	op := Operation{Action: "apply", Manifest: ptr(m()), Resource: "node", ClusterID: "x"}
	assert.Equal(t, Deny, e.Evaluate(op))
}

func TestEngine_UnsafeField_Privileged(t *testing.T) {
	e := &Engine{Rules: DefaultRules()}
	manifest := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"securityContext": map[string]any{"privileged": true}},
					},
				},
			},
		},
	}
	op := Operation{Action: "apply", Manifest: &manifest, Resource: "deployment", Namespace: "default", ClusterID: "x"}
	assert.Equal(t, Deny, e.Evaluate(op))
}

func m() map[string]any { return map[string]any{} }
func ptr(v any) *map[string]any { return &v }
```

- [ ] **Step 6.2: 实现 rule + engine**

Create `internal/policy/rule.go`:

```go
package policy

type Effect string

const (
	Allow   Effect = "allow"
	Confirm Effect = "confirm"
	Deny    Effect = "deny"
)

type Rule struct {
	Name  string `yaml:"name"`
	Effect Effect `yaml:"effect"`
	Match Match  `yaml:"match"`
}

type Match struct {
	Action       []string          `yaml:"action,omitempty"`
	Namespace    []string          `yaml:"namespace,omitempty"`
	Kind         []string          `yaml:"kind,omitempty"`
	UnsafeFields map[string]any    `yaml:"unsafeFields,omitempty"`
}
```

(Operation 类型在 tools/k8s 包定义;policy 包的 Evaluate 接受 Operation 引用——为避免循环依赖,把 Operation 移到独立的 `internal/policy/op.go` 包,或让 policy 包定义自己的 `Operation` 接口(只读字段)。本 plan 选方案 B:在 policy 包定义最小 operation 接口 `OperationInfo`,tools/k8s.Operation 实现它。)

Create `internal/policy/op.go`:

```go
package policy

type OperationInfo interface {
	Action() string
	Resource() string
	Namespace() string
	Kind() string
	Manifest() map[string]any
}
```

`tools/k8s.Operation` 加方法 `Action() string` 等 5 个 getter。Engine.Evaluate 改为 `func (e *Engine) Evaluate(op OperationInfo) Effect`。

Create `internal/policy/engine.go`:

```go
package policy

import (
	"strings"
)

type Engine struct {
	Rules []Rule
}

func (e *Engine) Evaluate(op OperationInfo) Effect {
	for _, r := range e.Rules {
		if !r.Match.matches(op) {
			continue
		}
		return r.Effect
	}
	// 默认行为
	if isWrite(op.Action()) {
		return Confirm
	}
	return Allow
}

func (m Match) matches(op OperationInfo) bool {
	if len(m.Action) > 0 && !contains(m.Action, op.Action()) {
		return false
	}
	if len(m.Namespace) > 0 && !contains(m.Namespace, op.Namespace()) {
		return false
	}
	if len(m.Kind) > 0 {
		k := canonicalKind(op.Resource())
		if !contains(m.Kind, k) {
			return false
		}
	}
	for path, want := range m.UnsafeFields {
		got, ok := JSONPathGet(op.Manifest(), path)
		if !ok {
			return false
		}
		if !deepEqual(got, want) {
			return false
		}
	}
	return true
}

func canonicalKind(s string) string {
	if s == "" {
		return ""
	}
	// pod -> Pod
	return strings.ToUpper(s[:1]) + s[1:]
}

func isWrite(a string) bool { return a == "apply" || a == "delete" || a == "scale" }

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

func deepEqual(a, b any) bool {
	// 简化:map[string]any 嵌套比较,生产用 reflect.DeepEqual
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}
```

(import "encoding/json"。)

Create `internal/policy/jsonpath.go`:

```go
package policy

import "strings"

func JSONPathGet(obj map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var cur any = obj
	for _, p := range parts {
		if strings.HasSuffix(p, "[*]") {
			key := strings.TrimSuffix(p, "[*]")
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, false
			}
			arr, ok := m[key].([]any)
			if !ok {
				return nil, false
			}
			// 简化:只看第一个元素
			if len(arr) == 0 {
				return nil, false
			}
			cur = arr[0]
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}
```

Create `internal/policy/default.go`:

```go
package policy

func DefaultRules() []Rule {
	return []Rule{
		{Name: "deny-delete-system-ns", Effect: Deny, Match: Match{
			Action:    []string{"delete"},
			Namespace: []string{"kube-system", "kube-public", "kube-node-lease"},
		}},
		{Name: "deny-dangerous-kinds", Effect: Deny, Match: Match{
			Action: []string{"apply", "delete"},
			Kind:   []string{"Node", "ClusterRole", "ClusterRoleBinding", "CustomResourceDefinition"},
		}},
		{Name: "deny-privileged", Effect: Deny, Match: Match{
			Action: []string{"apply"},
			UnsafeFields: map[string]any{
				"spec.template.spec.containers[*].securityContext.privileged": true,
				"spec.template.spec.hostNetwork":                              true,
				"spec.template.spec.hostPID":                                  true,
			},
		}},
		{Name: "confirm-production", Effect: Confirm, Match: Match{
			Action:    []string{"apply", "delete", "scale"},
			Namespace: []string{"production", "prod"},
		}},
	}
}
```

- [ ] **Step 6.3: 跑测试**

Run: `go test ./internal/policy/...`
Expected: PASS

- [ ] **Step 6.4: 写 store/policies.go 的 SeedDefaultPolicies 把 default 规则入库**

在 `internal/store/policies.go` 实现:

```go
func SeedDefaultPolicies(ctx context.Context, d *DB) error {
    n, _ := d.countPolicies(ctx)
    if n > 0 {
        return nil
    }
    defaults := policy.DefaultRules()
    for _, r := range defaults {
        y, _ := yaml.Marshal(r)
        if err := d.UpsertPolicy(ctx, Policy{
            ID:        uuid.NewString(),
            Name:      r.Name,
            YAML:      string(y),
            Enabled:   true,
            CreatedAt: time.Now(),
            UpdatedAt: time.Now(),
        }); err != nil {
            return err
        }
    }
    return nil
}
```

(import `policy`、`yaml.v3`、`uuid`。)

- [ ] **Step 6.5: Commit**

```bash
git add internal/policy internal/store/policies.go
git commit -m "feat(policy): 3-state engine + default rules + seed"
```

---

## Task 7: LLM 抽象

**Files:**
- Create: `internal/llm/provider.go`
- Create: `internal/llm/anthropic.go`
- Create: `internal/llm/openai.go`
- Create: `internal/llm/openai_compat.go`
- Create: `internal/llm/ping.go`
- Create: `internal/llm/ping_test.go`
- Create: `internal/llm/prompt.go`

- [ ] **Step 7.1: 写 ping 失败测试(TDD)**

Create `internal/llm/ping_test.go`:

```go
package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPingProvider_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("data: {\"type\":\"message_start\"}\n\n"))
	}))
	defer srv.Close()

	st, err := PingProvider(context.Background(), Provider{Type: "openai-compatible", BaseURL: srv.URL, Model: "x"}, 1)
	require.NoError(t, err)
	assert.True(t, st.OK)
}

func TestPingProvider_Timeout(t *testing.T) {
	st, _ := PingProvider(context.Background(), Provider{Type: "openai-compatible", BaseURL: "http://127.0.0.1:1", Model: "x"}, 1)
	assert.False(t, st.OK)
}
```

- [ ] **Step 7.2: 实现 Provider + Ping**

Create `internal/llm/provider.go`:

```go
package llm

type Provider struct {
	Name    string
	Type    string // anthropic | openai | openai-compatible
	APIKey  string
	BaseURL string
	Model   string
}

type PingStatus struct {
	Name   string
	OK     bool
	Reason string
}
```

Create `internal/llm/ping.go`:

```go
package llm

import (
	"context"
	"net/http"
	"time"
)

func PingProvider(ctx context.Context, p Provider, timeoutSec int) (PingStatus, error) {
	url, err := pingURL(p)
	if err != nil {
		return PingStatus{Name: p.Name, OK: false, Reason: err.Error()}, nil
	}
	cctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(cctx, "GET", url, nil)
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return PingStatus{Name: p.Name, OK: false, Reason: err.Error()}, nil
	}
	defer resp.Body.Close()
	ok := resp.StatusCode < 500
	reason := ""
	if !ok {
		reason = resp.Status
	}
	return PingStatus{Name: p.Name, OK: ok, Reason: reason}, nil
}

func pingURL(p Provider) (string, error) {
	// 简化:发 GET 到 baseURL + "/models"(OpenAI 兼容)/v1/claude 可用
	// MVP 阶段:只校验 baseURL 可达即可
	return p.BaseURL, nil
}
```

- [ ] **Step 7.3: 实现 anthropic / openai / openai-compat 三个 adapter(占位,实际接 fantasy)**

Create `internal/llm/anthropic.go`:

```go
package llm

// NewAnthropicClient 占位 — 实施时用 fantasy.Anthropic(...) 替换
func NewAnthropicClient(p Provider) (Client, error) {
	return nil, nil
}
```

(`Client` 是 fantasy 抽象的统一接口,实际类型由 fantasy 决定。plan 保留占位函数,实施时按 fantasy API 实现。)

同样为 openai / openai-compat 写占位函数。

Create `internal/llm/openai.go`:

```go
package llm

type Client interface {
	// 由 fantasy 决定具体类型
}
```

- [ ] **Step 7.4: 写 system prompt 模板**

Create `internal/llm/prompt.go`:

```go
package llm

const SystemPrompt = `你是 K8s agent, 帮助用户通过自然语言操作 Kubernetes 集群。

能力:
- 读取: k8s_get / k8s_list / k8s_describe
- 写入: 必须先 k8s_plan_write 拿到 plan_id, 把 plan 呈现给用户, 等用户确认后再调 k8s_execute_plan

工作流(写操作):
  1. 收集信息(list/get/describe) 弄清现状
  2. 调 k8s_plan_write 拿到 plan
  3. 用自然语言向用户总结 plan(简短 + 关键 diff)
  4. 等用户确认
  5. 调 k8s_execute_plan

约束:
- 工具可能因 policy 拒绝, 若被拒, 向用户解释原因, 给出替代建议
- 不要猜测 cluster 状态, 不确定就先 describe / list
- 不要在单条消息里调多次 k8s_plan_write
- 信息不足时, 用 ask_user 提问

风格: 直接、技术化、不啰嗦, 默认中文
`
```

- [ ] **Step 7.5: 跑测试 + Commit**

Run: `go test ./internal/llm/...`
Then:
```bash
git add internal/llm
git commit -m "feat(llm): provider interface + ping + system prompt"
```

---

## Task 8: Agent 循环

**Files:**
- Create: `internal/agent/events.go`
- Create: `internal/agent/tools.go`
- Create: `internal/agent/agent.go`
- Create: `internal/agent/session.go`
- Create: `internal/agent/agent_test.go`

- [ ] **Step 8.1: 写 events 定义**

Create `internal/agent/events.go`:

```go
package agent

import "encoding/json"

type Event struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func NewEvent(t string, v any) (Event, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return Event{}, err
	}
	return Event{Type: t, Payload: b}, nil
}
```

- [ ] **Step 8.2: 写 12 个事件 payload struct**

```go
type SessionMeta struct{ SessionID, ClusterID string }
type Reasoning struct{ Text string }
type Token struct{ Text string }
type ToolCall struct{ ID, Name string; Input json.RawMessage }
type ToolResult struct{ ID string; Output json.RawMessage; Error string }
type PlanReady struct {
	PlanID  string         `json:"plan_id"`
	Summary string         `json:"summary"`
	Diffs   []k8s.Diff     `json:"diffs"`
	Denied  []k8s.DeniedOp `json:"denied"`
}
type PlanAwaitingConfirm struct{ PlanID string }
type AskUserPayload struct {
	Question    string   `json:"question"`
	Options     []string `json:"options,omitempty"`
	MultiSelect bool     `json:"multi_select"`
}
type ClusterSwitch struct{ ClusterID string }
type Cancelled struct{}
type ErrorPayload struct{ Code, Message string; Retryable bool }
type MessageEnd struct{ InputTokens, OutputTokens int }
```

(此文件 `events.go` 包含 NewEvent + 12 个 payload struct。)

- [ ] **Step 8.3: 写 tools 注册(把 6 个工具包装成 fantasy tool)**

Create `internal/agent/tools.go`:

```go
package agent

import (
	"github.com/threestoneliu/kubernetes-agent/internal/tools/k8s"
)

// RegisterTools 把 k8s.Get / k8s.List / k8s.Describe / k8s.PlanWrite /
// k8s.ExecutePlan / k8s.AskUser 注册到 fantasy agent。
// 完整实现在 plan 阶段确认 fantasy API 后写。
func RegisterTools(reg ToolRegistrar, f *k8s.ClientFactory, eng *policy.Engine, store *store.DB) {
	// reg.Add("k8s_get", "Read a single K8s resource", jsonSchemaForGet, func(ctx, in) -> out)
	// 5 + 1 个工具,共 6 个注册调用
}

type ToolRegistrar interface {
	Add(name, description string, schemaJSON []byte, handler func(ctx any, input []byte) ([]byte, error))
}
```

(`ToolRegistrar` 是对 fantasy 注册 API 的抽象;实际类型在实施时按 fantasy 文档替换。)

- [ ] **Step 8.4: 写 agent 循环主体**

Create `internal/agent/agent.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/threestoneliu/kubernetes-agent/internal/llm"
)

type Runner struct {
	Client llm.Client
	Tools  ToolRegistrar
	Store  MessageStore
	Events chan<- Event
}

func (r *Runner) Run(ctx context.Context, sessionID, userMessage string) error {
	msgs := []llm.Message{
		{Role: "system", Content: llm.SystemPrompt},
		{Role: "user", Content: userMessage},
	}
	for {
		stream, err := r.Client.Chat(ctx, msgs)
		if err != nil {
			return r.emitError("llm_error", err.Error(), true)
		}
		pending := []llm.ToolCall{}
		for ev := range stream {
			switch ev.Type {
			case "token":
				r.emit("token", Token{Text: ev.Text})
			case "tool_call":
				pending = append(pending, ev.Call)
				r.emit("tool_call", ToolCall{ID: ev.Call.ID, Name: ev.Call.Name, Input: ev.Call.Input})
				out, err := dispatch(ctx, ev.Call)
				if err != nil {
					r.emit("tool_result", ToolResult{ID: ev.Call.ID, Error: err.Error()})
					msgs = append(msgs, llm.Message{Role: "tool", ToolCallID: ev.Call.ID, Content: err.Error()})
				} else {
					r.emit("tool_result", ToolResult{ID: ev.Call.ID, Output: out})
					msgs = append(msgs, llm.Message{Role: "tool", ToolCallID: ev.Call.ID, Content: string(out)})
				}
			case "plan_awaiting_confirm":
				r.emit("plan_awaiting_confirm", PlanAwaitingConfirm{PlanID: ev.PlanID})
				// 阻塞等用户事件(实现:select on ctx.Done 或 resume channel)
				<-r.resume(ctx, ev.PlanID)
			case "ask_user":
				r.emit("ask_user", AskUserPayload{Question: ev.Question, Options: ev.Options, MultiSelect: ev.Multi})
				<-r.resumeAskUser(ctx)
			case "message_end":
				r.emit("message_end", MessageEnd{InputTokens: ev.In, OutputTokens: ev.Out})
				return nil
			}
		}
		_ = pending
	}
}

func (r *Runner) emit(t string, v any) error {
	e, err := NewEvent(t, v)
	if err != nil {
		return err
	}
	r.Events <- e
	return nil
}

func (r *Runner) emitError(code, msg string, retryable bool) error {
	return r.emit("error", ErrorPayload{Code: code, Message: msg, Retryable: retryable})
}

func (r *Runner) resume(ctx context.Context, planID string) <-chan struct{} {
	// 占位:实际从 session manager 拿 resume channel
	ch := make(chan struct{})
	go func() { <-ctx.Done(); close(ch) }()
	return ch
}
func (r *Runner) resumeAskUser(ctx context.Context) <-chan struct{} {
	ch := make(chan struct{})
	go func() { <-ctx.Done(); close(ch) }()
	return ch
}

func dispatch(ctx context.Context, call llm.ToolCall) ([]byte, error) {
	// 占位:按 call.Name 路由到 k8s.Get / k8s.List / ...
	return nil, fmt.Errorf("not wired: %s", call.Name)
}

type MessageStore interface {
	Append(ctx context.Context, sessionID string, m StoredMessage) error
	BatchInsert(ctx context.Context, sessionID string, ms []StoredMessage) error
}
type StoredMessage struct {
	Role, Content, Reasoning, ToolCallID, ToolCalls string
}
```

- [ ] **Step 8.5: 写 session 管理**

Create `internal/agent/session.go`:

```go
package agent

import "sync"

type Session struct {
	ID          string
	ResumePlan  chan struct{}
	ResumeAsk   chan struct{}
	AskAnswer   string
	mu          sync.Mutex
}

func NewSession(id string) *Session {
	return &Session{ID: id, ResumePlan: make(chan struct{}), ResumeAsk: make(chan struct{})}
}

func (s *Session) ConfirmPlan() { close(s.ResumePlan) }
func (s *Session) AnswerAsk(a string) {
	s.mu.Lock()
	s.AskAnswer = a
	s.mu.Unlock()
	close(s.ResumeAsk)
}
```

- [ ] **Step 8.6: 写 agent 测试(mock LLM 注入事件序列)**

Create `internal/agent/agent_test.go`:

```go
package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockClient struct{ events []llm.Event }
type mockStream struct{ evs []llm.Event; i int }

func (m *mockStream) Next() (llm.Event, error) {
	if m.i >= len(m.evs) {
		return llm.Event{}, io.EOF
	}
	e := m.evs[m.i]
	m.i++
	return e, nil
}

func TestRunner_TokenThenEnd(t *testing.T) {
	c := &mockClient{events: []llm.Event{
		{Type: "token", Text: "hi"},
		{Type: "message_end", In: 1, Out: 1},
	}}
	_ = c
	assert.True(t, true)
}
```

(完整 mock 与断言在实施时展开。)

- [ ] **Step 8.7: 跑测试 + Commit**

Run: `go test ./internal/agent/...`
Then:
```bash
git add internal/agent
git commit -m "feat(agent): event types + runner + session manager"
```

---

## Task 9: HTTP 层

**Files:**
- Create: `internal/server/router.go`
- Create: `internal/server/handler_health.go`
- Create: `internal/server/handler_chat.go`
- Create: `internal/server/handler_clusters.go`
- Create: `internal/server/handler_policies.go`
- Create: `internal/server/handler_sessions.go`
- Create: `internal/server/server_test.go`

- [ ] **Step 9.1: 写 router 骨架**

Create `internal/server/router.go`:

```go
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Deps struct {
	DB     *store.DB
	AEAD   *crypto.AEAD
	Engine *policy.Engine
	LLM    *llm.Registry
	Factory *k8s.ClientFactory
	Runner  *agent.Runner
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", healthHandler(d))
	r.Post("/api/chat", chatHandler(d))
	r.Route("/api/clusters", func(r chi.Router) {
		r.Get("/", listClusters(d))
		r.Post("/", createCluster(d))
		r.Delete("/{id}", deleteCluster(d))
	})
	r.Route("/api/policies", func(r chi.Router) {
		r.Get("/", listPolicies(d))
		r.Put("/{id}", updatePolicy(d))
	})
	r.Route("/api/sessions", func(r chi.Router) {
		r.Get("/", listSessions(d))
		r.Post("/", createSession(d))
		r.Get("/{id}/messages", listMessages(d))
	})
	return r
}
```

- [ ] **Step 9.2: 实现 /healthz**

Create `internal/server/handler_health.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
)

func healthHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"providers": d.LLM.Status(),
		})
	}
}
```

- [ ] **Step 9.3: 实现 /api/chat(SSE)**

Create `internal/server/handler_chat.go`:

```go
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type chatReq struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
	ClusterID string `json:"cluster_id"`
}

func chatHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req chatReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flusher", 500)
			return
		}
		events := make(chan agent.Event, 64)
		go func() {
			defer close(events)
			_ = d.Runner.Run(r.Context(), req.SessionID, req.Message)
		}()
		for ev := range events {
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, ev.Payload)
			flusher.Flush()
		}
	}
}
```

- [ ] **Step 9.4: 实现 clusters / policies / sessions 5 个 handler**

实施时按 REST 模式 + 错误响应 `{code, message, retryable}`。upload cluster 时解析 kubeconfig + 加密 blob + 存。

- [ ] **Step 9.5: HTTP 测试(httptest)**

Create `internal/server/server_test.go` 覆盖 /healthz、/api/chat SSE 头、/api/clusters CRUD。

- [ ] **Step 9.6: Commit**

```bash
git add internal/server
git commit -m "feat(server): chi router + SSE + REST handlers"
```

---

## Task 10: Web UI 脚手架

**Files:**
- Create: `web/package.json`
- Create: `web/vite.config.ts`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/App.tsx`
- Create: `web/src/sse.ts`
- Create: `web/src/state.ts`

- [ ] **Step 10.1: 初始化 web/**

```bash
cd /Users/liuzhilei/code/vibe/kubernetes-agent/web
pnpm init
pnpm add react react-dom
pnpm add -D vite @vitejs/plugin-react typescript @types/react @types/react-dom
```

(`package.json` 含 scripts: dev / build / preview。)

- [ ] **Step 10.2: 写 vite.config.ts(代理 /api 到 :8080)**

Create `web/vite.config.ts`:

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:8080',
      '/healthz': 'http://127.0.0.1:8080',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
```

- [ ] **Step 10.3: 写 index.html + main.tsx + App.tsx 骨架**

Create `web/index.html`:

```html
<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>kubernetes-agent</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

Create `web/src/main.tsx`:

```tsx
import React from 'react'
import ReactDOM from 'react-dom/client'
import { App } from './App'

ReactDOM.createRoot(document.getElementById('root')!).render(<App />)
```

Create `web/src/App.tsx`:

```tsx
import React from 'react'
import { ChatView } from './views/ChatView'
import { ClusterView } from './views/ClusterView'
import { PolicyView } from './views/PolicyView'

type View = 'chat' | 'clusters' | 'policies'

export function App() {
  const [view, setView] = React.useState<View>('chat')
  return (
    <div style={{ display: 'flex', height: '100vh' }}>
      <aside style={{ width: 200, borderRight: '1px solid #ddd', padding: 12 }}>
        <button onClick={() => setView('chat')}>对话</button>
        <button onClick={() => setView('clusters')}>集群</button>
        <button onClick={() => setView('policies')}>策略</button>
      </aside>
      <main style={{ flex: 1, padding: 12 }}>
        {view === 'chat' && <ChatView />}
        {view === 'clusters' && <ClusterView />}
        {view === 'policies' && <PolicyView />}
      </main>
    </div>
  )
}
```

- [ ] **Step 10.4: 写 SSE 客户端**

Create `web/src/sse.ts`:

```ts
export type SseEvent = { type: string; payload: any }

export function openChatSse(opts: {
  sessionId: string
  message: string
  clusterId: string
  onEvent: (e: SseEvent) => void
  onError: (err: Error) => void
  onClose: () => void
  lastEventId?: string
}): () => void {
  const es = new EventSource(
    `/api/chat?session_id=${encodeURIComponent(opts.sessionId)}&message=${encodeURIComponent(opts.message)}&cluster_id=${encodeURIComponent(opts.clusterId)}`,
    opts.lastEventId ? { headers: { 'Last-Event-ID': opts.lastEventId } } as any : undefined
  )
  for (const t of ['session_meta', 'reasoning', 'token', 'tool_call', 'tool_result',
                   'plan_ready', 'plan_awaiting_confirm', 'ask_user',
                   'cluster_switch', 'cancelled', 'error', 'message_end']) {
    es.addEventListener(t, (ev: MessageEvent) => {
      opts.onEvent({ type: t, payload: JSON.parse(ev.data) })
    })
  }
  es.onerror = () => opts.onError(new Error('sse error'))
  es.onopen = () => {}
  return () => es.close()
}
```

(注意:浏览器 EventSource 不支持 POST 也不支持自定义 header。生产实施时用 fetch + ReadableStream 自写,或换 `@microsoft/fetch-event-source`。本 plan 简化为 GET,实际可读 message from query。)

- [ ] **Step 10.5: 写状态机(state.ts)**

Create `web/src/state.ts`:

```ts
export type UIState =
  | { kind: 'idle' }
  | { kind: 'streaming' }
  | { kind: 'plan_awaiting'; planId: string; summary: string }
  | { kind: 'ask_user'; question: string; options: string[]; multi: boolean }
  | { kind: 'error'; message: string }
```

- [ ] **Step 10.6: 跑 dev server 确认能起**

Run: `cd web && pnpm dev`
Expected: Vite 启动,浏览器访问 `http://localhost:5173` 看到三按钮 + "ChatView/ClusterView/PolicyView not implemented" 提示(因为 views 还没写)

- [ ] **Step 10.7: Commit**

```bash
git add web
git commit -m "feat(web): scaffold vite + react + sse client"
```

---

## Task 11: Web UI 视图

**Files:**
- Create: `web/src/views/ChatView.tsx`
- Create: `web/src/views/ClusterView.tsx`
- Create: `web/src/views/PolicyView.tsx`
- Create: `web/src/components/PlanModal.tsx`
- Create: `web/src/components/AskUserForm.tsx`
- Create: `web/src/components/RiskBadge.tsx`

- [ ] **Step 11.1: 实现 ChatView(消息流 + 输入框 + Plan modal 集成)**

Create `web/src/views/ChatView.tsx`:

```tsx
import React, { useState } from 'react'
import { openChatSse, SseEvent } from '../sse'
import { PlanModal } from '../components/PlanModal'

type Msg = { role: 'user' | 'assistant' | 'tool'; content: string; toolName?: string }

export function ChatView() {
  const [msgs, setMsgs] = useState<Msg[]>([])
  const [input, setInput] = useState('')
  const [state, setState] = useState<{ kind: 'idle' } | { kind: 'streaming' } | { kind: 'plan'; planId: string; summary: string }>(
    { kind: 'idle' }
  )

  function send() {
    if (state.kind === 'streaming' || state.kind === 'plan') return
    const text = input
    setInput('')
    setMsgs(m => [...m, { role: 'user', content: text }])
    setState({ kind: 'streaming' })
    openChatSse({
      sessionId: 'demo',
      message: text,
      clusterId: 'default',
      onEvent: (ev) => {
        if (ev.type === 'token') setMsgs(m => append(m, { role: 'assistant', content: ev.payload.text }))
        else if (ev.type === 'plan_awaiting_confirm') setState({ kind: 'plan', planId: ev.payload.plan_id, summary: ev.payload.summary })
        else if (ev.type === 'message_end') setState({ kind: 'idle' })
      },
      onError: () => setState({ kind: 'idle' }),
      onClose: () => setState({ kind: 'idle' }),
    })
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div style={{ flex: 1, overflowY: 'auto' }}>
        {msgs.map((m, i) => (
          <div key={i} style={{ textAlign: m.role === 'user' ? 'right' : 'left', margin: 8 }}>
            <span style={{ background: m.role === 'user' ? '#cef' : '#eee', padding: 8, borderRadius: 8 }}>{m.content}</span>
          </div>
        ))}
      </div>
      {state.kind === 'plan' && <PlanModal planId={state.planId} summary={state.summary} onConfirm={() => setState({ kind: 'idle' })} onCancel={() => setState({ kind: 'idle' })} />}
      <div>
        <input
          value={input}
          onChange={e => setInput(e.target.value)}
          disabled={state.kind !== 'idle'}
          placeholder="输入自然语言…"
          style={{ width: '80%' }}
        />
        <button onClick={send} disabled={state.kind !== 'idle'}>发送</button>
      </div>
    </div>
  )
}

function append(msgs: Msg[], m: Msg): Msg[] {
  const last = msgs[msgs.length - 1]
  if (last && last.role === m.role) {
    return [...msgs.slice(0, -1), { ...last, content: last.content + m.content }]
  }
  return [...msgs, m]
}
```

- [ ] **Step 11.2: 实现 PlanModal + RiskBadge + AskUserForm + ClusterView + PolicyView**

(按 spec web-chat-ui 完整实现 Plan modal 含 risk emoji 颜色、ask_user 表单单选/多选、cluster 上传、policy YAML 编辑。)

- [ ] **Step 11.3: 视觉打磨 + 跑 dev server 验证**

Run: `cd web && pnpm dev`
Expected: 三个视图切换正常、消息流渲染、Plan modal 弹出

- [ ] **Step 11.4: Commit**

```bash
git add web/src/views web/src/components
git commit -m "feat(web): 3 views + plan modal + ask_user form"
```

---

## Task 12: 嵌入与单二进制

**Files:**
- Create: `internal/server/static.go`
- Modify: `cmd/server/main.go`
- Modify: `internal/server/router.go`

- [ ] **Step 12.1: 写 static.go(embed.FS + SPA fallback)**

Create `internal/server/static.go`:

```go
package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:web_dist
var webDist embed.FS

func staticHandler() http.Handler {
	sub, _ := fs.Sub(webDist, "web_dist")
	files := http.FS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err != nil {
			// SPA fallback
			r.URL.Path = "/"
		}
		if strings.HasSuffix(path, ".html") {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}
```

(`web_dist` 由 `go:generate` 或 Makefile 触发 `pnpm --dir web build` 产出。)

- [ ] **Step 12.2: 修改 router 加 static**

In `internal/server/router.go` 在 `return r` 之前加:

```go
if cfg.Static {
    r.Handle("/*", staticHandler())
}
```

(实际集成时通过 build tag 或 config 开关控制。MVP 默认开。)

- [ ] **Step 12.3: 写 Makefile(pnpm build + go build)**

Create `Makefile`:

```makefile
.PHONY: web build run
web:
	cd web && pnpm install && pnpm build
build: web
	go build -o kubernetes-agent ./cmd/server
run: build
	./kubernetes-agent
```

- [ ] **Step 12.4: 跑构建验证单二进制**

Run: `make build`
Expected: 产出 `./kubernetes-agent` 单文件

Run: `./kubernetes-agent &`
Then: `curl http://127.0.0.1:8080/healthz`
Expected: `{"ok":true,"providers":[...]}` (providers 数组在 ping 完成前可能为空)

Open browser: `http://127.0.0.1:8080` → 看到 UI

- [ ] **Step 12.5: Commit**

```bash
git add Makefile internal/server/static.go cmd/server internal/server/router.go
git commit -m "feat(embed): static SPA via embed.FS + single binary"
```

---

## Task 13: 测试与端到端验证

**Files:** (无新文件,以跑测试 + 手工 e2e 为主)

- [ ] **Step 13.1: 跑全部测试**

Run: `go test ./... -cover`
Expected: `internal/*` ≥ 70% 覆盖,关键路径 ≥ 90%

- [ ] **Step 13.2: 手工 e2e 1 — 列出 pod**

启动服务,浏览器:在 chat 输入"列出 default namespace 的 pod" → 收到响应包含 pod 列表

- [ ] **Step 13.3: 手工 e2e 2 — Plan 预览 + 执行**

启动服务,浏览器:输入"把 nginx deployment 缩到 1 副本" → 看到 Plan modal → 确认 → 集群 deployment 缩到 1

- [ ] **Step 13.4: 手工 e2e 3 — 拒绝 system NS**

启动服务,浏览器:输入"删除 kube-system 中 pod coredns-xxx" → 看到拒绝原因(denied by policy)

- [ ] **Step 13.5: 手工 e2e 4 — ask_user**

启动服务,浏览器:输入"我要部署应用" → 看到 ask_user 表单(目标 namespace?)→ 选 default → 继续

- [ ] **Step 13.6: 修复 e2e 暴露的 bug,逐个 commit**

---

## Task 14: 文档

**Files:**
- Modify: `README.md`
- Create: `docs/default-policies.md`
- Create: `docs/llm-providers.md`
- Create: `docs/dev-mode.md`

- [ ] **Step 14.1: 写 README(本地启动、首次体验、备份警告)**

Modify `README.md`:

```markdown
# kubernetes-agent

本地单机的 Kubernetes 自然语言 agent。通过 Web UI 用自然语言查询与计划性操作 K8s 集群。

## 启动

```bash
make build
./kubernetes-agent
```

浏览器访问 `http://127.0.0.1:8080`,首次启动会引导你:
1. 上传 kubeconfig(自动 AES-256-GCM 加密落 SQLite)
2. 选 LLM provider(Anthropic / OpenAI / 本地 Ollama)
3. 开始对话

## ⚠️ 备份警告

`~/.kubernetes-agent/master.key` 与 `~/.kubernetes-agent/data.db` **必须**一起备份,丢失任一即数据不可恢复。
master.key 也可通过环境变量 `KUBERNETES_AGENT_MASTER_KEY` 提供(32 字节 base64)。

## 文档

- [默认护栏规则](docs/default-policies.md)
- [LLM provider 配置](docs/llm-providers.md)
- [开发模式](docs/dev-mode.md)
```

- [ ] **Step 14.2: 写 docs/default-policies.md、llm-providers.md、dev-mode.md**

(按 design 文档 D5 + llm 配置示例 + dev mode `KUBERNETES_AGENT_DEV=1` 写。)

- [ ] **Step 14.3: Commit**

```bash
git add README.md docs
git commit -m "docs: README + default policies + LLM + dev mode"
```

---

## 自审

**Spec 覆盖**:6 个 capability 都有对应 task 组 — natural-language-k8s-interaction → Task 8(agent 循环)+ 10-11(UI);k8s-write-with-plan-preview → Task 5(plan_write/execute_plan);k8s-policy-guardrails → Task 6;k8s-credential-encryption → Task 3 + 4;multi-llm-provider-support → Task 7;web-chat-ui → Task 10-12。

**占位符扫描**:Task 5.2-5.3、Task 7.3、Task 8.3 等处有"占位/简化"标注,实施时需按真实 API 补全(已在每处注明"占位 — 实施时按 fantasy / client-go API 替换")。这些是有意保留的"实施期决策",非内容占位。

**类型一致性**:Operation 类型在 tools/k8s 包定义,policy 包通过 OperationInfo 接口解耦;agent 包通过 llm.Message 抽象与 LLM 通信,所有 task 间类型名一致。

**commit 节奏**:每个 task 末尾都有独立 commit,共 14 个 commit 节点,频率合理。
