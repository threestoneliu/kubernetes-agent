# Tasks · WebUI Redesign (Three-Column + Theme Switching)

## 1. Theme Context

- [x] 1.1 创建 `web/src/contexts/ThemeContext.tsx` — createContext + useTheme hook，state: 'dark'|'light'，toggle()，默认读 localStorage
- [x] 1.2 App.tsx 引入 ThemeContext，`<html data-theme={theme}>` 顶层设置
- [x] 1.3 主题切换入口按钮放在左侧导航栏底部

## 2. CSS 变量主题体系

- [x] 2.1 重写 `web/src/styles.css` — 完整 CSS 变量体系，:root 定义所有 token，两套主题变量覆盖
- [x] 2.2 实现 `[data-theme="dark"]` 变量块（Dark Pro 色值）
- [x] 2.3 实现 `[data-theme="light"]` 变量块（Light 色值）
- [x] 2.4 全局 `transition: background-color 150ms, color 150ms, border-color 150ms` 加入 * 或 body

## 3. App.tsx 三栏布局

- [x] 3.1 重构 App.tsx — `.app` 改为三栏 flex-row
- [x] 3.2 左侧 nav 栏：60px 宽，flex-col，图标按钮（💬/☸/🛡），底部主题切换按钮 + 设置
- [x] 3.3 中间 sessions-panel 栏：240px 宽，SessionPanel 组件
- [x] 3.4 右侧 main 栏：flex:1，ChatView/ClusterView/PolicyView
- [x] 3.5 nav 图标 hover 显示 tooltip（title 属性）

## 4. 三栏布局 CSS

- [x] 4.1 `.app` flex-row，height: 100vh，overflow hidden
- [x] 4.2 `.nav` 60px宽，flex-col，bg-sidebar，border-right
- [x] 4.3 `.nav-item` 图标按钮样式（40px 40px，border-radius: 10px，hover/active 状态）
- [x] 4.4 `.sessions-panel` 240px 宽，border-right，flex-col
- [x] 4.5 `.main` flex:1，flex-col，overflow hidden

## 5. 主题切换 UI

- [x] 5.1 nav 底部放置主题切换按钮（🌙图标），点击切换主题
- [x] 5.2 按钮样式：40px图标，hover 高亮
- [x] 5.3 点击后 ThemeContext.toggle() 触发 localStorage 写回 + state 更新

## 6. 三栏布局验证

- [x] 6.1 浏览器打开 localhost:8080，截图确认三栏布局正确
- [x] 6.2 截图确认 Dark Pro 主题正确（深色背景 + 蓝色强调）
- [x] 6.3 点击主题切换，确认切换到 Light 主题正常
- [x] 6.4 刷新页面，确认主题从 localStorage 恢复
- [x] 6.5 截图 Clusters 视图（三栏布局下正常）
- [x] 6.6 截图 Policies 视图

## 7. 提交

- [x] 7.1 git commit：feat(web): three-column layout + theme switching
- [x] 7.2 推送 remote
