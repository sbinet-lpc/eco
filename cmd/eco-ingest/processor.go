// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main // import "github.com/sbinet-lpc/eco/cmd/eco-ingest"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/sbinet-lpc/eco"
	"github.com/sbinet-lpc/eco/geo"
	"github.com/sbinet-lpc/eco/osm"
	"golang.org/x/xerrors"
)

type processor struct {
	osm      *osm.Client
	fixups   map[int32][]string // mission-id -> cleaned-up destination triplet
	missions []eco.Mission
	stats    *eco.Stats
}

func newProcessor(name string) (*processor, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, xerrors.Errorf("could not open fixups file %q: %+v", name, err)
	}
	defer f.Close()

	type fixup struct {
		ID   int32    `json:"id"`
		Dest []string `json:"dest"`
	}
	var raw []fixup
	err = json.NewDecoder(f).Decode(&raw)
	if err != nil {
		return nil, xerrors.Errorf("could not decode fixups file %q: %+v", name, err)
	}

	db := make(map[int32][]string, len(raw))
	for _, v := range raw {
		db[v.ID] = v.Dest
	}

	return &processor{
		osm: &osm.Client{
			UserAgent:       osm.UserAgent,
			AcceptLanguages: []string{"fr", "en"},
			AddressDetails:  true,
		},
		fixups: db,
		stats:  eco.NewStats(),
	}, nil
}

func (proc *processor) Process(raw Mission) {
	if !raw.isValid() {
		return
	}

	toks := proc.dest(raw)

	for i, tok := range toks {
		toks[i] = strings.Title(strings.ToLower(strings.TrimSpace(tok)))
		if toks[i] == "" {
			log.Printf("mission=%d destination=%s", raw.ID, raw.Destination)
			log.Fatalf("invalid empty token (n=%d): %q", i, toks)
		}
	}

	query := fmt.Sprintf("%s,%s", toks[1], toks[2])
	locs, err := proc.osm.Search(query)
	if err != nil {
		log.Printf("mission=%d destination=%s", raw.ID, raw.Destination)
		log.Fatalf("could not find destination for %q: %+v", query, err)
	}
	if len(locs) == 0 {
		log.Printf("mission=%d destination=%s", raw.ID, raw.Destination)
		log.Fatalf("could not find destination for %q", query)
	}
	if *dbgFlag {
		log.Printf("dest: %#v", locs)
	}

	loc := locs[0]
	lat, err := strconv.ParseFloat(loc.Lat, 64)
	if err != nil {
		log.Fatalf("could not parse lattitude: %+v", err)
	}
	lng, err := strconv.ParseFloat(loc.Lng, 64)
	if err != nil {
		log.Fatalf("could not parse longitude: %+v", err)
	}

	m := eco.Mission{
		ID:   raw.ID,
		Date: raw.Outbound.Date.UTC(),
		Start: eco.Location{
			Name: "Clermont-Ferrand",
			Lat:  clermont.Lat,
			Lng:  clermont.Lng,
		},
		Dest: eco.Location{
			Name: loc.DisplayName,
			Lat:  lat,
			Lng:  lng,
		},
		Dist:  2 * geo.Haversine(geo.Point{Lat: lat, Lng: lng}, clermont),
		Trans: raw.TransID(),
	}
	if m.Dist == 0 {
		// probably a Clermont-Fd intra-muros mission
		// add an ad-hoc estimation of 5km
		m.Dist = 5000
	}

	log.Printf("%v", m)

	proc.missions = append(proc.missions, m)
	proc.stats.Add(m)
}

func (proc *processor) dest(m Mission) []string {
	if dest, ok := proc.fixups[m.ID]; ok {
		return dest
	}

	return strings.Split(m.Destination, "///")
}

func (proc *processor) upload(addr string) error {
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}
	url := fmt.Sprintf("http://%s/api/update-db", addr)
	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(proc.missions)
	if err != nil {
		return xerrors.Errorf("could not encode missions to JSON: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return xerrors.Errorf("could not create POST request to eco-srv: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return xerrors.Errorf("could not send POST request to eco-srv: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return xerrors.Errorf("received an invalid status from eco-srv: %s (code=%d)",
			resp.Status, resp.StatusCode,
		)
	}

	log.Printf("uploaded %d mission(s)", len(proc.missions))

	return nil
}
