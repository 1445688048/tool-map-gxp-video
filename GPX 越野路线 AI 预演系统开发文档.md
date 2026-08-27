
# GPX 越野路线 AI 预演系统
## 完整开发迭代与技术设计文档 v0.2

> 本文档用于交给 AI Coding Agent。
>
> Agent 必须先阅读、分析现有项目代码和指定的开源参考项目，再按照本文档进行技术设计和分阶段开发。
>
> 本项目不是简单的 GPX 查看器，也不是简单的 GPX 转视频工具。
>
> 项目的核心定位是：
>
> **「越野 / 徒步 / 登山路线 AI 提前探路与 3D 预演系统」**
>
> 用户上传一条 GPX 路线后，系统通过 3D Terrain 沿路线进行模拟行进，根据路线中的距离、海拔、坡度、爬升、下降、关键点等信息，对路线进行自动分析和分段，并允许用户为不同路段添加文字说明，进一步生成 TTS 解说。
>
> 最终形成一个：
>
> **“看完视频，就大致了解整条越野路线怎么走、哪里爬升、哪里下降、哪里比较陡、哪里需要注意”的 3D 路线提前探路视频。**

---

# 一、项目最终目标

完整流程：

```text
GPX
 ↓
GPX 解析
 ↓
路线数据标准化
 ↓
海拔 / 坡度 / 爬升 / 下降分析
 ↓
自动路线分段
 ↓
识别关键点
 ↓
用户调整 Segment / Event
 ↓
AI 生成路线解说
 ↓
用户修改解说
 ↓
TTS
 ↓
生成 Story Timeline
 ↓
3D Route Player
 ↓
Intro 高空快速下降
 ↓
沿路线 3D 行进
 ↓
上坡 / 下坡 / 关键点自动切换镜头
 ↓
减速 / 停留
 ↓
文字 + TTS 解说
 ↓
继续路线
 ↓
到达终点
 ↓
Outro 镜头拔高
 ↓
完整路线总览
 ↓
显示路线统计
 ↓
导出视频
 ↓
MP4
````

最终用户应该获得这样的体验：

> “我还没有亲自去跑这条路线，但看完这个 3D 预演视频，我已经知道这条路线大概怎么走、总长多少、爬升多少、主要爬升在哪里、哪里是下降、哪里比较陡、哪里有关键点。”

---

# 二、核心产品定位

## 2.1 不是 GPX Viewer

普通 GPX Viewer：

```text
上传 GPX
 ↓
显示路线
 ↓
查看海拔
```

本项目：

```text
上传 GPX
 ↓
理解路线
 ↓
分析路线
 ↓
自动分段
 ↓
3D 模拟行进
 ↓
Camera 自动变化
 ↓
AI 解说
 ↓
TTS
 ↓
视频
```

---

# 三、三个开源项目的明确职责

本项目参考三个开源项目：

1. GPX_3D
2. TrailReplay
3. share-gpx

**绝对不要理解为把三个项目直接合并。**

三个项目的职责必须明确区分。

---

# 四、GPX_3D —— 核心 3D 技术底座

项目：

[https://github.com/stefanrattay1/GPX_3D](https://github.com/stefanrattay1/GPX_3D)

GPX_3D 是本项目最重要的技术参考。

## 4.1 主要借鉴

重点研究：

```text
MapLibre GL JS
3D Terrain
DEM / Elevation
Satellite / Raster
GPX Route
Route Follow
Camera Follow
Camera Height
Camera Pitch
Bearing
Playback
Playback Speed
Terrain
Tile Loading
Tile Cache
Browser Recording
Video Export
```

---

## 4.2 3D 地图

最终项目使用：

```text
Vue 3
+
TypeScript
+
MapLibre GL JS
+
3D Terrain
```

重点参考 GPX_3D：

```text
MapLibre 初始化
Terrain 配置
DEM Source
Raster/Satellite Source
Layer 管理
Camera 控制
```

不要直接把 GPX_3D 整个项目复制进来。

需要理解源码后重新组织。

---

## 4.3 Route Follow

重点参考 GPX_3D 的：

```text
Current Track Point
+
Look Ahead Point
+
Bearing
+
Pitch
+
Camera Position
```

最终重新实现：

```text
RoutePlayer
+
CameraEngine
```

---

## 4.4 Camera

GPX_3D 是本项目 Camera 的主要参考来源。

至少支持：

```text
Camera Height
Camera Pitch
Camera Bearing
Camera Distance
Look Ahead
```

用户必须可以在播放过程中实时调整：

```text
Speed
Camera Height
Camera Pitch
```

修改后立即生效。

---

## 4.5 Playback

参考 GPX_3D：

```text
Play
Pause
Stop
Seek
Speed
Progress
```

建议速度：

```text
0.25x
0.5x
1x
2x
5x
10x
```

具体范围由 Agent 根据实际实现决定。

---

## 4.6 Terrain

重点研究：

```text
Terrain Elevation
Terrain Exaggeration
Camera Height
Terrain Collision / Clearance
```

必须避免：

```text
Camera
 ↓
Terrain
```

即镜头钻入山体。

设计：

```text
Camera Height
>=
Terrain Elevation + Minimum Clearance
```

---

## 4.7 Tile Cache

研究 GPX_3D 的：

```text
Tile Loading
Tile Cache
```

最终增强为：

```text
Tile Cache
+
Tile Preload
+
Export Preflight
```

---

## 4.8 Browser Recording

研究 GPX_3D 的：

```text
Canvas
MediaRecorder
WebCodecs
```

最终统一封装：

```ts
VideoRenderer
```

未来允许替换成：

```text
Browser Renderer
FFmpeg Renderer
Headless Browser
GPU Renderer
```

---

# 五、TrailReplay —— 视频表现和叙事参考

项目：

[https://github.com/alexalmansa/TrailReplay](https://github.com/alexalmansa/TrailReplay)

TrailReplay 不作为核心 3D 地图引擎。

主要借鉴：

```text
Intro
Outro
Annotation
Route Overview
Route Statistics
Storytelling
Video Export
```

---

# 六、TrailReplay Intro

需要重点借鉴：

```text
高空
 ↓
