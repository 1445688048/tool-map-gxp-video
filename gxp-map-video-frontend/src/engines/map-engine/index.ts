import maplibregl, { LngLatBounds } from 'maplibre-gl'
import type { TrackPoint, Waypoint } from '@/types/gpx'
import '@/engines/map-engine/style.css'

function pointFeature(lng: number, lat: number, props: Record<string, unknown> = {}) {
  return { type: 'Feature' as const, geometry: { type: 'Point' as const, coordinates: [lng, lat] }, properties: props }
}

function emptyFeatureCollection() {
  return { type: 'FeatureCollection' as const, features: [] }
}

export class MapEngine {
  map: maplibregl.Map | null = null
  private trackSourceId = 'gpx-track-source'
  private trackLayerId = 'gpx-track'          // 淡色预览层：整条路线
  private revealSourceId = 'gpx-reveal-source' // 揭示层：已走的亮色线
  private revealLayerId = 'gpx-track-progress'
  private allCoords: [number, number, number][] = []
  // 进度标记：贴地 symbol 图层（DOM 标记不感知 3D 地形）
  private markerSourceId = 'progress-marker-source'
  private markerLayerId = 'progress-marker'
  private markerIconDefaultId = 'gxp-default-arrow'
  private markerIconCustomId = 'gxp-progress-icon'
  private markerImage: string | null = null
  private markerIconReady = false
  // 精灵帧动画（默认跑步小人）
  private spriteFrames: string[] = []
  private spriteFrame = 0
  private spriteFps = 6
  private frameTimer: ReturnType<typeof setInterval> | null = null
  private lastMarkerPoint: TrackPoint | null = null
  private lastMarkerBearing = 0
  private lastIconMode: 'sprite' | 'custom' | 'arrow' | null = null
  // 航点 / 起终点
  private wptSourceId = 'waypoints-source'
  private wptLayerId = 'waypoints-layer'
  private endpointsSourceId = 'endpoints-source'
  private endpointsLayerId = 'endpoints-layer'
  // 样式就绪标记与等待队列：load 事件只触发一次，
  // 不能依赖 isStyleLoaded() + once('load')（存在永远等不到的竞态）
  private styleReady = false
  private styleReadyQueue: (() => void)[] = []

  async init(containerId: string, center: [number, number], zoom: number) {
    this.map = new maplibregl.Map({
      container: containerId,
      style: this.getStyleSheet(),
      center,
      zoom,
      pitch: 60,
      bearing: 0,
      maxPitch: 85,
    })
    this.map.addControl(new maplibregl.NavigationControl(), 'top-right')
    this.map.on('load', () => {
      this.styleReady = true
      const queue = this.styleReadyQueue
      this.styleReadyQueue = []
      queue.forEach(fn => fn())
    })
    this.map.on('click', this.wptLayerId, (e) => {
      const f = e.features?.[0]
      const name = f?.properties?.name as string | undefined
      if (!f || !this.map) return
      const coords = (f.geometry as unknown as { coordinates: [number, number] }).coordinates
      new maplibregl.Popup({ offset: 12 })
        .setLngLat(coords)
        .setHTML(`<strong>${name ?? '航点'}</strong>`)
        .addTo(this.map)
    })
    return new Promise<void>((resolve) => {
      let done = false
      const finish = () => { if (!done) { done = true; resolve() } }
      this.map!.on('load', finish)
      setTimeout(finish, 5000)
    })
  }

  getStyleSheet() {
    return {
      version: 8 as const,
      sources: {
        'base-map': {
          type: 'raster' as const,
          tiles: ['/api/tiles/esri-satellite/{z}/{x}/{y}.png'],
          tileSize: 256,
          // 限制最大瓦片级别：更高 zoom 时复用低级瓦片放大（overzoom），
          // 避免高速飞行时持续请求 z17/18 新瓦片造成卡顿
          maxzoom: 16,
          attribution: '\u00A9 Esri',
        },
        'terrain-source': {
          type: 'raster-dem' as const,
          tiles: ['/api/tiles/terrain/{z}/{x}/{y}.png'],
          encoding: 'terrarium' as const,
          tileSize: 256,
          maxzoom: 15,
        },
      },
      layers: [{ id: 'base-layer', type: 'raster' as const, source: 'base-map' }],
      terrain: { source: 'terrain-source', exaggeration: 1.5 },
    }
  }

  // 样式未就绪时 addSource/addLayer 会抛异常，统一延后到 load 完成再执行
  private runWhenStyleReady(fn: () => void) {
    if (!this.map) return
    if (this.styleReady || this.map.isStyleLoaded()) {
      fn()
    } else {
      this.styleReadyQueue.push(fn)
    }
  }

