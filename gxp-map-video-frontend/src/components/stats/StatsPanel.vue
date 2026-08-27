<template>
  <div class="stats-panel">
    <div v-if="!routeStore.currentRoute" class="empty">
      <p>上传 GPX 文件开始</p>
    </div>
    <template v-else>
      <h3>{{ routeStore.currentRoute.name }}</h3>
      <div class="stat-row">
        <span class="label">总距离</span>
        <span class="value">{{ (routeStore.currentRoute.total_distance / 1000).toFixed(1) }} km</span>
      </div>
      <div class="stat-row">
        <span class="label">累计爬升</span>
        <span class="value ascent">{{ routeStore.currentRoute.total_ascent.toFixed(0) }} m</span>
      </div>
      <div class="stat-row">
        <span class="label">累计下降</span>
        <span class="value descent">{{ routeStore.currentRoute.total_descent.toFixed(0) }} m</span>
      </div>
      <div class="stat-row">
        <span class="label">最高点</span>
        <span class="value">{{ routeStore.currentRoute.max_elevation.toFixed(0) }} m</span>
      </div>
      <div class="stat-row">
        <span class="label">最低点</span>
        <span class="value">{{ routeStore.currentRoute.min_elevation.toFixed(0) }} m</span>
      </div>
      <div class="stat-row">
        <span class="label">点数</span>
        <span class="value">{{ routeStore.currentRoute.point_count.toLocaleString() }}</span>
      </div>
    </template>

    <!-- Live stats -->
    <div v-if="playbackStore.currentPoint" class="live-stats">
      <h4>当前位置</h4>
      <div class="stat-row">
        <span class="label">距离</span>
        <span class="value">{{ (playbackStore.currentPoint.distance / 1000).toFixed(2) }} km</span>
      </div>
      <div class="stat-row">
        <span class="label">海拔</span>
        <span class="value">{{ playbackStore.currentPoint.elevation.toFixed(0) }} m</span>
      </div>
      <div class="stat-row">
        <span class="label">坡度</span>
        <span class="value" :class="slopeClass(playbackStore.currentPoint.slope)">
          {{ playbackStore.currentPoint.slope > 0 ? '+' : '' }}{{ playbackStore.currentPoint.slope.toFixed(1) }}%
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouteStore } from '@/stores/route'
import { usePlaybackStore } from '@/stores/playback'

const routeStore = useRouteStore()
const playbackStore = usePlaybackStore()

function slopeClass(slope: number) {
  if (slope > 10) return 'steep'
  if (slope > 5) return 'moderate'
  if (slope < -10) return 'steep-down'
  if (slope < -5) return 'moderate-down'
  return ''
}
</script>

<style scoped>
.stats-panel {
  width: 260px;
  background: #16213e;
  border-left: 1px solid #0f3460;
  padding: 16px;
  overflow-y: auto;
  flex-shrink: 0;
}
.stats-panel h3 { font-size: 14px; color: #e94560; margin-bottom: 12px; }
.stats-panel h4 { font-size: 12px; color: #aaa; margin-bottom: 8px; }
.stat-row { display: flex; justify-content: space-between; padding: 4px 0; font-size: 13px; }
.stat-row .label { color: #8899aa; }
.stat-row .value { color: #eee; font-weight: 500; }
.ascent { color: #4ecca3; }
.descent { color: #533483; }
.steep { color: #e94560; }
.steep-down { color: #ff6b6b; }
.moderate { color: #f0a500; }
.moderate-down { color: #ffa500; }
.empty { color: #667; text-align: center; padding: 40px 0; }
.live-stats { margin-top: 16px; padding-top: 16px; border-top: 1px solid #0f3460; }
</style>
