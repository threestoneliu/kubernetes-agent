# Spec: session-search-sort-filter

## ADDED Requirements

### Requirement: 标题搜索
Web UI MUST 在会话面板顶部提供一个搜索框;用户输入文本时,UI MUST 用节流(300ms)调 `GET /api/sessions?q=<text>` 重新拉取并展示结果;后端 MUST 用 SQL `WHERE title LIKE '%q%' COLLATE NOCASE` 做大小写不敏感匹配。

#### Scenario: 搜索匹配
- **WHEN** 用户在搜索框输入 "demo" 且数据库中存在标题含 "demo"(任意大小写)的会话
- **THEN** 列表 MUST 只显示匹配的会话;非匹配项 MUST 不出现

#### Scenario: 搜索无匹配
- **WHEN** 用户输入的搜索词无任何会话标题匹配
- **THEN** 列表 MUST 显示空状态文案"无匹配会话"

#### Scenario: 清空搜索框
- **WHEN** 用户清空搜索框
- **THEN** 列表 MUST 回到默认的完整列表(updated_at desc,limit 100)

### Requirement: 排序与分页
Web UI MUST 在会话面板顶部提供排序下拉,选项 MUST 至少包含:`更新时间↓`(默认,updated_at desc)、`更新时间↑`、`创建时间↓`、`创建时间↑`、`标题 A→Z`、`标题 Z→A`。每次选择 MUST 调 `GET /api/sessions?sort=<col>&order=<asc|desc>` 重新拉取。后端 MUST 校验 sort 字段在白名单内,order 在 {asc, desc} 内,非法值返回 400。

#### Scenario: 切换排序
- **WHEN** 用户从"更新时间↓"切换到"标题 A→Z"
- **THEN** 列表 MUST 立即按 title 升序重排;后续搜索 MUST 保留当前排序

#### Scenario: 非法 sort 参数
- **WHEN** 前端发送 `?sort=password`
- **THEN** 后端 MUST 返回 400 + `code: invalid_sort`

### Requirement: 服务端搜索 / 排序 / 分页接口契约
`GET /api/sessions` MUST 接受 query 参数:`q`(默认空字符串)、`sort`(默认 `updated_at`,白名单 `{created_at, updated_at, title}`)、`order`(默认 `desc`,白名单 `{asc, desc}`)、`limit`(默认 100,最大 100)、`offset`(默认 0)。返回 `{sessions: [...]}`;空结果返回 `{sessions: []}` 而非 404。

#### Scenario: 默认查询
- **WHEN** GET /api/sessions 无 query 参数
- **THEN** MUST 返回 updated_at desc 前 100 条

#### Scenario: 带搜索 + 排序 + 分页
- **WHEN** GET /api/sessions?q=foo&sort=title&order=asc&limit=20&offset=40
- **THEN** MUST 返回 title LIKE '%foo%' COLLATE NOCASE 匹配,title asc 排序,跳过前 40 条取 20 条