<template>
  <div id="map-container"></div>
</template>

<script setup lang="ts">
import { onMounted, onBeforeUnmount, watch, ref } from 'vue'
import { useRouteStore } from '@/stores/route'
import { MapEngine } from '@/engines/map-engine'
import type { TrackPoint, Waypoint } from '@/types/gpx'

const routeStore = useRouteStore()

const mapEngine = ref<MapEngine | null>(null)
let stopWatch: (() => void) | null = null

onMounted(async () => {
  const engine = new MapEngine()
  mapEngine.value = engine

  await engine.init('map-container', [116.4, 40.0], 12)

  // Watch for route points (fires when points arrive after upload)
  stopWatch = watch(() => routeStore.trackPoints, (pts) => {
    if (pts.length > 0 && mapEngine.value) {
      mapEngine.value.loadRoute(pts)
    }
  })
})

onBeforeUnmount(() => {
  stopWatch?.()
  mapEngine.value?.destroy()
  mapEngine.value = null
})

// Expose map controls to the parent (App.vue)
defineExpose({
  getMap: () => mapEngine.value?.map ?? null,
  loadRoute: (pts: TrackPoint[]) => mapEngine.value?.loadRoute(pts),
  setProgressPoint: (p: TrackPoint, bearing?: number) => mapEngine.value?.setProgressPoint(p, bearing),
  removeProgressMarker: () => mapEngine.value?.removeProgressMarker(),
  setProgressMarkerImage: (url: string | null) => mapEngine.value?.setProgressMarkerImage(url),
  setProgressMarkerSprite: (url: string, opts?: { row?: number; cols?: number; frames?: number; fps?: number; size?: number }) => mapEngine.value?.setProgressMarkerSprite(url, opts),
  setWaypoints: (wpts: Waypoint[]) => mapEngine.value?.setWaypoints(wpts),
})
</script>

<style scoped>
#map-container { width: 100%; height: 100%; }
</style>