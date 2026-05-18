package types

import (
	"math"
)

type GeoLocation struct {
	Latitude  float64
	Longitude float64
}
type Distance float64

func (d Distance) Meters() float64 {
	return float64(d)
}
func (d Distance) Kilometers() float64 {
	return float64(d) / 1000
}

func (g GeoLocation) DistanceTo(other GeoLocation) Distance {
	const R = 6371000 // Radius of the Earth in meters
	lat1Rad := g.Latitude * math.Pi / 180
	lat2Rad := other.Latitude * math.Pi / 180
	deltaLatRad := (other.Latitude - g.Latitude) * math.Pi / 180
	deltaLonRad := (other.Longitude - g.Longitude) * math.Pi / 180

	a := math.Sin(deltaLatRad/2)*math.Sin(deltaLatRad/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLonRad/2)*math.Sin(deltaLonRad/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return Distance(R * c)
}
func (g GeoLocation) IsValid() bool {
	return g.Latitude >= -90 && g.Latitude <= 90 && g.Longitude >= -180 && g.Longitude <= 180
}
