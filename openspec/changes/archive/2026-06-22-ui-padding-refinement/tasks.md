## 1. CSS Padding Refinement

- [x] 1.1 Add `.header-bar { padding: 0 20px }`
- [x] 1.2 Change `.sessions-panel { padding: 0 }` to `{ padding: 16px 12px }`
- [x] 1.3 Add `.panel-title` style block (font-size: 12px, uppercase, muted, margin-bottom: 4px)
- [x] 1.4 Change `.panel-footer { padding-top: 0 }` to `{ padding-top: 12px }`
- [x] 1.5 Change `.main { padding: 0 }` to `{ padding: 16px }`
- [x] 1.6 Change `.chat-stream { border-radius: 12px }` to `{ padding: 20px; border-radius: 16px }`
- [x] 1.7 Verify light theme padding consistency
- [x] 1.8 Run `make build` and confirm success

## 2. Additional Styling Alignment (design-vs-implementation gap)

- [x] 2.1 Update `.toolbar { gap: 8px }` to `{ gap: 12px }` to match design
- [x] 2.2 Add full `.panel-footer button` style block (bg: transparent, border, border-radius: 8px, color: muted)
- [x] 2.3 Add `.label-tag` status badge style (blue glow background, primary color text)
- [x] 2.4 Add `.composer button` and `.composer button.primary` overrides (border: none, border-radius: 10px, padding: 9px 20px)
- [x] 2.5 Update ChatView status span to use `className="label-tag"` instead of `className="muted"`
- [x] 2.6 Update ChatView empty hint padding from `16` to `'8px 0'` to match design
- [x] 2.7 Add `SessionsPanel.tsx` `<div className="panel-title">会话列表</div>` element
