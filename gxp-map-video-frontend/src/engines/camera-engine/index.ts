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

// 视角跟随手感参数
export interface CameraTuning {
  // 航向死区（度）：偏差小于该值时镜头完全不转
  deadzone: number
  // 单帧最大转向角（度）：决定转弯的快慢
  maxRate: number
}

const DEFAULT_SETTINGS: CameraSettings = {
  altitude: 100,
  pitch: 60,
  // 前瞻点数：取得越长，基准航向越稳定，镜头不会跟着小弯扭动
  lookAheadPoints: 120,
  // 视线避障：镜头至少高出沿视线最高地形多少米
  minTerrainClearance: 80,
}

export class CameraEngine {
  private map: maplibregl.Map | null = null
  private settings: CameraSettings = { ...DEFAULT_SETTINGS }
  private lastBearing: number = 0
  private tuning: CameraTuning = { deadzone: 30, maxRate: 0.3 }
  // 避障后的实际镜头高度（阻尼平滑，升快降慢避免 zoom 抽搐）
  private effAltitude: number | null = null
  private effPitch: number | null = null

  setMap(map: maplibregl.Map) { this.map = map }
  setSettings(s: Partial<CameraSettings>) {
    this.settings = { ...this.settings, ...s }
    if (s.altitude !== undefined) this.effAltitude = s.altitude // 用户手动调整时立即生效
    if (s.pitch !== undefined) this.effPitch = s.pitch
  }
  setTuning(t: Partial<CameraTuning>) { this.tuning = { ...this.tuning, ...t } }
  getTuning(): CameraTuning { return { ...this.tuning } }
  getAltitude(): number { return this.settings.altitude }
  getPitch(): number { return this.settings.pitch }
  getEffectiveAltitude(): number { return this.effAltitude ?? this.settings.altitude }
  // DEM 高程查询（瓦片未加载时返回 null）
  queryElevation(lngLat: [number, number]): number | null {
    try {
      return this.map?.queryTerrainElevation?.(lngLat) ?? null
    } catch {
      return null
    }
  }

  calculateBearing(from: TrackPoint, to: TrackPoint): number {
    const lat1 = from.latitude * Math.PI / 180
    const lat2 = to.latitude * Math.PI / 180
    const dLon = (to.longitude - from.longitude) * Math.PI / 180
    const y = Math.sin(dLon) * Math.cos(lat2)
    const x = Math.cos(lat1) * Math.sin(lat2) -
              Math.sin(lat1) * Math.cos(lat2) * Math.cos(dLon)
    return ((Math.atan2(y, x) * 180 / Math.PI) + 360) % 360
  }

  // 方向去敏感：死区内完全保持航向，超出时才以限速平缓转向
  smoothBearing(current: number, target: number): number {
    let diff = target - current
    if (diff > 180) diff -= 360
    if (diff < -180) diff += 360
    if (Math.abs(diff) < this.tuning.deadzone) return current
    const change = Math.max(-this.tuning.maxRate, Math.min(this.tuning.maxRate, diff * 0.05))
    return (current + change + 360) % 360
  }

  // centerOverride：帧间插值出的精确位置，镜头中心用它比跳格到轨迹点更平滑
  getState(points: TrackPoint[], index: number, centerOverride?: TrackPoint): CameraState {
    if (index < 0 || index >= points.length) {
      const p = points[Math.max(0, Math.min(index, points.length - 1))]
      return { center: [p.longitude, p.latitude], zoom: 14, pitch: this.settings.pitch, bearing: 0 }
    }
    const current = centerOverride ?? points[index]
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

  apply(state: CameraState, duration = 300) {
    if (!this.map) return
    this.map.easeTo({
      center: state.center,
      zoom: state.zoom,
      pitch: state.pitch,
      bearing: state.bearing,
      duration,
      essential: true,
    })
  }

  // minAltitude：视线避障算出的最低镜头高度，经阻尼平滑后生效（升快降慢）。
  // 抬升的同时按比例压低俯角（最低 30°），视线更垂直、翻越山脊时目标点不容易被挡
  jump(state: CameraState, minAltitude?: number) {
    if (!this.map) return
    let zoom = state.zoom
    let pitch = state.pitch
    if (minAltitude != null) {
      if (this.effAltitude == null) this.effAltitude = this.settings.altitude
      this.effAltitude += (minAltitude - this.effAltitude) * (minAltitude > this.effAltitude ? 0.25 : 0.03)
      zoom = this.altitudeToZoom(this.effAltitude)
      const ratio = this.effAltitude / Math.max(1, this.settings.altitude)
      const wantPitch = Math.max(30, this.settings.pitch / Math.max(1, ratio * 0.7))
      if (this.effPitch == null) this.effPitch = this.settings.pitch
      this.effPitch += (wantPitch - this.effPitch) * 0.15
      pitch = this.effPitch
    } else {
      this.effPitch = this.settings.pitch
    }
    this.map.jumpTo({ center: state.center, zoom, pitch, bearing: state.bearing })
  }

  // 开场/结尾动画：围绕指定点的升降镜头
  jumpToOverview(point: TrackPoint, altitude: number, pitch: number, bearing: number) {
    if (!this.map) return
    this.map.jumpTo({
      center: [point.longitude, point.latitude],
      zoom: this.altitudeToZoom(altitude),
      pitch,
      bearing,
    })
  }

  fitToRoute(points: TrackPoint[]) {
    if (!this.map || points.length < 2) return
    const bounds = new maplibregl.LngLatBounds()
    for (const p of points) bounds.extend([p.longitude, p.latitude])
    this.map.fitBounds(bounds, { padding: 80, duration: 0 })
  }

  // 视角高度 → zoom：20m ~ 10000m 映射到 zoom 18 ~ 9
  private altitudeToZoom(alt: number): number {
    const minZoom = 9, maxZoom = 18
    const minAlt = 20, maxAlt = 10000
    const normalized = Math.max(0, Math.min(1, (maxAlt - alt) / (maxAlt - minAlt)))
    return minZoom + normalized * (maxZoom - minZoom)
  }
}
