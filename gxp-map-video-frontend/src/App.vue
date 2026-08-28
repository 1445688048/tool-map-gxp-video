<template>
  <div class="app">
    <header class="toolbar">
      <h1>GPX 越野路线 AI 预演</h1>
      <div class="toolbar-right">
        <select v-model="activePreset" class="preset-select">
          <option value="trail_running">🏃 越野跑</option>
          <option value="hiking">🥾 徒步登山</option>
          <option value="mtb">🚵 山地骑行</option>
        </select>
        <button @click="autoAnalyze" :disabled="!currentRouteId || analyzing" class="btn-analyze">
          {{ analyzing ? '分析中...' : '自动分段' }}
        </button>
        <button @click="generateCommentary" :disabled="!currentRouteId" class="btn-commentary">
          生成解说
        </button>
      </div>
    </header>

    <main class="main">
      <!-- Left panel: Segments -->
      <div v-if="showSegments" class="left-panel">
        <SegmentList :route-id="currentRouteId" @segment-updated="onSegmentUpdated" />
      </div>

      <!-- Map -->
      <MapContainer ref="mapContainerRef" />

      <!-- Right panel: Events + Intro/Outro -->
      <div v-if="showEvents" class="right-panel">
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
      v-if="segments.length > 0 || events.length > 0"
      :segments="segments"
      :events="events"
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
        <select v-model.number="speed" @change="onSpeedChange" class="select">
          <option value="0.25">0.25x</option>
          <option value="0.5">0.5x</option>
          <option value="1">1x</option>
          <option value="2">2x</option>
          <option value="5">5x</option>
          <option value="10">10x</option>
        </select>
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
        <input type="range" min="20" max="2000" step="10" v-model.number="cameraAlt" class="slider" />
        <span class="val">{{ cameraAlt }}m</span>
      </div>

      <div class="control-item">
        <span class="label">俯角</span>
        <input type="range" min="0" max="85" step="1" v-model.number="cameraPitch" class="slider" />
        <span class="val">{{ cameraPitch }}°</span>
      </div>

      <button @click="showSegments = !showSegments" :class="['btn-toggle', showSegments ? 'active' : '']">分段</button>
      <button @click="showEvents = !showEvents" :class="['btn-toggle', showEvents ? 'active' : '']">事件</button>

      <div class="progress-info" v-if="hasRoute">
        <span>{{ ((player?.getDistance() ?? 0) / 1000).toFixed(2) }} / {{ (totalDistance / 1000).toFixed(1) }} km</span>
      </div>
    </footer>

    <!-- Toast notifications -->
    <Teleport to="body">
      <div v-if="toast.show" :class="['toast', toast.type]">
        {{ toast.message }}
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import MapContainer from './components/map/MapContainer.vue'
import SegmentList from './components/segment/SegmentList.vue'
import EventEditor from './components/event/EventEditor.vue'
import TimelineEditor from './components/timeline/TimelineEditor.vue'
import RouteOverlay from './components/overlay/RouteOverlay.vue'
import IntroOutroConfig from './components/intro-outro/IntroOutroConfig.vue'
import { useRouteStore } from './stores/route'
import { usePlaybackStore } from './stores/playback'
import { CameraEngine } from './engines/camera-engine'
import { RoutePlayer } from './engines/route-player'
import type { RouteSegment, StoryEvent } from './types/gpx'

const routeStore = useRouteStore()
const playbackStore = usePlaybackStore()

const activePreset = ref('trail_running')
const analyzing = ref(false)
const showSegments = ref(true)
const showEvents = ref(false)
const segments = ref<RouteSegment[]>([])
const events = ref<StoryEvent[]>([])
const introOutroRef = ref<InstanceType<typeof IntroOutroConfig> | null>(null)

const hasRoute = computed(() => routeStore.currentRoute !== null)
const currentRouteId = computed(() => routeStore.currentRoute?.id ?? null)
const totalDistance = computed(() => routeStore.currentRoute?.total_distance ?? 0)
const isPlaying = computed(() => playbackStore.isPlaying)
const cameraAlt = ref(100)
const cameraPitch = ref(60)
const speed = ref(1)
const stepMode = ref<'off' | 'hill-skip'>('off')
const seekPos = ref(0)

let mapContainerRef: any = null
let cameraEngine: CameraEngine | null = null
let routePlayer: RoutePlayer | null = null
const player = computed(() => routePlayer)

const canPlay = computed(() => hasRoute.value && routePlayer !== null)

// Toast notification system (replaces alert())
const toast = ref({ show: false, message: '', type: 'info' as 'info' | 'error' | 'success' })
let toastTimer: ReturnType<typeof setTimeout> | null = null
function showToast(message: string, type: 'info' | 'error' | 'success' = 'info') {
  toast.value = { show: true, message, type }
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toast.value.show = false }, 3000)
}

watch(() => routeStore.trackPoints, (pts) => {
  if (pts.length < 2) return
  if (!cameraEngine) {
    cameraEngine = new CameraEngine()
    routePlayer = new RoutePlayer(cameraEngine)
    routePlayer.onComplete(() => {
      playbackStore.phase = 'ended'
    })
  }
  if (!routePlayer) return
  routePlayer.setPoints(pts)
  cameraEngine.setSettings({ altitude: cameraAlt.value, pitch: cameraPitch.value })
})

