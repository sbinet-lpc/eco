// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main // import "github.com/sbinet-lpc/eco/cmd/eco-ingest"

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sbinet-lpc/eco"
	"github.com/sbinet-lpc/eco/geo"
	"github.com/sbinet-lpc/eco/osm"
)

const (
	timefmtMission = "2006-01-02 15:04:05"
	timefmtJourney = "2006-01-02"
)

var (
	clermont = geo.Point{Lat: 45.7774551, Lng: 3.0819427}

	addrFlag = flag.String("addr", ":80", "address to eco-srv")
	idFlag   = flag.Int("id", 0, "enable verbose mode for a specific mission ID")
	dbgFlag  = flag.Bool("v", false, "enable verbose mode")
	dryFlag  = flag.Bool("dry", false, "enable dry mode (do not commit to eco-DB)")

	fixupsTIDFlag  = flag.String("fixups-tid", "fixups.tid.json", "path to transport IDs fixups")
	fixupsDestFlag = flag.String("fixups-dest", "fixups.dest.json", "path to destination fixups")

	fixupTIDs map[int32]eco.TransID
)

func main() {
	log.SetPrefix("eco-ingest: ")
	log.SetFlags(0)

	flag.Parse()

	lastID, err := getLastID(*addrFlag)
	if err != nil {
		log.Fatalf("could not retrieve last mission id: %+v", err)
	}

	fixupTIDs, err = loadTIDs(*fixupsTIDFlag)
	if err != nil {
		log.Fatalf("could not load TIDs db: %+v", err)
	}

	c, err := readCredentials()
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("mysql", c.Conn())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatalf("could not ping db: %+v", err)
	}

	// FIXME(sbinet): use a WHERE clause with ID_MISSION > lastID
	rows, err := db.Query("select * from view_mission order by ID_MISSION")
	if err != nil {
		log.Fatalf("could not select: %+v", err)
	}
	defer rows.Close()

	var (
		invalid  int64
		missions = make(map[int32][]Mission)
		allgood  = true
	)
	for rows.Next() {
		if rows.Err() != nil {
			break
		}
		var m RawMission
		err = rows.Scan(
			&m.ID, &m.Date, &m.Org, &m.Group,
			&m.Departure,
			&m.Destination,
			&m.Object,
			&m.Type,
			&m.Transport.Name,
			&m.Outbound.Date,
			&m.Outbound.Start,
			&m.Outbound.Stop,
			&m.Inbound.Date,
			&m.Inbound.Start,
			&m.Inbound.Stop,
			&m.Comment,
			&m.Valid,
			&m.Cost,
			&m.Residence.Familiale,
			&m.Residence.Return,
			&m.Housing,
			&m.Transport.ID,
			&m.Transport.Label,
		)
		if err != nil {
			log.Fatalf("could not scan row: %+v", err)
		}

		if *idFlag == int(m.ID) {
			log.Printf(
				"id=%d, org=%q, grp=%q mission:%q transport:%d|%s valid=%d comment=%q",
				m.ID, m.Org, m.Group, m.Destination, m.Transport.ID, m.Transport.Label,
				m.Valid, m.Comment,
			)
		}

		if m.ID <= lastID {
			continue
		}

		mm, ok := m.ToMission()
		if !ok {
			allgood = false
			invalid++
			log.Printf(
				"INVALID mission: id=%d, org=%q, grp=%q mission:%q transport:%d|%s valid=%d comment=%q (date=%v -> %v)",
				m.ID, m.Org, m.Group, m.Destination, m.Transport.ID, m.Transport.Label,
				m.Valid, m.Comment,
				string(m.Outbound.Date),
				string(m.Inbound.Date),
			)
			continue
		}

		if !mm.isValid() {
			invalid++
			continue
		}

		missions[mm.ID] = append(missions[mm.ID], mm)
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("could not iterate over select result: %+v", err)
	}
	log.Printf("missions:   %d", len(missions))
	log.Printf("invalid:    %d", invalid)
	if !allgood {
		log.Fatalf("could not handle at least one mission. check TIDs")
	}

	dups := 0
	mids := make([]int32, 0, len(missions))
	for id := range missions {
		mids = append(mids, id)
		if len(missions[id]) > 1 {
			dups++
		}
	}

	sort.Slice(mids, func(i, j int) bool {
		return mids[i] < mids[j]
	})
	log.Printf("multi-legs: %d", dups)

	if len(mids) == 0 {
		log.Printf("no new mission to process")
		return
	}

	proc, err := newProcessor(*fixupsDestFlag)
	if err != nil {
		log.Fatalf("could not create processor: %+v", err)
	}

	for _, id := range mids {
		m := chooseMission(missions[id])
		if *dbgFlag {
			log.Printf(
				"id=%d transport=%v, date=%s dest=%v",
				m.ID,
				m.Transport.Label,
				m.Outbound.Date.Format(timefmtJourney),
				m.Destination,
			)
		}

		err := proc.Process(m)
		if err != nil {
			log.Printf("could not process id=%d: %+v", id, err)
			allgood = false
			break
		}
	}

	if *dryFlag {
		log.Printf("dry mode enabled: no upload to %q eco-srv", *addrFlag)
		return
	}

	err = proc.upload(*addrFlag)
	if err != nil {
		log.Fatalf("could not upload new missions: %+v", err)
	}

	if !allgood {
		log.Fatalf("an error occurred during processing")
	}
}

