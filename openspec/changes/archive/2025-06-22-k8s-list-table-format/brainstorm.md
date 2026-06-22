## Design Summary

修改 `k8s_list` 工具返回类似 `kubectl get` 的表格格式。核心思路：请求 Kubernetes API 时使用 `meta.k8s.io` Table format，API server 原生返回表格结构数据（含列定义和行），直接透传给调用方。

## Agreed Approach

**方案：使用原生 Table format（meta.k8s.io/v1）**

请求时设置 `Accept: application/json;as=Table;v=v1;g=meta.k8s.io`，API server 直接返回 Table 格式数据。

返回结构：
```go
type TableOutput struct {
    Columns []string   `json:"columns"`  // 列头
    Rows    [][]string `json:"rows"`     // 每行数据
}
```

实现：在 `tools/k8s/list.go` 中新增 `ListTable` 函数，改造 `List` 使其返回表格数据。JSON tags 保持 `json:"columns"` / `json:"rows"`。

## Key Decisions

1. **不维护硬编码列定义表** — kubectl 官方列定义在 API server 内，原生支持最准确
2. **返回结构化数据而非纯文本** — 前端可渲染 `<table>` 或转纯文本
3. **表格格式化逻辑在 agent 层** — LLM 更适合生成自然语言摘要，表格直接透传

## Open Questions

无
