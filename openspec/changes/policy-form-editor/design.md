# Policy Form Editor — Design

## Overview

将策略编辑页面从纯 YAML 改为结构化表单 + YAML 混排模式。表单为主（左侧 60%），YAML 为辅（右侧 40%），实时双向同步。新建和编辑共用同一 Modal 组件。

## Component Design

### PolicyFormModal

```
Props:
  policy: Policy | null   // null = 新建模式
  onClose: () => void
  onCreated/OnSaved: () => void
  show: (msg: string) => void
```

**内部 State（表单 Model）：**
```typescript
interface PolicyForm {
  name: string
  effect: 'allow' | 'confirm' | 'deny'
  action: { apply: boolean; delete: boolean; scale: boolean }
  namespace: string[]      // 标签列表
  kind: string[]           // 标签列表
  unsafeFields: string     // 原始文本（YAML/JSON 格式）
  yamlText: string         // 右侧 YAML 编辑器内容
  yamlError: boolean       // YAML 解析是否失败
}
```

**布局（左右各半）：**
```
+--[新建策略 / 编辑策略]----------------------------+
|  表单区 (60%)          |  YAML 区 (40%)          |
|  name: [________]      |  name: xxx              |
|  effect: [v]           |  effect: xxx            |
|  action: [x] apply    |  match:                 |
|         [ ] delete    |      action: [...]      |
|         [x] scale     |      namespace: [...]   |
|  namespace:            |      kind: [...]       |
|  [input] +            |      unsafeFields:      |
|  [x kube-prod ]      |        {...}            |
|  [x staging    ]      |                        |
|  kind:                 |                        |
|  [input] +            |                        |
|  [x Pod        ]      |                        |
|  unsafeFields:        |                        |
|  [textarea...  ]      |                        |
|                        |                        |
|  [取消] [保存]         |                        |
+-------------------------------------------------+
```

## Form Fields

| 字段 | 控件类型 | 备注 |
|------|---------|------|
| `name` | 文本框 | 最长 64 字符 |
| `effect` | 下拉选择 | allow / confirm / deny |
| `action` | 复选框组 | apply / delete / scale，至少选一个 |
| `namespace` | 标签输入器 | 回车追加，× 删除，支持自由输入 |
| `kind` | 标签输入器 | 同上 |
| `unsafeFields` | 文本框 | 保留原始输入，不做解析 |

### 标签输入器交互

- 文本框输入后按 **回车** 或点击 **+** 按钮追加为标签
- 标签右侧有 **×** 按钮可删除
- 空标签不允许追加
- 重复标签不允许追加（输入时忽略已存在的值）

## Bidirectional Sync

### 表单 → YAML

每次表单字段变动，立即序列化为 YAML：
- 使用本地 `yaml.Marshal()` 或手写拼接
- 结果写入右侧 YAML 文本框
- `yamlError = false`

### YAML → 表单

右侧 YAML 文本框 `onChange` 时，延迟 ~300ms 后尝试解析：
- 解析成功 → 更新表单各字段，`yamlError = false`
- 解析失败 → `yamlError = true`，右侧边框变红色（`border: 1px solid #d32f2f`），左侧表单**保持不变**（不丢失用户编辑）

### 防抖

YAML → 表单同步加 300ms debounce，避免每次按键触发重解析。

## Behavior

### 新建模式（`policy == null`）

- 初始化表单为默认空值：
  ```
  name = ''
  effect = 'deny'
  action = { apply: true, delete: false, scale: false }
  namespace = []
  kind = []
  unsafeFields = ''
  ```
- YAML 侧显示对应的空模板

### 编辑模式（`policy != null`）

- 打开时从 `policy.yaml` 解析出各字段值
- 解析失败时：表单显示默认值，YAML 侧边框红色高亮
- 用户保存时：序列化表单为 YAML，调用 `updatePolicy(id, yaml)`

### 保存校验

- `name` 为空 → 阻止提交，input 红色边框
- `action` 全为 false → 阻止提交，label 红色提示
- YAML 解析失败 → 阻止提交

### 取消

- 关闭 Modal，表单状态丢弃（不脏检）

## Files to Modify

- `web/src/views/PolicyView.tsx` — 替换行内编辑和新 Modal 逻辑
- `web/src/api.ts` — 无需改动（API 不变）

## Implementation Order

1. 实现 `PolicyFormModal` 组件（内部 State + 左右布局）
2. 实现 `serializeToYaml(form)` 函数（表单 → YAML）
3. 实现 `parseYamlToForm(yaml)` 函数（YAML → 表单）
4. 实现双向 `onChange` + debounce
5. 实现标签输入器 `TagInput` 子组件
6. 替换 `PolicyView` 中的 `CreatePolicyModal` 和行内编辑为 `PolicyFormModal`
7. 调整列表布局，删除行内 YAML 编辑

## Testing

- 新建策略：表单填写 → 保存 → 列表显示正确
- 编辑策略：点击编辑 → 表单预填 → 修改 → 保存 → 列表更新
- YAML 手动破坏 → 边框红色，表单不变
- YAML 修复 → 边框恢复，表单同步
- 切换到其他 session/页面 → Modal 关闭，状态丢弃