const (
	idAvion int32 = iota + 1
	idBus
	idPassager
	idTrain
	idVoitureAdm
	idVoitureLoc
	idVoiturePers
	idAutres
)

type RawMission struct {
	ID          int32
	Date        []byte       // when was the mission drafted
	Org         sql.RawBytes // funding organization
	Group       sql.RawBytes // group funding the mission
	Departure   sql.RawBytes
	Destination sql.RawBytes
	Object      sql.RawBytes
	Type        int16
	Transport   struct {
		Name  sql.RawBytes
		ID    int32
		Label sql.RawBytes
	}
	Outbound  RawJourney
	Inbound   RawJourney
	Comment   sql.RawBytes
	Valid     int16
	Cost      sql.NullFloat64
	Residence struct {
		Familiale int8
		Return    sql.NullInt64
	}
	Housing sql.RawBytes
}

func (raw RawMission) ToMission() (Mission, bool) {
	date, err := time.Parse(timefmtMission, string(raw.Date))
	if err != nil {
		panic(err)
	}
	out := Mission{
		ID:          raw.ID,
		Date:        date.UTC(),
		Org:         string(raw.Org),
		Group:       string(raw.Group),
		Departure:   string(raw.Departure),
		Destination: string(raw.Destination),
		Object:      string(raw.Object),
		Type:        raw.Type,
		Outbound:    raw.Outbound.ToJourney(),
		Inbound:     raw.Inbound.ToJourney(),
		Comment:     string(raw.Comment),
		Valid:       raw.Valid,
		Cost:        -1,
		Housing:     string(raw.Housing),
	}
	out.Transport.Name = string(raw.Transport.Name)
	out.Transport.ID = raw.Transport.ID
	out.Transport.Label = string(raw.Transport.Label)
	if raw.Cost.Valid {
		out.Cost = raw.Cost.Float64
	}
	out.Residence.Familiale = raw.Residence.Familiale
	out.Residence.Return = -1
	if raw.Residence.Return.Valid {
		out.Residence.Return = raw.Residence.Return.Int64
	}

	if ok := out.checkTID(); !ok {
		return Mission{}, ok
	}

	return out, true
}

type Mission struct {
	ID          int32
	Date        time.Time // when was the mission drafted
	Org         string    // funding organization
	Group       string    // group funding the mission
	Departure   string
	Destination string
	Object      string
	Type        int16
	Transport   struct {
		Name  string
		ID    int32
		Label string
	}
	Outbound  Journey
	Inbound   Journey
	Comment   string
	Valid     int16
	Cost      float64
	Residence struct {
		Familiale int8
		Return    int64
	}
	Housing string
}

func (m Mission) isValid() bool {
	switch m.Valid {
	case 4:
		// rejected by manager
		return false
	case 5:
		// rejected by funder
		return false
	case 6:
		// accepted by funder, rejected by manager
		return false
	case 7:
		// accepted by manager, rejected by funder
		return false
	case 8:
		// rejected by manager & funder
		return false
	}
	return true
}

func (m Mission) TransID() eco.TransID {
	var id eco.TransID
	switch tid := m.Transport.ID; tid {
	case idAvion:
		id = eco.Plane
	case idBus:
		id = eco.Bus
	case idPassager:
		// free. cost already reported on somebody else
		id = eco.Passenger
	case idTrain:
		id = eco.Train
	case idVoitureAdm, idVoitureLoc, idVoiturePers:
		id = eco.Car
	case idAutres:
		id = fixupTID(m)
	}

	return id
}

func (m Mission) checkTID() bool {
	switch tid := m.Transport.ID; tid {
	default:
		return true
	case idAutres:
		_, ok := fixupTIDs[m.ID]
		return ok
	}
}

