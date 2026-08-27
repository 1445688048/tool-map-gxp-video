import type { TrackPoint, RouteSegment } from '@/types/gpx'
import { CameraEngine } from '../camera-engine'

export type ProgressCallback = (point: TrackPoint, progress: number) => void
export type CompleteCallback = () => void

export type StepMode = 'off' | 'hill-skip' // hill-skip: skip flat/slight slope, accelerate on climbs/descents

export interface StepConfig {
  mode: StepMode
  hillSlopeThreshold: number  // slope % above which we accelerate
  skipSlopeThreshold: number  // slope % below which we skip (flat)
  hillSpeedMultiplier: number // speed multiplier on hills
  skipSpeedMultiplier: number // speed multiplier on flat (high = fast-forward)
  stepJumpSize: number        // meters to jump when skipping
}

const DEFAULT_STEP_CONFIG: StepConfig = {
  mode: 'off',
  hillSlopeThreshold: 5,
  skipSlopeThreshold: 2,
  hillSpeedMultiplier: 3,
  skipSpeedMultiplier: 15,
  stepJumpSize: 200, // 200m jump on flat sections
}

const BASE_SPEED_MPS = 1.5 // ~5.4 km/h walking speed

export class RoutePlayer {
  private points: TrackPoint[] = []
  private cumulativeDistances: number[] = []
  private totalDistance = 0

  private currentIndex = 0
  private distance = 0
  private isPlaying = false
  private speed = 1
  private baseSpeed = BASE_SPEED_MPS

  private stepConfig: StepConfig = { ...DEFAULT_STEP_CONFIG }
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
  }

  get totalDistanceM() { return this.totalDistance }
  get progress() { return this.totalDistance > 0 ? this.distance / this.totalDistance : 0 }
  get currentPoint() { return this.points[this.currentIndex] || null }
  get isPlayingState() { return this.isPlaying }
  getDistance() { return this.distance }
  getStepConfig() { return this.stepConfig }

  setSpeed(s: number) { this.speed = s; this.stepConfig = { ...this.stepConfig } }
  setStepMode(mode: StepMode) { this.stepConfig = { ...this.stepConfig, mode } }
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
    this.lastTime = 0
  }

  seekTo(d: number) {
    this.distance = Math.max(0, Math.min(d, this.totalDistance))
    this.currentIndex = this.findIndexAtDistance(this.distance)
    this.updateCamera()
  }

  seekToIndex(index: number) {
    this.currentIndex = Math.max(0, Math.min(index, this.points.length - 1))
    this.distance = this.cumulativeDistances[this.currentIndex] || 0
    this.updateCamera()
  }

  private tick = () => {
    if (!this.isPlaying) return
    const now = performance.now()
    const delta = (now - this.lastTime) / 1000
    this.lastTime = now

    const point = this.points[this.currentIndex]
    let speedMultiplier = this.speed

    // Step-jump logic: apply different speed based on slope
    if (this.stepConfig.mode === 'hill-skip' && point) {
      const slope = point.slope
      if (slope > this.stepConfig.hillSlopeThreshold) {
        // Climbing: slow down slightly for drama
        speedMultiplier = this.speed * 0.8
      } else if (Math.abs(slope) < this.stepConfig.skipSlopeThreshold) {
        // Flat: fast-forward (skip)
        speedMultiplier = this.speed * this.stepConfig.skipSpeedMultiplier
      } else {
        // Moderate slope: normal-ish
        speedMultiplier = this.speed * 2
      }
    }

    const distanceDelta = this.baseSpeed * speedMultiplier * delta
    this.distance += distanceDelta

    if (this.distance >= this.totalDistance) {
      this.distance = this.totalDistance
      this.currentIndex = this.points.length - 1
      this.updateCamera()
      this.pause()
      this._onComplete?.()
      return
    }

    this.currentIndex = this.findIndexAtDistance(this.distance)
    this.updateCamera()
    this._onProgress?.(this.points[this.currentIndex], this.progress)

    this.animationFrame = requestAnimationFrame(this.tick)
  }

  private updateCamera() {
    if (this.points.length < 2) return
    const state = this.camera.getState(this.points, this.currentIndex)
    this.camera.jump(state)
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