快速下降
 ↓
进入路线区域
 ↓
逐渐减速
 ↓
对准路线起点
 ↓
开始路线
```

最终实现：

```text
IntroScene
```

参数化：

```ts
interface IntroConfig {
    duration: number

    startAltitude: number
    endAltitude: number

    startPitch: number
    endPitch: number

    startBearing?: number
    endBearing?: number

    easing: string
}
```

不要写死参数。

---

# 七、TrailReplay Outro

需要重点借鉴：

```text
到达终点
 ↓
减速
 ↓
Camera 上升
 ↓
Camera 后移
 ↓
完整路线显示
 ↓
路线统计
```

最终实现：

```text
OutroScene
```

显示：

```text
Total Distance
Total Ascent
Total Descent
Highest Elevation
Lowest Elevation
```

---

# 八、TrailReplay Annotation

借鉴路线标注思路。

最终支持：

```text
POI
WARNING
VIEWPOINT
REST
JUNCTION
DIFFICULTY
CUSTOM
```

例如：

```text
6.42km
主要爬升开始

8.15km
注意岔路

11.30km
观景点

15.20km
连续下降
```

---

# 九、TrailReplay Storytelling

重点借鉴其：

> “路线不仅是 GPX 数据，也可以被组织成一个有故事的路线视频。”

最终：

```text
Route
 ↓
Segment
 ↓
Story Event
 ↓
Camera
 ↓
Commentary
 ↓
TTS
 ↓
Timeline
```

---

# 十、share-gpx —— GPX / 地图交互参考

share-gpx：

> 主要作为 GPX 数据、地图展示、路线交互、Waypoint / Marker 等方面的参考。

**不作为本项目核心 3D 地图底座。**

---

## 10.1 重点研究

```text
GPX 文件处理
Track
Waypoint
Marker
地图交互
路线信息
GPX 数据展示
```

---

## 10.2 不使用 share-gpx 作为

不要将 share-gpx 作为：

```text
3D Terrain Engine
Camera Engine
Route Player
Video Renderer
```

的核心来源。

这些功能以 GPX_3D 为主要参考。

---

# 十一、三个项目的最终职责矩阵

| 功能               | GPX_3D | TrailReplay | share-gpx | 最终项目                    |
| ---------------- | ------ | ----------- | --------- | ----------------------- |
| GPX 解析           | ⭐⭐⭐    | ⭐⭐          | ⭐⭐⭐       | 自己统一                    |
| Track            | ⭐⭐⭐    | ⭐⭐⭐         | ⭐⭐⭐       | 自己统一                    |
| Waypoint         | ⭐⭐     | ⭐⭐⭐         | ⭐⭐⭐       | 自己统一                    |
| Marker           | ⭐⭐     | ⭐⭐⭐         | ⭐⭐⭐       | 自己实现                    |
| 3D Map           | ⭐⭐⭐    | ⭐⭐⭐         | ⭐⭐        | GPX_3D                  |
| 3D Terrain       | ⭐⭐⭐    | ⭐⭐⭐         | ⭐         | GPX_3D                  |
| Elevation        | ⭐⭐⭐    | ⭐⭐⭐         | ⭐⭐        | 自己统一                    |
| Route Follow     | ⭐⭐⭐    | ⭐⭐⭐         | ⭐         | GPX_3D                  |
| Camera           | ⭐⭐⭐    | ⭐⭐          | ⭐         | GPX_3D                  |
| Camera Height    | ⭐⭐⭐    | ⭐⭐          | ❌         | GPX_3D                  |
| Camera Pitch     | ⭐⭐⭐    | ⭐⭐          | ❌         | GPX_3D                  |
| Camera Bearing   | ⭐⭐⭐    | ⭐⭐⭐         | ❌         | GPX_3D + 自己             |
| Playback         | ⭐⭐⭐    | ⭐⭐⭐         | ⭐         | GPX_3D                  |
| Playback Speed   | ⭐⭐⭐    | ⭐⭐⭐         | ❌         | GPX_3D                  |
| Tile Cache       | ⭐⭐⭐    | ⭐⭐          | ❌         | GPX_3D + 自己增强           |
| Tile Preload     | ⭐⭐     | ⭐⭐          | ❌         | 自己实现                    |
| Intro            | ⭐⭐     | ⭐⭐⭐         | ❌         | TrailReplay             |
| Outro            | ⭐⭐     | ⭐⭐⭐         | ❌         | TrailReplay             |
| Annotation       | ⭐⭐     | ⭐⭐⭐         | ⭐⭐⭐       | TrailReplay + share-gpx |
| Route Overview   | ⭐⭐⭐    | ⭐⭐⭐         | ⭐⭐        | 自己实现                    |
| Route Statistics | ⭐⭐⭐    | ⭐⭐⭐         | ⭐⭐        | 自己统一                    |
| Storytelling     | ⭐      | ⭐⭐⭐         | ❌         | 自己实现                    |
| Scene            | ⭐⭐     | ⭐⭐⭐         | ❌         | 自己实现                    |
| Timeline         | ⭐⭐     | ⭐⭐⭐         | ❌         | 自己重新设计                  |
| Segment          | ⭐⭐     | ⭐⭐          | ❌         | 自己实现                    |
| 坡度分析             | ⭐⭐     | ⭐⭐          | ❌         | 自己实现                    |
| 爬升分析             | ⭐⭐⭐    | ⭐⭐⭐         | ⭐⭐        | 自己统一                    |
| AI Commentary    | ❌      | ❌           | ❌         | 自己实现                    |
| TTS              | ❌      | ❌           | ❌         | 自己实现                    |
| AI Camera        | ❌      | ❌           | ❌         | 后期实现                    |
| Video Recording  | ⭐⭐⭐    | ⭐⭐⭐         | ❌         | 自己统一                    |
| MP4              | ⭐⭐/⭐⭐⭐ | ⭐⭐⭐         | ❌         | 自己实现                    |
| Video Overlay    | ⭐⭐     | ⭐⭐⭐         | ⭐         | 自己实现                    |

---

# 十二、核心架构

最终不要形成三个项目的拼接结构。

必须统一成：

```text
                    GPX
                     │
                     ↓
              GPX Parser
                     │
                     ↓
               Route Model
                     │
             ┌───────┴───────┐
             ↓               ↓
       Route Analyzer      Waypoints
             │
             ↓
         Segments
             │
             ↓
        Story Builder
             │
       ┌─────┼─────┐
       ↓     ↓     ↓
    Camera  TTS   Overlay
       │     │     │
       └─────┼─────┘
             ↓
        Story Timeline
             │
             ↓
        Route Player
             │
             ↓
        MapLibre 3D
             │
             ↓
       Video Renderer
             │
             ↓
            MP4
