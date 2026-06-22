## 1. Backend Summary 生成验证

- [ ] 1.1 `summarizeOne()` 对 CREATE 操作生成"创建 Kind namespace/name"格式摘要
- [ ] 1.2 `summarizeOne()` 对 UPDATE 操作生成"更新 Kind namespace/name: 变更字段"格式摘要
- [ ] 1.3 `summarizeOne()` 对 DELETE 操作生成"删除 Kind namespace/name"格式摘要
- [ ] 1.4 `summarizeOne()` 对 SCALE 操作生成"调整 Kind namespace/name replicas: X → Y"格式摘要
- [ ] 1.5 `PlanWrite()` 在 `dryRun` 后正确填充 `diff.Summary` 字段

## 2. 前端 DiffCard 展示验证

- [ ] 2.1 DiffCard 正确渲染 backend `diff.summary` 字段，不使用前端 `summarizeChange()` 计算
- [ ] 2.2 CREATE 操作显示绿色 CREATE 标签
- [ ] 2.3 UPDATE/APPLY 操作显示蓝色 UPDATE 标签
- [ ] 2.4 DELETE 操作显示红色 DELETE 标签
- [ ] 2.5 SCALE 操作显示黄色 SCALE 标签
- [ ] 2.6 YAML 默认折叠，点击"查看完整 YAML"展开
- [ ] 2.7 YAML 渲染跳过 creationTimestamp/managedFields 等系统字段

## 3. Modal 确认流程端到端验证

- [ ] 3.1 Modal 点"确认执行"后直接调用 k8s_execute_plan，无须 chat 输入"yes"
- [ ] 3.2 Modal 点"取消"后 plan 状态重置，Session 保持阻塞等待新 plan
- [ ] 3.3 `Session.ResetPlan()` 在新 plan 等待前正确重置 `PlanResult`

## 4. 端到端测试

- [ ] 4.1 集成测试：创建 Deployment，验证 DiffCard 显示"创建 Deployment default/nginx"
- [ ] 4.2 集成测试：更新 Deployment replicas，验证 DiffCard 显示"更新 Deployment default/nginx: replicas: 1 → 3"
- [ ] 4.3 集成测试：删除 Deployment，验证 DiffCard 显示"删除 Deployment default/nginx"
