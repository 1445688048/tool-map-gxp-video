package api

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gxp-map-video-backend/internal/export"
	"gxp-map-video-backend/internal/gpx"
	"gxp-map-video-backend/internal/llm"
	"gxp-map-video-backend/internal/route"
	"gxp-map-video-backend/internal/segment"
	"gxp-map-video-backend/internal/tile"
	"gxp-map-video-backend/internal/tts"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handlers struct {
	DB        *gorm.DB
	RouteSvc  *route.Service
	Segmenter *segment.Segmenter
	TileCache *tile.Cache
	ExportSvc *export.Service
	TTS       tts.TTSProvider
	DataDir   string
}

func RegisterRoutes(r *gin.Engine, db *gorm.DB, tileCache *tile.Cache, dataDir string) {
	h := &Handlers{
		DB:        db,
		RouteSvc:  route.NewService(route.NewDB(db)),
		Segmenter: segment.New(),
		TileCache: tileCache,
		ExportSvc: export.New(db, tileCache, dataDir),
		TTS:       tts.NewEdgeTTSProvider(),
		DataDir:   dataDir,
	}

	api := r.Group("/api")
	{
		api.Static("/audio", filepath.Join(dataDir, "audio"))
		routes := api.Group("/routes")
		{
			routes.POST("", h.UploadGPX)
			routes.GET("", h.ListRoutes)
			routes.GET("/:id", h.GetRoute)
			routes.GET("/:id/points", h.GetRoutePoints)
			routes.GET("/:id/waypoints", h.GetWaypoints)
			routes.POST("/:id/arrange", h.ArrangeRoute)
			routes.GET("/:id/config", h.GetShowConfig)
			routes.PUT("/:id/config", h.SaveShowConfig)
			routes.POST("/:id/tts", h.SynthesizeEventTTS)
			routes.DELETE("/:id", h.DeleteRoute)
			routes.POST("/:id/analyze", h.AnalyzeRoute)
			routes.GET("/:id/segments", h.GetSegments)
			routes.PUT("/segments/:id", h.UpdateSegment)
			routes.POST("/segments/:id/merge", h.MergeSegment)
			routes.POST("/:id/segments/regenerate", h.RegenerateSegments)
			routes.POST("/:id/events", h.CreateEvent)
			routes.GET("/:id/events", h.GetEvents)
			routes.PUT("/events/:id", h.UpdateEvent)
			routes.DELETE("/events/:id", h.DeleteEvent)
		}
		config := api.Group("/config")
		{
			config.GET("/activity-presets", h.GetActivityPresets)
		}
		api.GET("/tts", h.SynthesizeSpeech)
		tiles := api.Group("/tiles")
		{
			tiles.GET("/:provider/:z/:x/:y", h.GetTile)
		}
		exp := api.Group("/export")
		{
			exp.POST("/preflight/:id", h.PreflightExport)
			exp.POST("/preload/:id", h.PreloadTiles)
			exp.POST("/:id", h.CreateExportTask)
			exp.GET("/tasks/:tid", h.GetExportTask)
			exp.DELETE("/tasks/:tid", h.CancelExportTask)
			exp.POST("/tasks/:tid/run", h.RunExportTask)
			exp.POST("/upload/:tid", h.UploadWebM)
		}
	}
}

func (h *Handlers) UploadGPX(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file"})
		return
	}
	if file.Size == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty file"})
		return
	}
	name := c.PostForm("name")
	if name == "" {
		name = filepath.Base(file.Filename)
	}
	data, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer data.Close()
	buf, err := io.ReadAll(data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	gpxPath := filepath.Join(h.DataDir, "uploads", filepath.Base(file.Filename))
	if err := os.WriteFile(gpxPath, buf, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result, err := h.RouteSvc.CreateFromGPX(name, gpxPath, buf)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *Handlers) ListRoutes(c *gin.Context) {
	routes, err := h.RouteSvc.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, routes)
}

func (h *Handlers) GetRoute(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	r, err := h.RouteSvc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, r)
}

func (h *Handlers) GetRoutePoints(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	points, err := h.RouteSvc.GetPoints(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, points)
}