```

---

# 十三、技术栈

## Backend

```text
Go
Gin
GORM
SQLite
```

负责：

```text
GPX Parsing
Route Analysis
Segment Analysis
Project Persistence
AI
TTS
Task Queue
Export Management
```

---

## Frontend

```text
Vue 3
TypeScript
Vite
MapLibre GL JS
```

可使用：

```text
ECharts
```

实现：

```text
Elevation Profile
```

---

# 十四、核心数据模型

整个项目必须围绕：

```text
Route
TrackPoint
Segment
Event
Scene
StoryTimeline
Task
Prompt
```

设计。

不要让地图、播放器、AI、TTS 各自维护一套路线数据。

---

# 十五、Route

建议：

```go
type Route struct {
    ID uint

    Name string

    GPXPath string

    TotalDistance float64

    TotalAscent float64
    TotalDescent float64

    MinElevation float64
    MaxElevation float64

    StartLat float64
    StartLng float64

    EndLat float64
    EndLng float64

    Status string

    CreatedAt time.Time
    UpdatedAt time.Time
}
```

Agent 可以根据实际项目结构调整。

---

# 十六、TrackPoint

至少：

```go
type TrackPoint struct {
    ID uint

    RouteID uint

    Index int

    Latitude float64
    Longitude float64

    Elevation float64

    Distance float64

    Slope float64

    Time *time.Time

    Speed float64
}
```

如果原始 GPX 没有时间或速度：

```text
允许为空 / 0
```

---

# 十七、路线分析

GPX 导入后自动计算：

```text
Total Distance
Total Ascent
Total Descent
Highest Point
Lowest Point
Average Slope
Maximum Slope
Continuous Climb
Continuous Descent
```

---

# 十八、海拔平滑

GPX 的海拔数据可能存在噪声。

不能直接：

```text
point[i].elevation - point[i-1].elevation
```

作为最终坡度依据。

必须：

```text
原始 Elevation
 ↓
Smoothing
 ↓
计算局部坡度
 ↓
趋势识别
```

可以研究：

```text
Moving Average
Savitzky-Golay
Median Filter
```

Agent 根据实际数据选择。

---

# 十九、坡度计算

基础：

```text
slope =
ΔElevation /
HorizontalDistance
```

最终使用：

```text
Slope %
```

例如：

```text
+3%
+8%
+15%
-10%
-20%
```

---

# 二十、自动 Segment 分析

系统至少识别：

```text
FLAT
CLIMB
STEEP_CLIMB
DESCENT
STEEP_DESCENT
```

不要根据单个 TrackPoint 判断。

必须考虑：

```text
连续距离
平均坡度
累计爬升
累计下降
趋势
```

---

# 二十一、Segment 阈值必须可配置

不要写死：

```text
if slope > 10%
```

应该有配置：

```text
flatThreshold
climbThreshold
steepClimbThreshold
descentThreshold
steepDescentThreshold
minimumSegmentDistance
smoothingWindow
```

例如：

```text
FLAT:
-5% ~ +5%

CLIMB:
+5% ~ +12%

STEEP_CLIMB:
> +12%

DESCENT:
-5% ~ -12%

STEEP_DESCENT:
< -12%
```

具体阈值只是初始建议，Agent 必须通过真实 GPX 测试调整。

---

# 二十二、Segment 合并

避免出现：

```text
CLIMB
FLAT
CLIMB
FLAT
CLIMB
```

这种大量碎片。

必须：

```text
检测趋势
 ↓
生成初始 Segment
 ↓
删除过短 Segment
 ↓
与相邻 Segment 合并
 ↓
生成最终 Segment
```

---

# 二十三、RouteSegment

建议：

```go
type RouteSegment struct {
    ID uint

    RouteID uint

    StartIndex int
    EndIndex int

    StartDistance float64
    EndDistance float64

    StartElevation float64
    EndElevation float64

    Distance float64

    ElevationGain float64
    ElevationLoss float64

    AverageSlope float64
    MaxSlope float64

    Type string
}
```

---

# 二十四、Event

用户可以手动添加：

```text
POI
WARNING
VIEWPOINT
REST
JUNCTION
DIFFICULTY
CUSTOM
```

例如：

```json
{
    "position": 6.42,
    "type": "JUNCTION",
    "title": "重要岔路",
    "description": "这里需要注意右侧岔路"
}
```

---

# 二十五、Story Event

最终视频不是简单：

```text
Segment → 播放
```

而是：

```text
Story Timeline
```

例如：

```text
INTRO

SEGMENT 1

COMMENTARY

SEGMENT 2

COMMENTARY

WARNING

SEGMENT 3

COMMENTARY

