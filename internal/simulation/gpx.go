// Package simulation provides GPX route parsing and live location simulation.
package simulation

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
)

// TrackPoint is a single lat/lon coordinate extracted from a GPX file.
type TrackPoint struct {
	Lat float64
	Lon float64
}

type gpxFile struct {
	XMLName   xml.Name     `xml:"gpx"`
	Tracks    []gpxTrack   `xml:"trk"`
	Routes    []gpxRoute   `xml:"rte"`
	Waypoints []gpxWaypoint `xml:"wpt"`
}

type gpxTrack struct {
	Segments []gpxSegment `xml:"trkseg"`
}

type gpxSegment struct {
	Points []gpxPoint `xml:"trkpt"`
}

type gpxRoute struct {
	Points []gpxPoint `xml:"rtept"`
}

type gpxWaypoint struct {
	Lat float64 `xml:"lat,attr"`
	Lon float64 `xml:"lon,attr"`
}

type gpxPoint struct {
	Lat float64 `xml:"lat,attr"`
	Lon float64 `xml:"lon,attr"`
}

// ParseGPX parses raw GPX XML bytes and returns an ordered list of track points.
// Prefers tracks → routes → waypoints.
func ParseGPX(data []byte) ([]TrackPoint, error) {
	var g gpxFile
	if err := xml.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("invalid GPX: %w", err)
	}

	var points []TrackPoint

	for _, trk := range g.Tracks {
		for _, seg := range trk.Segments {
			for _, p := range seg.Points {
				points = append(points, TrackPoint{Lat: p.Lat, Lon: p.Lon})
			}
		}
	}

	if len(points) == 0 {
		for _, rte := range g.Routes {
			for _, p := range rte.Points {
				points = append(points, TrackPoint{Lat: p.Lat, Lon: p.Lon})
			}
		}
	}

	if len(points) == 0 {
		for _, wpt := range g.Waypoints {
			points = append(points, TrackPoint{Lat: wpt.Lat, Lon: wpt.Lon})
		}
	}

	if len(points) < 2 {
		return nil, fmt.Errorf("GPX must contain at least 2 points")
	}

	return points, nil
}

// ParseGPXBase64 decodes a standard base64-encoded GPX and returns track points.
func ParseGPXBase64(encoded string) ([]TrackPoint, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	return ParseGPX(data)
}
