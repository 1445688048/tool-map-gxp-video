import type { TrackPoint, RouteSegment } from '@/types/gpx'
import { CameraEngine } from '../camera-engine'

export type ProgressCallback = (point: TrackPoint, progress: number, bearing: number) => void
export type CompleteCallback = () => void

export type StepMode = 'off' | 'hill-skip'

export interface StepConfig {
  mode: StepMode
  hillSlopeThreshold: number
  skipSlopeThreshold: number
  hillSpeedMultiplier: number
  skipSpeedMultiplier: number
  stepJumpSize: number
}

const DEFAULT_STEP_CONFIG: StepConfig = {
  mode: 'off',
  hillSlopeThreshold: 5,
  skipSlopeThreshold: 2,
  hillSpeedMultiplier: 3,
  skipSpeedMultiplier: 15,
  stepJumpSize: 200,
}

const BASE_SPEED_MPS = 1.5

export class RoutePlayer {
  private points: TrackPoint[] = []
  private cumulativeDistances: number[] = []
  private totalDistance = 0

  private currentIndex = 0
  private distance = 0
  // 两个轨迹点之间按剩余距离线性插值出的瞬时位置，供镜头/进度点按帧平滑移动
  private interpPoint: TrackPoint | null = null
  private isPlaying = false
  private speed = 1
  private baseSpeed = BASE_SPEED_MPS

  private stepConfig: StepConfig = { ...DEFAULT_STEP_CONFIG }
  // 视角跟随开关：关闭时播放照常推进，但镜头不跟拍
  private cameraFollow = true
  // 急弯自动减速
  private curveSlow = { enabled: true, strength: 0.6 }
  // 前方探山绕行
  private avoid = { enabled: true, lastProbeAt: 0, bearingOffset: 0, dampedOffset: 0 }
  private animationFrame: number | null = null
  private lastTime = 0

  private _onProgress?: ProgressCallback
  private _onComplete?: CompleteCallback

  constructor(private camera: CameraEngine) {}

  setPoints(points: TrackPoint[]) {
    this.points = points
    this.cumulativeDistances = this.calcCumulativeDistances(points)
    this.totalDistance = this.cumulativeDistances[this.cumulativeDistances.length - 1] || 0
    this.currentIndex = 0
    this.distance = 0
    this.interpPoint = null
  }

  get totalDistanceM() { return this.totalDistance }
  get progress() { return this.totalDistance > 0 ? this.distance / this.totalDistance : 0 }
  get currentPoint() { return this.interpPoint ?? this.points[this.currentIndex] ?? null }
  get isPlayingState() { return this.isPlaying }
  getDistance() { return this.distance }
  getStepConfig() { return this.stepConfig }

  setSpeed(s: number) {
    this.speed = s
    this.stepConfig = { ...this.stepConfig }
  }
  setStepMode(mode: StepMode) { this.stepConfig = { ...this.stepConfig, mode } }
  setStepConfig(patch: Partial<StepConfig>) { this.stepConfig = { ...this.stepConfig, ...patch } }
  setCameraFollow(on: boolean) { this.cameraFollow = on }
  setCurveSlow(enabled: boolean, strength: number) { this.curveSlow = { enabled, strength } }
  setAvoidAhead(enabled: boolean) {
    this.avoid.enabled = enabled
    if (!enabled) { this.avoid.bearingOffset = 0; this.avoid.dampedOffset = 0 }
  }
  onProgress(cb: ProgressCallback) { this._onProgress = cb }
  onComplete(cb: CompleteCallback) { this._onComplete = cb }

  play() {
    if (this.points.length === 0) return
    this.isPlaying = true
    this.lastTime = performance.now()
    this.tick()
  }

  pause() {
    this.isPlaying = false
    if (this.animationFrame !== null) {
      cancelAnimationFrame(this.animationFrame)
      this.animationFrame = null
    }
  }

  stop() {
    this.pause()
    this.currentIndex = 0
    this.distance = 0
    this.interpPoint = null
    this.lastTime = 0
  }

  seekTo(d: number) {
    this.distance = Math.max(0, Math.min(d, this.totalDistance))
    this.currentIndex = this.findIndexAtDistance(this.distance)
    this.interpPoint = this.interpolateAt(this.distance)
    this.updateCamera()
  }

  seekToIndex(index: number) {
    this.currentIndex = Math.max(0, Math.min(index, this.points.length - 1))
    this.distance = this.cumulativeDistances[this.currentIndex] || 0
    this.interpPoint = this.points[this.currentIndex] ?? null
    this.updateCamera()
  }

