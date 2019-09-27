// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command eco-osm queries the Nominatim service using OpenStreetMap data to locate a place.
package main

import (
	"flag"
	"log"
	"strings"

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

	log.Printf("main location: %v, %v", places[0].Lat, places[0].Lng)

	for i, place := range places {
		log.Printf("loc[%d]: %#v", i, place)
	}
}
