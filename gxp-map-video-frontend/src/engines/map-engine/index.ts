import maplibregl, { LngLatBounds } from 'maplibre-gl'
import type { TrackPoint } from '@/types/gpx'
import type { Feature, LineString, Point, GeoJSON } from 'geojson'
import '@/engines/map-engine/style.css'

export class MapEngine {
  map: maplibregl.Map | null = null
  private trackSourceId = 'gpx-track-source'
  private trackLayerId = 'gpx-track'
  private markerSourceId = 'progress-marker-source'
  private markerLayerId = 'progress-marker-layer'

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

  loadRoute(points: TrackPoint[]) {
    if (!this.map || points.length < 2) return
    if (this.map.getLayer(this.trackLayerId)) this.map.removeLayer(this.trackLayerId)
    if (this.map.getSource(this.trackSourceId)) this.map.removeSource(this.trackSourceId)

    const coords: [number, number, number][] = points.map(p => [p.longitude, p.latitude, p.elevation])
    this.map.addSource(this.trackSourceId, {
      type: 'geojson' as const,
      data: { type: 'Feature' as const, geometry: { type: 'LineString' as const, coordinates: coords }, properties: {} },
    })
    this.map.addLayer({
      id: this.trackLayerId, type: 'line' as const, source: this.trackSourceId,
      layout: { 'line-join': 'round' as const, 'line-cap': 'round' as const },
      paint: { 'line-color': '#e94560', 'line-width': 4, 'line-opacity': 0.9 },
    })

    const bounds = new LngLatBounds()
    for (const p of points) bounds.extend([p.longitude, p.latitude])
    this.map.fitBounds(bounds, { padding: 80, duration: 0 })
  }

  setProgressPoint(point: TrackPoint) {
    if (!this.map) return
    if (this.map.getLayer(this.markerLayerId)) this.map.removeLayer(this.markerLayerId)
    if (this.map.getSource(this.markerSourceId)) this.map.removeSource(this.markerSourceId)

    this.map.addSource(this.markerSourceId, {
      type: 'geojson' as const,
      data: { type: 'Feature' as const, geometry: { type: 'Point' as const, coordinates: [point.longitude, point.latitude] }, properties: {} },
    })
    this.map.addLayer({
      id: this.markerLayerId, type: 'circle' as const, source: this.markerSourceId,
      paint: { 'circle-radius': 8, 'circle-color': '#00ff88', 'circle-stroke-width': 2, 'circle-stroke-color': '#ffffff' },
    })
  }

  removeProgressMarker() {
    if (!this.map) return
    if (this.map.getLayer(this.markerLayerId)) this.map.removeLayer(this.markerLayerId)
    if (this.map.getSource(this.markerSourceId)) this.map.removeSource(this.markerSourceId)
  }

  getCanvas() { return this.map?.getCanvas() ?? null }
  destroy() { this.map?.remove(); this.map = null }
}
