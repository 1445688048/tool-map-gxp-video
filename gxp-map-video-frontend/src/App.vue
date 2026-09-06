<template>
  <div class="app">
    <header class="toolbar">
      <h1>GPX 越野路线 AI 预演</h1>
      <div class="toolbar-right">
        <select v-if="routeList.length" class="preset-select" :value="currentRouteId ?? ''" @change="onSelectRoute" title="切换路线">
          <option v-for="r in routeList" :key="r.id" :value="r.id">{{ r.name }}</option>
        </select>
        <button @click="showArrangeDialog = true" :disabled="!currentRouteId || arranging" class="btn-arrange">
          {{ arranging ? 'AI 编排中…' : 'AI 编排' }}
        </button>
        <button @click="showExportDialog = true" :disabled="!currentRouteId" class="btn-export">导出视频</button>
        <button @click="showUpload = true" class="btn-upload">导入 GPX</button>
      </div>
    </header>

    <main class="main">
      <!-- Left panel: Segments -->
      <div v-if="showSegments" class="left-panel">
        <SegmentList :route-id="currentRouteId" :active-segment-id="activeSegment?.id ?? null" @segment-updated="onSegmentUpdated" @segments-changed="onSegmentsChanged" />
      </div>

      <!-- Map -->
      <MapContainer ref="mapContainerRef" />

      <!-- Upload overlay (shown when no route is loaded, or opened from toolbar) -->
      <div v-if="!hasRoute || showUpload" class="upload-overlay">
        <div class="upload-overlay-inner">
          <button v-if="hasRoute" class="overlay-close" @click="showUpload = false">✕</button>
          <UploadZone @loaded="showUpload = false; void refreshRouteList()" />
        </div>
      </div>

      <!-- Right panel: Events + Intro/Outro（v-show 常驻挂载，保证 Intro/Outro 配置始终可读） -->
      <div v-show="showEvents" class="right-panel">
        <EventEditor :route-id="currentRouteId" @event-changed="onEventChanged" />
        <IntroOutroConfig ref="introOutroRef" />
      </div>

      <!-- Overlay -->
      <RouteOverlay
        :current-point="playbackStore.currentPoint"
        :total-distance="totalDistance"
        :segments="segments"
        :events="events"
        :visible="playbackStore.isPlaying"
      />
    </main>

    <!-- Timeline -->
    <TimelineEditor
      v-if="segments.length > 0 || events.length > 0 || timelineEvents.length > 0"
      :segments="segments"
      :events="displayEvents"
      :total-distance="totalDistance"
      :play-progress="playbackStore.progress * 100"
      @seek="onTimelineSeek"
    />

    <!-- Bottom controls -->
    <footer class="controls">
      <button @click="togglePlay" :disabled="!canPlay" class="btn-play">{{ isPlaying ? '⏸' : '▶' }}</button>
      <button :disabled="!hasRoute" @click="stop" class="btn">⏹</button>
      <button :disabled="!hasRoute" @click="reset" class="btn">↺</button>

      <div class="slider-group" v-if="hasRoute && player">
        <input type="range" min="0" :max="totalDistance" step="1" v-model.number="seekPos" class="seek-bar" />
      </div>

      <div class="control-group">
        <span class="label">速度</span>
        <select v-model.number="speed" @change="applyPlaybackRate" class="select" :disabled="routeDurationMin > 0">
          <option value="0.25">0.25x</option>
          <option value="0.5">0.5x</option>
          <option value="1">1x</option>
          <option value="2">2x</option>
          <option value="5">5x</option>
          <option value="10">10x</option>
          <option value="20">20x</option>
          <option value="50">50x</option>
          <option value="100">100x</option>
        </select>
      </div>

      <div class="control-group">
        <span class="label">全程时长</span>
        <select v-model.number="routeDurationMin" @change="applyPlaybackRate" class="select">
          <option :value="0">手动速度</option>
          <option v-for="m in durationOptions" :key="m" :value="m">{{ m }} 分钟</option>
        </select>
        <span class="val" v-if="routeDurationMin > 0 && effectiveRate > 0">≈{{ effectiveRate.toFixed(0) }}x</span>
      </div>

      <div class="control-group">
        <span class="label">步幅跳跃</span>
        <select v-model="stepMode" @change="onStepModeChange" class="select">
          <option value="off">关闭</option>
          <option value="hill-skip">上坡加速</option>
        </select>
      </div>

      <div class="control-item">
        <span class="label">高度</span>
        <input type="range" min="20" max="10000" step="20" v-model.number="cameraAlt" class="slider" />
        <span class="val">{{ cameraAlt }}m</span>
      </div>

      <div class="control-item">
        <span class="label">俯角</span>
        <input type="range" min="0" max="85" step="1" v-model.number="cameraPitch" class="slider" />
        <span class="val">{{ cameraPitch }}°</span>
      </div>

      <button @click="toggleFollow" :class="['btn-toggle', followCamera ? 'active' : '']" title="开启：镜头跟随当前位置；关闭：播放时镜头不移动，可自由查看">视角跟随</button>
      <button @click="showTuning = !showTuning" :class="['btn-toggle', showTuning ? 'active' : '']">调参</button>

      <button @click="showSegments = !showSegments" :class="['btn-toggle', showSegments ? 'active' : '']">分段</button>
      <button @click="showEvents = !showEvents" :class="['btn-toggle', showEvents ? 'active' : '']">事件</button>

      <div class="progress-info" v-if="hasRoute">
        <span>{{ ((player?.getDistance() ?? 0) / 1000).toFixed(2) }} / {{ (totalDistance / 1000).toFixed(1) }} km</span>
      </div>
    </footer>

    <!-- Camera tuning panel -->
    <div v-if="showTuning" class="tuning-panel">
      <div class="tuning-title">视角跟随调参</div>
      <div class="tuning-row">
        <span>前瞻距离</span>
        <input type="range" min="5" max="120" step="5" v-model.number="tune.lookAhead" class="slider" />
        <span class="val">{{ tune.lookAhead }} 点</span>
      </div>
      <div class="tuning-row">
        <span>转向死区</span>
        <input type="range" min="0" max="45" step="1" v-model.number="tune.deadzone" class="slider" />
        <span class="val">{{ tune.deadzone }}°</span>
      </div>
      <div class="tuning-row">
        <span>转向速率</span>
        <input type="range" min="0.1" max="3" step="0.1" v-model.number="tune.turnRate" class="slider" />
        <span class="val">{{ tune.turnRate.toFixed(1) }}</span>
      </div>
      <div class="tuning-section">急弯自动减速 / 前方探山</div>
      <div class="tuning-row">
        <span>启用</span>
        <input type="checkbox" v-model="tune.curveSlowOn" />
        <span>强度</span>
        <input type="range" min="0" max="1" step="0.1" v-model.number="tune.curveStrength" class="slider" />
        <span class="val">{{ Math.round(tune.curveStrength * 100) }}%</span>
      </div>
      <div class="tuning-row">
        <span>绕山探查</span>
        <input type="checkbox" v-model="tune.avoidOn" />
        <span class="val" style="width:auto">遇高山自动转向视野开阔侧</span>
      </div>
      <template v-if="stepMode !== 'off'">
        <div class="tuning-section">步幅跳跃（上坡加速模式）</div>
        <div class="tuning-row">
          <span>爬坡减速×</span>
          <input type="range" min="0.2" max="2" step="0.1" v-model.number="tune.hillMult" class="slider" />
          <span class="val">{{ tune.hillMult.toFixed(1) }}x</span>
        </div>
        <div class="tuning-row">
          <span>平路跳跃×</span>
          <input type="range" min="2" max="30" step="1" v-model.number="tune.skipMult" class="slider" />
          <span class="val">{{ tune.skipMult }}x</span>
        </div>
        <div class="tuning-row">
          <span>爬坡阈值</span>
          <input type="range" min="2" max="15" step="1" v-model.number="tune.hillSlope" class="slider" />
          <span class="val">{{ tune.hillSlope }}%</span>
        </div>
        <div class="tuning-row">
          <span>平路阈值</span>
          <input type="range" min="1" max="5" step="0.5" v-model.number="tune.skipSlope" class="slider" />
          <span class="val">{{ tune.skipSlope }}%</span>
        </div>
      </template>
      <button @click="resetTuning" class="btn-reset">恢复默认</button>
      <div class="tuning-section">自定义进度标记</div>
      <div class="tuning-row">
        <span>标记图片</span>
        <input type="file" accept="image/png,image/gif,image/svg+xml,image/webp" class="marker-file" @change="onMarkerFileChange" />
      </div>
      <div class="tuning-hint">支持 PNG / GIF 动图 / SVG / WebP，建议 32~64px 透明背景；GIF 会直接作为动画播放</div>
      <button v-if="markerImage" @click="clearMarkerImage" class="btn-reset">恢复默认跑步标记</button>
    </div>

    <!-- Toast notifications -->
    <Teleport to="body">
      <div v-if="toast.show" :class="['toast', toast.type]">
        {{ toast.message }}
      </div>
    </Teleport>

    <!-- AI 编排弹窗：比赛名称/组别/音色/时长 必填或选择，决定成片规则 -->
    <Teleport to="body">
      <div v-if="showArrangeDialog" class="arrange-dialog-mask" @click.self="showArrangeDialog = false">
        <div class="arrange-dialog">
          <h3>AI 编排演出</h3>
          <p class="arrange-tip">AI 将分析路线地形，按以下规则生成镜头参数、赛前探路解说与时间轴。导出视频将忠实执行这份编排。</p>
          <label>比赛名称 *<input v-model="arrangeRaceName" placeholder="如：北京100越野赛" /></label>
          <label>组别 *<input v-model="arrangeCategory" placeholder="如：100公里组" /></label>
          <label>解说音色 *
            <select v-model="arrangeVoice" class="export-select">
              <option value="zh-CN-XiaoxiaoNeural">晓晓（女·自然）</option>
              <option value="zh-CN-YunxiNeural">云希（男）</option>
              <option value="zh-CN-YunyangNeural">云扬（男·播音）</option>
              <option value="zh-CN-XiaoyiNeural">晓伊（女·活泼）</option>
            </select>
          </label>
          <label>视频时长 *
            <select v-model.number="arrangeDurationSec" class="export-select">
              <option :value="150">2.5 分钟</option>
              <option :value="300">5 分钟</option>
              <option :value="600">10 分钟</option>
            </select>
          </label>
          <div class="arrange-actions">
            <button class="btn-cancel" @click="showArrangeDialog = false">取消</button>
            <button
              class="btn-confirm"
              :disabled="arranging || !arrangeRaceName.trim() || !arrangeCategory.trim()"
              @click="onArrange"
            >
              {{ arranging ? 'AI 编排中…' : '开始编排' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- 视频导出弹窗：忠实执行编排配置，不在此改创作参数 -->
    <Teleport to="body">
      <div v-if="showExportDialog" class="arrange-dialog-mask" @click.self="showExportDialog = false">
        <div class="arrange-dialog">
          <h3>导出解说视频 (MP4)</h3>
          <p class="arrange-tip">
            将按 AI 编排配置执行：时长 {{ routeDurationMin }} 分钟 · 解说音色 {{ currentVoiceLabel }} ·
            {{ timelineEvents.length }} 个解说事件。录制预演画面与语音混流，后端转码 MP4。
          </p>
          <p class="arrange-tip">录制期间请不要切换窗口或最小化。</p>
          <div class="arrange-actions">
            <button class="btn-cancel" @click="showExportDialog = false">取消</button>
            <button class="btn-confirm" :disabled="exporting" @click="startExport">
              {{ exporting ? (exportStatus || '导出中…') : '开始导出' }}
            </button>
          </div>
          <p v-if="exportStatus && exporting" class="arrange-tip">{{ exportStatus }}</p>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import MapContainer from './components/map/MapContainer.vue'
import UploadZone from './components/upload/UploadZone.vue'
import SegmentList from './components/segment/SegmentList.vue'
import EventEditor from './components/event/EventEditor.vue'
import TimelineEditor from './components/timeline/TimelineEditor.vue'
import RouteOverlay from './components/overlay/RouteOverlay.vue'
import IntroOutroConfig from './components/intro-outro/IntroOutroConfig.vue'
import { useRouteStore } from './stores/route'
import { usePlaybackStore } from './stores/playback'
import { CameraEngine } from './engines/camera-engine'
import { RoutePlayer } from './engines/route-player'
import { api } from './api/client'
import markerSpriteUrl from './assets/yueyepao.png'
import type { Route, RouteSegment, StoryEvent, TrackPoint, ShowConfig, ShowEvent } from './types/gpx'

const routeStore = useRouteStore()
const playbackStore = usePlaybackStore()

const showSegments = ref(true)
const showEvents = ref(false)
const showUpload = ref(false)
const segments = ref<RouteSegment[]>([])
const events = ref<StoryEvent[]>([])
const introOutroRef = ref<InstanceType<typeof IntroOutroConfig> | null>(null)

const hasRoute = computed(() => routeStore.currentRoute !== null)
const currentRouteId = computed(() => routeStore.currentRoute?.id ?? null)
const totalDistance = computed(() => routeStore.currentRoute?.total_distance ?? 0)
const isPlaying = computed(() => playbackStore.isPlaying)
const cameraAlt = ref(1800)
const cameraPitch = ref(60)
const speed = ref(1)
// 全程时长模式：0 = 使用手动速度；>0 = 压缩整条路线到该时长（与真实耗时不关）
const routeDurationMin = ref(10)
const durationBase = [1, 2.5, 5, 10, 15, 30, 60]
const durationOptions = computed(() => {
  const set = new Set(durationBase)
  set.add(Math.max(1, Math.round(routeDurationMin.value * 2) / 2))
  return [...set].filter(v => v > 0).sort((a, b) => a - b)
})
const stepMode = ref<'off' | 'hill-skip'>('off')
const seekPos = ref(0)
const followCamera = ref(true)
const showTuning = ref(false)

// 视角跟随调参（默认值与引擎内置默认一致）
const TUNE_DEFAULTS = {
  lookAhead: 120,
  deadzone: 30,
  turnRate: 0.3,
  curveSlowOn: true,
  curveStrength: 0.6,
  avoidOn: true,
  hillMult: 3,
  skipMult: 15,
  hillSlope: 5,
  skipSlope: 2,
}
const tune = ref({ ...TUNE_DEFAULTS })

function applyTuning() {
  const t = tune.value
  cameraEngine.value?.setSettings({ lookAheadPoints: t.lookAhead })
  cameraEngine.value?.setTuning({ deadzone: t.deadzone, maxRate: t.turnRate })
  routePlayer.value?.setCurveSlow(t.curveSlowOn, t.curveStrength)
  routePlayer.value?.setAvoidAhead(t.avoidOn)
  routePlayer.value?.setStepConfig({
    hillSpeedMultiplier: t.hillMult,
    skipSpeedMultiplier: t.skipMult,
    hillSlopeThreshold: t.hillSlope,
    skipSlopeThreshold: t.skipSlope,
  })
}
watch(tune, applyTuning, { deep: true })

function resetTuning() {
  tune.value = { ...TUNE_DEFAULTS }
}

function toggleFollow() {
  followCamera.value = !followCamera.value
  routePlayer.value?.setCameraFollow(followCamera.value)
  if (followCamera.value && routePlayer.value) {
    // 重新开启时立即回到当前位置，避免镜头停在原地
    routePlayer.value.seekTo(routePlayer.value.getDistance())
  }
}

// 自定义进度标记图片（dataURL 存 localStorage，刷新后仍生效）
const MARKER_IMAGE_KEY = 'gxp-progress-marker-image'
const markerImage = ref<string | null>(localStorage.getItem(MARKER_IMAGE_KEY))

watch(markerImage, (url) => {
  mapContainerRef.value?.setProgressMarkerImage(url)
  try {
    if (url) localStorage.setItem(MARKER_IMAGE_KEY, url)
    else localStorage.removeItem(MARKER_IMAGE_KEY)
  } catch {
    // 图片过大超出 localStorage 配额时仅本次会话生效
  }
})

function onMarkerFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => { markerImage.value = reader.result as string }
  reader.readAsDataURL(file)
  ;(e.target as HTMLInputElement).value = ''
}

function clearMarkerImage() {
  markerImage.value = null
}

const mapContainerRef = ref<{
  getMap: () => any
  loadRoute: (pts: any[]) => void
  setProgressPoint: (p: any, bearing?: number) => void
  updateRouteProgress: (frac: number) => void
  removeProgressMarker: () => void
  setProgressMarkerImage: (url: string | null) => void
  setProgressMarkerSprite: (url: string, opts?: { row?: number; rows?: number; cols?: number; frames?: number; fps?: number; size?: number }) => void
  setWaypoints: (wpts: any[]) => void
} | null>(null)
const cameraEngine = ref<CameraEngine | null>(null)
const routePlayer = ref<RoutePlayer | null>(null)
const player = computed(() => routePlayer.value)

const canPlay = computed(() => hasRoute.value && routePlayer.value !== null)

// 时长模式下需要的速度倍率：总距离 / (目标秒数 × 基础速度1.5m/s)
const effectiveRate = computed(() => {
  if (!routeDurationMin.value || !routePlayer.value) return 0
  const total = routePlayer.value.totalDistanceM
  if (total <= 0) return 0
  return total / (routeDurationMin.value * 60 * 1.5)
})

// Toast notification system (replaces alert())
const toast = ref({ show: false, message: '', type: 'info' as 'info' | 'error' | 'success' })
let toastTimer: ReturnType<typeof setTimeout> | null = null
function showToast(message: string, type: 'info' | 'error' | 'success' = 'info') {
  toast.value = { show: true, message, type }
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toast.value.show = false }, 3000)
}

