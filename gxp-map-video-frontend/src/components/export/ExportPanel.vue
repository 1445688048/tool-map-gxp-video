<template>
  <div class="export-panel">
    <h3>视频导出</h3>
    <div class="export-status" v-if="task">
      <div class="status-line">
        <span :class="'status-dot ' + task.status.toLowerCase()"></span>
        <span>{{ statusText(task.status) }}</span>
      </div>
      <div v-if="task.status === 'RUNNING'" class="progress-bar">
        <div class="progress-fill" :style="{ width: (task.progress * 100) + '%' }"></div>
      </div>
      <div v-if="task.error_message" class="error-text">{{ task.error_message }}</div>
      <div v-if="task.output_path" class="output-path">
        输出: {{ task.output_path }}
      </div>
    </div>

    <div class="export-actions">
      <button @click="preflight" :disabled="loadingPreflight" class="btn-preflight">
        {{ loadingPreflight ? '检查中...' : '检查瓦片' }}
      </button>
      <button @click="preload" :disabled="loadingPreload || !routeId" class="btn-preload">
        {{ loadingPreload ? '预加载中...' : '预加载瓦片' }}
      </button>
      <button @click="startExport" :disabled="exporting || !routeId" class="btn-export">
        {{ exporting ? '录制中...' : '开始导出' }}
      </button>
      <button v-if="task?.status === 'RUNNING'" @click="cancelExport" class="btn-cancel">取消</button>
    </div>

    <div v-if="preflightResult" class="preflight-result">
      <div>总瓦片数: <strong>{{ preflightResult.total_tiles }}</strong></div>
      <div>缺失瓦片: <strong :class="{ bad: preflightResult.missing_tiles > 0 }">{{ preflightResult.missing_tiles }}</strong></div>
      <div>Zoom范围: {{ preflightResult.zoom_range }}</div>
    </div>

    <div class="recorder-info" v-if="isRecording">
      <div class="rec-dot"></div>
      <span>正在录制... {{ recordingTime }}s</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { api } from '@/api/client'
import type { ExportTask, ActivityPreset } from '@/types/gpx'
import { VideoRecorder } from '@/engines/video-recorder'
import { usePlaybackStore } from '@/stores/playback'

const props = defineProps<{ routeId: number | null }>()
const playbackStore = usePlaybackStore()

const task = ref<ExportTask | null>(null)
const preflightResult = ref<{ total_tiles: number; missing_tiles: number; zoom_range: string } | null>(null)
const loadingPreflight = ref(false)
const loadingPreload = ref(false)
const exporting = ref(false)
const isRecording = ref(false)
const recordingTime = ref(0)
let recorder: VideoRecorder | null = null
let recordTimer: ReturnType<typeof setInterval> | null = null

async function preflight() {
  if (!props.routeId) return
  loadingPreflight.value = true
  try {
    preflightResult.value = await api.preflightExport(props.routeId) as any
  } catch (e) {
    alert('预检失败: ' + (e instanceof Error ? e.message : String(e)))
  } finally {
    loadingPreflight.value = false
  }
}

async function preload() {
  if (!props.routeId) return
  loadingPreload.value = true
  try {
    await api.preloadTiles(props.routeId)
    alert('瓦片预加载完成')
    await preflight()
  } catch (e) {
    alert('预加载失败: ' + (e instanceof Error ? e.message : String(e)))
  } finally {
    loadingPreload.value = false
  }
}

async function startExport() {
  if (!props.routeId) return
  exporting.value = true
  isRecording.value = true
  recordingTime.value = 0

  // Create export task
  task.value = await api.createExportTask(props.routeId) as ExportTask

  // Start recording
  recorder = new VideoRecorder()
  try {
    await recorder.start()
    recordTimer = setInterval(() => { recordingTime.value++ }, 1000)

    // Play the route fully while recording
    playbackStore.play()
    const baseSpeed = 3 // faster playback for export
    if (recorder.setSpeed) recorder.setSpeed(baseSpeed)

    // Wait for playback to complete
    await new Promise<void>((resolve) => {
      const check = setInterval(() => {
        if (!playbackStore.isPlaying) {
          clearInterval(check)
          resolve()
        }
      }, 200)
    })
  } finally {
    if (recordTimer) clearInterval(recordTimer)
    isRecording.value = false
  }

  // Stop recording and upload
  const webmBlob = await recorder.stop()
  exporting.value = false

  // Upload WebM to backend
  const form = new FormData()
  form.append('video', webmBlob, `export_${task.value!.id}.webm`)
  await fetch(`/api/export/upload/${task.value!.id}`, { method: 'POST', body: form })

  alert('视频已上传，后端正在转码为MP4，请稍后查看任务状态')
}

async function cancelExport() {
  if (!task.value?.id) return
  await api.cancelExportTask(task.value.id)
  task.value = null
}

function statusText(status: string) {
  const map: Record<string, string> = {
    PENDING: '等待中', RUNNING: '导出中...', SUCCESS: '完成',
    FAILED: '失败', CANCELLED: '已取消',
  }
  return map[status] || status
}

const canExport = computed(() => props.routeId !== null)
</script>

<style scoped>
.export-panel { background: #16213e; padding: 12px; }
h3 { font-size: 13px; color: #e94560; margin-bottom: 10px; }
.export-actions { display: flex; flex-direction: column; gap: 6px; }
button { border: none; padding: 7px 12px; border-radius: 5px; font-size: 12px; cursor: pointer; font-weight: 500; transition: background 0.15s; }
.btn-preflight { background: #0f3460; color: #eee; }
.btn-preflight:hover:not(:disabled) { background: #1a3a6e; }
.btn-preload { background: #0f3460; color: #eee; }
.btn-preload:hover:not(:disabled) { background: #1a3a6e; }
.btn-export { background: #e94560; color: #fff; }
.btn-export:hover:not(:disabled) { background: #c73652; }
.btn-cancel { background: transparent; color: #e94560; border: 1px solid #e94560; }
button:disabled { opacity: 0.4; cursor: not-allowed; }
.export-status { margin-top: 10px; padding: 8px; background: #1a1a2e; border-radius: 4px; font-size: 12px; }
.status-line { display: flex; align-items: center; gap: 6px; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
.status-dot.pending { background: #8899aa; }
.status-dot.running { background: #f0a500; animation: blink 1s infinite; }
.status-dot.success { background: #4ecca3; }
.status-dot.failed { background: #e94560; }
.status-dot.cancelled { background: #666; }
@keyframes blink { 50% { opacity: 0.3; } }
.progress-bar { height: 4px; background: #0f3460; border-radius: 2px; margin-top: 6px; overflow: hidden; }
.progress-fill { height: 100%; background: #4ecca3; transition: width 0.3s; }
.error-text { color: #e94560; font-size: 11px; margin-top: 4px; }
.output-path { color: #8899aa; font-size: 10px; margin-top: 4px; word-break: break-all; }
.preflight-result { margin-top: 10px; padding: 8px; background: #1a1a2e; border-radius: 4px; font-size: 11px; color: #aaa; display: flex; flex-direction: column; gap: 3px; }
.preflight-result strong { color: #eee; }
.preflight-result .bad { color: #e94560; }
.recorder-info { margin-top: 8px; display: flex; align-items: center; gap: 6px; font-size: 12px; color: #e94560; }
.rec-dot { width: 8px; height: 8px; background: #e94560; border-radius: 50%; animation: blink 1s infinite; }
</style>
