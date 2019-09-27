// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eco_test // import "github.com/sbinet-lpc/eco"

import (
	"fmt"
	"testing"

	"github.com/sbinet-lpc/eco"
)

func TestCost(t *testing.T) {
	for _, tt := range []struct {
		a, b eco.TransID
		want bool
	}{
		{
			a: eco.Bike, b: eco.Bike,
			want: false,
		},
		{
			a: eco.Bus, b: eco.Train,
			want: false,
		},
		{
			a: eco.Bus, b: eco.Plane,
			want: true,
		},
		{
			a: eco.Bike, b: eco.Tramway,
			want: true,
		},
		{
			a: eco.Tramway, b: eco.Train,
			want: true,
		},
		{
			a: eco.Train, b: eco.Bus,
			want: true,
		},
		{
			a: eco.Bus, b: eco.Passenger,
			want: true,
		},
		{
			a: eco.Passenger, b: eco.Car,
			want: true,
		},
		{
			a: eco.Car, b: eco.Plane,
			want: true,
		},
		{
			a: eco.Plane, b: eco.Car,
			want: false,
		},
	} {
		t.Run(fmt.Sprintf("%s-%s", tt.a, tt.b), func(t *testing.T) {
			got := eco.CostLess(tt.a, tt.b)
			if got != tt.want {
				t.Fatalf("invalid cost: got=%v, want=%v", got, tt.want)
			}
		})
	}
}
