# 迭代 1：GPX → 3D Terrain → 路线跟随播放

> **状态**：待开发
> **目标**：上传真实 GPX，在 3D 地形中沿路线播放，具备完整的 Play / Pause / Seek / Speed 控制
> **前置迭代**：无
> **影响迭代**：迭代 2、迭代 3（所有后续迭代依赖本迭代的 Route / TrackPoint / CameraEngine）

---

## 一、迭代目标

完成系统的**核心播放链路**：

```
GPX 文件
    ↓
Go 后端解析 → Route + TrackPoint
    ↓
前端 MapLibre 3D 地形渲染路线
    ↓
相机沿路线平滑跟随播放
    ↓
实时显示距离 / 海拔 / 坡度 / 爬升 / 下降
```

**一句话验收**：上传 `map/北京100-lite.gpx`，点击播放，能看到 3D 地形上的路线动画，相机不穿地，转弯平滑，可调节 Speed / Camera Height / Camera Pitch。

---

## 二、不做的事

- 不做分段分析
- 不做 Story Timeline
- 不做 AI 解说 / TTS
- 不做 Intro / Outro
- 不做视频导出
- 不做用户系统

---

## 三、技术决策

### 3.1 技术栈确认

| 层次 | 技术 | 说明 |
|------|------|------|
| 后端 | Go + Gin + GORM + SQLite | 参考 share-gpx，纯 Go SQLite（modernc.org/sqlite），无 CGo 依赖 |
| 前端 | Vue 3 + TypeScript + Vite | 参考 TrailReplay 的模块划分 |
| 地图 | MapLibre GL JS（npm 包） | 不是 CDN，需要完整 ES Module + offscreen canvas 支持 |
| 瓦片源 | AWS Terrarium（DEM）+ ESRI Satellite | 通过 Go 后端代理，自带缓存 |
| 图表 | ECharts（按需引入） | 仅用于迭代 2 的海拔曲线，迭代 1 不引入 |
| 状态管理 | Pinia | Vue 3 官方推荐，替代 Redux/Zustand 思路 |

### 3.2 项目结构（两个独立仓库）

```
gxp-map-video-backend/          # Go 后端
├── cmd/server/
│   └── main.go
├── internal/
│   ├── gpx/                    # GPX 解析
│   ├── route/                  # Route 模型 + 分析
│   ├── tile/                   # 瓦片代理 + 缓存
│   ├── api/                    # HTTP handler
│   └── db/                     # SQLite 初始化 + migration
├── uploads/                    # GPX 文件存储（gitignore）
├── tile_cache/                 # 瓦片缓存（gitignore）
├── go.mod
└── go.sum

gxp-map-video-frontend/         # Vue 3 前端
├── src/
│   ├── engines/
│   │   ├── map-engine/         # MapLibre 初始化 + 管理
│   │   ├── route-player/       # 播放控制（Play/Pause/Seek）
│   │   └── camera-engine/      # 相机跟随 + 参数
│   ├── stores/
│   │   ├── route.ts            # Route 数据状态
│   │   └── playback.ts         # 播放状态
│   ├── components/
│   │   ├── map/                # 地图容器组件
│   │   ├── player/             # 播放控件
│   │   └── stats/              # 实时数据显示
│   ├── types/
│   │   └── gpx.ts              # GPX / Route / TrackPoint 类型
│   ├── api/                    # HTTP 请求封装
│   └── App.vue
├── vite.config.ts              # proxy 配置 → localhost:8080
├── package.json
└── tsconfig.json
```

### 3.3 联调方式

- 开发时：Vite proxy 将 `/api/*` 转发到 Go 后端 `localhost:8080`
- 前端端口：`localhost:5173`
- 后端端口：`localhost:8080`
- 生产部署：前后端独立，通过环境变量配置 API 地址

### 3.4 瓦片缓存方案

