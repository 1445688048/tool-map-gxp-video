package main

import (
	"log"
	"os"
	"path/filepath"

	"gxp-map-video-backend/internal/api"
	"gxp-map-video-backend/internal/db"
	"gxp-map-video-backend/internal/tile"

	"github.com/gin-gonic/gin"
)

func main() {
	dataDir := envOr("DATA_DIR", "data")
	port := envOr("PORT", "8080")

	// Create directories
	for _, dir := range []string{filepath.Join(dataDir, "uploads"), filepath.Join(dataDir, "tile_cache"), filepath.Join(dataDir, "audio")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("create dir %s: %v", dir, err)
		}
	}

	// Init DB
	dbPath := filepath.Join(dataDir, "app.db")
	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer database.Close()

	// Init tile cache
	tileCache := tile.NewCache(filepath.Join(dataDir, "tile_cache"))

	// Setup router
	r := gin.Default()

	// API routes
	api.RegisterRoutes(r, database.DB, tileCache, dataDir)

	log.Printf("server starting on :%s", port)
	log.Fatal(r.Run(":" + port))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
