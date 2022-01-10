// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main // import "github.com/sbinet-lpc/eco/cmd/eco-ingest"

import (
	"database/sql"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/sbinet-lpc/eco"
	"github.com/sbinet-lpc/eco/geo"
	"github.com/sbinet-lpc/eco/osm"
	"go.etcd.io/bbolt"
)

type processor struct {
	osm      *osm.Client
	fixups   map[int32][]string // mission-id -> cleaned-up destination triplet
	missions []eco.Mission
	summ     *eco.Summary
}

func newProcessor(name string) (*processor, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("could not open fixups file %q: %w", name, err)
	}
	defer f.Close()

	type fixup struct {
		ID   int32    `json:"id"`
		Dest []string `json:"dest"`
	}
	var raw []fixup
	err = json.NewDecoder(f).Decode(&raw)
	if err != nil {
		return nil, fmt.Errorf("could not decode fixups file %q: %w", name, err)
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
		summ:   eco.NewSummary(),
	}, nil
}

func (proc *processor) Process(raw Mission) error {
	if !raw.isValid() {
		return nil
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
		return fmt.Errorf("could not find destination for %q: %w", query, err)
	}
	if len(locs) == 0 {
		log.Printf("mission=%d destination=%s", raw.ID, raw.Destination)
		return fmt.Errorf("could not find destination for %q", query)
	}
	if *dbgFlag {
		log.Printf("dest: %#v", locs)
	}

	loc := locs[0]
	lat, err := strconv.ParseFloat(loc.Lat, 64)
	if err != nil {
		return fmt.Errorf("could not parse lattitude: %w", err)
	}
	lng, err := strconv.ParseFloat(loc.Lng, 64)
	if err != nil {
		return fmt.Errorf("could not parse longitude: %w", err)
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
	proc.summ.Add(m)
	return nil
}

func (proc *processor) dest(m Mission) []string {
	if dest, ok := proc.fixups[m.ID]; ok {
		return dest
	}

	return strings.Split(m.Destination, "///")
}

func (proc *processor) upload(db *bbolt.DB) error {
	if len(proc.missions) == 0 {
		return nil
	}

	err := db.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(bucketEco)
		if bkt == nil {
			return fmt.Errorf("could not access %q bucket", bucketEco)
		}

		id := make([]byte, 4)
		for _, m := range proc.missions {
			binary.LittleEndian.PutUint32(id, uint32(m.ID))
			buf, err := m.MarshalBinary()
			if err != nil {
				return fmt.Errorf("could not marshal mission %v: %w", m, err)
			}

			err = bkt.Put(id, buf)
			if err != nil {
				return fmt.Errorf("could not store mission %v: %w", m, err)
			}
		}
		return nil
	})

	if err != nil {
		err = fmt.Errorf("could not update eco db buckets: %w", err)
		log.Printf("%+v", err)
		return err
	}

	log.Printf("uploaded %d mission(s)", len(proc.missions))

	return nil
}

func convert(db *bbolt.DB, sqlDB *sql.DB) error {
	f, err := os.Create("eco.csv")
	if err != nil {
		return fmt.Errorf("could not create output CSV file: %w", err)
	}
	defer f.Close()

	const layout = "02/01/2006"

	w := csv.NewWriter(f)
	w.Comma = '\t'

	var ms []eco.Mission

	err = db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(bucketEco)
		if bkt == nil {
			return fmt.Errorf("could not find %q bucket", bucketEco)
		}
		return bkt.ForEach(func(k, v []byte) error {
			var (
				m   eco.Mission
				err = m.UnmarshalBinary(v)
			)
			if err != nil {
				return fmt.Errorf("could not unmarshal mission: %w", err)
			}
			ms = append(ms, m)
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("could not scan db: %+v", err)
	}

	sort.Slice(ms, func(i, j int) bool {
		return ms[i].ID < ms[j].ID
	})

	for _, m := range ms {
		dest := strings.Split(m.Dest.Name, ",")
		tid := int(transID(sqlDB, m.ID))
		if tid < 0 {
			continue
		}
		if int(m.ID) == *idFlag {
			log.Printf("csv> %+v", m)
		}
		rec := []string{
			strconv.Itoa(int(m.ID)),
			m.Date.Format(layout),
			"Clermont-Ferrand", "France",
			strings.TrimSpace(dest[0]),
			strings.TrimSpace(dest[len(dest)-1]),
			m.Trans.String(),
			strconv.Itoa(tid),
			"OUI",
			"N/A",
			"N/A",
		}
		err = w.Write(rec)
		if err != nil {
			return fmt.Errorf("could not write mission ID=%d: %w", m.ID, err)
		}
	}

	w.Flush()
	err = w.Error()
	if err != nil {
		return fmt.Errorf("could not flush CSV file: %w", err)
	}

	/*
		# mission	Date de départ	Ville de départ	Pays de départ	Ville de destination	Pays de destination	Mode de déplacement	Nb de personnes dans la voiture	Aller Retour (OUI si identiques, NON si différents)	Motif du déplacement (optionnel)	Statut de l'agent (optionnel)
		1	24/01/2019	Grenoble	France	Lyon Saint-Exupéry	France	bus		OUI	Colloque-congrès	ITA

	*/
	err = f.Close()
	if err != nil {
		return fmt.Errorf("could not save output CSV file: %w", err)
	}

	return nil
}

func transID(db *sql.DB, id int32) int32 {
	var (
		o   int32
		row = db.QueryRow("select ID_TRANSPORT from view_mission where ID_MISSION=?", id)
	)

	err := row.Scan(&o)
	if err != nil {
		// log.Printf("could not fetch ID_TRANSPORT for mission=%d, name=%q valid=%d: %+v", id, name, valid, err)
		return -1
	}

	return o
}
