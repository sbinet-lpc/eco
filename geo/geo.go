// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package geo // import "github.com/sbinet-lpc/eco/geo"

import "math"

type Point struct {
	Lat float64
	Lng float64
}

const deg2rad = math.Pi / 180.

// Haversine returns the distance in metres between 2 points, using
// the Haversine formula:
//  https://en.wikipedia.org/wiki/Haversine_formula
//
// Input points coordinates are assumed to be in degrees.
func Haversine(pt1, pt2 Point) float64 {
	const R = 6.371e6 // Earth radius in metres
	var (
		lat1 = pt1.Lat * deg2rad
		lat2 = pt2.Lat * deg2rad
		dLat = (pt2.Lat - pt1.Lat) * deg2rad
		dLng = (pt2.Lng - pt1.Lng) * deg2rad
	)

	a := hsin(dLat) + math.Cos(lat1)*math.Cos(lat2)*hsin(dLng)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

func hsin(theta float64) float64 {
	sin := math.Sin(0.5 * theta)
	return sin * sin
}
