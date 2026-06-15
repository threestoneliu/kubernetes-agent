# 默认护栏规则

## 概述

启动时通过 `store.SeedDefaultPolicies` 插入 4 条规则。**只在 policies 表为空时**插入，因此对默认规则的任何修改（包括禁用、改名、新增）都不会被覆盖；想恢复出厂值只能手动清空 policies 表。

规则由 `internal/policy/default.go` 中的 `DefaultRules()` 返回，引擎按 `internal/policy/engine.go` 中定义的顺序逐条 first-match 求值。

## 4 条规则

### 1. `deny-delete-system-ns`

| 字段 | 值 |
|------|-----|
| Effect | `deny` |
| Match.action | `delete` |
| Match.namespace | `kube-system`、`kube-public`、`kube-node-lease` |

**作用**：阻止在系统命名空间里删除任何资源。
**怎么改**：把 `Match.namespace` 改成你公司实际禁止的系统命名空间列表；如要放行某条，加一条 Effect 为 `allow` 且更具体的规则在它前面。

### 2. `deny-dangerous-kinds`

| 字段 | 值 |
|------|-----|
| Effect | `deny` |
| Match.action | `apply`、`delete` |
| Match.kind | `Node`、`ClusterRole`、`ClusterRoleBinding`、`CustomResourceDefinition` |

**作用**：禁止对集群级资源做写操作。`Match.kind` 匹配大小写不敏感（`Node` / `node` 等价）。
**怎么改**：在它前面加一条 `Effect: allow` 的规则并把 `kind` 限定到你需要放行的子集。

### 3. `deny-privileged`

| 字段 | 值 |
|------|-----|
| Effect | `deny` |
| Match.action | `apply` |
| Match.unsafeFields | 见下 |

`Match.unsafeFields` 是一组简化 JSONPath → 期望值的映射，**任一字段命中**就触发拒绝：

| JSONPath | 期望值 |
|----------|--------|
| `spec.template.spec.containers[*].securityContext.privileged` | `true` |
| `spec.template.spec.hostNetwork` | `true` |
| `spec.template.spec.hostPID` | `true` |

**作用**：拒绝任何让 Pod 提权或共享节点命名空间的 manifest。
**怎么改**：删掉某条目即可放行对应的提权向量；加新条目则引入新的拦截条件。

### 4. `confirm-production`

| 字段 | 值 |
|------|-----|
| Effect | `confirm` |
| Match.action | `apply`、`delete`、`scale` |
| Match.namespace | `production`、`prod` |

**作用**：不直接拒绝，而是要求用户在 Plan 预览里点确认才会真正执行。
**怎么改**：想换成「拒绝」把 `Effect` 改成 `deny`；想加更多命名空间就在 `Match.namespace` 里加。

## 如何自定义

三种入口，效果一致：

1. **Web UI**：打开 "策略" 视图（`/` → 顶部导航），可编辑 YAML 或一键启停。
2. **REST**：
   - `GET /api/policies` — 列出全部
   - `PUT /api/policies/{id}` — 全量替换一条规则的 YAML
   - `PATCH /api/policies/{id}/enabled` — 临时禁用 / 启用（不删规则）
3. **数据库**：直接改 `~/.kubernetes-agent/data.db` 里的 `policies` 表，**不推荐**——格式耦合内部 struct。

## Match 字段说明

```yaml
match:
  action:       [apply, delete, scale]   # 可选；空表示所有 action
  namespace:    [production, prod]        # 可选；空表示所有 namespace
  kind:         [Deployment, Node]       # 可选；空表示所有 kind
  unsafeFields:                          # 可选；简化 JSONPath → 期望值
    spec.template.spec.hostNetwork: true
```

| 字段 | 类型 | 例 |
|------|------|-----|
| `action` | `[]string` | 拦截所有 `delete`：`action: [delete]` |
| `namespace` | `[]string` | 只对生产命名空间生效：`namespace: [production, prod]` |
| `kind` | `[]string`，大小写不敏感 | 拦截 CRD：`kind: [CustomResourceDefinition]` |
| `unsafeFields` | `map[string]any` | 拒绝 hostNetwork：`unsafeFields: { "spec.template.spec.hostNetwork": true }` |

**求值顺序**：引擎按 `DefaultRules()` 顺序（也就是 `policies` 表里的 `position`）从头遍历，**first-match wins**。匹配过程中各字段是 AND 关系；任一字段不匹配就跳过该规则。

**无匹配时**：

- 读操作（`get` / `list` / `describe`）→ `allow`
- 写操作（`apply` / `delete` / `scale`）→ `confirm`

这个回退写=confirm 的策略是有意为之：默认让所有写操作进入 Plan 预览，由用户明确放行。

## JSONPath 简化语法

`unsafeFields` 的 key 用的是项目自实现的简化 JSONPath（见 `internal/policy/jsonpath.go`），**不**是完整 RFC 9535：

| 写法 | 含义 |
|------|------|
| `a.b.c` | 嵌套进入对象 |
| `a[*].b` | 数组通配；取第一个元素后继续 |
| 不支持 | 过滤器 `[?(@.x=='y')]` |
| 不支持 | 函数 `length()` / `keys()` 等 |
| 不支持 | 负索引 `[1]`、切片 `[0:2]`、多索引 `[0,2]` |

值匹配用 JSON 深相等（`true` ≠ `"true"`、数字与字符串不会互相转换）。

## 下一步

完整设计决策（三态语义 / first-match / unsafeFields 设计动机）见
[design.md → D5 护栏三态](openspec/changes/k8s-natural-language-agent/design.md#d5-护栏三态allow--confirm--deny--go-层强制)。
