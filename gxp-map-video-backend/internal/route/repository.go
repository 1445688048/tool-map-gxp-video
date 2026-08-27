package route

import (
	"gorm.io/gorm"
)

type DB struct {
	DB *gorm.DB
}

func NewDB(db *gorm.DB) *DB {
	return &DB{DB: db}
}

func (r *DB) CreateRoute(route *Route) error {
	return r.DB.Create(route).Error
}

func (r *DB) GetRoute(id uint) (*Route, error) {
	var route Route
	err := r.DB.First(&route, id).Error
	if err != nil {
		return nil, err
	}
	return &route, nil
}

func (r *DB) FindRoutes(out *[]Route) error {
	return r.DB.Find(out).Error
}

func (r *DB) DeleteRoute(id uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&TrackPoint{}, "route_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Delete(&RouteSegment{}, "route_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Delete(&StoryEvent{}, "route_id = ?", id).Error; err != nil {
			return err
		}
		return tx.Delete(&Route{}, id).Error
	})
}

func (r *DB) BulkCreateTrackPoints(points []TrackPoint) error {
	return r.DB.CreateInBatches(points, 500).Error
}

func (r *DB) GetTrackPoints(routeID uint) ([]TrackPoint, error) {
	var points []TrackPoint
	err := r.DB.Where("route_id = ?", routeID).Order("\"index\" ASC").Find(&points).Error
	return points, err
}
