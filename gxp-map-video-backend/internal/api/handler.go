package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"gxp-map-video-backend/internal/export"
	"gxp-map-video-backend/internal/route"
	"gxp-map-video-backend/internal/segment"
	"gxp-map-video-backend/internal/tile"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handlers struct {
	DB        *gorm.DB
	RouteSvc  *route.Service
	Segmenter *segment.Segmenter
	TileCache *tile.Cache
	ExportSvc *export.Service
	DataDir   string
}

func RegisterRoutes(r *gin.Engine, db *gorm.DB, tileCache *tile.Cache, dataDir string) {
	h := &Handlers{
		DB:        db,
		RouteSvc:  route.NewService(route.NewDB(db)),
		Segmenter: segment.New(),
		TileCache: tileCache,
		ExportSvc: export.New(db, tileCache, dataDir),
		DataDir:   dataDir,
	}

	api := r.Group("/api")
	{
		routes := api.Group("/routes")
		{
			routes.POST("", h.UploadGPX)
			routes.GET("", h.ListRoutes)
			routes.GET("/:id", h.GetRoute)
			routes.GET("/:id/points", h.GetRoutePoints)
			routes.DELETE("/:id", h.DeleteRoute)
			routes.POST("/:id/analyze", h.AnalyzeRoute)
			routes.GET("/:id/segments", h.GetSegments)
			routes.PUT("/segments/:sid", h.UpdateSegment)
			routes.POST("/:id/segments/regenerate", h.RegenerateSegments)
			routes.POST("/:id/events", h.CreateEvent)
			routes.GET("/:id/events", h.GetEvents)
			routes.PUT("/events/:eid", h.UpdateEvent)
			routes.DELETE("/events/:eid", h.DeleteEvent)
		}
		config := api.Group("/config")
		{
			config.GET("/activity-presets", h.GetActivityPresets)
		}
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
	buf := make([]byte, file.Size)
	if _, err := data.Read(buf); err != nil {
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
	var segs []route.RouteSegment
	h.DB.Where("route_id = ?", id).Order("start_distance ASC").Find(&segs)
	c.JSON(http.StatusOK, segs)
}

func (h *Handlers) UpdateSegment(c *gin.Context) {
	sid, ok := parseInt(c.Param("sid"))
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
		Type       string `json:"type"`
		Commentary string `json:"commentary"`
		Enabled    *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]interface{}{}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Commentary != "" {
		updates["commentary"] = req.Commentary
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	h.DB.Model(&seg).Updates(updates)
	c.JSON(http.StatusOK, seg)
}

func (h *Handlers) RegenerateSegments(c *gin.Context) {
	h.AnalyzeRoute(c)
}

func (h *Handlers) CreateEvent(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Position     float64 `json:"position" binding:"required"`
		EventType    string  `json:"event_type" binding:"required"`
		Title        string  `json:"title"`
		Description  string  `json:"description"`
		Script       string  `json:"script"`
		CameraPreset string  `json:"camera_preset"`
		HoldBefore   float64 `json:"hold_before"`
		HoldAfter    float64 `json:"hold_after"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	event := route.StoryEvent{
		RouteID:      id,
		Position:     req.Position,
		EventType:    req.EventType,
		Title:        req.Title,
		Description:  req.Description,
		Script:       req.Script,
		CameraPreset: req.CameraPreset,
		HoldBefore:   req.HoldBefore,
		HoldAfter:    req.HoldAfter,
	}
	h.DB.Create(&event)
	c.JSON(http.StatusCreated, event)
}

func (h *Handlers) GetEvents(c *gin.Context) {
	id, ok := parseInt(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var events []route.StoryEvent
	h.DB.Where("route_id = ?", id).Order("position ASC").Find(&events)
	c.JSON(http.StatusOK, events)
}

func (h *Handlers) UpdateEvent(c *gin.Context) {
	eid, ok := parseInt(c.Param("eid"))
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
	eid, ok := parseInt(c.Param("eid"))
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
	z, _ := strconv.Atoi(c.Param("z"))
	x, _ := strconv.Atoi(c.Param("x"))
	y, _ := strconv.Atoi(c.Param("y"))
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
	go h.ExportSvc.RunExportWithWebM(tid, dest)
	c.JSON(http.StatusCreated, gin.H{"message": "webm uploaded, export started"})
}

func parseInt(s string) (uint, bool) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil || v == 0 {
		return 0, false
	}
	return uint(v), true
}