```
前端 MapLibre
    ↓ GET /api/tiles/{provider}/{z}/{x}/{y}.png
Go Gin Handler
    ↓ 查磁盘 tile_cache/{provider}/{z}/{x}/{y}.png
    ↓ 命中 → 直接返回
    ↓ 未命中 → 转发到源站 → 写磁盘 → 返回
```

- 缓存目录：`tile_cache/`（gitignore，启动时自动创建）
- 过期策略：不自动过期，保留所有已下载瓦片
- 清理：手动删除或 `go run cmd/server/main.go clean-caches`

**数据来源**：
- Terrain DEM：`https://s3.amazonaws.com/elevation-tiles-prod/terrarium/{z}/{x}/{y}.png`
- Satellite：`https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{x}/{y}`
- 两者均无需 API Key

### 3.5 步幅跳跃（迭代 1 不实现，但预留接口）

迭代 1 只做**实时速度播放**（1x = 真实行进速度）。
迭代 2 将引入**步幅跳跃模式**（压缩比），这里先设计好接口，不实现逻辑。

```typescript
// 前端 playback store 中预留
interface PlaybackConfig {
    mode: 'realtime' | 'step-jump'  // 'step-jump' 留接口，迭代 2 实现
    speed: number                    // 0.25 / 0.5 / 1 / 2 / 5 / 10
    cameraAltitude: number           // 米
    cameraPitch: number              // 度
}
```

---

## 四、后端详细设计

### 4.1 数据模型

```go
// internal/route/models.go

// Route 代表一条上传的 GPX 路线
type Route struct {
    ID              uint      `gorm:"primaryKey" json:"id"`
    Name            string    `gorm:"size:255;not null" json:"name"`
    GPXPath         string    `gorm:"size:512;not null" json:"gpx_path"`
    TotalDistance   float64   `gorm:"type:decimal(10,3);not null" json:"total_distance"`
    TotalAscent     float64   `gorm:"type:decimal(8,1);not null" json:"total_ascent"`
    TotalDescent    float64   `gorm:"type:decimal(8,1);not null" json:"total_descent"`
    MinElevation    float64   `gorm:"type:decimal(8,1);not null" json:"min_elevation"`
    MaxElevation    float64   `gorm:"type:decimal(8,1);not null" json:"max_elevation"`
    StartLat        float64   `gorm:"type:decimal(10,7);not null" json:"start_lat"`
    StartLng        float64   `gorm:"type:decimal(10,7);not null" json:"start_lng"`
    EndLat          float64   `gorm:"type:decimal(10,7);not null" json:"end_lat"`
    EndLng          float64   `gorm:"type:decimal(10,7);not null" json:"end_lng"`
    PointCount      int       `gorm:"not null" json:"point_count"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}

// TrackPoint 代表路线上的一个点
type TrackPoint struct {
    ID         uint    `gorm:"primaryKey" json:"id"`
    RouteID    uint    `gorm:"index;not null" json:"route_id"`
    Index      int     `gorm:"not null" json:"index"`
    Latitude   float64 `gorm:"type:decimal(10,7);not null" json:"latitude"`
    Longitude  float64 `gorm:"type:decimal(10,7);not null" json:"longitude"`
    Elevation  float64 `gorm:"type:decimal(6,1);not null" json:"elevation"`
    Distance   float64 `gorm:"type:decimal(10,3);not null" json:"distance"`
    Slope      float64 `gorm:"type:decimal(6,2)" json:"slope"`
    Time       *time.Time `json:"time"`
    Speed      float64 `gorm:"type:decimal(6,2)" json:"speed"`
}
```

### 4.2 GPX 解析服务

```go
// internal/gpx/parse.go

package gpx

import (
    "encoding/xml"
    "math"
    "time"
)

// parseGPX 解析 GPX XML，返回原始点列表
func Parse(data []byte) ([]RawPoint, error)

