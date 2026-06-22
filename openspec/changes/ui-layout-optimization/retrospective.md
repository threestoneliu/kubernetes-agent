## Retrospective — ui-layout-optimization

### What Went Well

- 用户通過 HTML 設計稿直接確認方案，brainstorming 階段快速跳過
- OpenSpec artifacts 完整覆蓋全流程（brainstorm → design → proposal → specs → tasks → plan → verify → retrospective）
- 變更範圍清晰（CSS + React 組件結構），無跨層破壞

### What Could Improve

- 設計稿 HTML 與實際實現存在多處差異（toolbar gap、button 樣式、panel-title JSX 元素等），這些在規劃階段未充分識別
- 部分組件需要配合修改（ChatView status span、panel-title JSX 元素），不能只改 CSS
- 後期 CSS 調優過程中間距值反覆調整（gap vs margin vs padding），浪費來回 build 次數

### Open Questions

（無 — 所有問題已解決）
