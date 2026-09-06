package export

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"

	"gxp-map-video-backend/internal/route"
	"gxp-map-video-backend/internal/tile"

	"gorm.io/gorm"
)

type Service struct {
	db        *gorm.DB
	tileCache *tile.Cache
	dataDir   string
}

func New(db *gorm.DB, tileCache *tile.Cache, dataDir string) *Service {
	return &Service{db: db, tileCache: tileCache, dataDir: dataDir}
}

type PreflightResult struct {
	TotalTiles int    `json:"total_tiles"`
	Missing    int    `json:"missing_tiles"`
	ZoomRange  string `json:"zoom_range"`
}

func (s *Service) Preflight(routeID uint) (*PreflightResult, error) {
	var r route.Route
	if err := s.db.First(&r, routeID).Error; err != nil {
		return nil, err
	}

	minLat, minLng, maxLat, maxLng := r.StartLat, r.StartLng, r.EndLat, r.EndLng
	if minLat > maxLat {
		minLat, maxLat = maxLat, minLat
	}
	if minLng > maxLng {
		minLng, maxLng = maxLng, minLng
	}

	dLat := maxLat - minLat
	dLng := maxLng - minLng
	padLat := max(0.001, dLat*0.1)
	padLng := max(0.001, dLng*0.1)
	minLat -= padLat
	maxLat += padLat
	minLng -= padLng
	maxLng += padLng

	zoomRange := "8-14"
	var totalTiles int
	for z := 8; z <= 14; z++ {
		totalTiles += len(tile.BoundingBoxToTiles(minLat, minLng, maxLat, maxLng, z))
	}

	missingCount := 0
	for z := 8; z <= 14; z++ {
		tiles := tile.BoundingBoxToTiles(minLat, minLng, maxLat, maxLng, z)
		m, _ := s.tileCache.CheckMissing("esri-satellite", tiles)
		missingCount += len(m)
		mt, _ := s.tileCache.CheckMissing("terrain", tiles)
		missingCount += len(mt)
	}

	return &PreflightResult{
		TotalTiles: totalTiles,
		Missing:    missingCount,
		ZoomRange:  zoomRange,
	}, nil
}

func (s *Service) Preload(routeID uint) error {
	var r route.Route
	if err := s.db.First(&r, routeID).Error; err != nil {
		return err
	}

	minLat, minLng, maxLat, maxLng := r.StartLat, r.StartLng, r.EndLat, r.EndLng
	if minLat > maxLat {
		minLat, maxLat = maxLat, minLat
	}
	if minLng > maxLng {
		minLng, maxLng = maxLng, minLng
	}
	dLat := max(0.001, (maxLat-minLat)*0.1)
	dLng := max(0.001, (maxLng-minLng)*0.1)
	minLat -= dLat
	maxLat += dLat
	minLng -= dLng
	maxLng += dLng

	var wg sync.WaitGroup
	var failed atomic.Int32

	for z := 8; z <= 14; z++ {
		tiles := tile.BoundingBoxToTiles(minLat, minLng, maxLat, maxLng, z)
		for _, t := range tiles {
			wg.Add(2)
			go func(provider string, z, x, y int) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						log.Printf("tile download recovered z=%d x=%d y=%d provider=%s: %v", z, x, y, provider, r)
						failed.Add(1)
					}
				}()
				if _, err := s.tileCache.Get(provider, z, x, y); err != nil {
					failed.Add(1)
				}
			}("esri-satellite", t[0], t[1], t[2])
			go func(provider string, z, x, y int) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						log.Printf("tile download recovered z=%d x=%d y=%d provider=%s: %v", z, x, y, provider, r)
						failed.Add(1)
					}
				}()
				if _, err := s.tileCache.Get(provider, z, x, y); err != nil {
					failed.Add(1)
				}
			}("terrain", t[0], t[1], t[2])
		}
	}

	wg.Wait()
	if failed.Load() > 0 {
		return fmt.Errorf("failed to preload %d tiles", failed.Load())
	}
	return nil
}