type RawPoint struct {
    Lat, Lng, Ele float64
    Time          *time.Time
}
```

**计算逻辑（参考 share-gpx gpx/parse.go）：**

```
距离计算：haversine(lat1, lon1, lat2, lon2)
坡度计算：slope = Δelevation / horizontal_distance
海拔平滑：moving average（窗口大小可配置，默认 5 点）
累计爬升：sum(max(0, ele[i] - ele[i-1]))
累计下降：sum(max(0, ele[i-1] - ele[i]))
```

**坡度平滑策略**：
- 使用 5 点 moving average 对原始 elevation 平滑
- 再用平滑后的 elevation 计算 slope
- 避免 GPS 海拔噪声导致坡度跳变

### 4.3 路由设计

```
POST   /api/routes              上传 GPX，解析并保存 Route + TrackPoint
GET    /api/routes              列出所有路线（分页）
GET    /api/routes/:id          获取路线详情（含统计信息）
DELETE /api/routes/:id          删除路线及其 TrackPoint
GET    /api/routes/:id/points   获取该路线的所有 TrackPoint（用于前端渲染）

GET    /api/tiles/{provider}/{z}/{x}/{y}.png   瓦片代理 + 缓存
```

### 4.4 关键 API 响应格式

**POST /api/routes 上传成功后返回：**
```json
{
  "id": 1,
  "name": "北京100-lite",
  "total_distance": 42.356,
  "total_ascent": 856.2,
  "total_descent": 842.7,
  "min_elevation": 45.3,
  "max_elevation": 912.5,
  "start_lat": 40.0123,
  "start_lng": 116.2345,
  "end_lat": 40.0567,
  "end_lng": 116.3456,
  "point_count": 8523,
  "created_at": "2026-08-27T10:00:00Z"
}
```

**GET /api/routes/:id/points 返回：**
```json
[
  { "index": 0, "latitude": 40.0123, "longitude": 116.2345, "elevation": 45.3, "distance": 0.000, "slope": 0.00 },
  { "index": 1, "latitude": 40.0125, "longitude": 116.2350, "elevation": 47.1, "distance": 0.058, "slope": 3.10 },
  ...
]
```

---

## 五、前端详细设计

### 5.1 核心模块

#### map-engine（地图引擎）

```typescript
// src/engines/map-engine/index.ts

class MapEngine {
    map: maplibregl.Map | null = null

    async init(container: string, center: [number, number], zoom: number)
    loadRoute(coordinates: [number, number, number][])  // [lng, lat, ele]
    fitToRoute(bounds: LngLatBounds)
    getCanvas(): HTMLCanvasElement
    destroy()
}
```

**地图样式配置**（参考 GPX_3D static/js/map.js）：
```typescript
const MAP_STYLE = {
    version: 8,
    sources: {
        'base-map': {
            type: 'raster',
            tiles: ['/api/tiles/esri-satellite/{z}/{x}/{y}.png'],
            tileSize: 256,
        },
        'terrain-source': {
            type: 'raster-dem',
            tiles: ['/api/tiles/terrain/{z}/{x}/{y}.png'],
            encoding: 'terrarium',
            tileSize: 256,
            maxzoom: 15,
        }
    },
    layers: [
        { id: 'base-layer', type: 'raster', source: 'base-map' }
    ],
    terrain: { source: 'terrain-source', exaggeration: 1.5 }
}
```

#### camera-engine（相机引擎）

```typescript
// src/engines/camera-engine/index.ts

interface CameraState {
    center: [number, number]    // [lng, lat]
    zoom: number
    pitch: number
    bearing: number
}

class CameraEngine {
    private state: CameraState
    private settings: CameraSettings

    // 平滑插值参数（参考 TrailReplay cameraUtils.ts）
    bearingSmoothingFactor = 0.06
    bearingHistorySize = 20
    altitudeSmoothingFactor = 0.05

