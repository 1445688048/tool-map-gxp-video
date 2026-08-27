# 迭代 3：视频导出 → Tile Preload → 性能优化

> **状态**：待开发
> **目标**：基于迭代 2 的 Timeline，实现稳定的视频导出（后端 FFmpeg），以及 Tile Preload / Export Preflight 机制，支持 200km 长路线
> **前置迭代**：迭代 1 + 迭代 2（必须有完整的 Route / Segment / Event / Timeline）

---

## 一、迭代目标

在迭代 1 + 2 的基础上，完成：

```
Story Timeline
    ↓
ExportTimeline 生成（精确的帧级时间映射）
    ↓
Export Preflight（检查 Tile Cache 完整性）
    ↓
Tile Preload（补全缺失瓦片）
    ↓
后端异步导出任务
    ↓
FFmpeg 渲染（浏览器采集帧 → 后端编码）
    ↓
MP4 输出
```

**一句话验收**：点击"导出视频"，系统后台生成 MP4，用户可以关掉页面去做其他事，完成后收到通知并下载。50km 路线导出 < 5 分钟，200km 路线可导出。

---

## 二、不做的事

- 不做 Blender 渲染（文档明确禁止）
- 不做 GPU Renderer
- 不做前端 WebGL post-processing
- 不做用户系统

---

## 三、技术决策

### 3.1 导出方案：Timeline 驱动 + 后端 FFmpeg

**为什么不用纯浏览器导出？**
- 200km 路线视频时长可能 20-30 分钟
- 浏览器 MediaRecorder 长时间运行不稳定（内存泄漏、标签页休眠）
- 无法保证每帧精确对齐 Timeline（受帧率波动影响）
- 不符合文档 #四十九："导出不能用实时播放速度驱动"

**为什么用 Timeline 驱动？**
- 每帧的相机位置由 Timeline 精确计算，不受机器性能影响
- 导出质量 = 播放质量，不会出现丢帧导致的画面跳跃
- 可以预计算所有帧的相机状态，再统一渲染

**方案架构**：
```
1. 前端：将 ExportTimeline 发送到后端（JSON）
2. 后端：根据 Timeline 逐帧调用 MapLibre offscreen canvas 渲染
3. 后端：将每帧图片送入 FFmpeg 编码为 MP4
4. 后端：返回 MP4 下载链接
```

**备选方案（如果 offscreen canvas 在后端不可行）**：
```
1. 前端：启动浏览器录制（MediaRecorder），但按 Timeline 精确控制相机
2. 前端：录制完成后上传 WebM 到后端
3. 后端：FFmpeg 转码为 MP4
```
这个备选方案更简单，但精度略低（依赖浏览器帧率）。作为第一版实现方案，如果精度不达标再切换到完全后端方案。

**推荐**：先用**备选方案**（前端录制 + 后端转码），验证可行后再考虑完全后端方案。

### 3.2 任务队列

```go
// internal/export/task.go

type TaskStatus string

const (
    TaskPending   TaskStatus = "PENDING"
    TaskRunning   TaskStatus = "RUNNING"
    TaskSuccess   TaskStatus = "SUCCESS"
    TaskFailed    TaskStatus = "FAILED"
    TaskCancelled TaskStatus = "CANCELLED"
)

type ExportTask struct {
    ID          uint        `gorm:"primaryKey" json:"id"`
    RouteID     uint        `gorm:"index;not null" json:"route_id"`
    Status      TaskStatus  `gorm:"size:32;not null;default:'PENDING'" json:"status"`
    Progress    float64     `gorm:"default:0" json:"progress"`       // 0.0 ~ 1.0
    OutputPath  string      `gorm:"size:512" json:"output_path"`     // 生成的 MP4 路径
    ErrorMessage string     `gorm:"type:text" json:"error_message"`
    CreatedAt   time.Time   `json:"created_at"`
    UpdatedAt   time.Time   `json:"updated_at"`
}
```

**队列实现**：简单 In-Memory Queue（单实例），用 `sync.Mutex` 保护。
- MVP 阶段单用户，不需要 Redis
- 后续如果需要分布式，换成 Redis + worker 模式

### 3.3 Tile Preload 策略

