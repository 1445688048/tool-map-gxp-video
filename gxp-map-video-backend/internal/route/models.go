package route

import (
	"time"
)

// Route represents an uploaded GPX route
type Route struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Name           string    `gorm:"size:255;not null" json:"name"`
	GPXPath        string    `gorm:"size:512;not null" json:"gpx_path"`
	TotalDistance  float64   `gorm:"type:decimal(10,3);not null" json:"total_distance"`
	TotalAscent    float64   `gorm:"type:decimal(8,1);not null" json:"total_ascent"`
	TotalDescent   float64   `gorm:"type:decimal(8,1);not null" json:"total_descent"`
	MinElevation   float64   `gorm:"type:decimal(8,2);not null" json:"min_elevation"`
	MaxElevation   float64   `gorm:"type:decimal(8,2);not null" json:"max_elevation"`
	StartLat       float64   `gorm:"type:decimal(10,7);not null" json:"start_lat"`
	StartLng       float64   `gorm:"type:decimal(10,7);not null" json:"start_lng"`
	EndLat         float64   `gorm:"type:decimal(10,7);not null" json:"end_lat"`
	EndLng         float64   `gorm:"type:decimal(10,7);not null" json:"end_lng"`
	PointCount     int       `gorm:"not null" json:"point_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	TrackPoints []TrackPoint `gorm:"foreignKey:RouteID" json:"track_points,omitempty"`
}

// TrackPoint is a single point on the route
type TrackPoint struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	RouteID    uint      `gorm:"index;not null" json:"route_id"`
	Index      int       `gorm:"not null;index" json:"index"`
	Latitude   float64   `gorm:"type:decimal(10,7);not null" json:"latitude"`
	Longitude  float64   `gorm:"type:decimal(10,7);not null" json:"longitude"`
	Elevation  float64   `gorm:"type:decimal(8,2);not null" json:"elevation"`
	Distance   float64   `gorm:"type:decimal(10,3);not null" json:"distance"`
	Slope      float64   `gorm:"type:decimal(6,2)" json:"slope"`
	Time       *time.Time `json:"time"`
	Speed      float64   `gorm:"type:decimal(6,2)" json:"speed"`
}

// RouteSegment represents an analyzed segment of the route
type RouteSegment struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	RouteID        uint      `gorm:"index;not null" json:"route_id"`
	StartIndex     int       `gorm:"not null" json:"start_index"`
	EndIndex       int       `gorm:"not null" json:"end_index"`
	StartDistance  float64   `gorm:"type:decimal(10,3);not null" json:"start_distance"`
	EndDistance    float64   `gorm:"type:decimal(10,3);not null" json:"end_distance"`
	Distance       float64   `gorm:"type:decimal(10,3);not null" json:"distance"`
	StartElevation float64   `gorm:"type:decimal(8,2);not null" json:"start_elevation"`
	EndElevation   float64   `gorm:"type:decimal(8,2);not null" json:"end_elevation"`
	ElevationGain  float64   `gorm:"type:decimal(8,2);not null" json:"elevation_gain"`
	ElevationLoss  float64   `gorm:"type:decimal(8,2);not null" json:"elevation_loss"`
	AverageSlope   float64   `gorm:"type:decimal(6,2);not null" json:"average_slope"`
	MaxSlope       float64   `gorm:"type:decimal(6,2);not null" json:"max_slope"`
	Type           string    `gorm:"size:32;not null" json:"type"`
	CameraPreset   string    `gorm:"size:32" json:"camera_preset"`
	Commentary     string    `gorm:"type:text" json:"commentary"`
	Enabled        bool      `gorm:"default:true" json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// StoryEvent is a narrative event on the timeline
type StoryEvent struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	RouteID      uint      `gorm:"index;not null" json:"route_id"`
	Position     float64   `gorm:"type:decimal(10,3);not null" json:"position"` // km
	EventType    string    `gorm:"size:32;not null" json:"event_type"`
	Title        string    `gorm:"size:255" json:"title"`
	Description  string    `gorm:"type:text" json:"description"`
	Script       string    `gorm:"type:text" json:"script"`
	CameraPreset string    `gorm:"size:32" json:"camera_preset"`
	HoldBefore   float64   `gorm:"default:0" json:"hold_before"`
	HoldAfter    float64   `gorm:"default:0" json:"hold_after"`
	TTSStatus    string    `gorm:"size:32;default:'pending'" json:"tts_status"`
	TTSAudioURL  string    `gorm:"size:512" json:"tts_audio_url"`
	TTSDuration  float64   `gorm:"type:decimal(6,2)" json:"tts_duration"`
	Enabled      bool      `gorm:"default:true" json:"enabled"`
	Order        int       `gorm:"default:0" json:"order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ExportTask tracks async video export jobs
type ExportTask struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	RouteID      uint      `gorm:"index;not null" json:"route_id"`
	Status       string    `gorm:"size:32;not null;default:'PENDING'" json:"status"`
	Progress     float64   `gorm:"default:0" json:"progress"`
	OutputPath   string    `gorm:"size:512" json:"output_path"`
	ErrorMessage string    `gorm:"type:text" json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RouteShowConfig 每条路线的演出配置（LLM 生成或手动保存）
type RouteShowConfig struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	RouteID      uint      `gorm:"uniqueIndex;not null" json:"route_id"`
	Status       string    `gorm:"size:16;not null;default:'NONE'" json:"status"` // NONE | GENERATING | READY | FAILED
	Config       string    `gorm:"type:text" json:"config"`
	ErrorMessage string    `gorm:"type:text" json:"error_message,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}