    setTarget(point: TrackPoint, lookAheadIndex: number): CameraState
    smoothBearing(current: number, target: number): number
    apply(map: maplibregl.Map, duration?: number)
}
```

**关键算法**：

```typescript
// bearing 计算（参考 GPX_3D animation.js）
function calculateBearing(from: Point, to: Point): number {
    const lat1 = from.lat * Math.PI / 180
    const lat2 = to.lat * Math.PI / 180
    const dLon = (to.lng - from.lng) * Math.PI / 180
    const y = Math.sin(dLon) * Math.cos(lat2)
    const x = Math.cos(lat1) * Math.sin(lat2) -
              Math.sin(lat1) * Math.cos(lat2) * Math.cos(dLon)
    return ((Math.atan2(y, x) * 180 / Math.PI) + 360) % 360
}

// bearing 平滑 + 角度包裹（参考 TrailReplay smoothBearing）
function smoothBearing(current, target, factor, deadband = 4): number {
    let diff = target - current
    if (diff > 180) diff -= 360
    if (diff < -180) diff += 360
    if (Math.abs(diff) < deadband) return current
    const maxChange = 0.85
    const change = Math.max(-maxChange, Math.min(maxChange, diff * factor))
    return (current + change + 360) % 360
}
```

#### route-player（播放引擎）

```typescript
// src/engines/route-player/index.ts

class RoutePlayer {
    private points: TrackPoint[]
    private cumulativeDistances: number[]  // 预计算累计距离
    private totalDistance: number
    private currentIndex = 0
    private isPlaying = false
    private speed = 1  // m/s 的实际速度倍数

    // 距离匀速播放：相同距离间隔用相同时间
    // 而不是相同时间间隔用相同距离（那样陡坡会快、平路会慢）
    async play(startIndex?: number): Promise<void>
    pause()
    resume()
    stop()
    seekTo(distance: number): void
    seekToIndex(index: number): void
    getCurrentPoint(): TrackPoint | null
    getProgress(): number  // 0.0 ~ 1.0
}
```

**播放循环核心**（参考 GPX_3D animation.js FlyoverAnimation）：
```
每帧：
    elapsed = now - lastTime
    distanceDelta = speed * elapsed * playbackSpeedMultiplier
    currentIndex = findIndexAtDistance(currentDistance + distanceDelta)
    targetPoint = points[currentIndex]
    lookAheadPoint = points[min(currentIndex + lookAheadN, length-1)]
    cameraState = cameraEngine.setTarget(targetPoint, lookAheadPoint)
    cameraEngine.apply(map)
    updateStatsDisplay(targetPoint)
```

### 5.2 组件设计

```
src/components/
├── map/
│   ├── MapContainer.vue          # 地图容器，持 useRef<MapEngine>
│   └── RouteLayer.vue            # 路线渲染（GeoJSON line layer）
├── player/
│   ├── PlaybackControls.vue      # Play/Pause/Seek/Speed 控件
│   └── CameraControls.vue        # Height/Pitch 滑块
├── stats/
│   ├── StatsPanel.vue            # 右侧信息面板（距离/海拔/坡度等）
│   └── StatsBar.vue              # 底部浮动状态栏
└── upload/
    └── UploadZone.vue            # GPX 文件上传区域
```

### 5.3 Pinia Store

```typescript
// src/stores/route.ts
export const useRouteStore = defineStore('route', {
    state: () => ({
        currentRoute: null as Route | null,
        trackPoints: [] as TrackPoint[],
        loading: false,
        error: null as string | null,
    }),
    actions: {
        async uploadGPX(file: File): Promise<Route>
        async fetchPoints(routeId: number): Promise<TrackPoint[]>
    }
})

// src/stores/playback.ts
export const usePlaybackStore = defineStore('playback', {
    state: () => ({
        isPlaying: false,
        progress: 0,          // 0.0 ~ 1.0
        speed: 1,             // 倍速
        cameraAltitude: 100,  // 米
        cameraPitch: 60,      // 度
        currentPoint: null as TrackPoint | null,
    }),
    actions: {
        play()
        pause()
        seekTo(progress: number)
        setSpeed(speed: number)
        setCameraAltitude(alt: number)
        setCameraPitch(pitch: number)
    }
})
```

---

## 六、API 完整设计

### 6.1 类型定义

```typescript
// frontend/src/types/gpx.ts

