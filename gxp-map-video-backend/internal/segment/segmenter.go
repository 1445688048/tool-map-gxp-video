package segment

import (
	"math"

	"gxp-map-video-backend/internal/route"
)

// Type represents segment classification
type Type string

const (
	Flat          Type = "FLAT"
	Climb         Type = "CLIMB"
	SteepClimb    Type = "STEEP_CLIMB"
	Descent       Type = "DESCENT"
	SteepDescent  Type = "STEEP_DESCENT"
)

// Preset defines threshold configuration for an activity type
type Preset struct {
	Name                 string
	FlatRange            [2]float64
	ClimbRange           [2]float64
	SteepClimbThreshold  float64
	DescentRange         [2]float64
	SteepDescentThresh   float64
	MinSegmentDistance   float64
}

var Presets = map[string]Preset{
	"trail_running": {
		Name:                 "越野跑",
		FlatRange:            [2]float64{-3, 3},
		ClimbRange:           [2]float64{3, 8},
		SteepClimbThreshold:  8,
		DescentRange:         [2]float64{-8, -3},
		SteepDescentThresh:   -8,
		MinSegmentDistance:   200,
	},
	"hiking": {
		Name:                 "徒步登山",
		FlatRange:            [2]float64{-5, 5},
		ClimbRange:           [2]float64{5, 12},
		SteepClimbThreshold:  12,
		DescentRange:         [2]float64{-12, -5},
		SteepDescentThresh:   -12,
		MinSegmentDistance:   500,
	},
	"mtb": {
		Name:                 "山地骑行",
		FlatRange:            [2]float64{-4, 4},
		ClimbRange:           [2]float64{4, 10},
		SteepClimbThreshold:  10,
		DescentRange:         [2]float64{-10, -4},
		SteepDescentThresh:   -10,
		MinSegmentDistance:   300,
	},
}

// window represents a classified group of points
type window struct {
	startIdx int
	endIdx   int
	avgSlope float64
	maxSlope float64
	segType  Type
}

// Segmenter analyzes route points and creates segments
type Segmenter struct{}

func New() *Segmenter {
	return &Segmenter{}
}

// Segment creates route segments from track points using the given preset
func (s *Segmenter) Segment(points []route.TrackPoint, presetName string) ([]route.RouteSegment, error) {
	preset, ok := Presets[presetName]
	if !ok {
		preset = Presets["hiking"]
	}

	if len(points) == 0 {
		return nil, nil
	}

	// Group points into windows based on min segment distance
	windowSize := int(preset.MinSegmentDistance / 50)
	if windowSize < 3 {
		windowSize = 3
	}
	if windowSize > 50 {
		windowSize = 50
	}

	var windows []window
	for i := 0; i < len(points); i += windowSize {
		end := i + windowSize
		if end > len(points) {
			end = len(points)
		}

		startEle := points[i].Elevation
		endEle := points[end-1].Elevation
		dist := points[end-1].Distance - points[i].Distance

		avgSlope := 0.0
		maxSlope := 0.0
		if dist > 0 {
			avgSlope = (endEle - startEle) / dist * 100
		}
		// Find max slope in window
		for j := i; j < end; j++ {
			if j > i && points[j].Slope > maxSlope {
				maxSlope = points[j].Slope
			}
		}

		avgSlope = math.Round(avgSlope*100) / 100
		maxSlope = math.Round(maxSlope*100) / 100

		segType := classify(avgSlope, preset)
		windows = append(windows, window{
			startIdx: i,
			endIdx:   end - 1,
			avgSlope: avgSlope,
			maxSlope: maxSlope,
			segType:  segType,
		})
	}

	// Merge adjacent windows of same type
	var segments []route.RouteSegment
	var current *window

	for i := range windows {
		w := &windows[i]
		if current == nil {
			current = w
			continue
		}

		if w.segType == current.segType {
			// Merge
			current.endIdx = w.endIdx
			curDist := float64(current.endIdx - current.startIdx + 1)
			newDist := float64(w.endIdx - w.startIdx + 1)
			current.avgSlope = (current.avgSlope*curDist + w.avgSlope*newDist) / (curDist + newDist)
			if w.maxSlope > current.maxSlope {
				current.maxSlope = w.maxSlope
			}
		} else {
			segments = append(segments, toSegment(points, current))
			current = w
		}
	}

	if current != nil {
		segments = append(segments, toSegment(points, current))
	}

	// Filter out very short segments, merge into previous
	var filtered []route.RouteSegment
	for i := range segments {
		seg := &segments[i]
		if seg.Distance >= preset.MinSegmentDistance*0.5 {
			filtered = append(filtered, *seg)
		} else if len(filtered) > 0 {
			last := &filtered[len(filtered)-1]
			last.EndIndex = seg.EndIndex
			last.Distance += seg.Distance
			last.ElevationGain += seg.ElevationGain
			last.ElevationLoss += seg.ElevationLoss
			if seg.MaxSlope > last.MaxSlope {
				last.MaxSlope = seg.MaxSlope
			}
		}
	}

	return filtered, nil
}

func classify(avgSlope float64, preset Preset) Type {
	if avgSlope >= preset.SteepClimbThreshold {
		return SteepClimb
	}
	if avgSlope >= preset.ClimbRange[0] {
		return Climb
	}
	if avgSlope <= preset.SteepDescentThresh {
		return SteepDescent
	}
	if avgSlope <= preset.DescentRange[0] {
		return Descent
	}
	return Flat
}

func toSegment(points []route.TrackPoint, w *window) route.RouteSegment {
	start := points[w.startIdx]
	end := points[w.endIdx]
	return route.RouteSegment{
		StartIndex:     w.startIdx,
		EndIndex:       w.endIdx,
		StartDistance:  start.Distance,
		EndDistance:    end.Distance,
		Distance:       end.Distance - start.Distance,
		StartElevation: start.Elevation,
		EndElevation:   end.Elevation,
		ElevationGain:  math.Max(0, end.Elevation-start.Elevation),
		ElevationLoss:  math.Max(0, start.Elevation-end.Elevation),
		AverageSlope:   w.avgSlope,
		MaxSlope:       w.maxSlope,
		Type:           string(w.segType),
	}
}
