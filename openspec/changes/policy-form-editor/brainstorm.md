# Policy Form Editor — Brainstorm

## Design Summary

将策略编辑页面从纯 YAML 改为**结构化表单 + YAML 混排模式**。表单为主（左侧 60%），YAML 为辅（右侧 40%），实时双向同步。

## Alternatives Considered

### 方案 A：表单为主，YAML 为辅（采用）

- 表单占左 60%，YAML 占右 40%
- 表单变动 → 实时序列化更新右侧 YAML
- YAML 手动编辑 → 实时解析更新表单（解析失败时右侧红色高亮，左侧不变）
- 新建/编辑复用同一 Modal 组件，通过 `policy | null` prop 区分

**优点：** 最大程度降低普通用户门槛；YAML 并排保留高级用户能力；左右均分视觉平衡
**缺点：** 并排布局在窄屏下需响应式处理

### 方案 B：YAML 为主，表单为辅

- YAML 编辑器占 70%，底部折叠展开表单摘要
- 表单仅做人类可读摘要，非主要编辑入口

**优点：** 改动幅度小
**缺点：** 用户收益低，核心痛点未解决

### 方案 C：独立新建/编辑页面

- 新建策略跳转独立页面，编辑保持 Modal

**优点：** 适合复杂配置场景
**缺点：** 改动幅度大，路径切换成本高

## Agreed Approach

方案 A：表单为主（60%）+ YAML 为辅（40%），左右并排，实时双向同步，新建/编辑复用同一组件。

## Key Decisions

- `name` → 文本框
- `effect` → 下拉选择（allow / confirm / deny）
- `action` → 复选框（apply / delete / scale）
- `namespace` / `kind` → 标签输入器（回车追加，×删除）
- `unsafeFields` → 保留原始文本输入（用户直接输入 JSONPath）
- 新建 Modal：表单 + YAML 左右并排（左右各半）
- 编辑模式：同新建 Modal，传入已有 policy 做初始化
- YAML 解析失败 → 右侧红色边框高亮，左侧表单保持不变
- 表单变动 → 实时序列化同步到右侧 YAML 文本框

## Open Questions

无。