  loadRoute(points: TrackPoint[]) {
    if (!this.map || points.length < 2) return
    // 记录完整坐标用于进度揭示切片
    this.allCoords = points.map(p => [p.longitude, p.latitude, p.elevation] as [number, number, number])
    this.runWhenStyleReady(() => this.addRouteLayers(points))
  }

  // 进度揭示：亮线覆盖 0~frac 的已走部分（切片方案，确定渲染不依赖 line-gradient）
  updateRouteProgress(frac: number) {
    if (!this.map || !this.allCoords.length) return
    if (!this.map.getSource(this.revealSourceId)) return
    const f = Math.max(0, Math.min(1, frac))
    const total = this.allCoords.length
    const endIdx = Math.max(1, Math.floor(f * (total - 1)))
    // 抽稀到 ≤1200 个坐标，视觉无损
    const stride = Math.max(1, Math.ceil(endIdx / 1200))
    const coords: [number, number, number][] = []
    for (let i = 0; i <= endIdx; i += stride) coords.push(this.allCoords[i])
    const tail = this.allCoords[endIdx]
    if (coords[coords.length - 1] !== tail) coords.push(tail)
    const src = this.map.getSource(this.revealSourceId) as maplibregl.GeoJSONSource
    src.setData({
      type: 'Feature' as const,
      geometry: { type: 'LineString' as const, coordinates: coords },
      properties: {},
    })
  }

  private addRouteLayers(points: TrackPoint[]) {
    if (!this.map) return
    try {
      if (this.map.getLayer(this.trackLayerId)) this.map.removeLayer(this.trackLayerId)
      if (this.map.getLayer(this.revealLayerId)) this.map.removeLayer(this.revealLayerId)
      if (this.map.getSource(this.trackSourceId)) this.map.removeSource(this.trackSourceId)
      if (this.map.getSource(this.revealSourceId)) this.map.removeSource(this.revealSourceId)

      const coords: [number, number, number][] = points.map(p => [p.longitude, p.latitude, p.elevation])
      this.map.addSource(this.trackSourceId, {
        type: 'geojson' as const,
        data: { type: 'Feature' as const, geometry: { type: 'LineString' as const, coordinates: coords }, properties: {} },
      })
      // 预览层：整条路线淡色，让用户看到"将要走的路"
      this.map.addLayer({
        id: this.trackLayerId, type: 'line' as const, source: this.trackSourceId,
        layout: { 'line-join': 'round' as const, 'line-cap': 'round' as const },
        paint: { 'line-color': 'rgba(255, 255, 255, 0.35)', 'line-width': 4 },
      })
      // 进度揭示层：亮红色，数据随进度逐帧更新
      this.map.addSource(this.revealSourceId, { type: 'geojson' as const, data: emptyFeatureCollection() })
      this.map.addLayer({
        id: this.revealLayerId, type: 'line' as const, source: this.revealSourceId,
        layout: { 'line-join': 'round' as const, 'line-cap': 'round' as const },
        paint: { 'line-color': '#ff5a5f', 'line-width': 6 },
      })

      this.addEndpointsFlags(points)

      const bounds = new LngLatBounds()
      for (const p of points) bounds.extend([p.longitude, p.latitude])
      this.map.fitBounds(bounds, { padding: 80, duration: 0 })

      // 轨迹线重建后会盖住标记层，把标记挪回顶层
      if (this.map.getLayer(this.markerLayerId)) this.map.moveLayer(this.markerLayerId)
    } catch (e) {
      console.error('[MapEngine] addRouteLayers 失败:', e)
    }
  }

  // 起终点旗标
  private addEndpointsFlags(points: TrackPoint[]) {
    if (!this.map) return
    if (!this.map.hasImage('flag-start')) this.map.addImage('flag-start', this.buildFlagIcon('#4ecca3', '起'))
    if (!this.map.hasImage('flag-end')) this.map.addImage('flag-end', this.buildFlagIcon('#e94560', '终'))
    if (!this.map.getSource(this.endpointsSourceId)) {
      this.map.addSource(this.endpointsSourceId, { type: 'geojson' as const, data: emptyFeatureCollection() })
      this.map.addLayer({
        id: this.endpointsLayerId, type: 'symbol' as const, source: this.endpointsSourceId,
        layout: {
          'icon-image': ['get', 'icon'],
          'icon-anchor': 'bottom' as const,
          'icon-allow-overlap': true,
          'icon-ignore-placement': true,
          'icon-rotation-alignment': 'viewport' as const,
          'icon-pitch-alignment': 'viewport' as const,
        },
      } as any)
    }
    const a = points[0]
    const b = points[points.length - 1]
    const src = this.map.getSource(this.endpointsSourceId) as maplibregl.GeoJSONSource
    src.setData({
      type: 'FeatureCollection' as const,
      features: [
        pointFeature(a.longitude, a.latitude, { icon: 'flag-start' }),
        pointFeature(b.longitude, b.latitude, { icon: 'flag-end' }),
      ],
    })
  }

