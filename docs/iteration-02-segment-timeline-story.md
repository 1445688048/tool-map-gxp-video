# 迭代 2：路线分析 → 自动分段 → Story Timeline → 解说 → Intro/Outro

> **状态**：待开发
> **目标**：基于迭代 1 的 Route 数据，自动分析路线特征、分段、生成可编辑的 Story Timeline，实现 Intro/Outro 镜头叙事
> **前置迭代**：迭代 1（必须有完整的 Route / TrackPoint / CameraEngine）
> **影响迭代**：迭代 3（Timeline 是导出的数据基础）

---

## 一、迭代目标

在迭代 1 的基础上，完成以下能力：

```
Route 数据
    ↓
自动分析（坡度趋势 / 爬升段 / 下降段）
    ↓
自动生成 Segment（可配置阈值 + 活动类型预设）
    ↓
用户可编辑 Segment 类型和解说文字
    ↓
Story Timeline 编辑器（可视化时间轴）
    ↓
播放时：到达 Commentary Event → 减速 → 停留 → 显示文字 → 继续
    ↓
Intro 镜头（高空 → 下降 → 对准起点）
    ↓
Outro 镜头（终点 → 拔高 → 拉远 → 统计）
```

**一句话验收**：上传一条 GPX 后，系统自动分出爬升段/下降段/平缓段，用户可以为某段填写解说文字，播放到该段时自动减速并显示文字，开头有 Intro 动画，结尾有 Outro 动画。

---

## 二、不做的事

- 不做 AI 自动生成解说（保留接口，用手动输入替代）
- 不做 TTS（保留接口，用 MockTTSProvider）
- 不做视频导出（迭代 3 实现）
- 不做用户系统

---

## 三、技术决策

### 3.1 活动类型预设（分段阈值）

用户上传 GPX 时选择活动类型，系统自动套用对应阈值生成 Segment：

| 预设名称 | FLAT | CLIMB | STEEP_CLIMB | DESCENT | STEEP_DESCENT | 最小段距离 | 说明 |
|----------|------|-------|-------------|---------|---------------|-----------|------|
| 越野跑 | ±3% | +3~+8% | >+8% | -3~-8% | <-8% | 200m | 节奏快，短坡也算 climb |
| 徒步登山 | ±5% | +5~+12% | >+12% | -5~-12% | <-12% | 500m | 标准登山阈值 |
| 山地骑行 | ±4% | +4~+10% | >+10% | -4~-10% | <-10% | 300m | 中等敏感度 |
| 自定义 | 用户填 | | | | | | |

阈值以 JSON 配置存储，后端服务化，前端可动态加载。

```json
// config/activity_presets.json
{
  "trail_running": {
    "name": "越野跑",
    "flat_range": [-3, 3],
    "climb_range": [3, 8],
    "steep_climb_threshold": 8,
    "descent_range": [-8, -3],
    "steep_descent_threshold": -8,
    "min_segment_distance_m": 200
  },
  "hiking": {
    "name": "徒步登山",
    "flat_range": [-5, 5],
    "climb_range": [5, 12],
    "steep_climb_threshold": 12,
    "descent_range": [-12, -5],
    "steep_descent_threshold": -5,
    "min_segment_distance_m": 500
  },
  "mtb": {
    "name": "山地骑行",
    "flat_range": [-4, 4],
    "climb_range": [4, 10],
    "steep_climb_threshold": 10,
    "descent_range": [-10, -4],
    "steep_descent_threshold": -4,
    "min_segment_distance_m": 300
  }
}
```

### 3.2 分段算法

```
原始 TrackPoint（已平滑）
    ↓
滑动窗口计算平均坡度（窗口 = min_segment_distance / 平均步长）
    ↓
根据阈值标记每点的 Segment 类型
    ↓
合并相邻同类型点 → 初始 Segment 列表
    ↓
过滤：距离 < min_segment_distance_m 的 Segment 合并到相邻段
    ↓
输出：RouteSegment[]
```

**合并逻辑示例**：
```
初始：[FLAT 50m, CLIMB 200m, FLAT 30m, CLIMB 150m]
合并短段：[FLAT 80m, CLIMB 350m]   ← 30m FLAT 太短，合并到前面
```

### 3.3 Story Event 类型

