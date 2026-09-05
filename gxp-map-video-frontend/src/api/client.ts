import type { Route, TrackPoint, ActivityPreset, RouteSegment, StoryEvent, ExportTask, Waypoint, ShowConfig } from '@/types/gpx'

const BASE = '/api'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const resp = await fetch(BASE + path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }))
    throw new Error(err.error || resp.statusText)
  }
  return resp.json()
}

export const api = {
  uploadGPX: async (file: File, name?: string) => {
    const form = new FormData()
    form.append('file', file)
    if (name) form.append('name', name)
    const resp = await fetch(BASE + '/routes', { method: 'POST', body: form })
    if (!resp.ok) {
      const err = await resp.json().catch(() => ({ error: resp.statusText }))
      throw new Error(err.error || resp.statusText)
    }
    return resp.json()
  },
  listRoutes: () => request<Route[]>('/routes'),
  getRoute: (id: number) => request<Route>(`/routes/${id}`),
  getPoints: (id: number) => request<TrackPoint[]>(`/routes/${id}/points`),
  getWaypoints: (id: number) => request<Waypoint[]>(`/routes/${id}/waypoints`),
  arrangeWithAI: (id: number, opts: { durationSec: number; style?: string; voice?: string; raceName: string; category: string }) =>
    request<{ status: string }>(`/routes/${id}/arrange`, { method: 'POST', body: JSON.stringify(opts) }),
  getShowConfig: (id: number) =>
    request<{ status: 'NONE' | 'GENERATING' | 'READY' | 'FAILED'; config: ShowConfig | null; errorMessage: string | null; updatedAt?: string }>(`/routes/${id}/config`),
  saveShowConfig: (id: number, cfg: ShowConfig) =>
    request<{ status: string }>(`/routes/${id}/config`, { method: 'PUT', body: JSON.stringify(cfg) }),
  synthesizeEventTTS: (id: number, body: { text: string; voice: string }) =>
    request<{ url: string; durationSec: number }>(`/routes/${id}/tts`, { method: 'POST', body: JSON.stringify(body) }),
  deleteRoute: (id: number) => request<any>(`/routes/${id}`, { method: 'DELETE' }),

  analyzeRoute: (id: number, preset: string) =>
    request<RouteSegment[]>(`/routes/${id}/analyze`, { method: 'POST', body: JSON.stringify({ preset }) }),
  getSegments: (id: number) => request<RouteSegment[]>(`/routes/${id}/segments`),
	// 注意：后端把 segments/events 的按 ID 操作也注册在 /routes 组下
	updateSegment: (id: number, data: Partial<RouteSegment>) =>
		request<RouteSegment>(`/routes/segments/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
	mergeSegment: (id: number) =>
		request<RouteSegment>(`/routes/segments/${id}/merge`, { method: 'POST' }),

  createEvent: (routeId: number, data: Partial<StoryEvent>) =>
    request<StoryEvent>(`/routes/${routeId}/events`, { method: 'POST', body: JSON.stringify(data) }),
  getEvents: (routeId: number) => request<StoryEvent[]>(`/routes/${routeId}/events`),
  updateEvent: (id: number, data: Partial<StoryEvent>) =>
    request<StoryEvent>(`/routes/events/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteEvent: (id: number) => request<any>(`/routes/events/${id}`, { method: 'DELETE' }),

  getActivityPresets: () => request<Record<string, ActivityPreset>>('/config/activity-presets'),

  preflightExport: (id: number) => request<{ total_tiles: number; missing_tiles: number; zoom_range: string }>(`/export/preflight/${id}`, { method: 'POST' }),
  preloadTiles: (id: number) => request<any>(`/export/preload/${id}`, { method: 'POST' }),
  createExportTask: (id: number) => request<ExportTask>(`/export/${id}`, { method: 'POST' }),
  getExportTask: (id: number) => request<ExportTask>(`/export/tasks/${id}`),
  cancelExportTask: (id: number) => request<any>(`/export/tasks/${id}`, { method: 'DELETE' }),
}