OUTRO
```

---

# 二十六、StoryEvent

建议：

```go
type StoryEvent struct {
    ID uint

    RouteID uint

    Position float64

    SegmentID *uint

    Type string

    Title string

    Script string

    AudioPath string

    Duration float64

    CameraPreset string

    HoldBefore float64
    HoldAfter float64

    Enabled bool
}
```

---

# 二十七、Camera Engine

Camera 必须独立。

建议：

```ts
interface CameraState {
    altitude: number
    pitch: number
    bearing: number

    lookAhead: number

    distance?: number
}
```

---

# 二十八、Camera Preset

至少：

```text
FOLLOW_LOW
FOLLOW_NORMAL
FOLLOW_HIGH
BIRD_EYE
FOCUS
INTRO
OUTRO
```

---

# 二十九、不同 Segment 可以使用不同 Camera

例如：

```text
平路
Camera Height = 150m
Pitch = 55°

爬坡
Camera Height = 100m
Pitch = 65°

陡坡
Camera Height = 60m
Pitch = 70°

下降
Camera Height = 120m
Pitch = 60°
LookAhead = 300m
```

这些只是示例。

最终参数必须可调整。

---

# 三十、Camera Transition

禁止：

```text
Camera A
 ↓
瞬间
 ↓
Camera B
```

必须：

```text
Camera A
 ↓
Interpolation
 ↓
Camera B
```

支持：

```text
linear
easeIn
easeOut
easeInOut
```

---

# 三十一、路线急转弯处理

如果 GPX 出现：

```text
Bearing 10°
 ↓
Bearing 350°
```

不能直接旋转：

```text
340°
```

必须处理：

```text
Angle Wrapping
+
Bearing Smoothing
```

避免 Camera 突然旋转一整圈。

---

# 三十二、Route Player

播放器必须支持：

```text
Play
Pause
Stop
Seek
Speed
```

同时维护：

```ts
interface RoutePlaybackState {
    progress: number

    distance: number

    elevation: number

    slope: number

    segmentId?: string

    eventId?: string

    playing: boolean

    speed: number

    camera: CameraState
}
```

---

# 三十三、Playback 和 Timeline 必须解耦

实时播放：

```text
RoutePlayer
```

负责：

```text
当前路线位置
```

Timeline：

```text
StoryTimeline
```

负责：

```text
什么时候发生什么事情
```

---

# 三十四、AI Commentary

AI 不直接控制：

```text
Camera
Player
Timeline
```

AI 只负责：

```text
Route Data
 ↓
Segment
 ↓
生成 Commentary
```

例如输入：

```text
Segment:
Distance = 1.8km
Ascent = 320m
AverageSlope = 11%
MaxSlope = 19%
```

AI 输出：

```json
{
    "title": "主要爬升段",
    "script": "接下来进入本路线的主要爬升段，长度约 1.8 公里，累计爬升约 320 米。前半段坡度相对稳定，后半段会明显变陡。"
}
```

---

# 三十五、AI Prompt

Prompt 不允许硬编码在业务代码中。

建立：

```text
PromptTemplate
```

至少：

```text
route_summary
segment_commentary
poi_commentary
warning_commentary
```

后续允许后台编辑。

---

# 三十六、TTS

TTS 必须接口化：

```go
type TTSProvider interface {
    Generate(
        ctx context.Context,
        text string,
    ) (AudioResult, error)
}
```

返回：

```text
AudioPath
Duration
Format
```

---

# 三十七、MOSS-TTS-Nano

如果项目后续接入 MOSS-TTS-Nano：

第一阶段允许：

```text
MockTTSProvider
```

第二阶段：

```text
MOSS-TTS-Nano
```

原则：

> TTS Provider 必须与 Story / Timeline 解耦。

以后可以接其他本地或在线 TTS。

---

# 三十八、TTS 与 Timeline

这是核心。

不能只保存：

```text
script
```

必须得到：

```text
audio duration
```

例如：

```text
Script
 ↓
TTS
 ↓
Audio
 ↓
18.4s
 ↓
Timeline Event Duration = 18.4s
```

---

# 三十九、解说播放体验

不要简单：

```text
播放
 ↓
突然停止
 ↓
AI说话
 ↓
继续
```

建议：

```text
正常 1.0x
 ↓
0.6x
 ↓
0.2x
 ↓
Camera 调整
 ↓
0x
 ↓
TTS
 ↓
0.2x
 ↓
0.6x
 ↓
1.0x
```

形成自然的“提前探路”体验。

---

# 四十、Commentary Event

例如：

```text
Position = 6.42km

Transition In = 3s

Hold = 2s

TTS = 18s

Transition Out = 3s
```

实际：

```text
正常路线
 ↓
3秒减速
 ↓
镜头调整
 ↓
2秒停留
 ↓
18秒解说
 ↓
3秒恢复
 ↓
继续路线
```

---

# 四十一、Intro

第一版：

```text
High Altitude
 ↓
Fast Descent
 ↓
Route Area
 ↓
Camera Adjustment
 ↓
Start Point
 ↓
Route Playback
```

---

# 四十二、Outro

第一版：

```text
Finish
 ↓
Slow Down
 ↓
Camera Rise
 ↓
Camera Pull Back
 ↓
Full Route Overview
 ↓
Statistics
```

---

# 四十三、地图 Tile 加载

这是项目核心技术风险之一。

不能依赖：

```text
Camera 移动到哪里
 ↓
才开始请求 Tile
```

因为：

```text
高速播放
+
低 Camera
+
复杂 Terrain
```

可能导致：

```text
Tile 尚未加载
 ↓
地图空白
 ↓
视频出现缺失
```

---

# 四十四、Tile Preload

在线播放：

```text
当前位置
+
Look Ahead Area
```

提前加载：

```text
Terrain Tiles
Raster Tiles
```

例如：

```text
当前：
5.2km

预加载：
5.2km → 5.8km
```

具体范围根据：

```text
Speed
Camera Height
Viewport
Network
```

动态计算。

---

# 四十五、Export Preflight

导出之前：

```text
Export
 ↓
分析 Camera Path
 ↓