| 类型 | 含义 | 播放行为 |
|------|------|----------|
| COMMENTARY | 路线解说 | 减速 → 停留 → 显示文字 → 继续 |
| WARNING | 警告提示 | 减速 → 停留 → 显示警告 → 继续 |
| POI | 兴趣点 | 轻微减速 → 显示名称 → 继续 |
| VIEWPOINT | 观景点 | 减速 → 镜头抬高 → 停留 → 继续 |
| JUNCTION | 岔路口 | 减速 → 显示提示 → 继续 |
| REST | 休息点 | 明显减速 → 停留 → 继续 |

### 3.4 Intro / Outro 镜头设计

**IntroScene 参数**（参考 TrailReplay intro 设计）：
```typescript
interface IntroConfig {
    duration: number           // 默认 3s
    startZoom: number          // 高空俯视，如 8
    endZoom: number            // 接近路线，如 14
    startPitch: number         // 高空较平，如 30°
    endPitch: number           // 接近路线较陡，如 60°
    startAltitude: number      // 高空高度
    endAltitude: number        // 接近路线高度
}
```

**OutroScene 参数**：
```typescript
interface OutroConfig {
    duration: number           // 默认 4s
    startZoom: number          // 接近路线，如 14
    endZoom: number            // 拉远，如 9
    startPitch: number         // 如 60°
    endPitch: number           // 较平，如 40°
    showStats: boolean         // 是否显示路线统计
}
```

---

## 四、后端详细设计

### 4.1 新增数据模型

```go
// internal/segment/models.go

// RouteSegment 代表路线的一个分析段
type RouteSegment struct {
    ID              uint      `gorm:"primaryKey" json:"id"`
    RouteID         uint      `gorm:"index;not null" json:"route_id"`
    StartIndex      int       `gorm:"not null" json:"start_index"`
    EndIndex        int       `gorm:"not null" json:"end_index"`
    StartDistance   float64   `gorm:"type:decimal(10,3);not null" json:"start_distance"`
    EndDistance     float64   `gorm:"type:decimal(10,3);not null" json:"end_distance"`
    Distance        float64   `gorm:"type:decimal(10,3);not null" json:"distance"`
    StartElevation  float64   `gorm:"type:decimal(8,1);not null" json:"start_elevation"`
    EndElevation    float64   `gorm:"type:decimal(8,1);not null" json:"end_elevation"`
    ElevationGain   float64   `gorm:"type:decimal(8,1);not null" json:"elevation_gain"`
    ElevationLoss   float64   `gorm:"type:decimal(8,1);not null" json:"elevation_loss"`
    AverageSlope    float64   `gorm:"type:decimal(6,2);not null" json:"average_slope"`
    MaxSlope        float64   `gorm:"type:decimal(6,2);not null" json:"max_slope"`
    Type            string    `gorm:"size:32;not null" json:"type"` // FLAT/CLIMB/STEEP_CLIMB/DESCENT/STEEP_DESCENT
    CameraPreset    string    `gorm:"size:32" json:"camera_preset"` // FOLLOW_NORMAL/FOLLOW_HIGH等
    Commentary      string    `gorm:"type:text" json:"commentary"`  // 用户填写的解说文字
    Enabled         bool      `gorm:"default:true" json:"enabled"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}

