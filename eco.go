// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eco // import "github.com/sbinet-lpc/eco"

//go:generate brio-gen -p github.com/sbinet-lpc/eco -t Mission,Location -o gen_brio.go

import (
	"fmt"
	"time"

	"golang.org/x/xerrors"
)

type Mission struct {
	ID int32 `json:"id"`

	Date  time.Time `json:"date"`
	Start Location  `json:"start"`
	Dest  Location  `json:"dest"`
	Dist  float64   `json:"dist"`
	Trans TransID   `json:"transport_id"`
}

func (m Mission) String() string {
	return fmt.Sprintf("eco.Mission{id=%v %v dest=%q dist=%vkm trans=%v}",
		m.ID,
		m.Date.Format("2006-01-02"),
		m.Dest.Name, int64(m.Dist)/1000, m.Trans,
	)
}

type Location struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
}

type TransID byte

// List of transport IDs, sorted by cost.
const (
	Unknown TransID = iota
	Bike
	Tramway
	Train
	Bus
	Passenger
	Car
	Plane
)

func (tid TransID) String() string {
	switch tid {
	case Plane:
		return "plane"
	case Bus:
		return "bus"
	case Tramway:
		return "tramway"
	case Train:
		return "train"
	case Car:
		return "car"
	case Passenger:
		return "passenger"
	case Bike:
		return "bike"
	}
	panic(xerrors.Errorf("unknown transport ID %d", int(tid)))
}

// List of all known TransIDs
var TransIDs = []TransID{
	Bike,
	Tramway,
	Train,
	Bus,
	Passenger,
	Car,
	Plane,
}

// CostLess returns whether a is costing less than b in terms of CO2.
func CostLess(a, b TransID) bool {
	if a < b {
		return true
	}
	return false
}