计算需要的 Tile
 ↓
预加载 Tile
 ↓
检查 Cache
 ↓
确认没有缺失
 ↓
开始 Render
```

如果存在缺失：

```text
Export 不应该直接开始。
```

应该：

```text
继续下载
```

或者：

```text
提示资源未准备完成
```

---

# 四十六、Tile Cache

设计：

```text
TileCache
```

支持：

```text
Memory Cache
Disk Cache
```

至少分：

```text
Terrain
Raster
```

---

# 四十七、在线播放与导出必须使用同一个 Timeline

架构：

```text
                 Story Timeline
                       │
             ┌─────────┴─────────┐
             ↓                   ↓
        Live Player         Video Renderer
             ↓                   ↓
          浏览器播放             视频
```

不能维护两套路线逻辑。

---

# 四十八、Export Timeline

导出前生成：

```text
ExportTimeline
```

包含：

```text
StartTime
EndTime
RouteProgress
CameraState
Audio
Overlay
Scene
```

例如：

```text
00:00 - 00:12 Intro

00:12 - 00:42 Route

00:42 - 01:03 Commentary

01:03 - 01:48 Route

01:48 - 02:15 Commentary

02:15 - 03:00 Route

03:00 - 03:18 Outro
```

---

# 四十九、视频导出不能依赖实时播放速度

必须：

```text
Timeline Driven
```

不能：

```text
Real Time Driven
```

例如：

```text
Event Duration = 20s
```

导出时必须保证：

```text
视频中占 20s
```

不受：

```text
机器 FPS
网络
浏览器性能
```

影响。

---

# 五十、视频 Renderer

抽象：

```ts
interface VideoRenderer {
    prepare(): Promise<void>

    renderFrame(time: number): Promise<void>

    start(): Promise<void>

    stop(): Promise<void>

    export(): Promise<Blob>
}
```

具体实现可以：

```text
BrowserVideoRenderer
```

后续：

```text
FFmpegVideoRenderer
```

---

# 五十一、MP4

第一阶段：

```text
优先验证浏览器实际能力。
```

如果浏览器可以：

```text
直接 MP4
```

则直接使用。

如果不能稳定输出：

```text
Browser Recording
 ↓
WebM
 ↓
FFmpeg
 ↓
MP4
```

---

# 五十二、不要第一版引入 Blender

第一阶段优先：

```text
Vue
+
MapLibre
+
Canvas
+
MediaRecorder / WebCodecs
```

只有当浏览器渲染无法满足：

```text
画质
稳定性
性能
导出质量
```

时，再考虑：

```text
FFmpeg
Headless Browser
GPU Renderer
Blender
```

---

# 五十三、视频 Overlay

最终视频至少显示：

```text
Distance
Elevation
Current Slope
Segment
```

可以增加：

```text
Total Ascent
Total Descent
```

---

# 五十四、Elevation Profile

底部：

```text
Elevation Chart
```

当前点：

```text
Current Position
```

必须同步：

```text
Map
+
Route
+
Elevation
+
Timeline
```

---

# 五十五、完整 UI

建议：

```text
┌────────────────────────────────────────────┐
│ Toolbar                                    │
├──────────────────────────────────┬─────────┤
│                                  │         │
│                                  │ Route   │
│             3D Map               │ Info    │
│                                  │         │
│                                  │         │
├──────────────────────────────────┴─────────┤
│ Elevation Profile                          │
├────────────────────────────────────────────┤
│ Story Timeline                             │
├────────────────────────────────────────────┤
│ ▶  1x  Height  Pitch  LookAhead  Export   │
└────────────────────────────────────────────┘
```

---

# 五十六、Segment Editor

点击 Segment：

```text
Segment #4

Type:
STEEP_CLIMB

Distance:
1.82km

Elevation Gain:
320m

Average Slope:
11.2%

Max Slope:
19.4%

Commentary:
[________________________]

TTS:
[Generate]

Camera:

Height:
[100m]

Pitch:
[65°]

LookAhead:
[250m]
```

---

# 五十七、Event Editor

支持：

```text
Position
Type
Title
Description
Script
Camera
Hold
TTS
```

---

# 五十八、Frontend 模块建议

```text
src/
├── components/
│   ├── map/
│   ├── player/
│   ├── timeline/
│   ├── elevation/
│   ├── camera/
│   ├── segment/
│   ├── commentary/
│   └── export/
│
├── engines/
│   ├── map-engine/
│   ├── route-player/
│   ├── camera-engine/
│   ├── story-engine/
│   ├── tile-cache/
│   └── video-renderer/
│
├── stores/
│   ├── route.ts
│   ├── playback.ts
│   ├── segment.ts
│   ├── story.ts
│   └── export.ts
│
├── api/
│
└── types/
```

Agent 可以根据现有项目结构调整。

---

# 五十九、Backend 模块建议

```text
server/
├── app/
│   ├── models/
│   │   ├── routes/
│   │   ├── segments/
│   │   ├── events/
│   │   ├── prompts/
│   │   └── tasks/
│   │
│   ├── services/
│   │   ├── gpx/
│   │   ├── route_analysis/
│   │   ├── segmentation/
│   │   ├── ai/
│   │   ├── tts/
│   │   └── export/
│   │
│   ├── handlers/
│   └── repositories/
│
└── main.go
```

如果现有项目已有对应模块：

> 优先复用，不重复创建。

---

# 六十、API 初步设计

```text
POST   /api/routes

GET    /api/routes

GET    /api/routes/:id

DELETE /api/routes/:id

POST   /api/routes/:id/analyze

GET    /api/routes/:id/segments

POST   /api/routes/:id/segments/regenerate

PUT    /api/segments/:id

POST   /api/routes/:id/events

PUT    /api/events/:id

DELETE /api/events/:id

POST   /api/routes/:id/ai-story

POST   /api/events/:id/tts

POST   /api/routes/:id/export

