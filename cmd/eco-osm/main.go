// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command eco-osm queries the Nominatim service using OpenStreetMap data to locate a place.
package main

import (
	"flag"
	"log"
	"strconv"
	"strings"

	"github.com/sbinet-lpc/eco/geo"
	"github.com/sbinet-lpc/eco/osm"
)

func main() {
	log.SetPrefix("osm: ")
	log.SetFlags(0)

	var (
		addrFlag = flag.Bool("addr-details", false, "enable address details")
		langFlag = flag.String("lang", "", "comma-separated list of accepted language values (e.g. fr,en)")
	)

	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		log.Fatalf("missing query argument(s)")
	}

	query := strings.Join(flag.Args(), ",")

	cli := &osm.Client{
		UserAgent:      osm.UserAgent,
		AddressDetails: *addrFlag,
	}
	if *langFlag != "" {
		cli.AcceptLanguages = strings.Split(*langFlag, ",")
	}

	places, err := cli.Search(query)
	if err != nil {
		log.Fatalf("failed to query OpenStreetMap w/ %q: %+v", query, err)
	}
	if len(places) == 0 {
		log.Fatalf("could not find any location w/ %q", query)
	}

	clermont := geo.Point{Lat: 45.7774551, Lng: 3.0819427}
	lat, err := strconv.ParseFloat(places[0].Lat, 64)
	if err != nil {
		log.Fatalf("could not parse latitude: %+v", err)
	}
	lng, err := strconv.ParseFloat(places[0].Lng, 64)
	if err != nil {
		log.Fatalf("could not parse longitude: %+v", err)
	}

	dist := 2 * geo.Haversine(geo.Point{Lat: lat, Lng: lng}, clermont) / 1000
	log.Printf("main location: %v, %v -> %vkm", places[0].Lat, places[0].Lng, dist)

	for i, place := range places {
		log.Printf("loc[%d]: %#v", i, place)
	}
}
