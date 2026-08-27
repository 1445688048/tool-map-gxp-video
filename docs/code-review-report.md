# GPX 越野路线 AI 预演系统 — 代码评测报告

> 生成时间：2026-08-28
> 评测范围：后端 Go + 前端 Vue/TS 全量代码，覆盖代码规范、接口规范、链路联通性

---

## 一、项目总览

| 维度 | 状态 |
|------|------|
| 后端 `gxp-map-video-backend/` | Go 1.26.5 + Gin + GORM + SQLite，编译通过 |
| 前端 `gxp-map-video-frontend/` | Vue 3 + TypeScript + MapLibre GL JS，构建通过 |
| 服务联调 | 前后端均已验证可启动，基础 API 连通 |
| 测试数据 | 4 条路线（含 29km/91km），SQLite 有数据 |
| 空目录预留 | `internal/ai/`、`internal/story/`、`internal/tts/`（已占位，待实现） |

---

## 二、Critical 问题（必须修复，否则系统不稳定）

### C1. Goroutine 无 Panic 恢复 — 服务崩溃根因

**位置：** `internal/export/service.go` — `Preflight()` 和 `Preload()` 方法

**现象：** 服务器启动后处理 1-3 个请求即崩溃退出，无任何日志。

**根因：** `Preload()` 对每个瓦片并发启动 goroutine 调用 `tileCache.Get()`，该方法内部涉及 HTTP 网络请求和文件系统 I/O。一旦某个 goroutine panic（网络超时、磁盘 IO 错误等），由于没有 `recover()`，整个进程被杀死。

```go
// 当前危险代码（export/service.go）
for _, t := range tiles {
    wg.Add(1)
    go func(z, x, y int) {
        defer wg.Done()
        if _, err := s.tileCache.Get("esri-satellite", z, x, y); err != nil {
            failed.Add(1)
        }
    }(t[0], t[1], t[2])
    // ... terrain 同理
}
```

**修复方案：** 在每个 goroutine 入口加 recover：

```go
go func(z, x, y int) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("tile download recovered: %v", r)
        }
        wg.Done()
    }()
    if _, err := s.tileCache.Get("esri-satellite", z, x, y); err != nil {
        failed.Add(1)
    }
}(t[0], t[1], t[2])
```

---

### C2. `BoundingBoxToTiles` 外层循环写法错误

**位置：** `internal/tile/coord.go:25`

**现象：** 代码意图遍历单一 zoom 层，但写了 `for z := zoom; z >= zoom; z--`，虽然逻辑上只执行一次（z-- 后立即退出），但这是明显的笔误，会误导后续维护者。

```go
// 当前代码（coord.go:25）
for z := zoom; z >= zoom; z-- {  // 永远只跑一次，z-- 是笔误
```

**修复方案：** 去掉外层 for，直接写内层两重循环：

```go
for yy := y1; yy <= y2; yy++ {
    for xx := x1; xx <= x2; xx++ {
        tiles = append(tiles, [3]int{zoom, xx, yy})
    }
}
```

---

### C3. `uploadGPX` 读取文件时内存分配不安全

**位置：** `internal/api/handler.go:UploadGPX`

```go
buf := make([]byte, file.Size)
if _, err := data.Read(buf); err != nil {
```

`file.Size` 可能为 0（空文件），此时 `make([]byte, 0)` 后 `Read` 直接返回 EOF，不会报错但会创建一个空 GPX 上传成功。**应加 file.Size > 0 的前置校验。**

同时，对于大 GPX 文件（200km 路线可能有数 MB），一次性读入内存不够优雅，建议用 `io.ReadAll(data)` 或分块读取。

---

## 三、High 级别问题（影响稳定性或体验，建议本轮修复）

### H1. `haversine` 函数重复定义

**位置：**
- `internal/gpx/parse.go:128`
- `internal/route/service.go:113`

两个函数完全相同，违反 DRY 原则。应提取到 `internal/util/math.go` 或 `internal/tile/coord.go`。

