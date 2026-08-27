<template>
  <div class="controls">
    <button :disabled="!canPlay" @click="togglePlay" class="btn-play">{{ isPlaying ? '⏸' : '▶' }}</button>
    <button :disabled="!hasRoute" @click="stop" class="btn">⏹</button>
    <button :disabled="!hasRoute" @click="reset" class="btn">↺</button>

    <div class="slider-group" v-if="hasRoute && player">
      <input type="range" min="0" :max="totalDistance" step="1" v-model.number="seekPos" class="seek-bar" />
    </div>

    <div class="control-item" v-if="hasRoute">
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

    <div class="progress-info" v-if="hasRoute">
      <span>{{ ((player?.getDistance() ?? 0) / 1000).toFixed(2) }} / {{ (totalDistance / 1000).toFixed(1) }} km</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { usePlaybackStore } from '@/stores/playback'
import { useRouteStore } from '@/stores/route'
import { CameraEngine } from '@/engines/camera-engine'
import { RoutePlayer } from '@/engines/route-player'

const playbackStore = usePlaybackStore()
const routeStore = useRouteStore()

const hasRoute = computed(() => routeStore.currentRoute !== null)
const isPlaying = computed(() => playbackStore.isPlaying)
const totalDistance = computed(() => routeStore.currentRoute?.total_distance ?? 0)

const cameraAlt = ref(100)
const cameraPitch = ref(60)
const speed = ref(1)
const seekPos = ref(0)

let cameraEngine: CameraEngine | null = null
let routePlayer: RoutePlayer | null = null

const player = computed(() => routePlayer)

watch(() => routeStore.trackPoints, (pts) => {
  if (!cameraEngine) {
    cameraEngine = new CameraEngine()
    routePlayer = new RoutePlayer(cameraEngine)
  }
  if (pts.length > 0) {
    routePlayer!.setPoints(pts)
    cameraEngine!.setSettings({ altitude: cameraAlt.value, pitch: cameraPitch.value })
  }
}, { deep: true })

const canPlay = computed(() => hasRoute.value && routePlayer !== null)

function togglePlay() {
  if (!routePlayer) return
  if (playbackStore.isPlaying) {
    routePlayer.pause()
    playbackStore.pause()
  } else {
    routePlayer.play()
    playbackStore.play()
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
}

function reset() { stop() }

function onSpeedChange() {
  if (routePlayer) routePlayer.setSpeed(speed.value)
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
</script>

<style scoped>
.controls { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.btn-play { width: 40px; height: 32px; border-radius: 6px; font-size: 16px; cursor: pointer; border: 1px solid #0f3460; background: #0f3460; color: #eee; }
.btn-play:hover:not(:disabled) { background: #e94560; border-color: #e94560; }
.btn { padding: 4px 12px; border-radius: 6px; cursor: pointer; border: 1px solid #0f3460; background: transparent; color: #eee; }
.btn:hover:not(:disabled) { background: #0f3460; }
.btn:disabled, .btn-play:disabled { opacity: 0.4; cursor: not-allowed; }
.control-item { display: flex; align-items: center; gap: 6px; font-size: 12px; }
.control-item .label { color: #8899aa; }
.control-item .val { color: #eee; min-width: 40px; }
select.select { background: #0f3460; color: #eee; border: none; padding: 4px 8px; border-radius: 4px; }
.slider, .seek-bar { width: 80px; accent-color: #e94560; }
.progress-info { color: #4ecca3; font-size: 13px; margin-left: auto; }
</style>
