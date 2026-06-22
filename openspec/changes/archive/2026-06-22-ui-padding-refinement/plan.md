# Plan — UI Padding Refinement

## Goal

通过精确的 CSS 内边距调整，让 Kubernetes Agent Web UI 各区域（header、sessions-panel、chat-stream）拥有适度的呼吸空间，视觉上不再紧贴边框。

## Files

- Modify: `web/src/styles.css`

## Task 1: Update header-bar padding

**File:** `web/src/styles.css:94-103`

Add `padding: 0 20px` to `.header-bar`.

## Task 2: Update sessions-panel padding and add panel-title

**File:** `web/src/styles.css:191-203`

Change `.sessions-panel { padding: 0; gap: 0; }` → `{ padding: 16px 12px; gap: 8px; }`.

Add new `.panel-title` block after `.sessions-panel`.

## Task 3: Update panel-footer padding

**File:** `web/src/styles.css:492-495`

Change `.panel-footer { padding-top: 0; }` → `{ padding-top: 12px; }`.

## Task 4: Update main padding

**File:** `web/src/styles.css:206-214`

Change `.main { padding: 0; }` → `{ padding: 16px; }`.

## Task 5: Update chat-stream padding and border-radius

**File:** `web/src/styles.css:499-509`

Add `padding: 20px` and change `border-radius: 12px` → `16px`.

## Verification

Run `make build` to confirm no build errors.