GET    /api/tasks/:id
```

Agent 根据实际项目完善。

---

# 六十一、异步任务

以下任务不能阻塞 HTTP：

```text
AI Generation
TTS
Video Export
```

使用：

```text
In-Memory Queue
```

状态：

```text
PENDING
RUNNING
SUCCESS
FAILED
CANCELLED
```

---

# 六十二、音频存储

第一版：

```text
Local Disk
```

但必须抽象：

```go
type AudioStorage interface {
    Save(...)
    Open(...)
    Delete(...)
}
```

未来可替换：

```text
S3
OSS
MinIO
```

---

# 六十三、第一阶段不做

MVP 暂时不做：

```text
用户系统
社交
路线分享社区
照片
视频素材
天气
实时导航
手机 App
自动上传 YouTube
复杂 AI Agent
AI 危险判断
自动天气判断
多人协作
```

核心只做：

```text
GPX
+
3D Terrain
+
Route Analysis
+
Segment
+
Story
+
TTS
+
Video
```

---

# 六十四、开发迭代计划

---

## Phase 0 —— 开源项目源码调研

这一阶段：

**禁止直接进行大规模业务开发。**

必须先阅读：

```text
GPX_3D
TrailReplay
share-gpx
```

---

### GPX_3D 必须分析

```text
MapLibre 初始化
Terrain
DEM
Raster
GPX
Route Follow
Camera
Camera Height
Camera Pitch
Bearing
Playback
Speed
Tile
Cache
Recording
Export
```

---

### TrailReplay 必须分析

```text
Intro
Outro
Camera
Annotation
Route Overview
Elevation
Statistics
Timeline
Export
```

---

### share-gpx 必须分析

```text
GPX
Track
Waypoint
Marker
地图交互
路线信息组织
```

---

### Phase 0 输出

创建：

```text
docs/research/
├── gpx3d.md
├── trailreplay.md
├── share-gpx.md
└── technology-decision.md
```

---

# 六十五、Technology Decision 必须回答的问题

Agent 必须明确：

```text
1. GPX_3D 哪些代码可以直接借鉴？

2. 哪些代码应该重写？

3. 哪些功能只需要参考设计？

4. TrailReplay 哪些实现值得借鉴？

5. share-gpx 哪些数据处理值得借鉴？

6. 三个项目 License 是否允许当前使用方式？

7. Vue 3 + TypeScript 是否可以完整实现 GPX_3D 核心能力？

8. MapLibre Terrain 如何实现？

9. Tile Cache 如何实现？

10. Tile Preload 如何实现？

11. 浏览器视频录制能否满足需求？

12. MP4 是否需要 FFmpeg？

13. 导出是否可以和在线播放共用 Timeline？

14. 长路线如何保证性能？
```

---

# 六十六、Phase 1 —— GPX 基础能力

实现：

```text
上传 GPX
 ↓
解析 GPX
 ↓
保存 Route
 ↓
保存 TrackPoint
 ↓
显示基础地图路线
```

验收：

```text
真实 GPX
+
正确 Route
+
正确 Distance
+
正确 Elevation
```

---

# 六十七、Phase 2 —— 3D Terrain

实现：

```text
MapLibre
+
Terrain
+
Satellite
+
Route
```

验收：

```text
路线正确贴合地图
Terrain 正常
海拔正常
```

---

# 六十八、Phase 3 —— Route Player

实现：

```text
Follow Route
Play
Pause
Seek
Speed
Camera Height
Camera Pitch
```

必须支持：

```text
播放过程中修改 Camera
```

---

# 六十九、Phase 4 —— Camera Engine

实现：

```text
CameraState
CameraPreset
CameraTransition
Bearing Smoothing
Terrain Clearance
LookAhead
```

验收：

```text
Camera 不穿地
转弯平滑
不同高度正常
Pitch 正常
```

---

# 七十、Phase 5 —— Intro / Outro

实现：

```text
Intro
高空 → 快速下降 → 起点

Outro
终点 → 拔高 → 拉远 → 全路线
```

---

# 七十一、Phase 6 —— Route Analysis

实现：

```text
Elevation Smoothing
Slope
Ascent
Descent
```

---

# 七十二、Phase 7 —— Segment Engine

实现：

```text
FLAT
CLIMB
STEEP_CLIMB
DESCENT
STEEP_DESCENT
```

支持：

```text
Threshold Config
Minimum Segment Distance
Merge
```

---

# 七十三、Phase 8 —— Story Timeline

实现：

```text
Segment
Event
Scene
Camera
Duration
```

形成：

```text
StoryTimeline
```

---

# 七十四、Phase 9 —— Commentary

实现：

```text
用户输入文字
 ↓
Event
 ↓
播放时减速
 ↓
Camera
 ↓
文字显示
 ↓
继续
```

此阶段可以先不接 TTS。

---

# 七十五、Phase 10 —— TTS

实现：

```text
Script
 ↓
TTSProvider
 ↓
Audio
 ↓
Duration
 ↓
Timeline
```

---

# 七十六、Phase 11 —— AI Story

实现：

```text
Route
+
Segments
+
Events
 ↓
AI
 ↓
Story Events
```

用户可以：

```text
修改
删除
新增
```

---

# 七十七、Phase 12 —— Video Export

实现：

```text
Timeline
 ↓
Preflight
 ↓
Tile Preload
 ↓
Render
 ↓
Audio
 ↓
Overlay
 ↓
Video
```

最终：

```text
MP4
```

---

# 七十八、Phase 13 —— 性能优化

测试：

```text
5km
10km
20km
50km
100km
```

测试：

```text
GPX Point Count
FPS
Memory
Tile Count
Cache Size
Export Time
```

---

# 七十九、必须测试的极端情况

## 高速

```text
10x
```

---

## 低高度

```text
20m
```

---

## 陡坡

```text
30%+
```

---

## 急转弯

```text
Bearing 快速变化
```

---

## 长路线

```text
50km+
```

---

## 海拔噪声

不能出现：

```text
Climb
Flat
Climb
Flat
Climb
```

这种碎片化结果。

---

# 八十、性能目标

第一阶段至少做到：

```text
10km 路线
正常播放
稳定 FPS
```

第二阶段：

```text
50km+
```

必须保持：

```text
Camera Smooth
Tile Loading 可控
Memory 可控
```

---

# 八十一、未来 AI Camera Director

MVP 不做。

未来：

```text
Route Analysis
 ↓