---

### H2. `ExportPanel.vue` 中 `recorder.setSpeed(baseSpeed)` 是死代码

**位置：** `src/components/export/ExportPanel.vue`

```typescript
if (recorder.setSpeed) recorder.setSpeed(baseSpeed)
```

`VideoRecorder.setSpeed` 只设置了一个未使用的字段 `speedMultiplier`，不影响实际录制速度。导出时实际播放速度由 `RoutePlayer.setSpeed()` 控制，但 ExportPanel 从未调用它。**导出时播放速度始终为默认 1x，不会按预设 3x 加速录制。**

---

### H3. `App.vue` 中 `window.__lastSegments` 是临时 workaround

**位置：** `src/App.vue:generateCommentary()`

```typescript
if (seg.id && seg.commentary) {
    ;(window as any).__lastSegments = segments.value
}
```

这段代码将数据挂在 `window` 上，但没有真正调用 API 保存，是未完成的功能残留。**应删除该段代码。**

---

### H4. `PlaybackControls.vue` 组件未被使用（死代码）

**位置：** `src/components/player/PlaybackControls.vue`

该组件实现了完整的播放控制逻辑，但 `App.vue` 中没有引入，所有控制直接写在 `App.vue` 的 template 里。**应二选一：要么删除该文件，要么在 `App.vue` 中引入它并移除重复代码。**

---

### H5. 所有前端错误处理使用 `alert()`

**位置：** `EventEditor.vue`、`SegmentList.vue`、`ExportPanel.vue`、`UploadZone.vue`

```typescript
alert('上传失败: ' + (e instanceof Error ? e.message : String(e)))
```

全局体验差，无法样式化，且在移动端会遮挡内容。建议统一封装一个轻量级 toast 组件。

---

### H6. `UpdateSegment` 路由参数名不一致

**位置：** `internal/api/handler.go`

```go
routes.PUT("/segments/:sid", h.UpdateSegment)  // 用了 :sid
```

但其他路径统一用 `:id`，如 `routes.PUT("/events/:eid", ...)` 用的是 `:eid`，events 更新也用了 `eid`。segment 用 `sid` 与其他资源不统一。

---

### H7. Preflight/Preload 同步阻塞 HTTP 请求线程

**位置：** `internal/api/handler.go:PreflightExport`、`PreloadTiles`

200km 路线在 zoom 8-14 的瓦片总数可达数千~上万。`Preload()` 会发起大量并发 HTTP 请求，**单个 HTTP 请求线程被占用数十秒**，在此期间该 Gin worker 无法处理其他请求。

**建议：** Preload 改为异步任务（类似 ExportTask），返回 task ID 后轮询状态；Preflight 限制最大瓦片数（如超过 5000 个则返回错误提示用户分片处理）。

---

## 四、Medium 级别问题

### M1. GPX 解析不支持 Garmin 扩展时间戳

**位置：** `internal/gpx/parse.go`

Garmin 设备输出的 GPX 有时将时间戳放在 `<gpxtpx:time>` 命名空间下，而非标准 `<time>`。当前解析器只读取标准 `<time>` 标签，可能导致速度计算不准确。

**风险：** 低。大多数 GPX 文件使用标准 `<time>` 标签。

---

### M2. `CameraEngine.jump()` 导致相机跳动而非平滑过渡

**位置：** `engines/camera-engine/index.ts`

`RoutePlayer.updateCamera()` 调用的是 `jump()`（瞬间跳转），而不是 `apply()`（带动画过渡）。播放时摄像机移动是跳帧式的，视觉体验生硬。

---

### M3. 瓦片 HTTP 客户端超时 30s 过长

**位置：** `internal/tile/cache.go`

```go
client: &http.Client{Timeout: 30 * time.Second}
```

单个瓦片下载超时 30 秒，在 Preload 场景下会拖慢整体进度。建议降到 10 秒。

---

### M4. SQLite 未配置连接池

**位置：** `internal/db/db.go`