**问题**：200km 路线可能覆盖数百个 tile zoom level，如果导出前没有预加载，播放时会出现大片空白。

**策略**：
```
Export Preflight 阶段：
1. 解析 ExportTimeline，提取所有相机 pose
2. 对每个 pose，计算对应的 tile 坐标（z/x/y）
3. 去重 → 得到需要的所有 tile 集合
4. 检查 tile_cache 中已有的 tile
5. 缺失的 tile → 发起并行下载（并发数 = 10）
6. 下载完成后 → 开始导出
```

**预加载范围**：
- 只预加载 ExportTimeline 中实际会用到的 tile
- 不在播放阶段预加载（播放时按需加载，接受短暂闪烁）
- 导出阶段必须全量预加载（保证视频无空白）

### 3.4 瓦片缓存增强

迭代 1 的瓦片缓存只支持"查缓存 → 未命中则下载"。
迭代 3 需要增加：

```go
// internal/tile/cache.go

type TileCache struct {
    baseDir string
    // 内存中维护最近使用的 tile 元数据
    recentTiles map[string]tileMeta
}

type tileMeta struct {
    path     string
    z, x, y  int
    size     int64
    fetchedAt time.Time
}

// 新增方法
func (c *TileCache) GetMissingTiles(required []TileKey) []TileKey
func (c *TileCache) ParallelDownload(tiles []TileKey, concurrency int) error
func (c *TileCache) CheckCompleteness(required []TileKey) (missing int, total int)
```

---

## 四、后端详细设计

### 4.1 新增路由

```
POST   /api/routes/:id/export           创建导出任务
GET    /api/tasks/:id                   查询任务状态 + 进度
GET    /api/tasks/:id/download          下载生成的 MP4
DELETE /api/tasks/:id                   取消任务（仅 PENDING 状态）
GET    /api/tasks                       列出当前用户的所有任务
```

### 4.2 导出服务

```go
// internal/export/service.go

package export

type ExportService struct {
    taskQueue   *TaskQueue
    tileCache   *tile.TileCache
    renderer    VideoRenderer
}

// ExportRequest 前端发送的导出参数
type ExportRequest struct {
    Resolution    string  `json:"resolution"`    // "1920x1080"
    FPS           int     `json:"fps"`           // 30
    Quality       string  `json:"quality"`       // "high" | "medium" | "low"
    IncludeOverlay bool   `json:"include_overlay"`
    // Timeline 数据（从前端序列化发送）
    Timeline      json.RawMessage `json:"timeline"`
}

// ExportResult
type ExportResult struct {
    TaskID     uint   `json:"task_id"`
    Status     string `json:"status"`
    DownloadURL string `json:"download_url,omitempty"`
}

func (s *ExportService) CreateTask(routeID uint, req ExportRequest) (*ExportResult, error)
func (s *ExportService) GetTask(taskID uint) (*ExportTask, error)
func (s *ExportService) CancelTask(taskID uint) error
func (s *ExportService) runTask(task *ExportTask) error
```

### 4.3 VideoRenderer 接口

```go
// internal/export/renderer.go

package export

type VideoRenderer interface {
    Prepare(ctx context.Context, task *ExportTask, timeline []byte, resolution [2]int, fps int) error
    RenderFrame(ctx context.Context, task *ExportTask, frameIndex int) error
    Start(ctx context.Context, task *ExportTask) error
    Stop(ctx context.Context, task *ExportTask) error
    Export(ctx context.Context, task *ExportTask) (string, error) // 返回 MP4 路径
}

// BrowserVideoRenderer — 第一版实现
// 原理：前端通过 WebSocket 发送每一帧的相机状态，后端渲染并编码
// 注意：这实际上是"半后端"方案，前端仍需参与渲染
// 
// 更简单的实现：前端录制 WebM，后端转码 MP4
type BrowserVideoRenderer struct {
    ffmpegPath string  // FFmpeg 可执行文件路径
}

// FFmpegVideoRenderer — 完整版（纯后端）
// 原理：后端直接调用 MapLibre 的 offscreen canvas API 渲染每一帧
// 需要：Node.js + canvas 包 + maplibre-gl 的 offscreen 支持
type FFmpegVideoRenderer struct {
    nodeBin     string
    tempDir     string
}
```

