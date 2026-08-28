package tile

import "math"

// LatLngToTile converts lat/lng to tile coordinates at the given zoom level.
func LatLngToTile(lat, lng float64, zoom int) (int, int) {
	latRad := lat * math.Pi / 180
	n := math.Pow(2, float64(zoom))
	x := int((lng + 180) / 360 * n)
	y := int((1 - math.Log(math.Tan(latRad)+1/math.Cos(latRad))/math.Pi) / 2 * n)
	if y < 0 {
		y = 0
	}
	if y >= int(n) {
		y = int(n) - 1
	}
	if x < 0 {
		x = 0
	}
	if x >= int(n) {
		x = int(n) - 1
	}
	return x, y
}

// BoundingBoxToTiles returns all tile coordinates in a bounding box at the given zoom level.
func BoundingBoxToTiles(minLat, minLng, maxLat, maxLng float64, zoom int) [][3]int {
	x1, y1 := LatLngToTile(maxLat, minLng, zoom) // top-left
	x2, y2 := LatLngToTile(minLat, maxLng, zoom) // bottom-right
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	var tiles [][3]int
	for yy := y1; yy <= y2; yy++ {
		for xx := x1; xx <= x2; xx++ {
			tiles = append(tiles, [3]int{zoom, xx, yy})
		}
	}
	return tiles
}