func chooseMission(ms []Mission) Mission {
	if len(ms) == 1 {
		return ms[0]
	}

	if *dbgFlag {
		log.Printf("=== missions %d === (n=%d)", ms[0].ID, len(ms))
		for _, m := range ms {
			log.Printf("transport=%s, destination=%s", m.Transport.Label, m.Destination)
		}
	}

	costs := make([]eco.TransID, len(ms))
	for i, m := range ms {
		costs[i] = m.TransID()
	}
	switch {
	case reflect.DeepEqual(costs, []eco.TransID{eco.Train, eco.Bus}):
		return ms[0]
	case reflect.DeepEqual(costs, []eco.TransID{eco.Bus, eco.Train}):
		return ms[1]
	}

	var (
		cost = ms[0].TransID()
		j    = 0
	)
	for i := range costs {
		if !eco.CostLess(costs[i], cost) {
			cost = costs[i]
			j = i
		}
	}
	return ms[j]
}

type RawJourney struct {
	Date  []byte
	Start sql.RawBytes // starting point of the journey
	Stop  sql.RawBytes // destination of the journey
}

func (raw RawJourney) ToJourney() Journey {
	date, err := time.Parse(timefmtJourney, string(raw.Date))
	if err != nil {
		panic(err)
	}

	return Journey{
		Date:  date.UTC(),
		Start: string(raw.Start),
		Stop:  string(raw.Stop),
	}
}

type Journey struct {
	Date  time.Time
	Start string // Hour of departure
	Stop  string // Hour of arrival
}

type cred struct {
	User  string `json:"user"`
	Pwd   string `json:"password"`
	Host  string `json:"host"`
	DB    string `json:"db"`
	Table string `json:"table"`
}

type OSMReq struct {
	Mission Mission     `json:"mission"`
	Places  []osm.Place `json:"places"`
}

func (req OSMReq) LatLng() (float64, float64) {
	loc := req.Places[0]

	lat, err := strconv.ParseFloat(loc.Lat, 64)
	if err != nil {
		panic(fmt.Errorf("could not convert latitude: %w", err))
	}
	lng, err := strconv.ParseFloat(loc.Lng, 64)
	if err != nil {
		panic(fmt.Errorf("could not convert longitude: %w", err))
	}
	return lat, lng
}

func readCredentials() (cred, error) {
	var v cred
	f, err := os.Open("passwd")
	if err != nil {
		return v, fmt.Errorf("could not open credentials file: %w", err)
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&v)
	if err != nil {
		return v, fmt.Errorf("could not decode credentials file content: %w", err)
	}

	return v, nil
}

func (c cred) Conn() string {
	return fmt.Sprintf("%s:%s@tcp(%s:3306)/%s", c.User, c.Pwd, c.Host, c.DB)
}

func getLastID(addr string) (int32, error) {
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}
	url := fmt.Sprintf("http://%s/api/last-id", addr)
	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("could not GET last-id: %w", err)
	}
	defer resp.Body.Close()

	var raw struct {
		ID int32 `json:"id"`
	}
	err = json.NewDecoder(resp.Body).Decode(&raw)
	if err != nil {
		return 0, fmt.Errorf("could not decode last-id response: %w", err)
	}

	return raw.ID, nil
}

func fixupTID(m Mission) eco.TransID {
	tid, ok := fixupTIDs[m.ID]
	if !ok {
		panic(fmt.Errorf("invalid mission-id=%d, (tid=%d|%v) comment=%q", m.ID, m.Transport.ID, m.Transport.Label, m.Comment))
	}
	return tid
}

func loadTIDs(name string) (map[int32]eco.TransID, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("could not open tid db file: %w", err)
	}
	defer f.Close()

	var raw []struct {
		ID  int32  `json:"id"`
		TID string `json:"tid"`
	}
	err = json.NewDecoder(f).Decode(&raw)
	if err != nil {
		return nil, fmt.Errorf("could not decode tid db file: %w", err)
	}

	db := make(map[int32]eco.TransID, len(raw))
	tids := make(map[string]eco.TransID)
	for _, tid := range []eco.TransID{
		eco.Bike,
		eco.Tramway,
		eco.Train,
		eco.Bus,
		eco.Passenger,
		eco.Car,
		eco.Plane,
	} {
		tids[tid.String()] = tid
	}

	for _, v := range raw {
		tid, ok := tids[v.TID]
		if !ok {
			return nil, fmt.Errorf("could not find eco.TransID corresponding to %q", v.TID)
		}
		db[v.ID] = tid
	}

	return db, nil
}