  private tick = () => {
    if (!this.isPlaying) return
    const now = performance.now()
    const delta = (now - this.lastTime) / 1000
    this.lastTime = now

    const point = this.points[this.currentIndex]
    let speedMultiplier = this.speed

    if (this.stepConfig.mode === 'hill-skip' && point) {
      const slope = point.slope
      if (slope > this.stepConfig.hillSlopeThreshold) {
        speedMultiplier = this.speed * 0.8
      } else if (Math.abs(slope) < this.stepConfig.skipSlopeThreshold) {
        speedMultiplier = this.speed * this.stepConfig.skipSpeedMultiplier
      } else {
        speedMultiplier = this.speed * 2
      }
    }
    speedMultiplier *= this.curveSpeedFactor()

    const distanceDelta = this.baseSpeed * speedMultiplier * delta
    this.distance += distanceDelta

    if (this.distance >= this.totalDistance) {
      this.distance = this.totalDistance
      this.currentIndex = this.points.length - 1
      this.interpPoint = this.points[this.currentIndex]
      this.updateCamera()
      this.pause()
      this._onComplete?.()
      return
    }

    this.currentIndex = this.findIndexAtDistance(this.distance)
    this.interpPoint = this.interpolateAt(this.distance)
    this.updateCamera()
    this._onProgress?.(this.currentPoint, this.progress, this.trailBearing())

    this.animationFrame = requestAnimationFrame(this.tick)
  }

  // 前进方向：当前点指向 lookahead 处的方位角，供进度标记旋转
  private trailBearing(): number {
    const i = this.currentIndex
    const ahead = this.points[Math.min(i + 60, this.points.length - 1)]
    const cur = this.points[i]
    if (!cur || !ahead) return 0
    return this.camera.calculateBearing(cur, ahead)
  }

  // 急弯减速：前方 80 点内的转向角越大，前进越慢
  private curveSpeedFactor(): number {
    if (!this.curveSlow.enabled) return 1
    const pts = this.points
    const i = this.currentIndex
    if (i < 4 || i >= pts.length - 80) return 1
    const near = pts[Math.min(i + 4, pts.length - 1)]
    const mid = pts[i]
    const far = pts[Math.min(i + 80, pts.length - 1)]
    const dirNow = this.camera.calculateBearing(mid, near)
    const dirAhead = this.camera.calculateBearing(mid, far)
    let diff = Math.abs(dirAhead - dirNow)
    if (diff > 180) diff = 360 - diff
    // 20° 内不减速，90° 以上减到最低速
    const t = Math.min(1, Math.max(0, (diff - 20) / 70))
    return 1 - this.curveSlow.strength * t
  }

  // 在 currentIndex 与下一点之间按剩余距离比例插值，得到帧间连续位置
  private interpolateAt(distance: number): TrackPoint | null {
    const pts = this.points
    if (pts.length === 0) return null
    const idx = this.currentIndex
    const p0 = pts[idx]
    if (!p0 || idx >= pts.length - 1) return p0 ?? null
    const d0 = this.cumulativeDistances[idx]
    const d1 = this.cumulativeDistances[idx + 1]
    const seg = d1 - d0
    const frac = seg > 0 ? Math.min(1, Math.max(0, (distance - d0) / seg)) : 0
    const p1 = pts[idx + 1]
    return {
      ...p0,
      latitude: p0.latitude + (p1.latitude - p0.latitude) * frac,
      longitude: p0.longitude + (p1.longitude - p0.longitude) * frac,
      elevation: p0.elevation + (p1.elevation - p0.elevation) * frac,
      distance,
    }
  }

  private updateCamera() {
    if (!this.cameraFollow) return
    if (this.points.length < 2) return
    // 镜头中心使用插值位置（避免每帧跳格）
    const target = this.interpPoint ?? this.points[this.currentIndex]
    const state = this.camera.getState(this.points, this.currentIndex, target)
    this.probeAhead(target, state.bearing)
    // 避障转向阻尼：平滑转到探查出的方向
    this.avoid.dampedOffset += (this.avoid.bearingOffset - this.avoid.dampedOffset) * 0.06
    state.bearing = (state.bearing + this.avoid.dampedOffset + 360) % 360
    // 视线避障：沿"目标点→相机"方向取轨迹已过段的高程做地形估计
    this.camera.jump(state, this.computeMinAltitude(target, state.bearing))
  }