  private buildFlagIcon(color: string, label: string) {
    const c = document.createElement('canvas')
    c.width = 48
    c.height = 64
    const ctx = c.getContext('2d')!
    // 旗杆
    ctx.strokeStyle = '#ffffff'
    ctx.lineWidth = 4
    ctx.lineCap = 'round'
    ctx.beginPath()
    ctx.moveTo(24, 6)
    ctx.lineTo(24, 46)
    ctx.stroke()
    // 旗面
    ctx.fillStyle = color
    ctx.beginPath()
    ctx.moveTo(26, 6)
    ctx.lineTo(46, 14)
    ctx.lineTo(26, 22)
    ctx.closePath()
    ctx.fill()
    // 文字
    ctx.font = 'bold 15px sans-serif'
    ctx.textAlign = 'center'
    ctx.lineWidth = 3
    ctx.strokeStyle = 'rgba(0,0,0,0.75)'
    ctx.strokeText(label, 24, 62)
    ctx.fillStyle = '#ffffff'
    ctx.fillText(label, 24, 62)
    return ctx.getImageData(0, 0, 48, 64)
  }

  // 默认进度图标：指向画布上方的箭头（bearing 0 = 北，随方向旋转）
  private buildArrowIcon() {
    const c = document.createElement('canvas')
    c.width = 44
    c.height = 44
    const ctx = c.getContext('2d')!
    ctx.translate(22, 22)
    ctx.fillStyle = '#00ff88'
    ctx.strokeStyle = '#ffffff'
    ctx.lineWidth = 3
    ctx.beginPath()
    ctx.moveTo(0, -17)
    ctx.lineTo(12, 12)
    ctx.lineTo(0, 5)
    ctx.lineTo(-12, 12)
    ctx.closePath()
    ctx.fill()
    ctx.stroke()
    return ctx.getImageData(0, 0, 44, 44)
  }

  setProgressPoint(point: TrackPoint, bearing = 0) {
    this.lastMarkerPoint = point
    this.lastMarkerBearing = bearing
    if (!this.map) return
    this.runWhenStyleReady(() => this.emitMarker())
  }

  // 当前应使用的标记模式：自定义图片 > 精灵动画 > 默认箭头
  private markerIconMode(): 'sprite' | 'custom' | 'arrow' {
    if (this.markerImage && this.markerIconReady) return 'custom'
    if (this.spriteFrames.length > 0) return 'sprite'
    return 'arrow'
  }

  private iconImageExpr(mode: 'sprite' | 'custom' | 'arrow'): any {
    if (mode === 'custom') return this.markerIconCustomId
    if (mode === 'sprite') return ['concat', 'gxp-run-', ['to-string', ['get', 'frame']]]
    return this.markerIconDefaultId
  }

  private emitMarker() {
    if (!this.map) return
    const point = this.lastMarkerPoint
    if (!point) return
    if (!this.map.hasImage(this.markerIconDefaultId)) this.map.addImage(this.markerIconDefaultId, this.buildArrowIcon())
    if (this.markerImage && !this.markerIconReady) return // 自定义图尚未加载完
    const mode = this.markerIconMode()
    if (mode === 'sprite') this.startFrameTimer()

    if (!this.map.getSource(this.markerSourceId)) {
      this.map.addSource(this.markerSourceId, { type: 'geojson' as const, data: emptyFeatureCollection() })
      this.map.addLayer({
        id: this.markerLayerId, type: 'symbol' as const, source: this.markerSourceId,
        layout: {
          'icon-image': this.iconImageExpr(mode),
          'icon-rotate': mode === 'sprite' ? 0 : (['get', 'bearing'] as any),
          'icon-rotation-alignment': mode === 'sprite' ? 'viewport' : 'map',
          'icon-pitch-alignment': mode === 'sprite' ? 'viewport' : 'map',
          // 精灵模式：图标底部锚定在轨迹点上（脚踩的位置）
          'icon-anchor': mode === 'sprite' ? 'bottom' : 'center',
          'icon-allow-overlap': true,
          'icon-ignore-placement': true,
        },
      } as any)
      this.lastIconMode = mode
    } else if (this.lastIconMode !== mode) {
      // 模式切换：更新图标表达式与朝向方式
      this.map.setLayoutProperty(this.markerLayerId, 'icon-image', this.iconImageExpr(mode))
      this.map.setLayoutProperty(this.markerLayerId, 'icon-rotate', mode === 'sprite' ? 0 : (['get', 'bearing'] as any))
      this.map.setLayoutProperty(this.markerLayerId, 'icon-rotation-alignment', mode === 'sprite' ? 'viewport' : 'map')
      this.map.setLayoutProperty(this.markerLayerId, 'icon-pitch-alignment', mode === 'sprite' ? 'viewport' : 'map')
      this.map.setLayoutProperty(this.markerLayerId, 'icon-anchor', mode === 'sprite' ? 'bottom' : 'center')
      this.lastIconMode = mode
    }
    const src = this.map.getSource(this.markerSourceId) as maplibregl.GeoJSONSource
    src.setData(pointFeature(point.longitude, point.latitude, { bearing: this.lastMarkerBearing, frame: this.spriteFrame % Math.max(1, this.spriteFrames.length || 1) }))
  }

