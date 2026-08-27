package export

import (
	"encoding/json"
	"fmt"
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

	// Compute tight bounding box from route start/end
	minLat, minLng, maxLat, maxLng := r.StartLat, r.StartLng, r.EndLat, r.EndLng
	if minLat > maxLat { minLat, maxLat = maxLat, minLat }
	if minLng > maxLng { minLng, maxLng = maxLng, minLng }

	// Add small padding (1% of bounding box, min 0.001 degrees)
	dLat := maxLat - minLat
	dLng := maxLng - minLng
	padLat := max(0.001, dLat*0.1)
	padLng := max(0.001, dLng*0.1)
	minLat -= padLat; maxLat += padLat
	minLng -= padLng; maxLng += padLng

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
	if minLat > maxLat { minLat, maxLat = maxLat, minLat }
	if minLng > maxLng { minLng, maxLng = maxLng, minLng }
	dLat := max(0.001, (maxLat-minLat)*0.1)
	dLng := max(0.001, (maxLng-minLng)*0.1)
	minLat -= dLat; maxLat += dLat; minLng -= dLng; maxLng += dLng

	var wg sync.WaitGroup
	var failed atomic.Int32

	for z := 8; z <= 14; z++ {
		tiles := tile.BoundingBoxToTiles(minLat, minLng, maxLat, maxLng, z)
		for _, t := range tiles {
			wg.Add(2)
			go func(z, x, y int) {
				defer wg.Done()
				if _, err := s.tileCache.Get("esri-satellite", z, x, y); err != nil {
					failed.Add(1)
				}
			}(t[0], t[1], t[2])
			go func(z, x, y int) {
				defer wg.Done()
				if _, err := s.tileCache.Get("terrain", z, x, y); err != nil {
					failed.Add(1)
				}
			}(t[0], t[1], t[2])
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
	if err := s.db.Create(&task).Error; err != nil { return nil, err }
	return &task, nil
}

func (s *Service) RunExport(taskID uint) error {
	var task route.ExportTask
	if err := s.db.First(&task, taskID).Error; err != nil { return err }
	task.Status = "RUNNING"; s.db.Save(&task)

	var r route.Route
	s.db.Preload("TrackPoints").First(&r, task.RouteID)
	var segments []route.RouteSegment
	s.db.Where("route_id = ?", task.RouteID).Order("start_distance ASC").Find(&segments)
	var events []route.StoryEvent
	s.db.Where("route_id = ?", task.RouteID).Order("position ASC").Find(&events)

	type oSeg struct { StartDist float64 `json:"start_distance"`; EndDist float64 `json:"end_distance"`; Type string `json:"type"`; Commentary string `json:"commentary"` }
	type oEvent struct { Position float64 `json:"position"`; Type string `json:"event_type"`; Script string `json:"script"`; Title string `json:"title"` }
	type oData struct {
		Stats struct { TotalDistance float64 `json:"total_distance"`; Ascent float64 `json:"ascent"`; Descent float64 `json:"descent"`; MaxElevation float64 `json:"max_elevation"`; MinElevation float64 `json:"min_elevation"` } `json:"stats"`
		Segments  []oSeg   `json:"segments"`
		Events    []oEvent `json:"events"`
	}
	od := oData{}
	od.Stats.TotalDistance = r.TotalDistance / 1000
	od.Stats.Ascent = r.TotalAscent
	od.Stats.Descent = r.TotalDescent
	od.Stats.MaxElevation = r.MaxElevation
	od.Stats.MinElevation = r.MinElevation
	for _, seg := range segments {
		od.Segments = append(od.Segments, oSeg{StartDist: seg.StartDistance/1000, EndDist: seg.EndDistance/1000, Type: seg.Type, Commentary: seg.Commentary})
	}
	for _, ev := range events {
		od.Events = append(od.Events, oEvent{Position: ev.Position, Type: ev.EventType, Script: ev.Script, Title: ev.Title})
	}

	os.MkdirAll(filepath.Join(s.dataDir, "exports"), 0755)
	overlayJSON, _ := json.Marshal(od)
	overlayPath := filepath.Join(s.dataDir, "exports", fmt.Sprintf("overlay_%d.json", taskID))
	os.WriteFile(overlayPath, overlayJSON, 0644)

	outputPath := filepath.Join(s.dataDir, "exports", fmt.Sprintf("route_%d_%d.mp4", task.RouteID, taskID))
	webmPath := filepath.Join(s.dataDir, "exports", fmt.Sprintf("input_%d.webm", taskID))

	args := []string{"-y"}
	if _, err := os.Stat(webmPath); err == nil {
		args = append(args, "-i", webmPath)
	} else {
		args = append(args, "-f", "lavfi", "-i", "color=c=black:s=1920x1080:d=1")
	}
	args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-shortest", outputPath)

	cmd := exec.Command("ffmpeg", args...)
	if err := cmd.Run(); err != nil {
		task.Status = "FAILED"; task.ErrorMessage = "FFmpeg: " + err.Error(); s.db.Save(&task)
		return err
	}
	task.Status = "SUCCESS"; task.Progress = 1.0; task.OutputPath = outputPath
	s.db.Save(&task)
	return nil
}

func (s *Service) RunExportWithWebM(taskID uint, webmPath string) error {
	var task route.ExportTask
	if err := s.db.First(&task, taskID).Error; err != nil { return err }
	task.Status = "RUNNING"; s.db.Save(&task)

	var r route.Route
	s.db.Preload("TrackPoints").First(&r, task.RouteID)
	var segments []route.RouteSegment
	s.db.Where("route_id = ?", task.RouteID).Order("start_distance ASC").Find(&segments)
	var events []route.StoryEvent
	s.db.Where("route_id = ?", task.RouteID).Order("position ASC").Find(&events)

	type oSeg struct { StartDist float64 `json:"start_distance"`; EndDist float64 `json:"end_distance"`; Type string `json:"type"`; Commentary string `json:"commentary"` }
	type oEvent struct { Position float64 `json:"position"`; Type string `json:"event_type"`; Script string `json:"script"`; Title string `json:"title"` }
	type oData struct {
		Stats struct { TotalDistance float64 `json:"total_distance"`; Ascent float64 `json:"ascent"`; Descent float64 `json:"descent"`; MaxElevation float64 `json:"max_elevation"`; MinElevation float64 `json:"min_elevation"` } `json:"stats"`
		Segments  []oSeg   `json:"segments"`
		Events    []oEvent `json:"events"`
	}
	od := oData{}
	od.Stats.TotalDistance = r.TotalDistance / 1000
	od.Stats.Ascent = r.TotalAscent
	od.Stats.Descent = r.TotalDescent
	od.Stats.MaxElevation = r.MaxElevation
	od.Stats.MinElevation = r.MinElevation
	for _, seg := range segments {
		od.Segments = append(od.Segments, oSeg{StartDist: seg.StartDistance/1000, EndDist: seg.EndDistance/1000, Type: seg.Type, Commentary: seg.Commentary})
	}
	for _, ev := range events {
		od.Events = append(od.Events, oEvent{Position: ev.Position, Type: ev.EventType, Script: ev.Script, Title: ev.Title})
	}
	os.MkdirAll(filepath.Join(s.dataDir, "exports"), 0755)
	overlayJSON, _ := json.Marshal(od)
	overlayPath := filepath.Join(s.dataDir, "exports", fmt.Sprintf("overlay_%d.json", taskID))
	os.WriteFile(overlayPath, overlayJSON, 0644)

	outputPath := filepath.Join(s.dataDir, "exports", fmt.Sprintf("route_%d_%d.mp4", task.RouteID, taskID))
	args := []string{"-y", "-i", webmPath, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-shortest", outputPath}
	cmd := exec.Command("ffmpeg", args...)
	if err := cmd.Run(); err != nil {
		task.Status = "FAILED"; task.ErrorMessage = "FFmpeg: " + err.Error(); s.db.Save(&task)
		return err
	}
	task.Status = "SUCCESS"; task.Progress = 1.0; task.OutputPath = outputPath
	s.db.Save(&task)
	return nil
}
