<template>
  <div class="route-overlay" :style="{ opacity: visible ? 1 : 0 }">
    <div v-if="currentPoint" class="overlay-content">
      <div class="overlay-row dist-row">
        <span class="ov-label">距离</span>
        <span class="ov-value">{{ (currentPoint.distance / 1000).toFixed(2) }} <span class="ov-unit">km</span></span>
        <span class="ov-sep">/</span>
        <span class="ov-value total">{{ (totalDistance / 1000).toFixed(1) }} <span class="ov-unit">km</span></span>
      </div>
      <div class="overlay-row">
        <span class="ov-label">海拔</span>
        <span class="ov-value">{{ currentPoint.elevation.toFixed(0) }} <span class="ov-unit">m</span></span>
      </div>
      <div class="overlay-row">
        <span class="ov-label">坡度</span>
        <span class="ov-value" :class="slopeClass(currentPoint.slope)">
          {{ currentPoint.slope > 0 ? '+' : '' }}{{ currentPoint.slope.toFixed(1) }}%
        </span>
      </div>
      <div v-if="commentary" class="commentary-box">
        <div class="comm-type">{{ commentaryType }}</div>
        <div class="comm-text">{{ commentary }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { TrackPoint, RouteSegment, StoryEvent } from '@/types/gpx'

const props = defineProps<{
  currentPoint: TrackPoint | null
  totalDistance: number
  segments: RouteSegment[]
  events: StoryEvent[]
  visible: boolean
}>()

const commentary = computed(() => {
  if (!props.currentPoint) return ''
  const d = props.currentPoint.distance
  const seg = props.segments.find(s => d >= s.start_distance && d <= s.end_distance && s.commentary)
  if (seg?.commentary) return seg.commentary
  const ev = props.events.find(e => Math.abs(e.position * 1000 - d) < 100 && e.script)
  if (ev?.script) return ev.script
  return ''
})

const commentaryType = computed(() => {
  if (!props.currentPoint) return ''
  const d = props.currentPoint.distance
  const seg = props.segments.find(s => d >= s.start_distance && d <= s.end_distance)
  if (seg) return seg.type === 'FLAT' ? '平缓段' : seg.type === 'CLIMB' ? '爬升段' : seg.type === 'STEEP_CLIMB' ? '陡坡段' : seg.type === 'DESCENT' ? '下降段' : '陡降段'
  const ev = props.events.find(e => Math.abs(e.position * 1000 - d) < 100)
  if (ev) return ev.event_type
  return ''
})

function slopeClass(slope: number) {
  if (slope > 10) return 'steep'
  if (slope > 5) return 'moderate-up'
  if (slope < -10) return 'steep-down'
  if (slope < -5) return 'moderate-down'
  return ''
}
</script>

<style scoped>
.route-overlay {
  position: absolute;
  bottom: 80px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(22, 33, 62, 0.92);
  border: 1px solid #0f3460;
  border-radius: 8px;
  padding: 10px 20px;
  min-width: 280px;
  text-align: center;
  transition: opacity 0.2s;
  pointer-events: none;
  z-index: 10;
}
.overlay-row { display: flex; align-items: center; justify-content: center; gap: 8px; font-size: 14px; }
.ov-label { color: #8899aa; font-size: 12px; }
.ov-value { color: #eee; font-weight: 600; }
.ov-unit { font-size: 11px; color: #8899aa; font-weight: 400; }
.ov-sep { color: #444; }
.ov-value.total { color: #e94560; }
.steep { color: #e94560; }
.moderate-up { color: #f0a500; }
.steep-down { color: #ff6b6b; }
.moderate-down { color: #ffa500; }
.commentary-box {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid #0f3460;
  text-align: left;
}
.comm-type { font-size: 10px; color: #4ecca3; text-transform: uppercase; letter-spacing: 1px; margin-bottom: 2px; }
.comm-text { font-size: 12px; color: #ccc; line-height: 1.4; }
</style>