// GetWaypoints 返回路线 GPX 文件中的航点（从磁盘重新解析）
func (h *Handlers) GetWaypoints(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	r, err := h.RouteSvc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	data, err := os.ReadFile(r.GPXPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read gpx: " + err.Error()})
		return
	}
	result, err := gpx.Parse(data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "parse gpx: " + err.Error()})
		return
	}
	if result.Waypoints == nil {
		result.Waypoints = []gpx.Waypoint{}
	}
	c.JSON(http.StatusOK, result.Waypoints)
}

func (h *Handlers) DeleteRoute(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.RouteSvc.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handlers) AnalyzeRoute(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Preset string `json:"preset" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	points, err := h.RouteSvc.GetPoints(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	segments, err := h.Segmenter.Segment(points, req.Preset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.DB.Where("route_id = ?", id).Delete(&route.RouteSegment{})
	for i := range segments {
		segments[i].RouteID = id
		h.DB.Create(&segments[i])
	}
	c.JSON(http.StatusOK, segments)
}

func (h *Handlers) GetSegments(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var segments []route.RouteSegment
	if err := h.DB.Where("route_id = ?", id).Order("start_distance ASC").Find(&segments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, segments)
}

func (h *Handlers) UpdateSegment(c *gin.Context) {
	sid, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var seg route.RouteSegment
	if err := h.DB.First(&seg, sid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req struct {
		Commentary string `json:"commentary"`
		Enabled    *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]interface{}{}
	if req.Commentary != "" {
		updates["commentary"] = req.Commentary
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	h.DB.Model(&seg).Updates(updates)
	c.JSON(http.StatusOK, seg)
}

// MergeSegment merges the segment with the one right after it (custom segmentation).
func (h *Handlers) MergeSegment(c *gin.Context) {
	sid, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var seg route.RouteSegment
	if err := h.DB.First(&seg, sid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "segment not found"})
		return
	}
	// Find the next segment (smallest start_distance greater than current)
	var next route.RouteSegment
	if err := h.DB.Where("route_id = ? AND start_distance > ?", seg.RouteID, seg.StartDistance).
		Order("start_distance ASC").First(&next).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no next segment to merge"})
		return
	}
	merged := seg
	merged.EndIndex = next.EndIndex
	merged.EndDistance = next.EndDistance
	merged.Distance = next.EndDistance - seg.StartDistance
	merged.EndElevation = next.EndElevation
	merged.ElevationGain = seg.ElevationGain + next.ElevationGain
	merged.ElevationLoss = seg.ElevationLoss + next.ElevationLoss
	if next.MaxSlope > merged.MaxSlope {
		merged.MaxSlope = next.MaxSlope
	}
	if merged.Distance > 0 {
		avg := (merged.EndElevation - merged.StartElevation) / merged.Distance * 100
		merged.AverageSlope = math.Round(avg*100) / 100
		merged.Type = classifySegment(merged.AverageSlope)
	}
	if err := h.DB.Save(&merged).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.DB.Delete(&route.RouteSegment{}, next.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, merged)
}

func (h *Handlers) RegenerateSegments(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	points, err := h.RouteSvc.GetPoints(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	segments, err := h.Segmenter.Segment(points, "trail_running")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.DB.Where("route_id = ?", id).Delete(&route.RouteSegment{})
	for i := range segments {
		segments[i].RouteID = id
		h.DB.Create(&segments[i])
	}
	c.JSON(http.StatusOK, segments)
}

func (h *Handlers) CreateEvent(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var event route.StoryEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	event.RouteID = id
	if event.EventType == "" {
		event.EventType = "COMMENTARY"
	}
	if err := h.DB.Create(&event).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, event)
}

func (h *Handlers) GetEvents(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var events []route.StoryEvent
	if err := h.DB.Where("route_id = ?", id).Order("position ASC").Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, events)
}

func (h *Handlers) UpdateEvent(c *gin.Context) {
	eid, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var event route.StoryEvent
	if err := h.DB.First(&event, eid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req struct {
		Position     float64 `json:"position"`
		EventType    string  `json:"event_type"`
		Title        string  `json:"title"`
		Description  string  `json:"description"`
		Script       string  `json:"script"`
		CameraPreset string  `json:"camera_preset"`
		HoldBefore   float64 `json:"hold_before"`
		HoldAfter    float64 `json:"hold_after"`
		Enabled      *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]interface{}{}
	if req.Position != 0 {
		updates["position"] = req.Position
	}
	if req.EventType != "" {
		updates["event_type"] = req.EventType
	}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Script != "" {
		updates["script"] = req.Script
	}
	if req.CameraPreset != "" {
		updates["camera_preset"] = req.CameraPreset
	}
	if req.HoldBefore != 0 {
		updates["hold_before"] = req.HoldBefore
	}
	if req.HoldAfter != 0 {
		updates["hold_after"] = req.HoldAfter
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	h.DB.Model(&event).Updates(updates)
	c.JSON(http.StatusOK, event)
}

func (h *Handlers) DeleteEvent(c *gin.Context) {
	eid, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	h.DB.Delete(&route.StoryEvent{}, eid)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handlers) GetActivityPresets(c *gin.Context) {
	c.JSON(http.StatusOK, segment.Presets)
}

func (h *Handlers) GetTile(c *gin.Context) {
	provider := c.Param("provider")
	z, errZ := strconv.Atoi(c.Param("z"))
	x, errX := strconv.Atoi(c.Param("x"))
	y, errY := strconv.Atoi(strings.TrimSuffix(c.Param("y"), ".png"))
	if errZ != nil || errX != nil || errY != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tile coords"})
		return
	}
	data, err := h.TileCache.Get(provider, z, x, y)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "image/png", data)
}

func (h *Handlers) PreflightExport(c *gin.Context) {
	id, _ := parseInt(c.Param("id"))
	result, err := h.ExportSvc.Preflight(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handlers) PreloadTiles(c *gin.Context) {
	id, _ := parseInt(c.Param("id"))
	if err := h.ExportSvc.Preload(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "preload complete"})
}

func (h *Handlers) CreateExportTask(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	task, err := h.ExportSvc.CreateTask(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *Handlers) GetExportTask(c *gin.Context) {
	tid, ok := parseInt(c.Param("tid"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var task route.ExportTask
	if err := h.DB.First(&task, tid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handlers) CancelExportTask(c *gin.Context) {
	tid, ok := parseInt(c.Param("tid"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	h.DB.Model(&route.ExportTask{}).Where("id = ?", tid).Update("status", "CANCELLED")
	c.JSON(http.StatusOK, gin.H{"message": "cancelled"})
}

func (h *Handlers) RunExportTask(c *gin.Context) {
	tid, ok := parseInt(c.Param("tid"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	go h.ExportSvc.RunExport(tid)
	c.JSON(http.StatusOK, gin.H{"message": "export started"})
}

func (h *Handlers) UploadWebM(c *gin.Context) {
	tid, ok := parseInt(c.Param("tid"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	file, err := c.FormFile("video")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file"})
		return
	}
	dest := filepath.Join(h.DataDir, "exports", filepath.Base(file.Filename))
	if err := c.SaveUploadedFile(file, dest); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 目标分辨率（录制舞台尺寸），转码时归一化
	w := strings.TrimSpace(c.PostForm("width"))
	ht := strings.TrimSpace(c.PostForm("height"))
	go h.ExportSvc.RunExportWithWebM(tid, dest, w, ht)
	c.JSON(http.StatusCreated, gin.H{"message": "webm uploaded, export started"})
}

// SynthesizeSpeech 处理 GET /api/tts，将文本合成为 MP3 音频流。
func (h *Handlers) SynthesizeSpeech(c *gin.Context) {
	text := c.Query("text")
	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'text' parameter"})
		return
	}
	voice := c.Query("voice")

	c.Header("Content-Type", "audio/mpeg")

	// 优先使用 Edge-TTS
	err := h.TTS.Synthesize(text, voice, c.Writer)
	if err == nil {
		return // 成功
	}

	// TODO: 在这里实现降级逻辑
	// 1. 调用 MOSS-TTS-Nano 或其他本地/备用 TTS 服务
	// 2. 如果降级也失败，返回 500 错误
	// 3. 记录降级日志
	log.Printf("Edge-TTS failed: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "TTS service temporarily unavailable"})
}

func classifySegment(avgSlope float64) string {
	if avgSlope >= 8 {
		return "STEEP_CLIMB"
	}
	if avgSlope >= 3 {
		return "CLIMB"
	}
	if avgSlope <= -8 {
		return "STEEP_DESCENT"
	}
	if avgSlope <= -3 {
		return "DESCENT"
	}
	return "FLAT"
}

// ArrangeRoute 调用 LLM 生成演出配置（异步任务）：立即返回 GENERATING，
// 生成在后台进行（推理模型可能耗时数分钟），前端轮询 GET /config 获取状态
func (h *Handlers) ArrangeRoute(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		DurationSec int    `json:"durationSec"`
		Style       string `json:"style"`
		Voice       string `json:"voice"`
		RaceName    string `json:"raceName"`
		Category    string `json:"category"`
	}
	_ = c.ShouldBindJSON(&req)
	// 比赛名称与组别为必填参数（用于完赛收尾解说），不做自动提取
	if strings.TrimSpace(req.RaceName) == "" || strings.TrimSpace(req.Category) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "raceName 与 category 为必填参数"})
		return
	}
	if req.DurationSec <= 0 {
		req.DurationSec = 600
	}
	req.DurationSec = int(math.Max(60, math.Min(1800, float64(req.DurationSec))))
	if strings.TrimSpace(req.Style) == "" {
		req.Style = "专业越野解说，热血但不夸张"
	}
	if !llm.ValidVoice(req.Voice) {
		req.Voice = "zh-CN-XiaoxiaoNeural"
	}

	if _, err := h.RouteSvc.GetByID(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	h.setShowStatus(uint(id), "GENERATING", "")
	go h.generateShowConfig(uint(id), req.DurationSec, strings.TrimSpace(req.Style), req.Voice,
		strings.TrimSpace(req.RaceName), strings.TrimSpace(req.Category))
	c.JSON(http.StatusAccepted, gin.H{"status": "GENERATING"})
}

func (h *Handlers) setShowStatus(routeID uint, status string, errMsg string) {
	var sc route.RouteShowConfig
	isNew := h.DB.Where("route_id = ?", routeID).First(&sc).Error != nil
	sc.RouteID = routeID
	sc.Status = status
	sc.ErrorMessage = errMsg
	if isNew {
		h.DB.Create(&sc)
	} else {
		h.DB.Model(&sc).Updates(map[string]interface{}{"status": status, "error_message": errMsg, "updated_at": time.Now()})
	}
}

func (h *Handlers) generateShowConfig(routeID uint, durationSec int, style, voice, raceName, category string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("generateShowConfig panic: %v", r)
			h.setShowStatus(routeID, "FAILED", fmt.Sprintf("internal panic: %v", r))
		}
	}()

	r, err := h.RouteSvc.GetByID(routeID)
	if err != nil {
		h.setShowStatus(routeID, "FAILED", "route not found")
		return
	}
	points, err := h.RouteSvc.GetPoints(routeID)
	if err != nil || len(points) < 2 {
		h.setShowStatus(routeID, "FAILED", "no track points")
		return
	}
	var segments []route.RouteSegment
	h.DB.Where("route_id = ?", routeID).Order("start_distance ASC").Find(&segments)

	gpxData, err := os.ReadFile(r.GPXPath)
	if err != nil {
		h.setShowStatus(routeID, "FAILED", "read gpx: "+err.Error())
		return
	}
	parsed, err := gpx.Parse(gpxData)
	if err != nil {
		h.setShowStatus(routeID, "FAILED", "parse gpx: "+err.Error())
		return
	}

	totalKm := r.TotalDistance / 1000
	if totalKm <= 0 {
		totalKm = 1
	}
	hints := map[string]any{
		"durationSec": durationSec,
		"style":       style,
		"voice":       voice,
		"secPerKm":    math.Round(float64(durationSec)/totalKm*100) / 100,
		"raceName":    raceName,
		"category":    category,
		"minEvents":   len(parsed.Waypoints) + 2,
	}
	payload := llm.BuildPayload(r, points, segments, parsed.Waypoints, hints)
	userJSON, _ := json.Marshal(payload)
	system := llm.LoadSystemPrompt(h.DataDir)

	client := llm.NewClient()
	content, err := client.Chat(system, string(userJSON))
	if err == nil {
		var cfg *llm.ShowConfig
		cfg, err = llm.ParseAndClamp(content, totalKm)
		if err == nil {
			// 保证每个途经点都有事件（LLM 遗漏时用真实数据补齐）
			llm.EnsureWaypointEvents(cfg, llm.WaypointsAtKm(parsed.Waypoints, points))
		}
		if err != nil {
			// 带错误说明重试一次
			retry := string(userJSON) + "\n\n上次输出存在以下问题，请修正后重新输出完整 JSON：" + err.Error()
			content, err = client.Chat(system, retry)
			if err == nil {
				cfg, err = llm.ParseAndClamp(content, totalKm)
				if err == nil {
					llm.EnsureWaypointEvents(cfg, llm.WaypointsAtKm(parsed.Waypoints, points))
				}
			}
		}
		if err == nil {
			raw, _ := json.Marshal(cfg)
			var sc route.RouteShowConfig
			isNew := h.DB.Where("route_id = ?", routeID).First(&sc).Error != nil
			sc.RouteID = routeID
			sc.Status = "READY"
			sc.Config = string(raw)
			sc.ErrorMessage = ""
			if isNew {
				h.DB.Create(&sc)
			} else {
				h.DB.Model(&sc).Updates(map[string]interface{}{
					"status": "READY", "config": string(raw), "error_message": "", "updated_at": time.Now(),
				})
			}
			log.Printf("generateShowConfig route=%d 完成，%d 个事件", routeID, len(cfg.Events))
			return
		}
	}
	log.Printf("generateShowConfig route=%d 失败: %v", routeID, err)
	h.setShowStatus(routeID, "FAILED", err.Error())
}

func (h *Handlers) saveShowConfig(routeID uint, raw string) {
	var sc route.RouteShowConfig
	isNew := h.DB.Where("route_id = ?", routeID).First(&sc).Error != nil
	sc.RouteID = routeID
	sc.Status = "READY"
	sc.Config = raw
	sc.ErrorMessage = ""
	if isNew {
		h.DB.Create(&sc)
	} else {
		h.DB.Model(&sc).Updates(map[string]interface{}{"status": "READY", "config": raw, "updated_at": time.Now()})
	}
}

// GetShowConfig 返回演出配置及生成状态
func (h *Handlers) GetShowConfig(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var sc route.RouteShowConfig
	if err := h.DB.Where("route_id = ?", id).First(&sc).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "NONE", "config": nil, "errorMessage": nil})
		return
	}
	var cfg any
	if sc.Status == "READY" && sc.Config != "" {
		json.Unmarshal([]byte(sc.Config), &cfg)
	}
	c.JSON(http.StatusOK, gin.H{
		"status":       sc.Status,
		"config":       cfg,
		"errorMessage": sc.ErrorMessage,
		"updatedAt":    sc.UpdatedAt,
	})
}

// SaveShowConfig 手动保存演出配置
func (h *Handlers) SaveShowConfig(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body failed"})
		return
	}
	var cfg llm.ShowConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config json"})
		return
	}
	totalKm := 1.0
	var r route.Route
	if err := h.DB.First(&r, id).Error; err == nil {
		totalKm = r.TotalDistance / 1000
	}
	fixed, err := llm.ParseAndClamp(string(body), totalKm)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	raw, _ := json.Marshal(fixed)
	h.saveShowConfig(uint(id), string(raw))
	c.JSON(http.StatusOK, fixed)
}

// SynthesizeEventTTS 合成事件解说音频并落盘，返回可播放 URL 与估算时长
func (h *Handlers) SynthesizeEventTTS(c *gin.Context) {
	if _, ok := parseInt(c.Param("id")); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Text  string `json:"text"`
		Voice string `json:"voice"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing text"})
		return
	}
	if !llm.ValidVoice(req.Voice) {
		req.Voice = "zh-CN-XiaoxiaoNeural"
	}
	sum := sha1.Sum([]byte(req.Voice + "|" + strings.TrimSpace(req.Text)))
	hash := hex.EncodeToString(sum[:])[:16]
	audioDir := filepath.Join(h.DataDir, "audio")
	os.MkdirAll(audioDir, 0755)
	path := filepath.Join(audioDir, hash+".mp3")
	url := "/api/audio/" + hash + ".mp3"

	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "create file: " + err.Error()})
			return
		}
		err = h.TTS.Synthesize(req.Text, req.Voice, f)
		f.Close()
		if err != nil {
			os.Remove(path)
			c.JSON(http.StatusBadGateway, gin.H{"error": "TTS: " + err.Error()})
			return
		}
	}
	st, err := os.Stat(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Edge-TTS MP3 约 48kbps：按文件大小估算时长，前端可用 audio 元素修正
	durationSec := math.Round(float64(st.Size())*8/48000*10) / 10
	c.JSON(http.StatusOK, gin.H{"url": url, "durationSec": durationSec, "cached": true})
}

func parseInt(s string) (uint, bool) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil || v == 0 {
		return 0, false
	}
	return uint(v), true
}