  // 前方探山：正前被高山挡住时，探查左右两侧哪边遮挡最小，镜头转向那一侧"绕山看"
  private probeAhead(target: TrackPoint, baseBearing: number) {
    const now = performance.now()
    if (now - this.avoid.lastProbeAt < 300) return
    this.avoid.lastProbeAt = now
    if (!this.avoid.enabled) { this.avoid.bearingOffset *= 0.8; return }
    const h = this.camera.getEffectiveAltitude()
    const offsets = [0, -40, 40, -80, 80]
    const dists = [300, 700, 1200]
    const mPerDegLat = 110540
    const mPerDegLng = 111320 * Math.cos(target.latitude * Math.PI / 180)
    // 返回该方向上最高地形相对目标点的高差；无 DEM 数据时返回 null
    const blockFor = (offsetDeg: number): number | null => {
      let block = 0
      let any = false
      for (const d of dists) {
        const br = (baseBearing + offsetDeg) * Math.PI / 180
        const lng = target.longitude + Math.sin(br) * d / mPerDegLng
        const lat = target.latitude + Math.cos(br) * d / mPerDegLat
        const e = this.camera.queryElevation([lng, lat])
        if (e == null) continue
        any = true
        block = Math.max(block, e - target.elevation)
      }
      return any ? block : null
    }
    const front = blockFor(0)
    if (front == null) { this.avoid.bearingOffset *= 0.8; return }
    const threshold = Math.max(200, h * 0.5)
    if (front < threshold) { this.avoid.bearingOffset *= 0.8; return } // 前方不堵，缓慢回正
    let bestOffset = 0
    let bestBlock = front
    for (const off of [-40, 40, -80, 80]) {
      const b = blockFor(off)
      if (b != null && b < bestBlock) { bestBlock = b; bestOffset = off }
    }
    // 一侧遮挡明显更小才转向（至少低 150m）
    if (bestOffset !== 0 && bestBlock <= front - 150) {
      this.avoid.bearingOffset += (bestOffset - this.avoid.bearingOffset) * 0.5
    }
  }

  // 身后轨迹的高程就是刚经过的地形：若高于视线，则要求镜头抬高。
  // 性能关键：150ms 限流；逐点纯数学评估 + 2 次 DEM 修正查询
  private lastClearanceAt = 0
  private lastClearance = 0

  private computeMinAltitude(target: TrackPoint, bearing: number): number {
    const now = performance.now()
    if (now - this.lastClearanceAt < 150) return this.lastClearance
    this.lastClearanceAt = now

    const h = this.camera.getEffectiveAltitude()
    const horiz = h * Math.tan(this.camera.getPitch() * Math.PI / 180)
    if (horiz <= 0) { this.lastClearance = h; return h }
    const brRad = bearing * Math.PI / 180
    const metersPerDegLat = 110540
    const metersPerDegLng = 111320 * Math.cos(target.latitude * Math.PI / 180)
    let need = h
    let acc = 0
    let idx = this.currentIndex
    const cum = this.cumulativeDistances
    let nextDemT = 0.5
    while (idx > 0 && acc < horiz) {
      acc += cum[idx] - cum[idx - 1]
      idx--
      const t = acc / horiz
      if (t < 0.08) continue
      // 轨迹高程近似地形；每 35% 视距补一次 DEM 修正（捕捉偏离路线的山脊）
      let terrainE = this.points[idx].elevation
      if (t >= nextDemT) {
        nextDemT += 0.35
        const lng = target.longitude - Math.sin(brRad) * horiz * t / metersPerDegLng
        const lat = target.latitude - Math.cos(brRad) * horiz * t / metersPerDegLat
        const demE = this.camera.queryElevation([lng, lat])
        if (demE != null && demE > terrainE) terrainE = demE
      }
      const lineH = target.elevation + h * t
      if (terrainE > lineH) {
        need = Math.max(need, (terrainE - target.elevation) / t + 80)
      }
    }
    // 前方 500m 内轨迹爬升显著时提前抬镜（预判即将到来的山体遮挡）
    let upMax = target.elevation
    let ahead = 0
    let j = this.currentIndex
    while (j < this.points.length - 1 && ahead < 500) {
      ahead += cum[j + 1] - cum[j]
      j++
      upMax = Math.max(upMax, this.points[j].elevation)
    }
    const climb = upMax - target.elevation
    if (climb > 200) need = Math.max(need, climb + 150)
    this.lastClearance = need
    return need
  }

  private calcCumulativeDistances(points: TrackPoint[]): number[] {
    const dists: number[] = [0]
    for (let i = 1; i < points.length; i++) {
      const d = haversine(points[i-1].latitude, points[i-1].longitude, points[i].latitude, points[i].longitude)
      dists.push(dists[i-1] + d)
    }
    return dists
  }

  private findIndexAtDistance(target: number): number {
    const dists = this.cumulativeDistances
    let lo = 0, hi = dists.length - 1
    while (lo < hi - 1) {
      const mid = (lo + hi) >> 1
      if (dists[mid] <= target) lo = mid
      else hi = mid
    }
    return dists[hi] <= target ? hi : lo
  }
}

function haversine(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const R = 6371000
  const dLat = (lat2 - lat1) * Math.PI / 180
  const dLon = (lon2 - lon1) * Math.PI / 180
  const a = Math.sin(dLat/2)**2 + Math.cos(lat1*Math.PI/180)*Math.cos(lat2*Math.PI/180)*Math.sin(dLon/2)**2
  return R * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1-a))
}
