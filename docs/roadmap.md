# 路线图

本 change (`k8s-natural-language-agent`) 落地了"读 + 写 + Plan 预览 + 护栏"的最小闭环。后续 change 路线图(顺序仅表示时间预期,不代表承诺):

| 序号 | 主题 | 范围 |
|------|------|------|
| 1 | 本 change | 本地单机 + 读/写 + Plan/护栏 |
| 2 | 多步事务 + 诊断 | `k8s_logs` 工具、跨资源 plan、解释为什么 pod 起不来 |
| 3 | Helm 集成 | helm-go SDK,agent 能 install/upgrade/rollback chart |
| 4 | 多用户长驻部署 | 登录、会话隔离、租户策略、Postgres 适配 |
| 5 | 多集群 | cluster 切换 UI、跨集群 plan、cluster_group 策略 |
| 6 | UI 打磨 | 审计日志页、可视化 diff 渲染、Plan 历史回放 |

> 历史草稿见 [openspec/changes/k8s-natural-language-agent/brainstorm.md](../openspec/changes/k8s-natural-language-agent/brainstorm.md#后续-change-路线图参考)。该路径在 OpenSpec archive 后会变动;本文档为稳定版本。