**第一版推荐实现**：前端录制 WebM → 后端 FFmpeg 转 MP4

理由：
1. 实现简单，不需要 offscreen canvas 后端
2. 前端已有完整的 MapLibre 渲染能力
3. FFmpeg 转码是轻量操作（只是改变容器和编码）
4. 可以满足 MP4 输出需求

### 4.4 ExportPreflight 服务

```go
// internal/export/preflight.go

type PreflightResult struct {
    OK                bool     `json:"ok"`
    MissingTiles      int      `json:"missing_tiles"`
    TotalTiles        int      `json:"total_tiles"`
    EstimatedDuration string   `json:"estimated_duration"`
    NeedsPreload      bool     `json:"needs_preload"`
    Warnings          []string `json:"warnings,omitempty"`
}

func CheckPreflight(routeID uint) (*PreflightResult, error)
func PreloadTiles(routeID uint) error
```

**Preflight 流程**：
1. 读取该 Route 的 ExportTimeline
2. 解析 Timeline 中的所有相机 pose
3. 对每个 pose 计算 tile 覆盖范围（zoom -2 到 zoom +1，确保边缘覆盖）
4. 统计去重后的 tile 集合
5. 与 tile_cache 对比，找出缺失的 tile
6. 返回 PreflightResult

**Preflight 阈值**：
- missing_tiles / total_tiles < 5% → OK，可以直接导出
- 5% ~ 20% → 警告，建议先 Preload
- > 20% → 阻止导出，必须先 Preload

### 4.5 前端 → 后端导出流程

```
用户点击"导出"
    ↓
前端：执行 Export Preflight（调用 /api/routes/:id/export/preflight）
    ↓
如果缺失瓦片 > 阈值：
    前端：提示用户"正在预加载地图资源..."
    前端：调用 /api/routes/:id/export/preload
    前端：等待 preload 完成
    ↓
前端：启动录制（MediaRecorder on canvas）
    前端：按 ExportTimeline 驱动相机播放（不调用普通播放，用专用导出模式）
    前端：每帧回调中发送相机状态到后端（可选，用于后端记录）
    ↓
录制完成 → WebM Blob
    ↓
前端：上传 WebM → POST /api/routes/:id/export/upload
    ↓
后端：FFmpeg 转码 WebM → MP4
    后端：返回 MP4 下载链接
    ↓
前端：提供下载
```

**简化版（推荐第一版）**：
```
前端直接录制成 WebM，下载后提示用户自行用在线工具转 MP4
或者：前端上传 WebM，后端 FFmpeg 转 MP4 后提供下载
```

### 4.6 视频 Overlay 数据

导出时叠加的数据层：
```typescript
interface VideoOverlay {
    distance: number        // 当前距离（km）
    elevation: number       // 当前海拔（m）
    slope: number           // 当前坡度（%）
    segmentType?: string    // 当前 Segment 类型
    segmentCommentary?: string // 当前 Segment 解说
    totalAscent?: number    // 累计爬升
    totalDescent?: number   // 累计下降
}
```

Overlay 渲染在 canvas 上（MapLibre 的 overlay DOM 或 Canvas 层），录制时自动包含。

---

## 五、前端详细设计

### 5.1 新增组件

```
src/components/
├── export/
│   ├── ExportPanel.vue       # 导出面板（分辨率/帧率/质量选择）
│   ├── ExportProgress.vue    # 导出进度条 + 状态
│   └── ExportHistory.vue     # 历史导出任务列表
└── overlay/
    └── VideoOverlay.vue      # 视频叠加层（距离/海拔/坡度/Segment）
```

### 5.2 ExportTimeline 数据结构

