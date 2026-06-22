# Plan: k8s_list 表格格式化

## 1. 扩展 ClientFactory 接口

**文件**: `internal/tools/k8s/client.go`

给 `ClientFactory` 接口添加：
```go
RESTConfig(clusterID string) (*rest.Config, error)
```

实现 `KubeconfigClientFactory.RESTConfig` 返回缓存的 RESTConfig。

---

## 2. 新增 Table 请求方法

**文件**: `internal/tools/k8s/list.go`

新增 `ListTable` 函数，使用原生 Table format：

```go
func ListTable(ctx context.Context, f ClientFactory, in ListInput) (*TableOutput, error) {
    cfg, err := f.RESTConfig(in.ClusterID)
    // 构造 URL: /api/v1 或 /apis/{group}/{version}/namespaces/{ns}/{resource}
    // 设置 Header: Accept = "application/json;as=Table;v=v1;g=meta.k8s.io"
    // GET 请求，URL 参数: ?labelSelector=...
    // 解析响应: metav1.Table
    // 返回 TableOutput{Columns: [...], Rows: [[...], ...]}
}
```

---

## 3. 改造 List 工具返回表格

**文件**: `internal/agent/tools.go`

将 `k8s_list` 工具的 handler 从返回原始 JSON 改为返回 `TableOutput`。这样 LLM 收到的是格式化表格数据，便于生成自然语言摘要。

---

## 4. 依赖

- `k8s.io/client-go/rest` (已间接引入)
- `k8s.io/apimachinery/pkg/apis/meta/v1` (Table 类型)
