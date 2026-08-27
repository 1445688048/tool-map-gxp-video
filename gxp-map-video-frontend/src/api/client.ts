import type { Route, TrackPoint, ActivityPreset, RouteSegment, StoryEvent, ExportTask } from '@/types/gpx'

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
  uploadGPX: (file: File, name?: string) => {
    const form = new FormData()
    form.append('file', file)
    if (name) form.append('name', name)
    return fetch(BASE + '/routes', { method: 'POST', body: form }).then(r => r.json())
  },
  listRoutes: () => request<Route[]>('/routes'),
  getRoute: (id: number) => request<Route>(`/routes/${id}`),
  getPoints: (id: number) => request<TrackPoint[]>(`/routes/${id}/points`),
  deleteRoute: (id: number) => request<any>(`/routes/${id}`, { method: 'DELETE' }),

  analyzeRoute: (id: number, preset: string) =>
    request<RouteSegment[]>(`/routes/${id}/analyze`, { method: 'POST', body: JSON.stringify({ preset }) }),
  getSegments: (id: number) => request<RouteSegment[]>(`/routes/${id}/segments`),
  updateSegment: (id: number, data: Partial<RouteSegment>) =>
    request<RouteSegment>(`/segments/${id}`, { method: 'PUT', body: JSON.stringify(data) }),

  createEvent: (routeId: number, data: Partial<StoryEvent>) =>
    request<StoryEvent>(`/routes/${routeId}/events`, { method: 'POST', body: JSON.stringify(data) }),
  getEvents: (routeId: number) => request<StoryEvent[]>(`/routes/${routeId}/events`),
  updateEvent: (id: number, data: Partial<StoryEvent>) =>
    request<StoryEvent>(`/events/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteEvent: (id: number) => request<any>(`/events/${id}`, { method: 'DELETE' }),

  getActivityPresets: () => request<Record<string, ActivityPreset>>('/config/activity-presets'),

  preflightExport: (id: number) => request<{ total_tiles: number; missing_tiles: number; zoom_range: string }>(`/export/preflight/${id}`, { method: 'POST' }),
  preloadTiles: (id: number) => request<any>(`/export/preload/${id}`, { method: 'POST' }),
  createExportTask: (id: number) => request<ExportTask>(`/export/${id}`, { method: 'POST' }),
  getExportTask: (id: number) => request<ExportTask>(`/export/tasks/${id}`),
  cancelExportTask: (id: number) => request<any>(`/export/tasks/${id}`, { method: 'DELETE' }),
}