interface RawPoint {
    lat: number
    lng: number
    ele: number
    time?: string
}

interface Route {
    id: number
    name: string
    gpx_path: string
    total_distance: number    // 米
    total_ascent: number      // 米
    total_descent: number     // 米
    min_elevation: number     // 米
    max_elevation: number     // 米
    start_lat: number
    start_lng: number
    end_lat: number
    end_lng: number
    point_count: number
    created_at: string
    updated_at: string
}

interface TrackPoint {
    id: number
    route_id: number
    index: number
    latitude: number
    longitude: number
    elevation: number
    distance: number      // 累计距离（米）
    slope: number         // 坡度百分比
    time?: string
    speed?: number
}

interface RouteStats {
    total_distance: number
    total_ascent: number
    total_descent: number
    min_elevation: number
    max_elevation: number
    avg_slope: number
    max_slope: number
    point_count: number
}
```

### 6.2 后端 API 详细

**POST /api/routes**
- 请求：multipart/form-data，字段 `file`（.gpx）
- 响应：`Route`（不含 TrackPoint 列表，点数据通过单独接口获取）
- 处理：解析 GPX → 计算统计 → 保存 Route + 批量插入 TrackPoint

**GET /api/routes/:id/points**
- 查询参数：`?limit=10000`（分页，默认全量，最大 50000）
- 响应：`TrackPoint[]`
- 用途：前端加载路线数据到内存，用于播放

**GET /api/tiles/{provider}/{z}/{x}/{y}.png**
- provider：`esri-satellite` | `terrain`
- 缓存策略：磁盘缓存，无过期

---

## 七、验收标准

### 功能验收

- [ ] 上传任意 `.gpx` 文件，后端正确解析，不报错
- [ ] 上传后前端显示路线统计（总距离、爬升、下降、最高/最低海拔）
- [ ] 3D 地形正确显示（AWS Terrarium DEM）
- [ ] 卫星底图正确显示（ESRI）
- [ ] 路线在 3D 地形上正确渲染（红色折线）
- [ ] 点击 Play，相机从起点沿路线平滑移动
- [ ] 相机不穿入地形（camera altitude >= terrain elevation + 50m clearance）
- [ ] 急转弯时 bearing 平滑过渡，不突然旋转
- [ ] Play/Pause/Stop 正常工作
- [ ] Seek（拖进度条）能跳转到任意位置
- [ ] Speed 可调（0.25x / 0.5x / 1x / 2x / 5x），立即生效
- [ ] Camera Height 可调（20m - 2000m），立即生效
- [ ] Camera Pitch 可调（0° - 85°），立即生效
- [ ] 右侧/底部实时显示当前距离、海拔、坡度

### 数据验收

- [ ] `map/北京100-lite.gpx` 解析结果与手工计算一致（距离误差 < 1%）
- [ ] 坡度计算经过平滑，不会出现相邻点坡度跳变（+30% → -30%）
- [ ] TrackPoint 全部保存进 SQLite，可查询

### 性能验收

- [ ] 10km 路线（~2000 点）上传解析 < 1s
- [ ] 50km 路线（~10000 点）上传解析 < 3s
- [ ] 播放 60fps 下帧率稳定（不丢帧）
- [ ] 内存占用合理（50km 路线 < 200MB）

---

## 八、第三方项目借鉴来源

| 功能 | 参考项目 | 具体文件 | 借鉴内容 | License |
|------|----------|----------|----------|---------|
| GPX 解析 | share-gpx | `gpx/parse.go` | haversine 距离、elevationGain、buildProfile | MIT |
| 瓦片代理缓存 | GPX_3D | `app.py` → `/tiles/{provider}/{z}/{x}/{y}` | Flask 代理逻辑移植到 Go | MIT |
| 地图初始化 | GPX_3D | `static/js/map.js` | MapLibre 3D terrain 配置、GeoJSON 路线层 | MIT |
| 相机跟随 | GPX_3D | `static/js/animation.js` → `FlyoverAnimation` | bearing 计算、look-ahead、距离匀速插值 | MIT |
| Bearing 平滑 | TrailReplay | `app/src/components/map/cameraUtils.ts` → `smoothBearing` | 角度包裹 + 死区 + 最大变化率 | MIT（需署名） |
| 地形避让 | GPX_3D | `static/js/animation.js` → `terrainClearance` | minTerrainClearance 逻辑 | MIT |

**License 合规**：
- GPX_3D：MIT — 可直接参考，无需修改代码，但需保留版权声明
- TrailReplay：MIT + 署名要求 — 参考算法实现，不能复制代码，需标注
- share-gpx：MIT — 同上

**核心原则**：参考算法和架构思路，用自己的 Vue/TypeScript/Go 重写，不直接复制代码。

---

## 九、风险与注意事项

### 风险 1：MapLibre 3D Terrain 瓦片加载延迟

**现象**：相机移动到新区域时，地形瓦片还没加载完，出现平面/空白。
**对策**：迭代 1 不处理（允许初次加载时有短暂闪烁），迭代 3 做 Tile Preload。

### 风险 2：长路线 TrackPoint 全量传输

**现象**：200km 路线 40,000 个点，JSON 约 5MB，前端一次性加载可能卡顿。
**对策**：接口增加 `?limit` 分页参数，前端按需加载（先加载前 10000 点，滚动加载更多）。迭代 1 先用全量，迭代 3 优化。

### 风险 3：SQLite 并发写入

**现象**：多个用户同时上传 GPX 时，SQLite 写锁冲突。
**对策**：单用户 MVP 阶段不需要担心。后续如需多用户，加 `PRAGMA journal_mode = WAL`（write-ahead logging），modernc.org/sqlite 支持。

---

## 十、开发顺序

```
1. Go 后端脚手架（main.go + gin 路由 + SQLite 初始化）
2. GPX 解析服务（复用 share-gpx 思路，自己写 Go 版）
3. Route / TrackPoint GORM 模型 + migration
4. POST /api/routes 上传接口
5. GET /api/routes/:id/points 接口
6. 瓦片代理 + 缓存（GET /api/tiles/...）
7. Vue 项目初始化（Vite + TypeScript + MapLibre npm 包）
8. MapEngine 初始化 + 3D 地形配置
9. 路线 GeoJSON 渲染层
10. CameraEngine（bearing 计算 + 平滑 + 地形避让）
11. RoutePlayer（播放循环 + Play/Pause/Seek/Speed）
12. 前端 UI 联调（UploadZone + PlaybackControls + StatsPanel）
13. 用真实 GPX 测试验收
```

---

## 十一、交付物

- `docs/iterations/iteration-01-gpx-3d-playback.md`（本文档）
- Go 后端完整代码（gxp-map-video-backend/）
- Vue 前端完整代码（gxp-map-video-frontend/）
- `docs/research/gpx3d.md` / `trailreplay.md` / `share-gpx.md`（Phase 0 调研文档）
- `docs/THIRD_PARTY.md`（第三方 License 记录）

---

## 十二、下一阶段准备

迭代 1 完成后，迭代 2 将基于：
- 已有的 Route / TrackPoint 数据模型
- 已有的 CameraEngine
- 新增 Segment / Event / StoryEvent 模型
- 新增分段算法服务
- 新增 Timeline 编辑器组件

**迭代 1 到迭代 2 的接口兼容**：
- Route API 不变
- TrackPoint API 不变
- 新增 `/api/routes/:id/segments` 等接口
- 前端在现有 MapEngine 基础上叠加 Segment 可视化
