// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main // import "github.com/sbinet-lpc/eco/cmd/eco-stats"

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/sbinet-lpc/eco"
)

func main() {
	log.SetPrefix("eco-stats: ")
	log.SetFlags(0)

	var (
		addrFlag      = flag.String("addr", ":8080", "[host]:port address of eco-srv")
		citiesFlag    = flag.Bool("cities", false, "display cities stats")
		countriesFlag = flag.Bool("countries", false, "display countries stats")
	)

	flag.Parse()

	addr := *addrFlag
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}

	log.Printf("querying %q...", addr)

	req, err := http.Get(fmt.Sprintf("http://%s/api/stats", addr))
	if err != nil {
		log.Fatalf("could not query stats: %+v", err)
	}
	defer req.Body.Close()

	var stats eco.Stats
	err = json.NewDecoder(req.Body).Decode(&stats)
	if err != nil {
		log.Fatalf("could not decode JSON stats: %+v", err)
	}

	log.Printf("missions:    %d", stats.Missions)
	log.Printf("time period: %v -> %s",
		stats.Start.Format("2006-01-02"),
		stats.Stop.Format("2006-01-02"),
	)

	type entry struct {
		name  string
		count int
	}

	sort := func(m map[string]int) []entry {
		vs := make([]entry, 0, len(m))
		for k := range m {
			vs = append(vs, entry{k, m[k]})
		}
		sort.Slice(vs, func(i, j int) bool {
			ii := vs[i]
			jj := vs[j]
			if ii.name == jj.name {
				return ii.count < jj.count
			}
			return ii.name < jj.name
		})
		return vs
	}

	cities := sort(stats.Cities)
	if *citiesFlag {
		log.Printf("=== cities ===")
		for _, v := range cities {
			log.Printf("%-10s %d", v.name, v.count)
		}
	}

	if *countriesFlag {
		countries := sort(stats.Countries)
		log.Printf("=== countries ===")
		for _, v := range countries {
			log.Printf("%-10s %d", v.name, v.count)
		}
	}

	tids := []eco.TransID{
		eco.Bike,
		eco.Tramway,
		eco.Train,
		eco.Bus,
		eco.Passenger,
		eco.Car,
		eco.Plane,
	}

	log.Printf("=== transport ===")
	for _, k := range tids {
		v := stats.TransIDs[k]
		log.Printf("%-10s %d", k, v)
	}

	log.Printf("=== distances ===")
	for _, k := range tids {
		v := stats.Dists[k]
		log.Printf("%-10s %d km", k, v)
	}
}