```typescript
// frontend/src/types/timeline.ts

interface ExportTimeline {
    version: string           // "1.0"
    routeId: number
    createdAt: string
    segments: TimelineSegment[]
    events: TimelineEvent[]
    intro: IntroConfig
    outro: OutroConfig
    settings: ExportSettings
}

interface TimelineSegment {
    segmentId: number
    startTime: number     // 秒
    endTime: number       // 秒
    startDistance: number // 米
    endDistance: number   // 米
}

interface TimelineEvent {
    eventId: number
    type: string
    startTime: number     // 秒（相对 Timeline 起点）
    holdBefore: number    // 秒
    holdAfter: number     // 秒
    ttsDuration: number   // 秒
}

interface ExportSettings {
    resolution: [number, number]  // [1920, 1080]
    fps: number                   // 30
    quality: 'low' | 'medium' | 'high'
    includeOverlay: boolean
}
```

### 5.3 导出模式下的播放控制

普通播放模式和导出模式共用同一个 RoutePlayer，但导出模式有以下不同：

```typescript
// src/engines/route-player/index.ts（扩展）

class RoutePlayer {
    // 导出模式：完全按照 Timeline 的时间轴播放，不依赖 real-time
    isExportMode = false
    exportTimeline: ExportTimeline | null = null

    // 导出模式播放
    async playForExport(timeline: ExportTimeline, onFrame?: (frame: FrameData) => void): Promise<void>

    // 普通模式播放（迭代 1 已有）
    play(): Promise<void>
}
```

**导出模式的核心差异**：
- 不使用 `requestAnimationFrame` 驱动（受浏览器帧率影响）
- 使用 `setInterval` 或 `setTimeout` 按固定间隔推进（如 30fps = 33.33ms/帧）
- 每一帧的相机状态从 Timeline 精确计算，不依赖上一帧
- 可选：每帧回调通知前端（用于前端录制同步）

### 5.4 录制实现

```typescript
// src/engines/video-recorder/index.ts

class VideoRecorder {
    private canvas: HTMLCanvasElement
    private mediaRecorder: MediaRecorder | null = null
    private chunks: Blob[] = []

    async start(recordingCanvas: HTMLCanvasElement, mimeType?: string): Promise<void>
    stop(): Promise<Blob>  // 返回 WebM Blob
    getSupportedMimeType(): string
}
```

**录制 + 导出的完整前端流程**：
```typescript
async function exportVideo(routeId: number, settings: ExportSettings) {
    // 1. Preflight
    const preflight = await api.checkPreflight(routeId)
    if (!preflight.ok) {
        await api.preloadTiles(routeId)
    }

    // 2. 构建 ExportTimeline
    const timeline = buildExportTimeline(routeId)

    // 3. 启动录制
    const recorder = new VideoRecorder()
    await recorder.start(mapCanvas)

    // 4. 按 Timeline 播放
    const player = getRoutePlayer()
    await player.playForExport(timeline, (frame) => {
        // 可选：每帧回调，用于调试
    })

    // 5. 停止录制
    const webmBlob = await recorder.stop()

    // 6. 上传到后端转码
    const result = await api.uploadForExport(routeId, webmBlob)

    return result.downloadUrl
}
```

---

## 六、性能目标

### 6.1 路线长度 vs 导出时间

| 路线长度 | 估计视频时长（1x 真实速度） | 估计导出时间（8Mbps, 30fps, 1080p） |
|----------|---------------------------|-------------------------------------|
| 10km | ~3 min | ~1 min |
| 50km | ~15 min | ~3 min |
| 100km | ~30 min | ~6 min |
| 200km | ~60 min | ~12 min |

**注**：实际导出时间取决于视频时长 × 压缩比。FFmpeg 实时编码通常 0.5x - 2x 实时速度。

### 6.2 Tile 缓存规模

| 路线长度 | 估计覆盖 tile 数（z8-z15） | 估计缓存大小 |
|----------|--------------------------|-------------|
| 10km | ~200 | ~50 MB |
| 50km | ~800 | ~200 MB |
| 100km | ~1,500 | ~400 MB |
| 200km | ~3,000 | ~800 MB |

**注意**：tile_cache 目录应该加入 `.gitignore`，不提交到仓库。

---

## 七、验收标准

### 功能验收

