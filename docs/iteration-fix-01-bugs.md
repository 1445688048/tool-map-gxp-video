# 迭代 Fix-01：代码评测问题修复

> **状态**：已完成
> **目标**：修复代码评测报告中列出的所有 P0/P1 级别问题
> **前置迭代**：v0.1 初始版本
> **影响范围**：后端稳定性、前端体验、代码规范

---

## 修复清单

| # | 级别 | 问题 | 文件 | 状态 |
|---|------|------|------|------|
| C1 | P0 | Goroutine panic 恢复 — 服务崩溃根因 | `export/service.go` | ✅ |
| C2 | P0 | `BoundingBoxToTiles` 循环笔误 | `tile/coord.go` | ✅ |
| C3 | P0 | 空 GPX 文件上传校验 | `api/handler.go` | ✅ |
| H1 | P1 | 消除重复 `haversine` 函数 | `tile/math.go` (新) | ✅ |
| H2 | P1 | 导出时正确设置播放速度 | `ExportPanel.vue` | ✅ |
| H3 | P1 | 删除 `window.__lastSegments` 临时代码 | `App.vue` | ✅ |
| H4 | P1 | 清理 `PlaybackControls.vue` 死代码 | `components/player/` | ✅ |
| H5 | P1 | 统一前端错误提示（替代 alert） | 各 `.vue` 组件 | ✅ |
| H6 | P1 | 统一路由参数命名 `:sid` → `:id` | `api/handler.go` | ✅ |
| H7 | P1 | Preload goroutine panic 恢复 + 代码重构 | `export/service.go` | ✅ |
| M2 | P2 | 相机平滑过渡（easeTo 300ms） | `camera-engine/index.ts` + `route-player/index.ts` | ✅ |
| M3 | P2 | 瓦片超时缩短到 10s | `tile/cache.go` | ✅ |
| M4 | P2 | SQLite 连接池配置 | `db/db.go` | ✅ |
| M5 | P2 | GORM 日志降级为 Warn | `db/db.go` | ✅ |
| M6 | P2 | 接入 `IntroOutroConfig.vue` | `App.vue` | ✅ |

---

## 变更摘要

### 后端
- `internal/export/service.go`：goroutine 加 defer recover；合并 RunExport/RunExportWithWebM 为 runExportInternal
- `internal/tile/coord.go`：修复 BoundingBoxToTiles 死循环写法
- `internal/tile/math.go`：新建公共 haversine 函数
- `internal/gpx/parse.go`：改用 tile.Haversine
- `internal/route/service.go`：改用 tile.Haversine，移除重复定义
- `internal/api/handler.go`：空文件校验、统一参数名 `:id`
- `internal/tile/cache.go`：超时 30s → 10s
- `internal/db/db.go`：连接池配置、日志降级 Warn

### 前端
- `App.vue`：移除 window.__lastSegments、接入 IntroOutroConfig、全局 Toast 系统、null 安全检查
- `ExportPanel.vue`：导出时正确设置 3x 播放速度、Toast 错误提示
- `camera-engine/index.ts`：新增 easeTo 平滑过渡（300ms）
- `route-player/index.ts`：改用 easeTo 替代 jumpTo
- `SegmentList.vue` / `EventEditor.vue` / `UploadZone.vue`：alert → showToast
- 删除 `components/player/PlaybackControls.vue`（死代码）
