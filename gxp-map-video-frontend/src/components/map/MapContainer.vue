<template>
  <div id="map-container"></div>
</template>

<script setup lang="ts">
import { onMounted, onBeforeUnmount } from 'vue'
import { useRouteStore } from '@/stores/route'
import { usePlaybackStore } from '@/stores/playback'
import { MapEngine } from '@/engines/map-engine'
import { CameraEngine } from '@/engines/camera-engine'
import { RoutePlayer } from '@/engines/route-player'

const routeStore = useRouteStore()
const playbackStore = usePlaybackStore()

let mapEngine: MapEngine | null = null
let cameraEngine: CameraEngine | null = null
let routePlayer: RoutePlayer | null = null

onMounted(async () => {
  mapEngine = new MapEngine()
  cameraEngine = new CameraEngine()
  routePlayer = new RoutePlayer(cameraEngine)

  await mapEngine.init('map-container', [116.4, 40.0], 12)
  cameraEngine.setMap(mapEngine.map!)

  // Watch for route points
  const unwatchPoints = () => {
    if (routeStore.trackPoints.length > 0 && mapEngine) {
      mapEngine.loadRoute(routeStore.trackPoints)
      if (routePlayer) routePlayer.setPoints(routeStore.trackPoints)
    }
  }
  unwatchPoints()

  // Update progress marker
  const poll = setInterval(() => {
    if (playbackStore.isPlaying && routePlayer?.currentPoint && mapEngine) {
      mapEngine.setProgressPoint(routePlayer.currentPoint)
    }
  }, 50)

  onBeforeUnmount(() => {
    clearInterval(poll)
    mapEngine?.destroy()
  })
})
</script>

<style scoped>
#map-container { width: 100%; height: 100%; }
</style>