// 子组件通过 window 事件统一上报 toast，这里桥接到本地 toast
function onGlobalToast(e: Event) {
  const detail = (e as CustomEvent<{ message?: string; type?: 'info' | 'error' | 'success' }>).detail
  showToast(detail?.message ?? String(e), detail?.type ?? 'info')
}

// 启动时自动加载最近一条路线，并接管 gxp-toast 事件
onMounted(async () => {
  window.addEventListener('gxp-toast', onGlobalToast)
  mapContainerRef.value?.setProgressMarkerImage(markerImage.value || null)
  mapContainerRef.value?.setProgressMarkerSprite(markerSpriteUrl, { row: 0, rows: 1, cols: 3, fps: 7, size: 64 })
  void refreshRouteList()
  try {
    const routes = await api.listRoutes()
    if (routes.length > 0 && !routeStore.currentRoute) {
      const latest = [...routes].sort((a, b) => +new Date(b.updated_at) - +new Date(a.updated_at))[0]
      routeStore.currentRoute = latest
      await routeStore.fetchPoints(latest.id)
    }
  } catch {
    // 后端不可用时页面仍可打开，上传后即可用
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('gxp-toast', onGlobalToast)
})

watch(() => routeStore.trackPoints, (pts) => {
  if (pts.length < 2) return
  if (!cameraEngine.value) {
    const cam = new CameraEngine()
    cameraEngine.value = cam
    routePlayer.value = new RoutePlayer(cam)
    routePlayer.value.onComplete(() => {
      playbackStore.pause()
      // 到达终点且启用 Outro 时，播放结尾镜头（从终点高空拉起）
      const outro = introOutroRef.value?.outroConfig
      const pts = routeStore.trackPoints
      if (outro?.enabled && pts.length > 1) {
        playbackStore.phase = 'outro'
        const end = pts[pts.length - 1]
        const tail = pts[Math.max(0, pts.length - 31)]
        // 结尾航向沿路线最后一段的方向
        const endBearing = cameraEngine.value?.calculateBearing(tail, end) ?? 0
        animateOverview(end, cameraAlt.value, outro.endAltitude, cameraPitch.value, outro.endPitch, outro.duration, 'outro', outro.spiralDeg ?? 180, () => {
          playbackStore.phase = 'ended'
        }, endBearing)
      } else {
        playbackStore.phase = 'ended'
      }
    })
    // 每帧同步进度点与 HUD（插值位置），镜头同步在播放器内部完成
    routePlayer.value.onProgress((pt, progress, bearing) => {
      playbackStore.setCurrentPoint(pt)
      mapContainerRef.value?.setProgressPoint(pt, bearing)
      mapContainerRef.value?.updateRouteProgress(progress)
    })
    // Attach the maplibre map (created inside MapContainer) so camera drives the map
    cam.setMap(mapContainerRef.value?.getMap?.() ?? null)
  }
  if (!routePlayer.value) return
  routePlayer.value.setPoints(pts)
  applyPlaybackRate()
  applyTuning()
  cameraEngine.value.setSettings({ altitude: cameraAlt.value, pitch: cameraPitch.value })
})

// Current distance in meters for active segment highlighting
const activeSegment = computed(() => {
  const dist = seekPos.value
  if (!dist || segments.value.length === 0) return null
  return segments.value.find(s => dist >= s.start_distance && dist <= s.end_distance) ?? null
})

function togglePlay() {
  if (!routePlayer.value) return
  if (playbackStore.isPlaying) {
    routePlayer.value.pause()
    playbackStore.pause()
    return
  }
  // 从起点出发且启用 Intro 时，先播开场镜头（螺旋俯冲到起点）
  const intro = introOutroRef.value?.introConfig
  const pts = routeStore.trackPoints
  if (intro?.enabled && routePlayer.value.getDistance() < 1 && pts.length > 1) {
    playbackStore.phase = 'intro'
    animateOverview(pts[0], intro.startAltitude, cameraAlt.value, intro.startPitch, cameraPitch.value, intro.duration, 'intro', intro.spiralDeg ?? 360, beginPlayback)
  } else {
    beginPlayback()
  }
}

function beginPlayback() {
  if (!routePlayer.value) return
  playbackStore.phase = 'playing'
  routePlayer.value.play()
  playbackStore.play()
  // 解说语音未合成的后台补齐（不阻塞播放，合成完自动生效）
  if (timelineEvents.value.some(e => !e.audioUrl)) void synthesizeTimeline()
  // 进度条按 100ms 同步即可；进度点/HUD/镜头由播放器每帧回调驱动
  const poll = setInterval(() => {
    if (!playbackStore.isPlaying) { clearInterval(poll); return }
    if (routePlayer.value) {
      seekPos.value = routePlayer.value.getDistance()
    }
    void handleTimelineEvents()
  }, 100)
}

// 开场/结尾的升降镜头：绕指定点 easeInOut 插值高度、俯角与螺旋角度
function animateOverview(
  pt: TrackPoint,
  fromAlt: number, toAlt: number,
  fromPitch: number, toPitch: number,
  durationSec: number,
  phase: 'intro' | 'outro',
  spiralDeg: number,
  onDone: () => void,
  baseBearingOverride?: number,
) {
  const cam = cameraEngine.value
  const pts = routeStore.trackPoints
  if (!cam || pts.length < 2) { onDone(); return }
  // 航向默认取路线起段的趋势方向，与跟拍衔接
  const ahead = pts[Math.min(30, pts.length - 1)]
  const baseBearing = baseBearingOverride ?? cam.calculateBearing(pt, ahead)
  const start = performance.now()
  const step = (now: number) => {
    if (playbackStore.phase !== phase) return // 被 stop() 打断
    const t = Math.min(1, (now - start) / (durationSec * 1000))
    const e = t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2
    const alt = fromAlt + (toAlt - fromAlt) * e
    const pitch = fromPitch + (toPitch - fromPitch) * e
    const bearing = (baseBearing + spiralDeg * e + 360) % 360
    cam.jumpToOverview(pt, alt, pitch, bearing)
    if (t < 1) requestAnimationFrame(step)
    else onDone()
  }
  requestAnimationFrame(step)
}

function stop() {
  routePlayer.value?.pause()
  routePlayer.value?.stop()
  playbackStore.pause()
  playbackStore.seekTo(0)
  playbackStore.phase = 'idle'
  // 中断时间轴事件与语音
  timelineAudio?.pause()
  timelineAudio = null
  tEventBusy = false
  tEventIdx = 0
}

function reset() { stop() }

function onSpeedChange() {
  applyPlaybackRate()
}

// 应用播放速率：全程时长模式优先，否则用手动速度
function applyPlaybackRate() {
  if (!routePlayer.value) return
  if (routeDurationMin.value > 0 && effectiveRate.value > 0) {
    routePlayer.value.setSpeed(effectiveRate.value)
  } else {
    routePlayer.value.setSpeed(speed.value)
  }
}

// 时长档位 → 建议镜头高度：速度越快视角越高，避免贴地"飞奔"感
const DURATION_ALTITUDE: Record<number, number> = { 5: 3000, 10: 1800, 15: 1200, 30: 900, 60: 600 }
watch(routeDurationMin, (min) => {
  const alt = DURATION_ALTITUDE[min]
  if (alt) cameraAlt.value = alt
})

function onStepModeChange() {
  if (routePlayer.value) routePlayer.value.setStepMode(stepMode.value)
}

watch([cameraAlt, cameraPitch], ([alt, pitch]) => {
  if (cameraEngine.value) cameraEngine.value.setSettings({ altitude: alt, pitch })
})

function syncMapProgress() {
  const pt = routePlayer.value?.currentPoint ?? null
  playbackStore.setCurrentPoint(pt)
  if (routePlayer.value) mapContainerRef.value?.updateRouteProgress(routePlayer.value.progress)
  if (pt && mapContainerRef.value) {
    mapContainerRef.value.setProgressPoint(pt)
  } else if (mapContainerRef.value) {
    mapContainerRef.value.removeProgressMarker()
  }
}

watch(seekPos, (val) => {
  if (routePlayer.value) {
    routePlayer.value.seekTo(val)
    playbackStore.seekTo(routePlayer.value.progress)
    syncMapProgress()
  }
})

async function onSegmentsChanged() {
  if (!currentRouteId.value) return
  try {
    segments.value = await api.getSegments(currentRouteId.value)
  } catch { segments.value = [] }
}

function onSegmentUpdated(seg: RouteSegment) {
  const idx = segments.value.findIndex(s => s.id === seg.id)
  if (idx >= 0) segments.value[idx] = seg
}

function onEventChanged(ev: StoryEvent) {
  const idx = events.value.findIndex(e => e.id === ev.id)
  if (idx >= 0) events.value[idx] = ev
  else events.value.push(ev)
}

function onTimelineSeek(dist: number) {
  if (routePlayer.value) {
    routePlayer.value.seekTo(dist)
    playbackStore.seekTo(routePlayer.value.progress)
    syncMapProgress()
    rearmTimeline(dist)
  }
}

// ── AI 编排：LLM 生成演出配置 → 应用 → 合成解说语音 ──────────────────
const arranging = ref(false)
const showArrangeDialog = ref(false)
const arrangeRaceName = ref('')
const arrangeCategory = ref('')
const arrangeVoice = ref('zh-CN-XiaoxiaoNeural')
const arrangeDurationSec = ref(600)
const timelineEvents = ref<ShowEvent[]>([])
let tEventIdx = 0
let tEventBusy = false
let timelineAudio: HTMLAudioElement | null = null
let synthRunning = false

function applyConfig(cfg: ShowConfig) {
  // 播放时长：按配置精确设置（0.5 分钟粒度）
  const mins = cfg.playback?.totalDurationSec ? cfg.playback.totalDurationSec / 60 : 10
  routeDurationMin.value = Math.max(1, Math.round(mins * 2) / 2)
  if (cfg.playback?.stepMode) stepMode.value = cfg.playback.stepMode === 'hill-skip' ? 'hill-skip' : 'off'

  const cam = cfg.camera
  if (cam) {
    if (cam.altitude) cameraAlt.value = cam.altitude
    if (cam.pitch) cameraPitch.value = cam.pitch
    tune.value = {
      ...tune.value,
      lookAhead: cam.lookAhead ?? tune.value.lookAhead,
      deadzone: cam.deadzone ?? tune.value.deadzone,
      turnRate: cam.turnRate ?? tune.value.turnRate,
      curveSlowOn: cam.curveSlow?.on ?? tune.value.curveSlowOn,
      curveStrength: cam.curveSlow?.strength ?? tune.value.curveStrength,
      avoidOn: cam.avoidAhead ?? tune.value.avoidOn,
    }
  }
  const io = introOutroRef.value
  if (cfg.intro && io) {
    io.introConfig = {
      enabled: cfg.intro.enabled, duration: cfg.intro.duration,
      startPitch: cfg.intro.startPitch, endPitch: cfg.intro.endPitch,
      startAltitude: cfg.intro.startAltitude, endAltitude: cfg.intro.endAltitude,
      spiralDeg: cfg.intro.spiralDeg,
    }
  }
  if (cfg.outro && io) {
    io.outroConfig = {
      enabled: cfg.outro.enabled, duration: cfg.outro.duration,
      startPitch: cameraPitch.value,
      endPitch: cfg.outro.endPitch, endAltitude: cfg.outro.endAltitude,
      spiralDeg: cfg.outro.spiralDeg,
    }
  }
  timelineEvents.value = (cfg.events ?? []).map(e => {
    const typeLabel: Record<string, string> = { COMMENTARY: '解说', WARNING: '注意', POI: '兴趣点', VIEWPOINT: '观景点', JUNCTION: '岔路', REST: '休息点' }
    return { ...e, title: e.title?.trim() || typeLabel[e.type] || '解说事件' }
  })
  tEventIdx = 0
  applyTuning()
  applyPlaybackRate()
  cameraEngine.value?.setSettings({ altitude: cameraAlt.value, pitch: cameraPitch.value })
}

async function onArrange() {
  if (!currentRouteId.value || arranging.value) return
  if (!arrangeRaceName.value.trim() || !arrangeCategory.value.trim()) {
    showToast('请填写比赛名称与组别', 'error')
    return
  }
  arranging.value = true
  showArrangeDialog.value = false
  // 编排确定时长与音色，同步到当前播放设置
  routeDurationMin.value = Math.max(1, Math.round((arrangeDurationSec.value / 60) * 2) / 2)
  try {
    // 异步任务：后端立即返回 GENERATING，前端轮询直到 READY/FAILED
    await api.arrangeWithAI(currentRouteId.value, {
      durationSec: arrangeDurationSec.value,
      voice: arrangeVoice.value,
      raceName: arrangeRaceName.value.trim(),
      category: arrangeCategory.value.trim(),
    })
    const started = Date.now()
    let cfg: ShowConfig | null = null
    let done = false
    while (!done && Date.now() - started < 8 * 60_000) {
      await sleep(5000)
      const resp = await api.getShowConfig(currentRouteId.value)
      if (resp.status === 'FAILED') throw new Error(resp.errorMessage || 'AI 生成失败')
      if (resp.status === 'READY' && resp.config) { cfg = resp.config; done = true }
    }
    if (!cfg) throw new Error('生成超时，请稍后重试')
    applyConfig(cfg)
    // 以编排的音色合成语音（事件各自的 voice 已由 AI 按规则指定）
    lastSynthKey = ''
    void synthesizeTimeline()
    showToast(`AI 编排完成：${cfg.meta?.title ?? ''}（${cfg.events?.length ?? 0} 个解说事件）`, 'success')
  } catch (e) {
    showToast('AI 编排失败: ' + (e instanceof Error ? e.message : String(e)), 'error')
  } finally {
    arranging.value = false
  }
}

const sleep = (ms: number) => new Promise<void>(r => setTimeout(r, ms))

// ── 视频导出流程 ─────────────────────────────────────────────────────
function startRecording(): { rec: MediaRecorder; stop: () => Promise<Blob> } {
  const canvas = document.querySelector('.maplibregl-canvas') as HTMLCanvasElement | null
  if (!canvas) throw new Error('未找到地图画布')
  const stream = canvas.captureStream(30)
  ensureAudioGraph()
  const audioTrack = audioDest!.stream.getAudioTracks()[0]
  if (audioTrack) stream.addTrack(audioTrack)
  const mime = ['video/webm;codecs=vp9,opus', 'video/webm;codecs=vp8,opus', 'video/webm']
    .find(t => MediaRecorder.isTypeSupported(t)) ?? 'video/webm'
  const chunks: Blob[] = []
  const rec = new MediaRecorder(stream, { mimeType: mime, videoBitsPerSecond: 8_000_000 })
  rec.ondataavailable = e => { if (e.data.size > 0) chunks.push(e.data) }
  rec.start(1000)
  return {
    rec,
    stop: () => new Promise<Blob>(resolve => {
      rec.onstop = () => resolve(new Blob(chunks, { type: mime }))
      rec.stop()
    }),
  }
}

async function startExport() {
  if (exporting.value || !currentRouteId.value) return
  exporting.value = true
  showExportDialog.value = false
  try {
    ensureAudioGraph()
    exportMode = true
    // 1. 语音以编排为准：未合成的补齐（已合成直接跳过）
    exportStatus.value = '检查解说语音…'
    lastSynthKey = ''
    await synthesizeTimeline()
    // 2. 创建导出任务
    const task = await api.createExportTask(currentRouteId.value)
    const tid = task.id
    // 3. 重置到起点并武装时间轴
    stop()
    rearmTimeline(0)
    // 4. 开始录制（含开场动画）
    exportStatus.value = '录制中…'
    const recording = startRecording()
    togglePlay() // 从起点触发 Intro + 播放
    // 5. 等待播放 + Outro 结束
    await new Promise<void>(resolve => {
      const t = setInterval(() => {
        if (!exporting.value || playbackStore.phase === 'ended') { clearInterval(t); resolve() }
      }, 500)
    })
    // 6. 停止录制并上传
    exportStatus.value = '录制完成，上传中…'
    const blob = await recording.stop()
    exportMode = false
    timelineAudio?.pause()
    const form = new FormData()
    form.append('video', blob, `route${currentRouteId.value}_${tid}.webm`)
    await fetch(`/api/export/upload/${tid}`, { method: 'POST', body: form })
    // 7. 轮询后端转码状态
    exportStatus.value = '后端转码 MP4 中…'
    let out = ''
    for (let i = 0; i < 80; i++) {
      await sleep(3000)
      const t = await api.getExportTask(tid)
      if (t.status === 'SUCCESS') { out = t.output_path; break }
      if (t.status === 'FAILED') throw new Error(t.error_message || '转码失败')
    }
    exportStatus.value = '完成: ' + out
    showToast('MP4 导出完成: ' + out, 'success')
  } catch (e) {
    exportMode = false
    exportStatus.value = '导出失败'
    showToast('导出失败: ' + (e instanceof Error ? e.message : String(e)), 'error')
  } finally {
    exporting.value = false
  }
}

async function synthesizeTimeline() {
  if (synthRunning || !currentRouteId.value) return
  const key = timelineEvents.value.map(e => `${e.voice}|${e.script.slice(0, 10)}`).join(',')
  if (key === lastSynthKey) return
  synthRunning = true
  lastSynthKey = key
  const list = timelineEvents.value
  let done = 0
  for (const ev of list) {
    try {
      const r = await api.synthesizeEventTTS(currentRouteId.value, {
        text: ev.script,
        voice: ev.voice || 'zh-CN-XiaoxiaoNeural',
      })
      ev.audioUrl = r.url
      ev.audioSec = r.durationSec
    } catch { /* 单条失败不影响其余 */ }
    done++
    showToast(`解说语音合成中 ${done}/${list.length}`, 'info')
  }
  showToast('解说语音全部就绪', 'success')
  synthRunning = false
}

// ── 时间轴引擎：语音为画外音随画面播放；仅途经点稍微停留 ─────────────
async function handleTimelineEvents() {
  if (tEventBusy || !timelineEvents.value.length || !routePlayer.value || !playbackStore.isPlaying) return
  const ev = timelineEvents.value[tEventIdx]
  if (!ev) return
  if (routePlayer.value.getDistance() < ev.atKm * 1000) return
  tEventBusy = true
  const isWaypoint = ev.type === 'POI' || ev.type === 'REST'
  showToast(`🎙 ${ev.title || '解说'}`, 'info')

  // 途经点：稍微停留（最多 2 秒）表达"到站"，随后恢复行进
  if (isWaypoint) {
    const hold = Math.min(2, (ev.holdBefore ?? 1) || 1)
    routePlayer.value.pause()
    await sleep(hold * 1000)
    if (playbackStore.isPlaying) routePlayer.value.play()
  }

  // 语音作为画外音播放：不阻塞行进，上一条未播完自动让位
  if (ev.audioUrl) {
    timelineAudio?.pause()
    const a = new Audio(ev.audioUrl)
    timelineAudio = a
    // 导出时把语音接入录制混流（观众仍能从扬声器听到）
    if (exportMode && audioCtx && audioDest) {
      try {
        const src = audioCtx.createMediaElementSource(a)
        src.connect(audioCtx.destination)
        src.connect(audioDest)
      } catch { /* 已连接过则忽略 */ }
    }
    a.play().catch(() => {})
  }

  if (isWaypoint && (ev.holdAfter ?? 0) > 0) {
    const hold = Math.min(2, ev.holdAfter)
    routePlayer.value.pause()
    await sleep(hold * 1000)
    if (playbackStore.isPlaying) routePlayer.value.play()
  }
  tEventIdx++
  tEventBusy = false
}

function rearmTimeline(dist: number) {
  tEventIdx = timelineEvents.value.findIndex(e => e.atKm * 1000 > dist)
  if (tEventIdx < 0) tEventIdx = timelineEvents.value.length
}

// ── 视频导出：忠实执行编排配置（音色/时长以编排为准，导出不改创作参数）──
const routeList = ref<Route[]>([])
const showExportDialog = ref(false)
const exporting = ref(false)
const exportStatus = ref('')
let audioCtx: AudioContext | null = null
let audioDest: MediaStreamAudioDestinationNode | null = null
let exportMode = false
let lastSynthKey = ''

const currentVoiceLabel = computed(() => {
  const names: Record<string, string> = {
    'zh-CN-XiaoxiaoNeural': '晓晓', 'zh-CN-YunxiNeural': '云希',
    'zh-CN-YunyangNeural': '云扬', 'zh-CN-XiaoyiNeural': '晓伊',
  }
  const v = timelineEvents.value.find(e => e.voice)?.voice ?? ''
  return names[v] ?? '晓晓'
})

function ensureAudioGraph() {
  if (!audioCtx) {
    audioCtx = new AudioContext()
    audioDest = audioCtx.createMediaStreamDestination()
  }
  if (audioCtx.state === 'suspended') void audioCtx.resume()
}

async function refreshRouteList() {
  try { routeList.value = await api.listRoutes() } catch { /* 后端不可用 */ }
}

function onSelectRoute(e: Event) {
  const id = parseInt((e.target as HTMLSelectElement).value)
  const r = routeList.value.find(x => x.id === id)
  if (!r || r.id === currentRouteId.value) return
  stop()
  routeStore.clearRoute()
  routeStore.currentRoute = r
  void routeStore.fetchPoints(id)
}

// 时间轴事件也显示在时间线上
const displayEvents = computed<StoryEvent[]>(() => [
  ...events.value,
  ...timelineEvents.value.map((e, i) => ({
    id: 900000 + i, route_id: currentRouteId.value ?? 0, position: e.atKm,
    event_type: e.type, title: e.title, script: e.script,
    hold_before: e.holdBefore, hold_after: e.holdAfter,
    tts_audio_url: e.audioUrl ?? '', tts_duration: e.audioSec ?? 0,
    tts_status: '', description: '', camera_preset: '', enabled: true, order: i,
    created_at: '', updated_at: '',
  } as StoryEvent)),
])

watch(currentRouteId, async (id) => {
  if (!id) { segments.value = []; events.value = []; return }
  try {
    segments.value = await (await fetch(`/api/routes/${id}/segments`)).json()
  } catch { segments.value = [] }
  try {
    events.value = await (await fetch(`/api/routes/${id}/events`)).json()
  } catch { events.value = [] }
  // GPX 航点显示到地图
  try {
    const wpts = await api.getWaypoints(id)
    mapContainerRef.value?.setWaypoints(wpts)
  } catch { /* 无航点则忽略 */ }
  // 恢复该路线的演出配置（仅 READY 状态自动应用）
  try {
    const resp = await api.getShowConfig(id)
    if (resp.status === 'READY' && resp.config) applyConfig(resp.config)
  } catch { /* 无配置则忽略 */ }
})
</script>

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #1a1a2e; color: #eee; }
.app { display: flex; flex-direction: column; height: 100vh; overflow: hidden; position: relative; }
.toolbar { padding: 10px 20px; background: #16213e; border-bottom: 1px solid #0f3460; display: flex; align-items: center; justify-content: space-between; }
.toolbar h1 { font-size: 16px; font-weight: 600; color: #e94560; }
.toolbar-right { display: flex; align-items: center; gap: 10px; }
.route-name { color: #8899aa; font-size: 12px; max-width: 320px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.btn-upload { background: #4ecca3; color: #16213e; border: none; padding: 5px 12px; border-radius: 5px; font-size: 12px; cursor: pointer; font-weight: 600; }
.btn-upload:hover { background: #3bb890; }
.btn-arrange { background: #533483; color: #eee; border: none; padding: 5px 12px; border-radius: 5px; font-size: 12px; cursor: pointer; font-weight: 600; }
.btn-arrange:hover:not(:disabled) { background: #6a429f; }
.btn-arrange:disabled { opacity: 0.5; cursor: not-allowed; }
.arrange-dialog-mask { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.6); z-index: 10000; display: flex; align-items: center; justify-content: center; }
.arrange-dialog { width: 360px; background: #16213e; border: 1px solid #0f3460; border-radius: 10px; padding: 20px; }
.arrange-dialog h3 { font-size: 15px; color: #e94560; margin-bottom: 8px; }
.arrange-dialog label { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: #8899aa; margin-bottom: 10px; }
.arrange-dialog input { background: #0f3460; color: #eee; border: 1px solid #2a3a5e; border-radius: 5px; padding: 7px 10px; font-size: 13px; outline: none; }
.arrange-dialog input:focus { border-color: #e94560; }
.arrange-tip { font-size: 11px; color: #667; margin-bottom: 10px; line-height: 1.5; }
.export-select { background: #0f3460; color: #eee; border: 1px solid #2a3a5e; border-radius: 5px; padding: 7px 10px; font-size: 13px; outline: none; width: 100%; }
.arrange-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 6px; }
.arrange-actions .btn-cancel { background: transparent; color: #8899aa; border: 1px solid #0f3460; padding: 6px 14px; border-radius: 5px; cursor: pointer; font-size: 12px; }
.arrange-actions .btn-confirm { background: #e94560; color: #fff; border: none; padding: 6px 14px; border-radius: 5px; cursor: pointer; font-size: 12px; font-weight: 600; }
.arrange-actions .btn-confirm:disabled { opacity: 0.4; cursor: not-allowed; }
.upload-overlay-inner { position: relative; }
.overlay-close { position: absolute; top: 10px; right: 10px; z-index: 1; width: 26px; height: 26px; border-radius: 50%; border: 1px solid #2a3a5e; background: #0f3460; color: #8899aa; cursor: pointer; font-size: 12px; }
.overlay-close:hover { color: #eee; background: #e94560; border-color: #e94560; }
.main { flex: 1; display: flex; position: relative; overflow: hidden; }
.upload-overlay { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; z-index: 10; background: rgba(26, 26, 46, 0.6); }
.upload-overlay .upload-zone { width: 460px; max-width: 90%; background: #16213e; border-radius: 10px; }
.left-panel { position: absolute; left: 0; top: 0; bottom: 0; z-index: 5; overflow: hidden; }
.right-panel { position: absolute; right: 0; top: 0; bottom: 0; z-index: 5; display: flex; flex-direction: column; }
.controls { padding: 8px 20px; background: #16213e; border-top: 1px solid #0f3460; display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.btn-play { width: 38px; height: 30px; border-radius: 6px; font-size: 15px; cursor: pointer; border: 1px solid #0f3460; background: #0f3460; color: #eee; }
.btn-play:hover:not(:disabled) { background: #e94560; border-color: #e94560; }
.btn { padding: 4px 10px; border-radius: 6px; cursor: pointer; border: 1px solid #0f3460; background: transparent; color: #eee; font-size: 12px; }
.btn:hover:not(:disabled) { background: #0f3460; }
.btn:disabled, .btn-play:disabled { opacity: 0.4; cursor: not-allowed; }
.control-group, .control-item { display: flex; align-items: center; gap: 5px; font-size: 11px; }
.control-item .label { color: #8899aa; }
.control-item .val { color: #eee; min-width: 36px; }
select.select { background: #0f3460; color: #eee; border: none; padding: 3px 6px; border-radius: 4px; font-size: 11px; }
.slider, .seek-bar { width: 70px; accent-color: #e94560; }
.btn-toggle { background: transparent; color: #8899aa; border: 1px solid #0f3460; padding: 3px 8px; border-radius: 4px; font-size: 11px; cursor: pointer; }
.btn-toggle.active { background: #0f3460; color: #e94560; border-color: #e94560; }
.tuning-panel { position: absolute; left: 12px; bottom: 64px; z-index: 30; background: rgba(22, 33, 62, 0.96); border: 1px solid #0f3460; border-radius: 8px; padding: 10px 12px; display: flex; flex-direction: column; gap: 7px; width: 250px; }
.tuning-title { font-size: 12px; color: #e94560; font-weight: 600; }
.tuning-section { font-size: 10px; color: #4ecca3; margin-top: 2px; }
.tuning-row { display: flex; align-items: center; gap: 6px; font-size: 11px; color: #8899aa; }
.tuning-row > span:first-child { width: 60px; flex-shrink: 0; }
.tuning-row .val { width: 44px; text-align: right; color: #eee; flex-shrink: 0; }
.tuning-panel input[type="range"] { flex: 1; width: auto; accent-color: #e94560; }
.btn-reset { background: transparent; color: #8899aa; border: 1px solid #0f3460; padding: 3px 8px; border-radius: 4px; font-size: 11px; cursor: pointer; align-self: flex-start; }
.btn-reset:hover { color: #eee; background: #0f3460; }
.marker-file { flex: 1; font-size: 10px; color: #8899aa; min-width: 0; }
.marker-file::file-selector-button { background: #0f3460; color: #eee; border: 1px solid #2a3a5e; border-radius: 3px; padding: 2px 6px; font-size: 10px; cursor: pointer; margin-right: 6px; }
.tuning-hint { font-size: 10px; color: #667; line-height: 1.4; }
.progress-info { color: #4ecca3; font-size: 12px; margin-left: auto; }
.toast { position: fixed; bottom: 80px; left: 50%; transform: translateX(-50%); padding: 8px 20px; border-radius: 6px; font-size: 13px; z-index: 9999; animation: fadeIn 0.2s; }
.toast.info { background: #0f3460; color: #eee; }
.toast.error { background: #e94560; color: #fff; }
.toast.success { background: #4ecca3; color: #16213e; }
@keyframes fadeIn { from { opacity: 0; transform: translateX(-50%) translateY(10px); } to { opacity: 1; transform: translateX(-50%) translateY(0); } }
</style>
