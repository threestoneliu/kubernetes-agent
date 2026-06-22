# Design — UI Padding Refinement

## 概述

本次为纯 CSS 调整，目标是解决当前页面元素紧贴边框、缺乏呼吸感的问题。在不改变暗色主题、不调整布局结构的前提下，通过精确的 padding 和圆角微调改善视觉效果。

## 技术方案

### 改动文件

**唯一改动文件：** `web/src/styles.css`

### CSS 改动明细

#### 1. `.header-bar` — 顶部导航栏
```css
.header-bar {
  /* 改动前 */
  padding: 0;
  /* 改动后 */
  padding: 0 20px;
}
```
左右各 20px，内元素不贴边。

#### 2. `.sessions-panel` — 会话列表面板
```css
.sessions-panel {
  /* 改动前 */
  padding: 0;
  /* 改动后 */
  padding: 16px 12px;
}
```
顶部/底部 16px，左右 12px（已有右侧 border 距主内容区 1px）。

#### 3. `.panel-title` — 新增面板标题
```css
.panel-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--fg-muted);
  letter-spacing: 0.05em;
  text-transform: uppercase;
  margin-bottom: 8px;
  padding: 0 4px;
}
```
置于 `.sessions-panel` 顶部 session-list 上方，替代原来空白状态。

#### 4. `.panel-footer` — 底部操作栏
```css
.panel-footer {
  padding-top: 12px;  /* 改动前: 0 */
}
```

#### 5. `.main` — 主内容区
```css
.main {
  padding: 16px;      /* 改动前: 0 */
  gap: 12px;         /* 保持: 12px toolbar 与 chat-stream 之间间距 */
}
```

#### 6. `.chat-stream` — 聊天气泡区
```css
.chat-stream {
  padding: 20px;                         /* 新增: 20px 内边距 */
  border-radius: 16px;                    /* 改动前: 12px → 16px */
}
```

#### 7. `.composer` — 输入区域
```css
.composer {
  padding: 0 4px;   /* 已存在，保持不变 */
}
```

### 无改动区域

以下样式保持不变：颜色变量、按钮样式（除圆角外）、Bubble 样式、Modal 样式、Badge 样式、Input/Select 样式。

## Light Theme 适配

`[data-theme="light"]` 下上述选择器若无独立覆盖，则自动继承改动。若 chat-stream 在浅色下背景色需要调整，则在 `data-theme="light"` 下单独覆盖 `.chat-stream` 的背景色确保与 `--bg` 一致。

## 测试计划

- [ ] 验证暗色主题下 header、sessions-panel、chat-stream、composer 内边距符合设计
- [ ] 验证浅色主题下内边距一致
- [ ] 确认 `make build` 构建通过
