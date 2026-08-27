package route

import (
	"math"

	"gxp-map-video-backend/internal/gpx"
)

// Service handles route operations
type Service struct {
	db *DB
}

func NewService(db *DB) *Service {
	return &Service{db: db}
}

// CreateFromGPX parses a GPX file and creates a Route with TrackPoints
func (s *Service) CreateFromGPX(name, gpxPath string, data []byte) (*Route, error) {
	result, err := gpx.Parse(data)
	if err != nil {
		return nil, err
	}

	// Smooth elevation using moving average (window=5)
	smoothed := smoothElevation(result.Points, 5)

	// Build route
	route := &Route{
		Name:           name,
		GPXPath:        gpxPath,
		TotalDistance:  result.TotalDistance,
		TotalAscent:    result.TotalAscent,
		TotalDescent:   result.TotalDescent,
		MinElevation:   result.MinElevation,
		MaxElevation:   result.MaxElevation,
		StartLat:       result.Points[0].Lat,
		StartLng:       result.Points[0].Lng,
		EndLat:         result.Points[len(result.Points)-1].Lat,
		EndLng:         result.Points[len(result.Points)-1].Lng,
		PointCount:     len(result.Points),
	}

	// Build track points with distance and slope
	points := make([]TrackPoint, len(smoothed))
	var cumDist float64

	for i, p := range smoothed {
		points[i] = TrackPoint{
			RouteID:    0, // set after insert
			Index:      i,
			Latitude:   p.Lat,
			Longitude:  p.Lng,
			Elevation:  p.Ele,
			Distance:   math.Round(cumDist*1000) / 1000,
			Slope:      0, // computed below
			Time:       p.Time,
		}
		if i > 0 {
			prev := smoothed[i-1]
			dist := haversine(prev.Lat, prev.Lng, p.Lat, p.Lng)
			cumDist += dist
			points[i].Distance = math.Round(cumDist*1000) / 1000
			// slope = elevation_delta / horizontal_distance * 100
			elevDiff := p.Ele - prev.Ele
			if dist > 0 {
				points[i].Slope = math.Round(elevDiff/dist*100*100) / 100
			}
		}
	}

	// Save route
	if err := s.db.CreateRoute(route); err != nil {
		return nil, err
	}

	// Set route ID and save points in batches
	for i := range points {
		points[i].RouteID = route.ID
	}
	if err := s.db.BulkCreateTrackPoints(points); err != nil {
		return nil, err
	}

	// Refresh route with count
	s.db.DB.Model(route).Update("point_count", len(points))
	s.db.DB.First(route, route.ID)

	return route, nil
}

func (s *Service) GetAll() ([]Route, error) {
	var routes []Route
	if err := s.db.FindRoutes(&routes); err != nil {
		return nil, err
	}
	return routes, nil
}

func (s *Service) GetByID(id uint) (*Route, error) {
	return s.db.GetRoute(id)
}

func (s *Service) GetPoints(routeID uint) ([]TrackPoint, error) {
	return s.db.GetTrackPoints(routeID)
}

func (s *Service) Delete(id uint) error {
	return s.db.DeleteRoute(id)
}

// haversine same as in parse.go
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// smoothElevation applies a moving average to elevation values
func smoothElevation(points []gpx.RawPoint, window int) []gpx.RawPoint {
	if len(points) <= window {
		return points
	}

	half := window / 2
	result := make([]gpx.RawPoint, len(points))
	copy(result, points)

	for i := half; i < len(points)-half; i++ {
		sum := 0.0
		for j := i - half; j <= i+half; j++ {
			sum += points[j].Ele
		}
		result[i].Ele = math.Round(sum/float64(window)*10) / 10
	}

	return result
}
