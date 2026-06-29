# Retrospective: policy-form-editor

## What Worked Well

- **表单+YAML 双向同步**：debounce 设计合理，用户即能实时预览又能手动编辑
- **TagInput 组件复用**：独立组件，可测试，可复用到其他表单场景
- **高危操作确认**：页面侧提前校验（YAML 解析失败 / action 未选 / name 为空），避免无效请求
- **批量提交**：19 个 tasks 一次 commit，提交历史干净

## Friction Points

- **JSX 语法错误调试**：标签闭合耗时间（`</form>` 写成 `</form>` 导致整文件解析失败）
- **js-yaml ESM 导入**：Rolldown 下 `import yaml from 'js-yaml'` 默认导出缺失，需改 `import * as yaml`
- **CSS 变量未定义**：`var(--accent)` 无定义导致标签背景透明不可见

## Lessons Learned

- **HTML/JSX 闭合标签错误影响整文件构建**：健壮的构建验证（Rolldown 的 `builtin:vite-transform` 报错明确）应及时响应
- **设计时明确变量定义**：使用 CSS 变量前先查 `styles.css`，未定义的变量是视觉 bug 的根因
- **前后端并行开发**：前端独立组件（TagInput / PolicyFormModal）可以先写好单元验证，再集成到 PolicyView

## Misses

- **无单元测试**：TagInput / PolicyFormModal 未写 React Testing Library 单元测试，纯手工验证
- **无 TypeScript 类型校验**：PolicyForm 的 TypeScript 类型前端未跑 `tsc --noEmit` 预检
- **未做 YAML 往返一致性测试**：序列化/解析往返只做了手工 node 脚本验证，无自动化用例

## Action Items

- 为 TagInput 添加 RTL（right-to-left）支持国际化场景
- 为 PolicyFormModal 添加表单提交埋点（analytics event）
- 考虑迁移到 React Testing Library 做组件测试