```go
db, err := gorm.Open(sqlite.Open(path), &gorm.Config{...})
```

未调用 `sqlDB.SetMaxOpenConns()` 等配置。高并发写入时可能触发 SQLite 锁等待。

---

### M5. GORM AutoMigrate 日志级别过高

**位置：** `internal/db/db.go`

Logger 配置为 `logger.Info`，每次启动打印大量 CREATE TABLE/INSERT 迁移 SQL。生产环境应改为 `logger.Warn`。

---

### M6. `IntroOutroConfig.vue` 组件未被引用

**位置：** `src/components/intro-outro/IntroOutroConfig.vue`

完整实现但未在任何页面引入，属于悬空代码。迭代 2 文档中提到要支持 Intro/Outro 镜头，但该组件尚未接入 `App.vue`。

---

## 五、接口规范审查

### ✅ 做得好的地方
- RESTful 风格一致：GET 查询、POST 创建、PUT 更新、DELETE 删除
- JSON 错误格式统一：`{"error": "..."}`
- 路径参数命名清晰
- 瓦片代理接口简洁：`GET /api/tiles/:provider/:z/:x/:y`

### ⚠️ 需改进

| # | 问题 | 影响 |
|---|------|------|
| 1 | `UpdateSegment` 路由参数名为 `:sid`，其他资源统一用 `:id` | 一致性差，容易混淆 |
| 2 | `GET /api/routes` 和 `GET /api/routes/:id/points` 无分页 | 200km 路线 9000+ 点，未来需要分页 |
| 3 | 无鉴权中间件 | 内网工具可接受，对外发布需补 |
| 4 | 导出进度靠轮询 `GET /api/export/tasks/:tid` | 可考虑 WebSocket 实时推送 |
| 5 | `POST /api/export/upload/:tid` 使用 FormData，但 `client.ts` 中 `uploadGPX` 和其余 API 使用不同风格 | 前端请求工具不统一 |

---

## 六、链路联通性验证

### ✅ 已验证通链路

```
前端上传 GPX
  → POST /api/routes
  → 后端解析 XML → Route + TrackPoint 存入 SQLite
  → GET /api/routes/:id/points
  → MapEngine.loadRoute() → MapLibre 渲染路线

点击"自动分段"
  → POST /api/routes/:id/analyze
  → Segmenter.Segment() → RouteSegment 存入 SQLite
  → GET /api/routes/:id/segments
  → SegmentList 展示分段列表

点击播放
  → RoutePlayer.tick() → requestAnimationFrame
  → CameraEngine.getState() → jumpTo()
  → MapEngine.setProgressPoint()
  → RouteOverlay 显示实时数据

瓦片代理
  → GET /api/tiles/esri-satellite/{z}/{x}/{y}
  → TileCache.Get() → 磁盘缓存命中或远程下载
```

### ⚠️ 潜在断链风险

| 环节 | 风险描述 |
|------|----------|
| 瓦片加载 | ESRI/Terrarium 是公网服务，大陆地区可能需要代理才能访问；当前无备用瓦片源 |
| 视频导出 | FFmpeg 未安装，`RunExportWithWebM()` 执行会失败；MediaRecorder WebM 录制在 Safari 上兼容性未知 |
| Preload | 200km 路线瓦片数可能上万，当前同步方式会长时间阻塞 |
| TTS/AI | 接口已预留（`StoryEvent.tts_status`、`commentary` 字段），但 `internal/tts/` 和 `internal/ai/` 目录为空，暂无实现 |

---

## 七、代码规范问题

### Go 后端

| 问题 | 文件 | 说明 |
|------|------|------|
| `haversine` 重复定义 | `gpx/parse.go:128` + `route/service.go:113` | 两个文件各有一个完全相同的函数 |
| `RunExport` / `RunExportWithWebM` 重复 | `export/service.go` | 两段代码 ~80 行重复，仅差异是输入源（black color vs webm file） |
| `parseInt` 工具函数散落 | `handler.go` 多处 | 相同的 `parseInt` 可提取为包级 helper |
| 无请求 ID / 结构化日志 | `main.go` | 当前只用 `log.Printf`，生产环境不便排查 |
| 未使用 `ai/`、`story/`、`tts/` 空目录 | 目录存在但无代码 | 预留目录，符合计划 |

