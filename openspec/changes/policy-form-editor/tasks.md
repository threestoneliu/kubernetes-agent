## 1. TagInput 子组件

- [x] 1.1 创建 `TagInput` React 组件，支持 `string[]` value、`onChange`、占位符文本
- [x] 1.2 组件内文本框输入后按 Enter 追加标签，显示 `×` 删除按钮
- [x] 1.3 追加时去重、空值过滤

## 2. YAML <-> Form 序列化函数

- [x] 2.1 实现 `serializeFormToYaml(form: PolicyForm): string` — 将表单 Model 序列化为合规 YAML 字符串
- [x] 2.2 实现 `parseYamlToForm(yaml: string): PolicyForm | null` — 解析 YAML 为表单 Model，解析失败返回 null
- [x] 2.3 验证 `serializeFormToYaml(parseYamlToForm(x)) === x` 往返一致性（用 jest 或手工验证）

## 3. PolicyFormModal 组件

- [x] 3.1 搭建左右分栏布局（CSS `display: flex`，左 60% 右 40%，各带标题）
- [x] 3.2 实现 `name` 文本框和 `effect` 下拉选择
- [x] 3.3 实现 `action` 三复选框（apply/delete/scale）+ 保存按钮 disabled 校验
- [x] 3.4 集成 `TagInput` 组件实现 `namespace` 和 `kind` 标签输入
- [x] 3.5 实现 `unsafeFields` 多行文本框
- [x] 3.6 表单 onChange → 实时序列化到右侧 YAML 面板（无 debounce）
- [x] 3.7 YAML 面板 onChange → debounce 300ms → parseYamlToForm → 更新表单（失败时右侧红色边框）
- [x] 3.8 新建模式（`policy=null`）初始化默认空表单；编辑模式（`policy!=null`）从 `policy.yaml` 解析初始化
- [x] 3.9 保存时全量校验（name 非空 + action 至少一个 + YAML 无解析错误），API 调用后关闭 Modal

## 4. PolicyView 集成

- [x] 4.1 替换 `CreatePolicyModal` 为 `PolicyFormModal`（传入 `policy=null`）
- [x] 4.2 替换行内"编辑 YAML"按钮，改为调用 `PolicyFormModal`（传入当前 policy）
- [x] 4.3 移除列表中的行内 textarea YAML 编辑器
- [x] 4.4 删除 `CreatePolicyModal` 组件和相关的 `yaml` state
