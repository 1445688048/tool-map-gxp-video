export interface RawPoint {
  lat: number
  lng: number
  ele: number
  time?: string
}

export interface Route {
  id: number
  name: string
  gpx_path: string
  total_distance: number
  total_ascent: number
  total_descent: number
  min_elevation: number
  max_elevation: number
  start_lat: number
  start_lng: number
  end_lat: number
  end_lng: number
  point_count: number
  created_at: string
  updated_at: string
}

export interface TrackPoint {
  id: number
  route_id: number
  index: number
  latitude: number
  longitude: number
  elevation: number
  distance: number
  slope: number
  time?: string
  speed?: number
}

export interface RouteSegment {
  id: number
  route_id: number
  start_index: number
  end_index: number
  start_distance: number
  end_distance: number
  distance: number
  start_elevation: number
  end_elevation: number
  elevation_gain: number
  elevation_loss: number
  average_slope: number
  max_slope: number
  type: 'FLAT' | 'CLIMB' | 'STEEP_CLIMB' | 'DESCENT' | 'STEEP_DESCENT'
  camera_preset?: string
  commentary: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface StoryEvent {
  id: number
  route_id: number
  position: number
  event_type: string
  title: string
  description: string
  script: string
  camera_preset: string
  hold_before: number
  hold_after: number
  tts_status: string
  tts_audio_url: string
  tts_duration: number
  enabled: boolean
  order: number
  created_at: string
  updated_at: string
}

export interface ExportTask {
  id: number
  route_id: number
  status: 'PENDING' | 'RUNNING' | 'SUCCESS' | 'FAILED' | 'CANCELLED'
  progress: number
  output_path: string
  error_message: string
  created_at: string
  updated_at: string
}

export interface ActivityPreset {
  name: string
  flat_range: [number, number]
  climb_range: [number, number]
  steep_climb_threshold: number
  descent_range: [number, number]
  steep_descent_threshold: number
  min_segment_distance_m: number
}
