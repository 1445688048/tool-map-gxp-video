import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Route, TrackPoint } from '@/types/gpx'
import { api } from '@/api/client'

export const useRouteStore = defineStore('route', () => {
  const currentRoute = ref<Route | null>(null)
  const trackPoints = ref<TrackPoint[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function uploadGPX(file: File): Promise<Route> {
    loading.value = true
    error.value = null
    try {
      const result = await api.uploadGPX(file) as Route
      currentRoute.value = result
      await fetchPoints(result.id)
      return result
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Upload failed'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function fetchPoints(routeId: number): Promise<TrackPoint[]> {
    trackPoints.value = await api.getPoints(routeId)
    return trackPoints.value
  }

  function clearRoute() {
    currentRoute.value = null
    trackPoints.value = []
  }

  return { currentRoute, trackPoints, loading, error, uploadGPX, fetchPoints, clearRoute }
})
