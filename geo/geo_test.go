package geo

import (
	"math"
	"testing"
)

func TestHaversine(t *testing.T) {
	for _, tt := range []struct {
		pt1, pt2 Point
		want     float64
	}{
		{
			pt1:  Point{},
			pt2:  Point{},
			want: 0,
		},
		{
			pt1:  Point{50, 5},
			pt2:  Point{58, 3},
			want: 899000,
		},
		{
			pt1:  Point{50.0359, 5.4253},
			pt2:  Point{58.3838, 3.0412},
			want: 940000,
		},
		{
			pt1:  Point{38.898556, -77.037852},
			pt2:  Point{38.897147, -77.043934},
			want: 549,
		},
		{
			pt1:  Point{45.7774551, 3.0819427}, // Clermont-Ferrand, France
			pt2:  Point{48.8566101, 2.3514992}, // Paris, France
			want: 346000,
		},
		{
			pt1:  Point{45.7774551, 3.0819427}, // Clermont-Ferrand, France
			pt2:  Point{45.7774551, 3.0819427}, // Clermont-Ferrand, France
			want: 0,
		},
		{
			pt1:  Point{45.7774551, 3.0819427}, // Clermont-Ferrand, France
			pt2:  Point{46.2334715, 6.0555674}, // CERN, Switzerland
			want: 235000,
		},
		{
			pt1:  Point{45.7774551, 3.0819427},        // Clermont-Ferrand, France
			pt2:  Point{45.6963425, 4.73594802991681}, // Lyon, France
			want: 129000,
		},
	} {
		t.Run("", func(t *testing.T) {
			got := Haversine(tt.pt1, tt.pt2)
			if !approxEqual(got, tt.want) {
				t.Fatalf("invalid haversine distance: got=%v, want=%v", got, tt.want)
			}
		})
	}
}

func approxEqual(a, b float64) bool {
	// require only km-level precision.
	return math.Abs(a-b) <= 1000
}
