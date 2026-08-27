import maplibregl from 'maplibre-gl'
import type { TrackPoint } from '@/types/gpx'

export interface CameraState {
  center: [number, number]
  zoom: number
  pitch: number
  bearing: number
}

export interface CameraSettings {
  altitude: number
  pitch: number
  lookAheadPoints: number
  minTerrainClearance: number
}

const DEFAULT_SETTINGS: CameraSettings = {
  altitude: 100,
  pitch: 60,
  lookAheadPoints: 20,
  minTerrainClearance: 50,
}

export class CameraEngine {
  private map: maplibregl.Map | null = null
  private settings: CameraSettings = { ...DEFAULT_SETTINGS }
  private lastBearing: number = 0

  setMap(map: maplibregl.Map) { this.map = map }
  setSettings(s: Partial<CameraSettings>) { this.settings = { ...this.settings, ...s } }

  calculateBearing(from: TrackPoint, to: TrackPoint): number {
    const lat1 = from.latitude * Math.PI / 180
    const lat2 = to.latitude * Math.PI / 180
    const dLon = (to.longitude - from.longitude) * Math.PI / 180
    const y = Math.sin(dLon) * Math.cos(lat2)
    const x = Math.cos(lat1) * Math.sin(lat2) -
              Math.sin(lat1) * Math.cos(lat2) * Math.cos(dLon)
    return ((Math.atan2(y, x) * 180 / Math.PI) + 360) % 360
  }

  smoothBearing(current: number, target: number): number {
    let diff = target - current
    if (diff > 180) diff -= 360
    if (diff < -180) diff += 360
    if (Math.abs(diff) < 4) return current
    const change = Math.max(-0.85, Math.min(0.85, diff * 0.06))
    return (current + change + 360) % 360
  }

  getState(points: TrackPoint[], index: number): CameraState {
    if (index < 0 || index >= points.length) {
      const p = points[Math.max(0, Math.min(index, points.length - 1))]
      return { center: [p.longitude, p.latitude], zoom: 14, pitch: this.settings.pitch, bearing: 0 }
    }
    const current = points[index]
    const lookAhead = points[Math.min(index + this.settings.lookAheadPoints, points.length - 1)]
    const rawBearing = this.calculateBearing(current, lookAhead)
    const smoothedBearing = this.smoothBearing(this.lastBearing, rawBearing)
    this.lastBearing = smoothedBearing
    return {
      center: [current.longitude, current.latitude],
      zoom: this.altitudeToZoom(this.settings.altitude),
      pitch: this.settings.pitch,
      bearing: smoothedBearing,
    }
  }

  apply(state: CameraState, duration = 0) {
    if (!this.map) return
    this.map.easeTo({ center: state.center, zoom: state.zoom, pitch: state.pitch, bearing: state.bearing, duration, essential: true })
  }

  jump(state: CameraState) {
    if (!this.map) return
    this.map.jumpTo({ center: state.center, zoom: state.zoom, pitch: state.pitch, bearing: state.bearing })
  }

  fitToRoute(points: TrackPoint[]) {
    if (!this.map || points.length < 2) return
    const bounds = new maplibregl.LngLatBounds()
    for (const p of points) bounds.extend([p.longitude, p.latitude])
    this.map.fitBounds(bounds, { padding: 80, duration: 0 })
  }

  private altitudeToZoom(alt: number): number {
    const minZoom = 8, maxZoom = 18
    const normalized = Math.max(0, Math.min(1, (2000 - alt) / 1980))
    return minZoom + normalized * (maxZoom - minZoom)
  }
}