function togglePlay() {
  if (!routePlayer) return
  if (playbackStore.isPlaying) {
    routePlayer.pause()
    playbackStore.pause()
  } else {
    playbackStore.phase = 'playing'
    if (routePlayer) { routePlayer.play(); playbackStore.play() }
    const poll = setInterval(() => {
      if (!playbackStore.isPlaying) { clearInterval(poll); return }
      if (routePlayer) seekPos.value = routePlayer.getDistance()
    }, 100)
  }
}

function stop() {
  routePlayer?.pause()
  routePlayer?.stop()
  playbackStore.pause()
  playbackStore.seekTo(0)
  playbackStore.phase = 'idle'
}

function reset() { stop() }

function onSpeedChange() {
  if (routePlayer) routePlayer.setSpeed(speed.value)
}

function onStepModeChange() {
  if (routePlayer) routePlayer.setStepMode(stepMode.value)
}

watch([cameraAlt, cameraPitch], ([alt, pitch]) => {
  if (cameraEngine) cameraEngine.setSettings({ altitude: alt, pitch })
})

watch(seekPos, (val) => {
  if (routePlayer) {
    routePlayer.seekTo(val)
    playbackStore.seekTo(routePlayer.progress)
  }
})

async function autoAnalyze() {
  if (!currentRouteId.value) return
  analyzing.value = true
  try {
    segments.value = await (await fetch(`/api/routes/${currentRouteId.value}/analyze`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ preset: activePreset.value }),
    })).json()
    showToast(`分析完成，共 ${segments.value.length} 个分段`, 'success')
  } catch (e) {
    showToast('分析失败: ' + (e instanceof Error ? e.message : String(e)), 'error')
  } finally {
    analyzing.value = false
  }
}

function generateCommentary() {
  if (segments.value.length === 0) return
  let updated = false
  for (const seg of segments.value) {
    if (seg.commentary && seg.commentary.length > 0) continue
    seg.commentary = generateTemplateCommentary(seg)
    updated = true
  }
  if (updated) {
    showToast('解说模板已生成，请手动保存修改', 'info')
  }
}

function generateTemplateCommentary(seg: RouteSegment): string {
  const distKm = (seg.distance / 1000).toFixed(2)
  const gain = seg.elevation_gain.toFixed(0)
  const loss = seg.elevation_loss.toFixed(0)
  const avgSlope = seg.average_slope.toFixed(1)

  const templates: Record<string, string> = {
    FLAT: `平缓路段，长度${distKm}公里，${gain}米爬升${loss}米下降，平均坡度${avgSlope}%，地形相对稳定。`,
    CLIMB: `进入爬升段，长度${distKm}公里，累计爬升${gain}米，平均坡度+${avgSlope}%，注意控制呼吸节奏。`,
    STEEP_CLIMB: `陡坡爬升！长度${distKm}公里，累计爬升${gain}米，最大坡度+${seg.max_slope}%，建议步行通过。`,
    DESCENT: `进入下降段，长度${distKm}公里，累计下降${loss}米，平均坡度${avgSlope}%，注意控制速度。`,
    STEEP_DESCENT: `陡降路段！长度${distKm}公里，累计下降${loss}米，最大坡度${seg.max_slope}%，小心湿滑路面。`,
  }
  return templates[seg.type] ?? `路线路段，长度${distKm}公里。`
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
  if (routePlayer) {
    routePlayer.seekTo(dist)
    playbackStore.seekTo(routePlayer.progress)
  }
}

watch(currentRouteId, async (id) => {
  if (!id) { segments.value = []; events.value = []; return }
  try {
    segments.value = await (await fetch(`/api/routes/${id}/segments`)).json()
  } catch { segments.value = [] }
  try {
    events.value = await (await fetch(`/api/routes/${id}/events`)).json()
  } catch { events.value = [] }
})
</script>

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #1a1a2e; color: #eee; }
.app { display: flex; flex-direction: column; height: 100vh; overflow: hidden; position: relative; }
.toolbar { padding: 10px 20px; background: #16213e; border-bottom: 1px solid #0f3460; display: flex; align-items: center; justify-content: space-between; }
.toolbar h1 { font-size: 16px; font-weight: 600; color: #e94560; }
.toolbar-right { display: flex; align-items: center; gap: 10px; }
.preset-select { background: #0f3460; color: #eee; border: none; padding: 5px 10px; border-radius: 5px; font-size: 12px; }
.btn-analyze { background: #4ecca3; color: #16213e; border: none; padding: 5px 12px; border-radius: 5px; font-size: 12px; cursor: pointer; font-weight: 600; }
.btn-analyze:hover:not(:disabled) { background: #3bb890; }
.btn-analyze:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-commentary { background: #533483; color: #eee; border: none; padding: 5px 12px; border-radius: 5px; font-size: 12px; cursor: pointer; }
.main { flex: 1; display: flex; position: relative; overflow: hidden; }
.left-panel { position: absolute; left: 0; top: 0; bottom: 0; z-index: 5; }
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
.progress-info { color: #4ecca3; font-size: 12px; margin-left: auto; }
.toast { position: fixed; bottom: 80px; left: 50%; transform: translateX(-50%); padding: 8px 20px; border-radius: 6px; font-size: 13px; z-index: 9999; animation: fadeIn 0.2s; }
.toast.info { background: #0f3460; color: #eee; }
.toast.error { background: #e94560; color: #fff; }
.toast.success { background: #4ecca3; color: #16213e; }
@keyframes fadeIn { from { opacity: 0; transform: translateX(-50%) translateY(10px); } to { opacity: 1; transform: translateX(-50%) translateY(0); } }
</style>
