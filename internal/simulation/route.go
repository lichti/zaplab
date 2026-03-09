package simulation

import "math"

const earthRadiusKm = 6371.0

// haversine returns the great-circle distance in km between two lat/lon points.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// bearing returns the initial bearing in degrees (0–360) from p1 to p2.
func bearing(lat1, lon1, lat2, lon2 float64) float64 {
	lat1R := lat1 * math.Pi / 180
	lat2R := lat2 * math.Pi / 180
	dLonR := (lon2 - lon1) * math.Pi / 180
	y := math.Sin(dLonR) * math.Cos(lat2R)
	x := math.Cos(lat1R)*math.Sin(lat2R) - math.Sin(lat1R)*math.Cos(lat2R)*math.Cos(dLonR)
	return math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)
}

// RoutePoint is an interpolated position on the route.
type RoutePoint struct {
	Lat      float64
	Lon      float64
	Bearing  uint32
	SpeedMps float32
}

// Route precomputes cumulative distances along a sequence of track points.
type Route struct {
	Points    []TrackPoint
	cumDist   []float64 // cumulative distance in km at each point index
	Total     float64   // total route length in km
}

// NewRoute builds a Route and precomputes cumulative distances.
func NewRoute(points []TrackPoint) *Route {
	cum := make([]float64, len(points))
	total := 0.0
	for i := 1; i < len(points); i++ {
		total += haversine(points[i-1].Lat, points[i-1].Lon, points[i].Lat, points[i].Lon)
		cum[i] = total
	}
	return &Route{Points: points, cumDist: cum, Total: total}
}

// PointAt returns the interpolated position at distKm along the route,
// calculated for the given speedKmh.
func (r *Route) PointAt(distKm, speedKmh float64) RoutePoint {
	if distKm <= 0 {
		b := bearing(r.Points[0].Lat, r.Points[0].Lon, r.Points[1].Lat, r.Points[1].Lon)
		return RoutePoint{Lat: r.Points[0].Lat, Lon: r.Points[0].Lon, Bearing: uint32(b), SpeedMps: float32(speedKmh / 3.6)}
	}
	n := len(r.Points)
	if distKm >= r.Total {
		return RoutePoint{Lat: r.Points[n-1].Lat, Lon: r.Points[n-1].Lon, Bearing: 0, SpeedMps: 0}
	}

	// Binary search for the segment [lo, hi] that contains distKm.
	lo, hi := 0, n-1
	for lo+1 < hi {
		mid := (lo + hi) / 2
		if r.cumDist[mid] <= distKm {
			lo = mid
		} else {
			hi = mid
		}
	}

	// Linear interpolation within the segment.
	segLen := r.cumDist[hi] - r.cumDist[lo]
	t := 0.0
	if segLen > 0 {
		t = (distKm - r.cumDist[lo]) / segLen
	}
	lat := r.Points[lo].Lat + t*(r.Points[hi].Lat-r.Points[lo].Lat)
	lon := r.Points[lo].Lon + t*(r.Points[hi].Lon-r.Points[lo].Lon)
	b := bearing(r.Points[lo].Lat, r.Points[lo].Lon, r.Points[hi].Lat, r.Points[hi].Lon)

	return RoutePoint{
		Lat:      lat,
		Lon:      lon,
		Bearing:  uint32(b),
		SpeedMps: float32(speedKmh / 3.6),
	}
}