- [ ] 点击导出，系统检查 Tile Cache 完整性
- [ ] Tile 缺失时自动预加载（显示进度）
- [ ] Preflight 检查结果正确（missing_tiles 数量准确）
- [ ] 前端录制 WebM 正常，无黑屏/卡顿
- [ ] 后端成功接收 WebM 并转码为 MP4
- [ ] MP4 下载链接有效，可播放
- [ ] Overlay 数据正确叠加在视频上（距离/海拔/坡度）
- [ ] 导出任务状态可查询（PENDING → RUNNING → SUCCESS）
- [ ] 长路线（100km+）导出成功，无内存溢出
- [ ] 在线播放和导出使用同一个 Timeline 数据

### 数据验收

- [ ] 导出的 MP4 中，相机位置与 Timeline 完全一致（逐帧对比）
- [ ] TTS Duration 正确进入 Timeline（如果使用了 TTS）
- [ ] Intro/Outro 在视频中正确呈现

### 性能验收

- [ ] 10km 路线导出 < 2 min
- [ ] 50km 路线导出 < 5 min
- [ ] Preflight + Preload 100km 路线 < 2 min（网络良好）
- [ ] 导出过程中前端 CPU < 80%，内存 < 500MB

---

## 八、第三方项目借鉴来源

| 功能 | 参考项目 | 具体文件 | 借鉴内容 |
|------|----------|----------|----------|
| 视频录制 | GPX_3D | `static/js/recorder.js` → `VideoRecorder` | MediaRecorder API 用法、mimeType 检测、FFmpeg WASM fallback |
| 确定性导出 | TrailReplay | `app/src/components/playback/PlaybackProvider.tsx` → `isDeterministicExport` | 导出模式与普通播放模式分离的设计 |
| Tile 预热 | TrailReplay | `app/src/components/map/hooks/useTilePreload.ts` | `preloading` 阶段设计、安全超时机制 |
| 导出质量估算 | TrailReplay | `app/src/utils/videoExport.ts` → `estimateFileSize` | 基于 bitrate 估算文件大小的公式 |
| 帧率控制 | GPX_3D | `static/js/recorder.js` → `recordFrameByFrame()` | 离屏 canvas + captureStream(0) 的帧级控制思路 |

---

## 九、风险与注意事项

### 风险 1：WebM 转 MP4 的兼容性

**现象**：某些浏览器不支持 `video/mp4` MIME type，MediaRecorder 只能输出 WebM。
**对策**：
1. 优先尝试 `video/mp4;codecs=h264`（Safari 支持）
2. 其次 `video/webm;codecs=vp9`（Chrome/Edge 支持）
3. 最后 `video/webm`（通用 fallback）
4. 后端 FFmpeg 统一转码为 MP4（H.264 + AAC），确保跨平台兼容性

### 风险 2：长视频录制内存压力

**现象**：60 分钟的视频录制，WebM Blob 可能达到 2-3GB，浏览器内存可能撑不住。
**对策**：
1. 分片段录制（每 5 分钟一个 chunk），最后合并
2. 或者：直接在后端用 FFmpeg 接收流式数据
3. 第一版先实现完整录制，如果内存问题出现再拆分

### 风险 3：FFmpeg 后端依赖

**现象**：服务器需要安装 FFmpeg，不同发行版安装方式不同。
**对策**：
1. 提供 Docker 镜像，内置 FFmpeg
2. 提供 `scripts/install-ffmpeg.sh` 自动安装脚本
3. 在 `README.md` 中明确 FFmpeg 版本要求（>= 4.0）

---

## 十、开发顺序

```
1. 后端：ExportTask 数据模型 + 任务队列
2. 后端：Export API（create/get/cancel）
3. 后端：Preflight 服务（分析 Timeline 需要的 tile）
4. 后端：Tile 并行下载 + 缓存
5. 后端：WebM 上传 + FFmpeg 转码
6. 前端：ExportPanel 组件
7. 前端：VideoRecorder 类（MediaRecorder 封装）
8. 前端：ExportTimeline 构建器
9. 前端：VideoOverlay 组件
10. 前端：导出完整流程联调
11. 性能测试（10km / 50km / 100km）
```

---

## 十一、交付物

- 新增后端服务：export、preflight
- 新增前端组件：ExportPanel、ExportProgress、ExportHistory、VideoOverlay
- 新增引擎：VideoRecorder、ExportTimelineBuilder
- `docs/iterations/iteration-03-video-export-preload.md`（本文档）
