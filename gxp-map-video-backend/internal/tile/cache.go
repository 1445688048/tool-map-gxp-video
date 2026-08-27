package tile

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cache manages tile storage and retrieval
type Cache struct {
	baseDir string
	client  *http.Client
}

type tileProvider struct {
	url    string
	header map[string]string
}

var providers = map[string]tileProvider{
	"esri-satellite": {
		url: "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{x}/{y}",
		header: map[string]string{"User-Agent": "gxp-map-video/1.0"},
	},
	"terrain": {
		url: "https://s3.amazonaws.com/elevation-tiles-prod/terrarium/{z}/{x}/{y}.png",
		header: map[string]string{"User-Agent": "gxp-map-video/1.0"},
	},
}

func NewCache(baseDir string) *Cache {
	os.MkdirAll(baseDir, 0755)
	return &Cache{
		baseDir: baseDir,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Get serves a tile from cache or fetches from source
func (c *Cache) Get(provider string, z, x, y int) ([]byte, error) {
	providerInfo, ok := providers[provider]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	cachePath := filepath.Join(c.baseDir, provider, fmt.Sprintf("%d", z), fmt.Sprintf("%d", x), fmt.Sprintf("%d.png", y))

	// Check cache
	if data, err := os.ReadFile(cachePath); err == nil {
		return data, nil
	}

	// Fetch from source
	url := fmt.Sprintf(providerInfo.url, z, x, y)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range providerInfo.header {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("tile request failed: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Save to cache
	dir := filepath.Dir(cachePath)
	os.MkdirAll(dir, 0755)
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		// Don't fail if cache write fails
	}

	return data, nil
}

// CheckMissing returns tiles that are not in cache
func (c *Cache) CheckMissing(provider string, tiles [][3]int) ([][3]int, error) {
	var missing [][3]int
	for _, t := range tiles {
		path := filepath.Join(c.baseDir, provider, fmt.Sprintf("%d", t[0]), fmt.Sprintf("%d", t[1]), fmt.Sprintf("%d.png", t[2]))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, t)
		}
	}
	return missing, nil
}

// DownloadTiles downloads missing tiles in parallel
func (c *Cache) DownloadTiles(provider string, tiles [][3]int, concurrency int) error {
	if len(tiles) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(tiles))
	sem := make(chan struct{}, concurrency)

	for _, t := range tiles {
		wg.Add(1)
		sem <- struct{}{}
		go func(z, x, y int) {
			defer wg.Done()
			defer func() { <-sem }()

			data, err := c.Get(provider, z, x, y)
			if err != nil {
				errChan <- err
				return
			}
			_ = data
		}(t[0], t[1], t[2])
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to download %d/%d tiles", len(errs), len(tiles))
	}
	return nil
}

// BaseDir returns the cache base directory path
func (c *Cache) BaseDir() string {
	return c.baseDir
}
