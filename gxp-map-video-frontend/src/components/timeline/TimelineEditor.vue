<template>
  <div class="timeline-editor">
    <div class="tl-header">
      <h3>故事时间线</h3>
      <span class="tl-total">{{ (totalDistance / 1000).toFixed(1) }} km</span>
    </div>
    <div class="tl-track" @click="onTrackClick">
      <div class="tl-route-line"></div>
      <!-- Segment markers -->
      <div
        v-for="(seg, i) in segments"
        :key="seg.id"
        class="tl-segment"
        :style="{
          left: segPercent(seg.start_distance) + '%',
          width: segWidth(seg) + '%',
          backgroundColor: segColor(seg.type),
          opacity: seg.enabled ? 0.8 : 0.3
        }"
        :title="`${typeLabel(seg.type)} ${seg.start_distance.toFixed(0)}m-${seg.end_distance.toFixed(0)}m`"
      ></div>
      <!-- Event markers -->
      <div
        v-for="ev in events"
        :key="ev.id"
        class="tl-event"
        :style="{ left: posPercent(ev.position * 1000) + '%' }"
        :title="`${ev.title || ev.event_type} @ ${ev.position.toFixed(2)}km`"
      >
        {{ eventIcon(ev.event_type) }}
      </div>
      <!-- Play head -->
      <div class="tl-playhead" :style="{ left: playProgress + '%' }">
        <div class="tl-playhead-dot"></div>
      </div>
    </div>
    <div class="tl-legend">
      <span class="legend-item"><span class="dot" style="background:#8899aa"></span>平缓</span>
      <span class="legend-item"><span class="dot" style="background:#f0a500"></span>爬升</span>
      <span class="legend-item"><span class="dot" style="background:#e94560"></span>陡坡</span>
      <span class="legend-item"><span class="dot" style="background:#533483"></span>下降</span>
      <span class="legend-item"><span class="dot" style="background:#ff6b6b"></span>陡降</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { RouteSegment, StoryEvent } from '@/types/gpx'

const props = defineProps<{
  segments: RouteSegment[]
  events: StoryEvent[]
  totalDistance: number
  playProgress: number  // 0-100
}>()

function segPercent(dist: number) {
  if (!props.totalDistance) return 0
  return (dist / props.totalDistance) * 100
}

function segWidth(seg: RouteSegment) {
  if (!props.totalDistance) return 0
  return ((seg.end_distance - seg.start_distance) / props.totalDistance) * 100
}

function posPercent(posM: number) {
  if (!props.totalDistance) return 0
  return (posM / props.totalDistance) * 100
}

function segColor(type: string) {
  const map: Record<string, string> = {
    FLAT: '#8899aa', CLIMB: '#f0a500', STEEP_CLIMB: '#e94560',
    DESCENT: '#533483', STEEP_DESCENT: '#ff6b6b',
  }
  return map[type] || '#8899aa'
}

function typeLabel(type: string) {
  const map: Record<string, string> = {
    FLAT: '平缓', CLIMB: '爬升', STEEP_CLIMB: '陡坡',
    DESCENT: '下降', STEEP_DESCENT: '陡降',
  }
  return map[type] || type
}

function eventIcon(type: string) {
  const map: Record<string, string> = {
    COMMENTARY: '🎙️', WARNING: '⚠️', POI: '📍',
    VIEWPOINT: '🏔️', JUNCTION: '🔀', REST: '🧘',
  }
  return map[type] || '📌'
}

function onTrackClick(e: MouseEvent) {
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const pct = ((e.clientX - rect.left) / rect.width) * 100
  const dist = (pct / 100) * props.totalDistance
  emit('seek', dist)
}

const emit = defineEmits<{ seek: [distance: number] }>()
</script>

<style scoped>
.timeline-editor { background: #16213e; border-top: 1px solid #0f3460; padding: 8px 16px; }
.tl-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
.tl-header h3 { font-size: 12px; color: #e94560; }
.tl-total { font-size: 11px; color: #8899aa; }
.tl-track { position: relative; height: 40px; background: #1a1a2e; border-radius: 4px; cursor: pointer; overflow: visible; }
.tl-route-line { position: absolute; top: 50%; left: 0; right: 0; height: 2px; background: #2a3a5e; transform: translateY(-50%); }
.tl-segment { position: absolute; top: 8px; bottom: 8px; border-radius: 2px; transition: opacity 0.15s; }
.tl-segment:hover { opacity: 1 !important; }
.tl-event { position: absolute; top: 50%; transform: translate(-50%, -50%); font-size: 16px; cursor: pointer; z-index: 2; filter: drop-shadow(0 0 2px rgba(0,0,0,0.8)); }
.tl-playhead { position: absolute; top: 0; bottom: 0; width: 2px; background: #e94560; z-index: 3; pointer-events: none; }
.tl-playhead-dot { position: absolute; top: 4px; left: 50%; transform: translateX(-50%); width: 8px; height: 8px; background: #e94560; border-radius: 50%; }
.tl-legend { display: flex; gap: 10px; margin-top: 6px; flex-wrap: wrap; }
.legend-item { display: flex; align-items: center; gap: 3px; font-size: 10px; color: #8899aa; }
.dot { width: 8px; height: 8px; border-radius: 2px; display: inline-block; }
</style>
