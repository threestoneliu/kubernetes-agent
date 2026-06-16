# Spec: k8s-credential-encryption

## ADDED Requirements

### Requirement: AES-256-GCM 加密
系统 MUST 使用 AES-256-GCM 加密所有用户上传的 kubeconfig 内容,且 MUST NOT 使用更弱的算法(如 AES-CBC、ECB)或自实现加密。

#### Scenario: 上传 kubeconfig
- **WHEN** 用户在 Web UI 上传 kubeconfig 文件
- **THEN** 系统 MUST 解析 kubeconfig,提取 `server` 与 `user` 字段明文存 `clusters` 表,把整个 kubeconfig YAML 用 AES-256-GCM 加密后存 `kubeconfig_blob` BLOB

#### Scenario: 列出 cluster
- **WHEN** 用户在集群管理页查看 cluster 列表
- **THEN** 系统 MUST 仅展示明文 `name`、`server`、`user` 字段,MUST NOT 在列表页解密或展示 kubeconfig 内容

### Requirement: Master key 来源
master key MUST 来自以下两个来源之一(优先级从高到低):环境变量 `KUBERNETES_AGENT_MASTER_KEY` · 文件 `~/.kubernetes-agent/master.key`(权限 MUST 为 0600)。

#### Scenario: 环境变量优先
- **WHEN** `KUBERNETES_AGENT_MASTER_KEY` 环境变量存在且为 32 字节 base64
- **THEN** 系统 MUST 使用该环境变量作为 master key,MUST NOT 读取文件

#### Scenario: 文件作为后备
- **WHEN** `KUBERNETES_AGENT_MASTER_KEY` 不存在
- **THEN** 系统 MUST 从 `~/.kubernetes-agent/master.key` 读取 32 字节,文件不存在或权限不为 0600 MUST 拒绝启动

#### Scenario: 首次启动生成 key
- **WHEN** 首次启动且 master key 文件不存在
- **THEN** 系统 MUST 生成 32 字节随机数据,base64 编码后写入 `~/.kubernetes-agent/master.key`,文件权限 MUST 为 0600,且 MUST 拒绝以 root 启动(避免权限被忽略)

### Requirement: 加密格式
密文 BLOB MUST 包含三段拼接:`nonce(12 bytes)` + `ciphertext(N bytes)` + `tag(16 bytes)`,且 MUST 全部存储在同一个 `BLOB` 字段。

#### Scenario: 解密成功
- **WHEN** 系统从 `clusters` 表读取 `kubeconfig_blob` 并尝试解密
- **THEN** 系统 MUST 先取前 12 字节作为 nonce,中间部分作为 ciphertext,最后 16 字节作为 GCM tag,认证失败 MUST 返回错误

#### Scenario: GCM tag 验证失败
- **WHEN** 密文被篡改
- **THEN** 解密 MUST 因 GCM tag 验证失败返回错误,MUST NOT 返回任何明文

### Requirement: 解密生命周期
kubeconfig 明文 MUST 仅在用户当前会话期间存在于内存,会话结束或客户端断开 MUST 立即清除。

#### Scenario: 用户切换 cluster
- **WHEN** 用户在 UI 切换到另一个 cluster
- **THEN** 系统 MUST 解密新 cluster 的 kubeconfig 并载入 memory,且 MUST 清除旧 cluster 的明文

#### Scenario: 服务关闭
- **WHEN** 用户停止服务(`Ctrl+C`)或 session 结束
- **THEN** 系统 MUST 主动清零所有内存中的 kubeconfig 明文 buffer

### Requirement: 备份语义
master key 文件与 SQLite 数据库 MUST 一起备份才能恢复数据,文档 MUST 显眼位置说明此限制。

#### Scenario: README 警告
- **WHEN** 用户首次启动完成
- **THEN** UI MUST 提示"master.key 与 data.db 必须一起备份,丢失任一即数据不可恢复",且 README MUST 包含相同警告

### Requirement: 不引入 KMS
本 change MUST NOT 引入外部密钥管理服务(AWS KMS / Vault / SOPS 等),MVP 接受本地 master key 依赖 OS 文件权限。

#### Scenario: 启动期无 KMS 调用
- **WHEN** 服务启动
- **THEN** 系统 MUST NOT 发起任何外部网络请求用于获取 master key

### Requirement: 加密模块可独立测试
`internal/crypto` 包 MUST 提供 round-trip 测试,确认加密-解密-再加密-再解密流程结果一致,且 MUST 提供"明文特征不出现于密文"的 sanity 测试。

#### Scenario: Round-trip 一致
- **WHEN** 给定任意明文,加密后立即解密
- **THEN** 解密结果 MUST 与原明文字节级一致

#### Scenario: 密文不可区分
- **WHEN** 用相同 key 加密两个不同明文
- **THEN** 两个密文 MUST 因 nonce 随机而完全不同,且 MUST NOT 出现明显明文特征(固定头部、可识别字符串)