// StoryEvent 代表 Timeline 中的一个事件
type StoryEvent struct {
    ID          uint      `gorm:"primaryKey" json:"id"`
    RouteID     uint      `gorm:"index;not null" json:"route_id"`
    Position    float64   `gorm:"type:decimal(10,3);not null" json:"position"` // km
    EventType   string    `gorm:"size:32;not null" json:"event_type"` // COMMENTARY/WARNING/POI/VIEWPOINT/JUNCTION/REST
    Title       string    `gorm:"size:255" json:"title"`
    Description string    `gorm:"type:text" json:"description"`
    Script      string    `gorm:"type:text" json:"script"` // 解说文字（对应 TTS 输入）
    CameraPreset string   `gorm:"size:32" json:"camera_preset"`
    HoldBefore  float64   `gorm:"default:0" json:"hold_before"` // 到达前减速停留时间（秒）
    HoldAfter   float64   `gorm:"default:0" json:"hold_after"`  // 离开前停留时间（秒）
    TTSStatus   string    `gorm:"size:32;default:'pending'" json:"tts_status"` // pending/generating/success/failed
    TTSAudioURL string    `gorm:"size:512" json:"tts_audio_url"`
    TTSDuration float64   `gorm:"type:decimal(6,2)" json:"tts_duration"` // 秒，TTS 生成后填入
    Enabled     bool      `gorm:"default:true" json:"enabled"`
    Order       int       `gorm:"default:0" json:"order"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

### 4.2 新增路由

```
POST   /api/routes/:id/analyze          触发路线分析（计算 Segment + 建议 Event）
GET    /api/routes/:id/segments         获取路线所有 Segment
PUT    /api/segments/:id                更新 Segment（修改类型/解说/相机预设）
DELETE /api/segments/:id                删除 Segment
POST   /api/routes/:id/segments/regenerate  重新生成分段（用新阈值）

GET    /api/routes/:id/events           获取路线所有 StoryEvent
POST   /api/routes/:id/events           创建 StoryEvent
PUT    /api/events/:id                  更新 StoryEvent
DELETE /api/events/:id                  删除 StoryEvent

GET    /api/config/activity-presets     获取活动类型预设配置

POST   /api/events/:id/tts              为 Event 生成 TTS（目前返回 mock）
```

### 4.3 分段算法服务

```go
// internal/route/segmenter.go

package route

type Segmenter struct {
    presets ActivityPresets
}

type ActivityPresets struct {
    TrailRunning ActivityPreset
    Hiking       ActivityPreset
    MTB          ActivityPreset
}

type ActivityPreset struct {
    FlatRange              [2]float64  // [-3, 3]
    ClimbRange             [2]float64  // [3, 8]
    SteepClimbThreshold    float64     // 8
    DescentRange           [2]float64  // [-8, -3]
    SteepDescentThreshold  float64     // -8
    MinSegmentDistanceM    float64     // 200
}

// Segment 核心接口
func (s *Segmenter) Segment(points []TrackPoint, preset string) ([]RouteSegment, error)
```

**算法伪代码**：
```
1. 对 points 按 min_segment_distance 分组（滑动窗口）
2. 计算每组的平均坡度
3. 根据阈值分配类型
4. 生成初始 Segment 列表
5. 遍历 Segment 列表，合并距离 < min_segment_distance 且类型相同的相邻段
6. 返回最终 Segment 列表
```

### 4.4 TTS Provider 接口（预留）

```go
// internal/tts/provider.go

package tts

type Provider interface {
    Generate(ctx context.Context, text string) (*AudioResult, error)
}

type AudioResult struct {
    AudioPath string    // 音频文件路径
    Duration  float64   // 音频时长（秒）
    Format    string    // "wav" | "mp3" | "ogg"
}

// MockTTSProvider — 第一版用，模拟 TTS
type MockTTSProvider struct{}

func (m *MockTTSProvider) Generate(ctx context.Context, text string) (*AudioResult, error) {
    // 估算：中文约 3 字/秒
    duration := float64(len([]rune(text))) / 3.0
    return &AudioResult{
        AudioPath: "",  // 第一版不生成实际音频
        Duration:  duration,
        Format:    "mock",
    }, nil
}
```

---

## 五、前端详细设计

### 5.1 新增组件

```
src/components/
├── segment/
│   ├── SegmentList.vue       # 左侧 Segment 列表（可点击编辑）
│   └── SegmentEditor.vue     # 右侧编辑面板（类型/解说/相机参数）
├── timeline/
│   ├── Timeline.vue          # 底部 Timeline 编辑器（可视化时间轴）
│   ├── TimelineEvent.vue     # 单个 Event 组件
│   └── TimelineTrack.vue     # Timeline 轨道（Segment + Event 混合显示）
├── event/
│   └── EventEditor.vue       # Event 编辑面板（位置/类型/标题/脚本/相机）
└── intro-outro/
    └── IntroOutroConfig.vue  # Intro/Outro 参数配置
```

### 5.2 CameraEngine 扩展

迭代 1 的 CameraEngine 只支持跟随模式。迭代 2 需要增加：

```typescript
// src/engines/camera-engine/index.ts（扩展）

type CameraMode = 'follow' | 'intro' | 'outro' | 'overview' | 'focus'

interface CameraTransition {
    from: CameraState
    to: CameraState
    duration: number      // 毫秒
    easing: 'linear' | 'easeIn' | 'easeOut' | 'easeInOut'
}

class CameraEngine {
    // 新增：场景切换
    playIntro(config: IntroConfig): Promise<void>
    playOutro(config: OutroConfig): Promise<void>
    playTransition(transition: CameraTransition): Promise<void>

    // 新增：Event 触发时的减速/停留
    applyEventSlowdown(event: StoryEvent): void
    resumePlayback(): void
}
```

### 5.3 播放引擎扩展

```typescript
// src/engines/route-player/index.ts（扩展）

class RoutePlayer {
    // 新增：Timeline 感知播放
    private timeline: StoryTimeline
    private currentEvent: StoryEvent | null = null

    // 每帧检查是否到达 Event
    checkEventTrigger(point: TrackPoint): void

    // Event 触发时的行为
    onEventTriggered(event: StoryEvent): void
    onEventCompleted(event: StoryEvent): void
}
```

**播放状态机**：
```
IDLE → PLAYING → [到达 Event] → SLOWING → HOLD → [解说播放] → RESUMING → PLAYING → ... → [到达终点] → OUTRO → ENDED
```

### 5.4 Timeline 编辑器 UI

```
┌────────────────────────────────────────────────────────────┐
│ Timeline                                                   │
│                                                            │
│  Intro [3s]                                                │
│  ├─ Segment 1 (FLAT) [2.1km]                               │
│  │   💬 "起点平缓，适合热身"                                 │
│  ├─ Segment 2 (CLIMB) [1.8km, 爬升320m]                    │
│  │   💬 "主要爬升段，注意节奏"                               │
│  ├─ Event: WARNING @ 6.42km [岔路口]                        │
│  ├─ Segment 3 (STEEP_CLIMB) [0.5km, 爬升120m]              │
│  │   💬 "最陡的一段，小心"                                   │
│  ├─ Segment 4 (DESCENT) [3.2km, 下降280m]                  │
│  │   💬 "长下坡，注意刹车"                                   │
│  └─ Segment 5 (FLAT) [1.0km]                               │
│                                                             │
│  Outro [4s]                                                │
│                                                             │
│  总时长: 47.3s  |  10.5km  |  爬升 856m  |  下降 843m       │
└────────────────────────────────────────────────────────────┘
```

**交互**：
- 拖拽 Event 调整位置
- 点击 Segment/Event 打开编辑面板
- 右键删除
- 拖拽边界调整 Segment 范围

---

## 六、AI Commentary 预留设计

### 6.1 PromptTemplate 模型

```go
// internal/ai/models.go

type PromptTemplate struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    Name      string    `gorm:"size:64;not null;unique" json:"name"` // route_summary / segment_commentary / poi_commentary
    Content   string    `gorm:"type:text;not null" json:"content"`   // Prompt 模板
    Variables []string  `gorm:"type:json" json:"variables"`          // [distance, ascent, avg_slope, max_slope]
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### 6.2 规则模板（第一版）

不用 AI API，用 Go 模板字符串 + 数据插值：

```go
// internal/ai/template.go

var SegmentCommentaryTemplate = `接下来进入{type}段，长度约{distance}公里，
累计{gain_or_loss}{elevation}米，平均坡度{avg_slope}%，
{tip}`

func GenerateSegmentCommentary(seg RouteSegment, preset string) string {
    // 根据 seg.Type 选择模板和 tip 文案
    // 插入 seg.Distance, seg.ElevationGain, seg.AverageSlope 等数据
}
```

**预期输出示例**：
```
接下来进入主要爬升段，长度约1.8公里，累计爬升320米，平均坡度11.2%，
前半段坡度相对稳定，后半段会明显变陡，注意控制呼吸节奏。
```

### 6.3 AI 接口预留

```go
// internal/ai/service.go

type AIService interface {
    GenerateSegmentCommentary(seg RouteSegment) (*CommentaryResult, error)
    GenerateRouteSummary(route Route) (*CommentaryResult, error)
}

// 第一版：RuleBasedAI（规则模板）
// 后续：OpenAIService / TongyiService 等

type CommentaryResult struct {
    Title  string
    Script string
}
```

---

## 七、验收标准

### 功能验收

- [ ] 上传 GPX 后，可自动生成分段（基于选择的.activity类型）
- [ ] 分段结果合理：无碎片化（不会出现 CLIMB-FLAT-CLIMB-FLAT 交替）
- [ ] 可切换活动类型重新生成分段
- [ ] 可手动修改任意 Segment 的类型
- [ ] 可手动修改任意 Segment 的解说文字
- [ ] 可添加 StoryEvent（选择类型、位置、标题、脚本）
- [ ] 可删除 StoryEvent
- [ ] 可在 Timeline 中拖拽 Event 调整位置
- [ ] 播放时到达 Event 位置自动减速 → 停留 → 显示文字 → 继续
- [ ] Intro 镜头正常播放（高空下降 → 对准起点）
- [ ] Outro 镜头正常播放（终点拔高 → 拉远 → 显示统计）
- [ ] TTS 接口可调用（当前返回 mock duration）

### 数据验收

- [ ] Segment 类型分布合理（长路线应有多种类型，短路线可能只有 FLAT + 1-2 个 CLIMB/DESCENT）
- [ ] Event Position 与路线距离精确匹配（误差 < 10m）
- [ ] TTS Duration 正确进入 Timeline 总时长计算

### 性能验收

- [ ] 50km 路线自动分段 < 2s
- [ ] Timeline 编辑器流畅（100+ Event 不卡顿）

---

## 八、第三方项目借鉴来源

| 功能 | 参考项目 | 具体文件 | 借鉴内容 |
|------|----------|----------|----------|
| Intro 镜头设计 | TrailReplay | `app/src/components/playback/PlaybackProvider.tsx` → `INTRO_DURATION` | 阶段状态机（idle→preloading→intro→playing→outro→ended） |
| Outro 镜头设计 | TrailReplay | 同上 → `OUTRO_DURATION` | 到达终点后自动触发 outro |
| Camera 平滑过渡 | TrailReplay | `app/src/components/map/cameraUtils.ts` → `smoothBearing/smoothPitch/smoothZoom` | 各参数独立平滑 + 死区 + 最大变化率限制 |
| 地形感知相机 | TrailReplay | `app/src/components/map/hooks/useTrailPlaybackCamera.ts` | `calculateTerrainAwareAdjustments` 思路 |
| Activity 预设配置 | 本项目新增 | — | 基于越野跑/登山/骑行不同运动特征设计阈值 |
| TTS Provider 接口 | 本项目新增 | — | 参考文档 #三十六，抽象接口方便后续替换 |

---

## 九、风险与注意事项

### 风险 1：分段阈值不适配所有路线

**现象**：某些路线坡度数据噪声大，即使用 moving average 平滑，分段仍可能出现碎片。
**对策**：
1. 提供"手动调整"能力（用户可合并相邻段）
2. 提供"重新生成"按钮，切换阈值后重算
3. 阈值作为可配置项存入数据库，不硬编码

### 风险 2：Event 位置与关键点不对齐

**现象**：用户在 6.42km 处添加 Event，但播放器的当前位置是 6.418km 或 6.423km。
**对策**：Event 触发时用 `findNearestPoint(position)` 找最近的 TrackPoint，误差 < 5m 时触发。

### 风险 3：Intro/Outro 与播放循环的协调

**现象**：用户在 Intro 播放过程中点 Pause，状态混乱。
**对策**：
- Intro/Outro 期间允许 Pause/Seek
- Seek 到 Intro 区域内时，从当前位置恢复而非重播 Intro
- Outro 结束后回到 IDLE 状态

---

## 十、开发顺序

```
1. 后端：ActivityPresets 配置 + 分段算法服务
2. 后端：RouteSegment GORM 模型 + 分段 API
3. 后端：StoryEvent GORM 模型 + Event API
4. 后端：TTS Provider 接口 + MockTTSProvider
5. 后端：AI 规则模板服务（SegmentCommentary）
6. 前端：SegmentList + SegmentEditor 组件
7. 前端：Timeline 编辑器组件
8. 前端：CameraEngine 扩展（Intro/Outro/Transition）
9. 前端：RoutePlayer 扩展（Event 触发逻辑）
10. 前端：UI 联调 + 真实 GPX 测试
```

---

## 十一、交付物

- 新增后端服务：segmenter、story、tts、ai
- 新增前端组件：SegmentList、SegmentEditor、Timeline、EventEditor、IntroOutroConfig
- 扩展 CameraEngine 和 RoutePlayer
- `docs/iterations/iteration-02-segment-timeline-story.md`（本文档）
