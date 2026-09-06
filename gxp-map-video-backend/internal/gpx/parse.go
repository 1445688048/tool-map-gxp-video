package gpx

import (
	"encoding/xml"
	"fmt"
	"math"
	"time"

	"gxp-map-video-backend/internal/tile"
)

type RawPoint struct {
	Lat, Lng, Ele float64
	Time          *time.Time
}

// Waypoint 是 GPX 中的航点（<wpt>），如补给点、观景台标记
type Waypoint struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Ele  float64 `json:"ele"`
}

type ParseResult struct {
	Points         []RawPoint
	Waypoints      []Waypoint
	TotalDistance  float64
	TotalAscent    float64
	TotalDescent   float64
	MinElevation   float64
	MaxElevation   float64
	Duration       *float64
}

type gpxRoute struct {
	Name     string     `xml:"name"`
	Points   []xmlPoint `xml:"rtept"`
}

type gpxDocument struct {
	XMLName   xml.Name   `xml:"gpx"`
	Tracks    []track    `xml:"trk"`
	Routes    []gpxRoute `xml:"rte"`
	Waypoints []gpxWpt   `xml:"wpt"`
}

type track struct {
	Name     string `xml:"name"`
	Segments []seg  `xml:"trkseg"`
}

type seg struct {
	Points []xmlPoint `xml:"trkpt"`
}

type xmlPoint struct {
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Ele  float64 `xml:"ele"`
	Time string  `xml:"time"`
}

type gpxWpt struct {
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Ele  float64 `xml:"ele"`
	Name string  `xml:"name"`
}

func Parse(data []byte) (*ParseResult, error) {
	var doc gpxDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse GPX XML: %w", err)
	}

	var allPoints []RawPoint
	for _, trk := range doc.Tracks {
		for _, seg := range trk.Segments {
			for _, xp := range seg.Points {
				p := RawPoint{Lat: xp.Lat, Lng: xp.Lon, Ele: xp.Ele}
				if xp.Time != "" {
					t, err := time.Parse(time.RFC3339, xp.Time)
					if err == nil {
						p.Time = &t
					}
				}
				allPoints = append(allPoints, p)
			}
		}
	}
	// 无轨迹时回退到 <rte> 路线点（规划路径，同为有序经纬度序列）
	if len(allPoints) == 0 {
		for _, rte := range doc.Routes {
			for _, xp := range rte.Points {
				allPoints = append(allPoints, RawPoint{Lat: xp.Lat, Lng: xp.Lon, Ele: xp.Ele})
			}
		}
	}

	if len(allPoints) == 0 {
		return nil, fmt.Errorf("GPX file contains no track points")
	}

	result := computeStats(allPoints)
	var wpts []Waypoint
	for _, w := range doc.Waypoints {
		wpts = append(wpts, Waypoint{Name: w.Name, Lat: w.Lat, Lng: w.Lon, Ele: w.Ele})
	}
	result.Waypoints = wpts
	return result, nil
}

func computeStats(points []RawPoint) *ParseResult {
	if len(points) == 0 {
		return &ParseResult{}
	}
	result := &ParseResult{
		Points:       points,
		MinElevation: points[0].Ele,
		MaxElevation: points[0].Ele,
	}

	hasElevation := false
	for _, p := range points {
		if p.Ele != 0 {
			hasElevation = true
			break
		}
	}

	var cumDist, ascent, descent float64
	for i := 1; i < len(points); i++ {
		dist := tile.Haversine(points[i-1].Lat, points[i-1].Lng, points[i].Lat, points[i].Lng)
		cumDist += dist
		if hasElevation {
			d := points[i].Ele - points[i-1].Ele
			if d > 0 {
				ascent += d
			} else {
				descent += -d
			}
			if points[i].Ele < result.MinElevation {
				result.MinElevation = points[i].Ele
			}
			if points[i].Ele > result.MaxElevation {
				result.MaxElevation = points[i].Ele
			}
		}
	}

	result.TotalDistance = math.Round(cumDist*100) / 100
	result.TotalAscent = math.Round(ascent)
	result.TotalDescent = math.Round(descent)
	result.MinElevation = math.Round(result.MinElevation*10) / 10
	result.MaxElevation = math.Round(result.MaxElevation*10) / 10

	if len(points) >= 2 && points[0].Time != nil && points[len(points)-1].Time != nil {
		d := points[len(points)-1].Time.Sub(*points[0].Time).Seconds()
		if d > 0 {
			result.Duration = &d
		}
	}
	return result
}