func (s *Service) CreateTask(routeID uint) (*route.ExportTask, error) {
	task := route.ExportTask{RouteID: routeID, Status: "PENDING", Progress: 0}
	if err := s.db.Create(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// runExportInternal is the shared export logic used by both RunExport and RunExportWithWebM.
func (s *Service) runExportInternal(taskID uint, inputSource, outW, outH string) error {
	var task route.ExportTask
	if err := s.db.First(&task, taskID).Error; err != nil {
		return err
	}
	task.Status = "RUNNING"
	s.db.Save(&task)

	var r route.Route
	s.db.Preload("TrackPoints").First(&r, task.RouteID)
	var segments []route.RouteSegment
	s.db.Where("route_id = ?", task.RouteID).Order("start_distance ASC").Find(&segments)
	var events []route.StoryEvent
	s.db.Where("route_id = ?", task.RouteID).Order("position ASC").Find(&events)

	type oSeg struct {
		StartDist  float64 `json:"start_distance"`
		EndDist    float64 `json:"end_distance"`
		Type       string  `json:"type"`
		Commentary string  `json:"commentary"`
	}
	type oEvent struct {
		Position float64 `json:"position"`
		Type     string  `json:"event_type"`
		Script   string  `json:"script"`
		Title    string  `json:"title"`
	}
	type oData struct {
		Stats struct {
			TotalDistance float64 `json:"total_distance"`
			Ascent        float64 `json:"ascent"`
			Descent       float64 `json:"descent"`
			MaxElevation  float64 `json:"max_elevation"`
			MinElevation  float64 `json:"min_elevation"`
		} `json:"stats"`
		Segments []oSeg   `json:"segments"`
		Events   []oEvent `json:"events"`
	}
	od := oData{}
	od.Stats.TotalDistance = r.TotalDistance / 1000
	od.Stats.Ascent = r.TotalAscent
	od.Stats.Descent = r.TotalDescent
	od.Stats.MaxElevation = r.MaxElevation
	od.Stats.MinElevation = r.MinElevation
	for _, seg := range segments {
		od.Segments = append(od.Segments, oSeg{
			StartDist:  seg.StartDistance / 1000,
			EndDist:    seg.EndDistance / 1000,
			Type:       seg.Type,
			Commentary: seg.Commentary,
		})
	}
	for _, ev := range events {
		od.Events = append(od.Events, oEvent{
			Position: ev.Position,
			Type:     ev.EventType,
			Script:   ev.Script,
			Title:    ev.Title,
		})
	}

	if err := os.MkdirAll(filepath.Join(s.dataDir, "exports"), 0755); err != nil {
		task.Status = "FAILED"
		task.ErrorMessage = "mkdir: " + err.Error()
		s.db.Save(&task)
		return err
	}
	overlayJSON, _ := json.Marshal(od)
	overlayPath := filepath.Join(s.dataDir, "exports", fmt.Sprintf("overlay_%d.json", taskID))
	os.WriteFile(overlayPath, overlayJSON, 0644)

	outputPath := filepath.Join(s.dataDir, "exports", fmt.Sprintf("route_%d_%d.mp4", task.RouteID, taskID))

	var args []string
	if inputSource == "" {
		args = []string{"-y", "-f", "lavfi", "-i", "color=c=black:s=1920x1080:d=1"}
	} else {
		args = []string{"-y", "-i", inputSource}
	}
	if outW != "" && outH != "" {
		args = append(args, "-vf", fmt.Sprintf("scale=%s:%s", outW, outH))
	}
	args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-shortest", outputPath)

	cmd := exec.Command("ffmpeg", args...)
	if err := cmd.Run(); err != nil {
		task.Status = "FAILED"
		task.ErrorMessage = "FFmpeg: " + err.Error()
		s.db.Save(&task)
		return err
	}
	task.Status = "SUCCESS"
	task.Progress = 1.0
	task.OutputPath = outputPath
	s.db.Save(&task)
	return nil
}

func (s *Service) RunExport(taskID uint) error {
	return s.runExportInternal(taskID, "", "", "")
}

func (s *Service) RunExportWithWebM(taskID uint, webmPath, outW, outH string) error {
	return s.runExportInternal(taskID, webmPath, outW, outH)
}
