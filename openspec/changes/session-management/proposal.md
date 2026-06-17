# Proposal · Session Management

## Why

`kubernetes-agent` 在 k8s-natural-language-agent change 落地了最小闭环(读 + 写 + Plan + 护栏),但会话管理能力缺失:用户只能在 ChatView 里"新建"会话,看不到历史会话、无法切换回去继续聊、无法重命名 / 删除 / 导出 / 搜索 / 排序。后端 `GET /api/sessions` 已有,前端从未调用。本 change 把这条接口以及历史会话的检索、命名、删除、导出能力补齐,并把 ChatView 改成 2-pane 让用户能像 IDE 一样在历史会话间穿梭。

直接收益:
- 用户可以保留几次不同主题的对话(例如"查看 default pod" vs "排查 coredns"),在它们之间切换
- 长时间使用后支持清理不需要的会话,避免 SQLite 累积
- 重要会话可导出 Markdown / JSON 备份
- 活跃 streaming session 不允许误删(后端 409 保护)

## What Changes

**会话列表展示**
- From: ChatView 顶部 toolbar 只有"会话: (新建)" dropdown,没有历史列表
- To: ChatView 改成 2-pane,左 300px SessionsPanel 显示历史会话(标题 + cluster + 更新时间),顶部 toolbar 含新建 / 搜索 / 排序
- Reason: 用户要求"无法选择会话继续聊天"
- Impact: 非破坏;新增左栏 UI 元素

**会话操作**
- From: 无法重命名 / 删除 / 导出历史会话
- To: SessionsPanel 每行 hover 弹 `⋯` 菜单,提供重命名 inline edit / 导出 Markdown / 导出 JSON / 删除(confirm 模态二次确认)
- Reason: 用户要求"没有办法管理历史会话"
- Impact: 新增 4 个后端 endpoint + 前端菜单

**服务端搜索 / 排序 / 分页**
- From: `GET /api/sessions` 只返回默认 100 条按 created_at desc,无搜索无排序
- To: 加 `?q=&sort=&order=&limit=&offset=`,标题 LIKE 大小写不敏感 + 任意字段升降序 + 翻页
- Reason: 历史会话多时找不到目标
- Impact: 现有调用方零改动(新参数都 optional,默认行为不变)

**活跃 session 删除保护**
- From: 任何 session 都能被 DELETE
- To: 如果 session 还在 agent.Session map(agent 持有),DELETE 返回 409 + `code: session_active`
- Reason: 避免删除正在进行的 turn 中途断流
- Impact: 仅后端 handler 加 5 行检查

**草稿保留**
- From: 切换会话时输入框内容丢失
- To: ChatView 维护 `drafts[sessionId]` map,切走前保存,切回来时回填
- Reason: 用户切换会话回来继续写时草稿不能丢
- Impact: 仅前端 state

**一键清空**
- From: 没有批量清理入口
- To: SessionsPanel 底部"清空全部"按钮,confirm 模态显示数量,DELETE /api/sessions
- Reason: 长时间使用后的清理诉求
- Impact: 新增 bulk endpoint + 前端确认

## Capabilities

### New Capabilities

- `session-list-and-select`: 会话列表展示 + 点选切换 + 当前 active 高亮
- `session-rename-and-delete`: 重命名 inline edit + 删除(confirm + active session 保护)
- `session-search-sort-filter`: 标题搜索 + 字段排序 + LIMIT/OFFSET 分页
- `session-export`: 单个会话导出 Markdown / JSON,流式下载
- `session-bulk-clear`: 一键清空全部历史会话

### Modified Capabilities

None. 现有 6 个 capability(natural-language-k8s-interaction / k8s-write-with-plan-preview / k8s-policy-guardrails / k8s-credential-encryption / multi-llm-provider-support / web-chat-ui)的 REQUIREMENTS 不变 — 本 change 在 web-chat-ui 的范围里只新增 UI 层,不改任何业务行为契约。

## Impact

**代码**:
- 后端 ~5 个新 endpoint,1 个 store 新增 5 个方法,~250 行
- 前端 SessionsPanel 新组件 ~300 行,ChatView 改 2-pane ~100 行,ConfirmModal 新组件 ~50 行
- 现有 sessions 表 schema 不变,数据无需迁移

**API**:
- 4 个新 endpoint + 1 个 GET 加 query 参数;无破坏性变更
- 导出走 `Content-Disposition: attachment`,浏览器原生下载,不需要 SDK

**测试**:
- Store 层 5 个新方法单测
- Handler 层 4 个新 endpoint httptest
- React Testing Library 组件测试
- Playwright 7 个 e2e 场景

**部署**:
- 0 数据库迁移
- 0 配置变更
- 0 新依赖(marked + dompurify 已经在 web/)