AI
 ↓
Camera Suggestion
```

例如：

```json
{
    "camera": {
        "altitude": 100,
        "pitch": 65,
        "lookAhead": 300
    }
}
```

AI 可以根据：

```text
Climb
Descent
Steep
Junction
Viewpoint
```

自动生成 Camera。

---

# 八十二、未来 AI Route Director

进一步：

```text
GPX
 ↓
Route Analysis
 ↓
AI
 ↓
自动生成：
    Segment
    Commentary
    Camera
    Story
    TTS
```

最终用户可能只需要：

```text
上传 GPX
 ↓
生成路线预演
```

---

# 八十三、未来视频模板

后期允许：

```text
Template
```

例如：

```text
Minimal
Adventure
Professional
Race
Training
```

模板控制：

```text
Overlay
Font
Camera
Intro
Outro
Commentary
```

---

# 八十四、最终数据关系

```text
Route
 │
 ├── TrackPoints
 │
 ├── Segments
 │      │
 │      └── StoryEvents
 │
 ├── Events
 │
 └── StoryTimeline
        │
        ├── Scenes
        ├── Camera
        ├── Audio
        └── Overlay
```

---

# 八十五、最终产品核心链路

```text
                    GPX
                     │
                     ↓
                GPX Parser
                     │
                     ↓
                 Route
                     │
                     ↓
              Route Analyzer
                     │
          ┌──────────┴──────────┐
          ↓                     ↓
      Elevation              Waypoint
          │
          ↓
        Slope
          │
          ↓
       Segment
          │
          ↓
    Story / Commentary
          │
     ┌────┴────┐
     ↓         ↓
   Camera     TTS
     │         │
     └────┬────┘
          ↓
      Timeline
          │
          ↓
     Route Player
          │
          ↓
      MapLibre
          │
          ↓
      3D Terrain
          │
          ↓
    Video Renderer
          │
          ↓
         MP4
```

---

# 八十六、开源项目最终借鉴原则

不要问：

> “哪个项目的代码可以复制？”

应该问：

> “哪个项目解决了什么问题？”

---

## GPX_3D

解决：

```text
如何在 3D Terrain 中播放 GPX
```

因此：

```text
3D
Terrain
Camera
Route Follow
Playback
Tile
Recording
```

以它为主要参考。

---

## TrailReplay

解决：

```text
如何把路线变成更有故事感的视频
```

因此：

```text
Intro
Outro
Annotation
Overview
Storytelling
Export
```

以它为主要参考。

---

## share-gpx

解决：

```text
如何处理和展示 GPX / Waypoint / Marker / 地图信息
```

因此：

```text
GPX
Waypoint
Marker
Route Info
```

作为数据和交互参考。

---

# 八十七、明确禁止

禁止：

```text
直接复制三个项目
+
强行拼接
```

禁止：

```text
三个项目各维护自己的 Route Model
```

禁止：

```text
三个项目各维护自己的 Camera
```

禁止：

```text
三个项目各维护自己的 Timeline
```

最终必须只有：

```text
一个 Route Model

一个 RoutePlayer

一个 CameraEngine

一个 StoryEngine

一个 TimelineEngine

一个 VideoRenderer
```

---

# 八十八、第三方 License

Agent 在 Phase 0 必须检查：

```text
GPX_3D LICENSE
TrailReplay LICENSE
share-gpx LICENSE
```

并记录：

```text
docs/THIRD_PARTY.md
```

格式：

```text
Project
Repository
License
Used Components
Usage Type
Modification
Attribution Requirement
Commercial Use
Redistribution Requirement
```

任何直接复制的代码必须明确来源。

---

# 八十九、Agent 开发规则

Agent 每一个 Phase 必须：

```text
1. 先理解当前代码。

2. 不破坏已有功能。

3. 优先复用现有架构。

4. 不重复创建已有模型。

5. 不为了完成任务引入不必要依赖。

6. 不直接复制整个第三方项目。

7. 第三方代码必须确认 License。

8. 核心功能必须使用真实 GPX 测试。

9. 不允许只用 Mock 证明核心功能完成。

10. 每个阶段完成后进行测试。
```

---

# 九十、每个 Phase 完成后的报告

Agent 必须输出：

```text
## 完成内容

## 修改文件

## 新增文件

## 技术实现

## 第三方项目借鉴

## License 注意事项

## 测试结果

## 当前问题

## 下一阶段计划
```

不能只输出：

```text
Done
```

---

# 九十一、第一阶段 Agent 执行顺序

Agent 收到本文档后：

```text
Step 1
检查当前项目结构

↓

Step 2
确认当前 Go / Gin / GORM / SQLite 架构

↓

Step 3
确认当前 Vue / TypeScript 架构

↓

Step 4
阅读 GPX_3D 源码

↓

Step 5
阅读 TrailReplay 源码

↓

Step 6
阅读 share-gpx 源码

↓

Step 7
检查三个项目 License

↓

Step 8
建立源码功能 Mapping

↓

Step 9
输出 Technology Decision

↓

Step 10
设计最终 Route / Segment / Event / Scene / Timeline 模型

↓

Step 11
设计 CameraEngine

↓

Step 12
设计 Tile Cache / Preload

↓

Step 13
设计 VideoRenderer

↓

Step 14
制定详细开发计划

↓

Step 15
开始 Phase 1
```

---

# 九十二、源码 Mapping 要求

Agent 必须尽可能定位到具体源码模块。

例如：

```text
GPX_3D

