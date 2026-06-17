# Spec: session-export

## ADDED Requirements

### Requirement: 导出 Markdown
Web UI MUST 在每条会话项的 hover 菜单提供"导出 Markdown"入口,点击 MUST 触发浏览器下载 `session-<id>.md`,Content-Type `text/markdown; charset=utf-8`。Markdown 内容 MUST 包含会话元数据头(title / cluster_id / created_at / updated_at),接每条消息按 `## <role>` 分节,assistant 消息内的 reasoning 折叠为 `<details>`/`<summary>`,tool_call + tool_result 序列化为 fenced JSON code block。

#### Scenario: 下载成功
- **WHEN** 用户点击"导出 Markdown"
- **THEN** 浏览器 MUST 弹出下载对话框,文件名 session-<id>.md;Content-Type MUST 为 text/markdown;Content-Disposition MUST 包含 attachment

#### Scenario: Markdown 包含元数据
- **WHEN** 导出会话(标题为 "列出 default pod",cluster=lzl)
- **THEN** 文件 MUST 以 `# 会话: 列出 default pod` 开头,包含 cluster_id / created_at / updated_at 字段

#### Scenario: 推理与工具块在 Markdown 中
- **WHEN** 会话包含 reasoning 和 tool_call 块
- **THEN** reasoning MUST 渲染为 `<details><summary>思考过程</summary>...</details>`;tool_call + tool_result MUST 渲染为 `🔧 <name>` 标题 + ```json fenced block

### Requirement: 导出 JSON
Web UI MUST 在每条会话项的 hover 菜单提供"导出 JSON"入口,点击 MUST 触发浏览器下载 `session-<id>.json`,Content-Type `application/json; charset=utf-8`。JSON MUST 包含 session row + 全部 messages + 全部 plans + 全部 audit_log 行,使用原始 store schema 的 JSON 序列化形式。

#### Scenario: JSON 包含全部数据
- **WHEN** 用户点击"导出 JSON"
- **THEN** 文件 MUST 是有效 JSON,顶层 MUST 包含 session / messages / plans / audit 4 个字段;messages MUST 包含该 session 的全部行(无分页)

#### Scenario: JSON 用于备份而非 round-trip
- **WHEN** 用户导入 JSON 到系统
- **THEN** MVP 阶段 MUST NOT 自动恢复,MUST 在 hover 菜单里说明"用于备份,不支持导入"

### Requirement: 导出接口契约
后端 `GET /api/sessions/{id}/export?format=md|json` MUST:format 非法返回 400;session 不存在返回 404;成功返回 200 + 正确 Content-Type + Content-Disposition;Markdown / JSON 内容 MUST 直接写入 response body,前端不做二次处理。

#### Scenario: 非法 format
- **WHEN** GET /api/sessions/{id}/export?format=xml
- **THEN** MUST 返回 400 + `code: invalid_format`

#### Scenario: 不存在的 session
- **WHEN** GET /api/sessions/<不存在>/export?format=md
- **THEN** MUST 返回 404 + `code: not_found`