  removeProgressMarker() {
    if (!this.map) return
    const src = this.map.getSource(this.markerSourceId) as maplibregl.GeoJSONSource | undefined
    src?.setData(emptyFeatureCollection())
  }

  // 精灵帧动画：把精灵图按网格切片注册为多帧图标，定时换帧
  setProgressMarkerSprite(url: string, opts?: { row?: number; rows?: number; cols?: number; fps?: number; size?: number }) {
    const row = opts?.row ?? 0
    const rows = opts?.rows ?? 1
    const cols = opts?.cols ?? 3
    const fps = opts?.fps ?? 7
    const size = opts?.size ?? 64
    this.runWhenStyleReady(() => {
      // 加时间戳绕过浏览器缓存，保证改图后立即生效
      this.loadSpriteFrames(`${url}${url.includes('?') ? '&' : '?'}t=${Date.now()}`, row, rows, cols, fps, size)
        .then(ids => {
          this.spriteFrames = ids
          this.emitMarker()
        })
        .catch(err => console.error('加载跑步动画失败:', err))
    })
  }

  private startFrameTimer() {
    if (this.frameTimer != null) return
    this.frameTimer = setInterval(() => {
      if (this.spriteFrames.length === 0) return
      this.spriteFrame = (this.spriteFrame + 1) % this.spriteFrames.length
      if (this.lastMarkerPoint) this.emitMarker()
    }, 1000 / this.spriteFps)
  }

  private async loadSpriteFrames(url: string, row: number, rows: number, cols: number, fps: number, size: number): Promise<string[]> {
    if (!this.map) return []
    const img = new Image()
    img.src = url
    await img.decode()
    // 按实际网格计算格子尺寸（不能假设正方形，否则会切掉半身）
    const cellW = img.naturalWidth / cols
    const cellH = img.naturalHeight / rows
    const ids: string[] = []
    for (let c = 0; c < cols; c++) {
      // 等比缩放：长边贴齐 size，短边按比例
      const scale = Math.min(size / cellW, size / cellH)
      const w = Math.max(1, Math.round(cellW * scale))
      const h = Math.max(1, Math.round(cellH * scale))
      const canvas = document.createElement('canvas')
      canvas.width = w
      canvas.height = h
      const ctx = canvas.getContext('2d')
      if (!ctx) continue
      ctx.drawImage(img, c * cellW, row * cellH, cellW, cellH, 0, 0, w, h)
      const id = `gxp-run-${c}`
      if (this.map.hasImage(id)) this.map.removeImage(id)
      this.map.addImage(id, ctx.getImageData(0, 0, w, h))
      ids.push(id)
    }
    // 启动换帧定时器（每帧 1000/fps 毫秒）
    this.spriteFps = fps
    this.startFrameTimer()
    return ids
  }

  // 设置自定义进度标记图片（dataURL，静态帧），传 null 恢复跑步动画
  setProgressMarkerImage(url: string | null) {
    this.markerImage = url
    this.markerIconReady = false
    if (url) {
      this.runWhenStyleReady(() => {
        if (!this.markerImage) return
        this.loadImageToMap(this.markerImage, this.markerIconCustomId, 40)
          .then(ok => { this.markerIconReady = ok; this.emitMarker() })
          .catch(() => { this.markerIconReady = false; this.emitMarker() })
      })
    } else {
      this.emitMarker()
    }
  }