Camera：
xxx.js

Route Follow：
xxx.js

Terrain：
xxx.js

Recording：
xxx.js
```

TrailReplay：

```text
Intro：
xxx.ts

Outro：
xxx.ts

Timeline：
xxx.ts
```

share-gpx：

```text
GPX：
xxx

Waypoint：
xxx
```

不能只说：

```text
“GPX_3D 使用了 MapLibre”
```

而要说明：

```text
“GPX_3D 哪个模块负责什么功能，以及我们准备如何在 Vue 中重构。”
```

---

# 九十三、技术决策原则

如果参考项目实现方式与本文档冲突：

```text
源码实际实现
>
本文档假设
```

Agent 必须：

```text
发现差异
 ↓
记录
 ↓
分析
 ↓
提出方案
 ↓
选择方案
 ↓
实现
```

不能默默按照错误假设开发。

---

# 九十四、最终 MVP 验收标准

必须能够：

```text
1. 上传真实 GPX。

2. 正确解析路线。

3. 显示 3D Terrain。

4. 显示 GPX 路线。

5. Camera 沿路线播放。

6. 可以 Play / Pause / Seek。

7. 可以实时调整：
   Speed
   Camera Height
   Camera Pitch

8. 显示：
   Distance
   Elevation
   Slope
   Ascent
   Descent

9. 自动识别：
   Flat
   Climb
   Steep Climb
   Descent
   Steep Descent

10. 用户可以修改 Segment。

11. 用户可以添加 Event。

12. 用户可以添加文字解说。

13. 播放到 Event：
    减速
    Camera Transition
    停留
    显示文字
    继续

14. 支持 TTS。

15. TTS Duration 自动进入 Timeline。

16. 支持 Intro：
    高空快速下降。

17. 支持 Outro：
    终点拔高。

18. 支持完整路线 Overview。

19. 支持视频 Overlay。

20. 导出视频。

21. 导出前执行 Tile Preload。

22. 不因为地图加载速度导致明显空白。

23. 在线播放和导出使用同一个 Timeline。
```

---

# 九十五、最终产品体验

最终效果应该类似：

```text
用户上传：

富士山越野路线.gpx

↓

系统：

路线长度：28.4km
累计爬升：1850m
累计下降：1820m
最高点：xxxxm

↓

自动分析：

平缓段
↓
主要爬升
↓
陡坡
↓
山脊
↓
连续下降
↓
技术路段
↓
最终下降

↓

AI：

生成路线讲解

↓

TTS：

生成语音

↓

预演：

高空
 ↓
快速下降
 ↓
进入山地
 ↓
沿路线前进
 ↓
进入爬升
 ↓
镜头抬高
 ↓
减速
 ↓
“接下来进入本路线的主要爬升段……”
 ↓
继续
 ↓
进入陡坡
 ↓
Camera 调整
 ↓
解说
 ↓
继续
 ↓
到达终点
 ↓
镜头拔高
 ↓
完整路线
 ↓
路线统计

↓

Export

↓

MP4
```

---

# 九十六、项目最终核心价值

最终项目不是：

> “把 GPX 画成一个 3D 视频。”

而是：

> **把 GPX 转换成一个可以被理解、被讲解、被预演的 3D 越野路线故事。**

核心能力：

```text
GPX
+
Terrain
+
Route Analysis
+
Segment
+
Camera
+
Story
+
AI
+
TTS
+
Timeline
+
Video
```

最终形成：

> **AI 3D 越野路线提前探路系统。**

---

# 九十七、最重要的架构原则

### 原则 1

GPX 是数据源。

### 原则 2

Route 是唯一的路线数据模型。

### 原则 3

Segment 是路线理解层。

### 原则 4

Story 是路线叙事层。

### 原则 5

Camera 是视觉层。

### 原则 6

TTS 是声音层。

### 原则 7

Timeline 是所有内容的时间统一层。

### 原则 8

Route Player 和 Video Renderer 使用同一个 Timeline。

### 原则 9

AI 生成内容，但不直接控制底层播放器。

### 原则 10

Camera 参数必须可配置。

### 原则 11

Segment 自动生成，但用户可以修改。

### 原则 12

TTS 的真实 Audio Duration 必须进入 Timeline。

### 原则 13

视频导出前必须进行 Tile Preload / Export Preflight。

### 原则 14

第一版优先浏览器端渲染，不急于引入 Blender。

### 原则 15

不要直接拼接三个开源项目。

---

# 九十八、给 Agent 的最终任务

请基于本文档开始工作。

**第一步不要直接大量写代码。**

首先：

```text
1. 分析当前项目。

2. 阅读 GPX_3D。

3. 阅读 TrailReplay。

4. 阅读 share-gpx。

5. 检查 License。

6. 输出源码 Mapping。

7. 输出第三方项目借鉴边界。

8. 输出 Technology Decision。

9. 输出最终系统架构。

10. 输出数据模型。

11. 输出 Camera Engine 设计。

12. 输出 Route Segment Algorithm。

13. 输出 Story Timeline 设计。

14. 输出 Tile Cache / Preload 设计。

15. 输出 Video Renderer 设计。

16. 输出完整开发计划。

17. 确认方案后再开始 Phase 1。
```

开发过程中：

```text
优先保证核心链路：

GPX
→
3D Terrain
→
Route Follow
→
Camera
→
Playback
→
Route Analysis
→
Segment
→
Timeline
→
Commentary
→
TTS
→
Video
```

不要提前过度开发 AI。

不要提前开发复杂用户系统。

不要提前引入 Blender。

不要为了“完成任务”而堆砌第三方代码。

最终目标只有一个：

> **让用户上传一条真实 GPX 后，可以通过 3D 地形、路线跟随、自动分段、Camera 变化、AI/TTS 解说和 Intro/Outro，生成一个真正具有“提前探路”价值的越野路线预演视频。**

```
```