### 前端 TypeScript/Vue

| 问题 | 文件 | 说明 |
|------|------|------|
| `'geojson' as const` 冗余断言 | `map-engine/index.ts` | MapLibre TS 类型已支持，不需要每处都加 |
| `window.__lastSegments` 临时挂载 | `App.vue` | workaround 代码，应删除 |
| 错误处理不统一 | 多个 `.vue` 文件 | 有的用 `alert()`，有的用 `console.error()` |
| `VideoRecorder.setSpeed` 无效 | `video-recorder/index.ts` | 设置了字段但录制时未使用 |
| 大量内联 CSS in Vue | 所有 `.vue` 文件 | 无统一 CSS 变量或主题系统，颜色硬编码 |

---

## 八、迭代完成度评估

| 迭代 | 目标 | 完成度 | 关键差距 |
|------|------|--------|----------|
| **迭代 1** GPX 3D 播放 | 上传 GPX → 地图 3D 渲染 → 沿路线播放 | **90%** | 相机用 `jumpTo` 不平滑；`PlaybackControls.vue` 死代码；无 Intro/Outro |
| **迭代 2** 分段时间线 | 自动分段 → 事件编辑 → 故事时间线 | **85%** | AI 解说未接入（按计划保留接口）；TTS 未接入（按计划保留接口）；`IntroOutroConfig.vue` 未接入 |
| **迭代 3** 视频导出 | 瓦片预下载 → 录制 → FFmpeg → MP4 | **70%** | FFmpeg 未安装；Preload 同步阻塞；`setSpeed` 在导出时不生效；无 Toast 错误提示 |

---

## 九、修复优先级

### P0 — 立即修复（崩溃/数据风险）
| # | 问题 | 涉及文件 |
|---|------|----------|
| C1 | Goroutine panic 恢复 | `internal/export/service.go` |
| C2 | `BoundingBoxToTiles` 循环笔误 | `internal/tile/coord.go` |
| C3 | 空文件上传校验 | `internal/api/handler.go` |

### P1 — 本轮修复（稳定性/体验）
| # | 问题 | 涉及文件 |
|---|------|----------|
| H1 | 消除重复 `haversine` | `gpx/parse.go` + `route/service.go` |
| H2 | 导出时正确设置播放速度 | `ExportPanel.vue` + `route-player/index.ts` |
| H3 | 删除 `window.__lastSegments` 临时代码 | `App.vue` |
| H4 | 清理 `PlaybackControls.vue` 死代码 | `components/player/` |
| H5 | 统一前端错误提示（替代 alert） | 所有 `.vue` 组件 |
| H6 | 统一路由参数命名（`:sid` → `:id`） | `handler.go` |
| H7 | Preload 改为异步任务 | `export/service.go` + `handler.go` |

### P2 — 后续优化
| # | 问题 | 涉及文件 |
|---|------|----------|
| M1 | GPX namespace 兼容性 | `gpx/parse.go` |
| M2 | 相机平滑过渡 | `camera-engine/index.ts` |
| M3 | 瓦片超时缩短到 10s | `tile/cache.go` |
| M4 | SQLite 连接池配置 | `db/db.go` |
| M5 | GORM 日志降级 | `db/db.go` |
| M6 | 接入 `IntroOutroConfig.vue` | `App.vue` |

### P3 — 长期
| # | 问题 | 涉及文件 |
|---|------|----------|
| - | 分页支持 | `handler.go` + `route/repository.go` |
| - | WebSocket 推送导出进度 | `handler.go` + `export/service.go` |
| - | 鉴权中间件 | `main.go` |
| - | 瓦片源容灾（备用源） | `tile/cache.go` |