  private async loadImageToMap(url: string, imageId: string, maxPx: number): Promise<boolean> {
    if (!this.map) return false
    const img = new Image()
    img.src = url
    await img.decode()
    const scale = Math.min(1, maxPx / Math.max(img.naturalWidth, img.naturalHeight, 1))
    const w = Math.max(1, Math.round(img.naturalWidth * scale))
    const h = Math.max(1, Math.round(img.naturalHeight * scale))
    const canvas = document.createElement('canvas')
    canvas.width = w
    canvas.height = h
    const ctx = canvas.getContext('2d')
    if (!ctx) return false
    ctx.drawImage(img, 0, 0, w, h)
    const data = ctx.getImageData(0, 0, w, h)
    if (this.map.hasImage(imageId)) this.map.removeImage(imageId)
    this.map.addImage(imageId, data)
    return true
  }

  // 航点（GPX wpt）：圆点 + 名字标签常显，点击弹出完整名称
  setWaypoints(wpts: Waypoint[]) {
    this.runWhenStyleReady(() => this.renderWaypoints(wpts))
  }

  private buildWaypointIcon(name: string) {
    const fontSize = 13
    const measure = document.createElement('canvas').getContext('2d')!
    measure.font = `bold ${fontSize}px sans-serif`
    const maxW = 120
    let text = name
    if (measure.measureText(text).width > maxW) {
      while (text.length > 1 && measure.measureText(text + '…').width > maxW) text = text.slice(0, -1)
      text += '…'
    }
    const tw = measure.measureText(text).width
    const c = document.createElement('canvas')
    c.width = Math.ceil(24 + tw + 10)
    c.height = 22
    const ctx = c.getContext('2d')!
    // 背景胶囊
    ctx.fillStyle = 'rgba(22, 33, 62, 0.88)'
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.3)'
    ctx.lineWidth = 1
    const pillX = 14, pillY = 1, pillW = c.width - pillX - 2, pillH = 20, r = 6
    ctx.beginPath()
    ctx.moveTo(pillX + r, pillY)
    ctx.arcTo(pillX + pillW, pillY, pillX + pillW, pillY + pillH, r)
    ctx.arcTo(pillX + pillW, pillY + pillH, pillX, pillY + pillH, r)
    ctx.arcTo(pillX, pillY + pillH, pillX, pillY, r)
    ctx.arcTo(pillX, pillY, pillX + pillW, pillY, r)
    ctx.closePath()
    ctx.fill()
    ctx.stroke()
    // 圆点
    ctx.fillStyle = '#38b6ff'
    ctx.strokeStyle = '#ffffff'
    ctx.lineWidth = 2
    ctx.beginPath()
    ctx.arc(8, 11, 5, 0, Math.PI * 2)
    ctx.fill()
    ctx.stroke()
    // 文字
    ctx.font = `bold ${fontSize}px sans-serif`
    ctx.fillStyle = '#ffffff'
    ctx.textBaseline = 'middle'
    ctx.fillText(text, pillX + 6, pillY + pillH / 2 + 1)
    return ctx.getImageData(0, 0, c.width, c.height)
  }

  private renderWaypoints(wpts: Waypoint[]) {
    if (!this.map) return
    if (!this.map.getSource(this.wptSourceId)) {
      this.map.addSource(this.wptSourceId, { type: 'geojson' as const, data: emptyFeatureCollection() })
      this.map.addLayer({
        id: this.wptLayerId, type: 'symbol' as const, source: this.wptSourceId,
        layout: {
          'icon-image': ['get', 'icon'],
          'icon-anchor': 'left' as const,
          'icon-allow-overlap': true,
          'icon-ignore-placement': true,
          'icon-rotation-alignment': 'viewport' as const,
          'icon-pitch-alignment': 'viewport' as const,
        },
      } as any)
    }
    const features: any[] = []
    const iconIds = new Map<string, string>()
    wpts.filter(w => w.lat !== 0 || w.lng !== 0).forEach((w, i) => {
      const name = w.name || '航点'
      let iconId = iconIds.get(name)
      if (!iconId) {
        iconId = `wpt-name-${i}`
        this.map!.addImage(iconId, this.buildWaypointIcon(name))
        iconIds.set(name, iconId)
      }
      features.push(pointFeature(w.lng, w.lat, { icon: iconId, name }))
    })
    const src = this.map.getSource(this.wptSourceId) as maplibregl.GeoJSONSource
    src.setData({ type: 'FeatureCollection' as const, features })
  }

  getCanvas() { return this.map?.getCanvas() ?? null }
  destroy() { this.map?.remove(); this.map = null }
}
