// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main // import "github.com/sbinet-lpc/eco/cmd/eco-srv"

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/sbinet-lpc/eco"
	"go.etcd.io/bbolt"
)

var (
	bucketUpdate = []byte("last-update")
	bucketEco    = []byte("eco")
	bucketOSM    = []byte("osm")
)

type server struct {
	mu   sync.RWMutex
	db   *bbolt.DB
	mid  int32     // last mission id
	last time.Time // last updated
}

func newServer(name string) (*server, error) {
	db, err := bbolt.Open("eco.db", 0644, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("could not open eco db: %w", err)
	}

	srv := &server{db: db, last: time.Now().UTC()}
	err = srv.init()
	if err != nil {
		return nil, fmt.Errorf("could not initialize eco server: %w", err)
	}

	return srv, nil
}

func (srv *server) init() error {
	err := srv.db.Update(func(tx *bbolt.Tx) error {
		upd, err := tx.CreateBucketIfNotExists(bucketUpdate)
		if err != nil {
			return fmt.Errorf("could not create %q bucket: %w", bucketUpdate, err)
		}
		if upd == nil {
			return fmt.Errorf("could not create %q bucket", bucketUpdate)
		}

		eco, err := tx.CreateBucketIfNotExists(bucketEco)
		if err != nil {
			return fmt.Errorf("could not create %q bucket: %w", bucketEco, err)
		}
		if eco == nil {
			return fmt.Errorf("could not create %q bucket", bucketEco)
		}

		osm, err := tx.CreateBucketIfNotExists(bucketOSM)
		if err != nil {
			return fmt.Errorf("could not create %q bucket: %w", bucketOSM, err)
		}
		if osm == nil {
			return fmt.Errorf("could not create %q bucket", bucketOSM)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("could not setup eco db buckets: %w", err)
	}

	err = srv.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(bucketEco)
		if bkt == nil {
			return fmt.Errorf("could not find %q bucket", bucketEco)
		}
		return bkt.ForEach(func(k, v []byte) error {
			id := int32(binary.LittleEndian.Uint32(k))
			if id > srv.mid {
				srv.mid = id
			}
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("could not find last mission id: %w", err)
	}

	err = srv.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(bucketUpdate)
		if bkt == nil {
			return fmt.Errorf("could not find %q bucket", bucketUpdate)
		}
		raw := bkt.Get(bucketUpdate)
		if raw == nil {
			return nil
		}

		return srv.last.UnmarshalBinary(raw)
	})
	if err != nil {
		return fmt.Errorf("could not find last-update: %w", err)
	}

	return nil
}

func (srv *server) Close() error {
	err := srv.db.Close()
	if err != nil {
		return fmt.Errorf("could not close eco db: %w", err)
	}

	return nil
}

func (srv *server) rootHandle(w http.ResponseWriter, r *http.Request) {
	stats, err := srv.stats()
	if err != nil {
		err = fmt.Errorf("could not compute eco stats: %w", err)
		log.Printf("%+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	srv.mu.RLock()
	last := srv.last.Format("2006-01-02 15:04:05")
	srv.mu.RUnlock()

	err = rootTmpl.Execute(w, map[string]interface{}{
		"Stats":   stats,
		"Updated": last,
	})
	if err != nil {
		err = fmt.Errorf("could not execute html template: %w", err)
		log.Printf("%+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (srv *server) apiLastID(w http.ResponseWriter, r *http.Request) {
	srv.mu.RLock()
	defer srv.mu.RUnlock()

	if r.Method != http.MethodGet {
		http.Error(w, "invalid HTTP method", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(struct {
		ID int32 `json:"id"`
	}{ID: srv.mid})

	if err != nil {
		log.Printf("could not encode last-mission ID: %+v", err)
		http.Error(
			w,
			fmt.Errorf("could not encode last-mission ID: %w", err).Error(),
			http.StatusInternalServerError,
		)
		return
	}
}

func (srv *server) apiStats(w http.ResponseWriter, r *http.Request) {
	srv.mu.RLock()
	defer srv.mu.RUnlock()

	if r.Method != http.MethodGet {
		http.Error(w, "invalid HTTP method", http.StatusBadRequest)
		return
	}

	summ := eco.NewSummary()
	err := srv.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(bucketEco)
		if bkt == nil {
			return fmt.Errorf("could not find bucket %q", bucketEco)
		}
		return bkt.ForEach(func(k, v []byte) error {
			var m eco.Mission
			err := m.UnmarshalBinary(v)
			if err != nil {
				return fmt.Errorf("could not unmarshal mission: %w", err)
			}

			summ.Add(m)

			return nil
		})
	})
	if err != nil {
		err = fmt.Errorf("could not process missions: %w", err)
		log.Printf("%+v", err)
		http.Error(
			w,
			fmt.Errorf("could not encode last-mission ID: %w", err).Error(),
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(summ)
	if err != nil {
		log.Printf("could not encode last-mission ID: %+v", err)
		http.Error(
			w,
			fmt.Errorf("could not encode last-mission ID: %w", err).Error(),
			http.StatusInternalServerError,
		)
		return
	}
}

func (srv *server) apiUpdateDB(w http.ResponseWriter, r *http.Request) {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	if r.Method != http.MethodPost {
		http.Error(w, "invalid HTTP method", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	var ms []eco.Mission
	err := json.NewDecoder(r.Body).Decode(&ms)
	if err != nil {
		log.Printf("could not decode update-db request payload: %+v", err)
		http.Error(w,
			fmt.Sprintf("could not decode update-db request payload: %+v", err),
			http.StatusInternalServerError,
		)
		return
	}

	if len(ms) == 0 {
		log.Printf("received an empty mission list")
		return
	}

	err = srv.db.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(bucketEco)
		if bkt == nil {
			return fmt.Errorf("could not access %q bucket", bucketEco)
		}

		id := make([]byte, 4)
		for _, m := range ms {
			binary.LittleEndian.PutUint32(id, uint32(m.ID))
			buf, err := m.MarshalBinary()
			if err != nil {
				return fmt.Errorf("could not marshal mission %v: %w", m, err)
			}

			err = bkt.Put(id, buf)
			if err != nil {
				return fmt.Errorf("could not store mission %v: %w", m, err)
			}
			if m.ID > srv.mid {
				srv.mid = m.ID
			}
		}
		return nil
	})

	if err != nil {
		err = fmt.Errorf("could not update eco db buckets: %w", err)
		log.Printf("%+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	srv.last = time.Now().UTC()
	err = srv.db.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(bucketUpdate)
		if bkt == nil {
			return fmt.Errorf("could not access %q bucket", bucketUpdate)
		}

		raw, err := srv.last.MarshalBinary()
		if err != nil {
			return fmt.Errorf("could not marshal last-update: %w", err)
		}

		return bkt.Put(bucketUpdate, raw)
	})

	log.Printf("updated eco db with %d missions (%d -> %d)", len(ms),
		ms[0].ID,
		ms[len(ms)-1].ID,
	)
}

func (srv *server) stats() (string, error) {
	srv.mu.RLock()
	defer srv.mu.RUnlock()

	summ := eco.NewSummary()
	err := srv.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(bucketEco)
		if bkt == nil {
			return fmt.Errorf("could not find bucket %q", bucketEco)
		}
		return bkt.ForEach(func(k, v []byte) error {
			var m eco.Mission
			err := m.UnmarshalBinary(v)
			if err != nil {
				return fmt.Errorf("could not unmarshal mission: %w", err)
			}

			summ.Add(m)

			return nil
		})
	})
	if err != nil {
		return "", fmt.Errorf("could not process missions: %w", err)
	}

	o := new(strings.Builder)
	fmt.Fprintf(o, "\n<h3>Stats</h3>\n")
	fmt.Fprintf(o, "\n<pre>\n")
	fmt.Fprintf(o, "missions:    %4d (executed)\n", summ.Executed.N)
	fmt.Fprintf(o, "missions:    %4d (planned)\n", summ.Planned.N)
	fmt.Fprintf(o, "missions:    %4d (all)\n", summ.All.N)
	fmt.Fprintf(o, "time period: %v -> %s\n",
		summ.Start.Format("2006-01-02"),
		summ.Stop.Format("2006-01-02"),
	)
	fmt.Fprintf(o, "\n</pre>\n")

	//	type entry struct {
	//		name  string
	//		count int
	//	}
	//
	//	sort := func(m map[string]int) []entry {
	//		vs := make([]entry, 0, len(m))
	//		for k := range m {
	//			vs = append(vs, entry{k, m[k]})
	//		}
	//		sort.Slice(vs, func(i, j int) bool {
	//			ii := vs[i]
	//			jj := vs[j]
	//			if ii.name == jj.name {
	//				return ii.count < jj.count
	//			}
	//			return ii.name < jj.name
	//		})
	//		return vs
	//	}
	//
	//	cities := sort(summ.Cities)
	//	fmt.Fprintf(o, "=== cities ===\n")
	//	for _, v := range cities {
	//		fmt.Fprintf(o, "%-10s %d\n", v.name, v.count)
	//	}
	//
	//	countries := sort(summ.Countries)
	//	fmt.Fprintf(o, "=== countries ===\n")
	//	for _, v := range countries {
	//		fmt.Fprintf(o, "%-10s %d\n", v.name, v.count)
	//	}

	tids := []eco.TransID{
		eco.Bike,
		eco.Tramway,
		eco.Train,
		eco.Bus,
		eco.Passenger,
		eco.Car,
		eco.Plane,
	}

	fmt.Fprintf(o, "<h3>Transport (executed, planned, all)</h3>\n")
	fmt.Fprintf(o, "\n<pre>\n")
	for _, k := range tids {
		v1 := summ.Executed.TransIDs[k]
		v2 := summ.Planned.TransIDs[k]
		v3 := summ.All.TransIDs[k]
		fmt.Fprintf(o, "%-10s %5d %5d %5d\n", k, v1, v2, v3)
	}
	fmt.Fprintf(o, "\n</pre>\n")

	fmt.Fprintf(o, "<h3>Distances (executed, planned, all)</h3>\n")
	fmt.Fprintf(o, "\n<pre>\n")
	for _, k := range tids {
		v1 := summ.Executed.Dists[k]
		v2 := summ.Planned.Dists[k]
		v3 := summ.All.Dists[k]
		fmt.Fprintf(o, "%-10s %8d km %8d km %8d km\n", k, v1, v2, v3)
	}
	fmt.Fprintf(o, "\n</pre>\n")

	fmt.Fprintf(o, "<h3>Summary (multiplicity, distance, CO2e -- only for executed)</h3>\n")
	fmt.Fprintf(o, "\n<pre>\n")
	for _, k := range tids {
		n := summ.Executed.TransIDs[k]
		dist := summ.Executed.Dists[k]
		co2 := eco.CostOf(k, float64(dist))
		fmt.Fprintf(o, "%-10s %8d %8d km %8.2f tCO2\n", k, n, dist, co2)
	}
	fmt.Fprintf(o, "\n</pre>\n")

	return o.String(), nil
}

const rootPage = `
<html>
        <head>
                <title>ecoLPC</title>
                <style>
                </style>
        </head>

        <body>
                <div id="header">
                        <h2>CO2 Evolution</h2>
                </div>
				<pre>Last Updated: {{.Updated}} (UTC)</pre>
				<div id="stats">
					{{.Stats}}
				</div>
                <div id="content">
					<img id="co2-plot" src="/plot/co2" alt="N/A"></img>
                </div>
				<br>
				<hr>
				<div>
					Source code is <a href="https://github.com/sbinet-lpc/eco">there</a>.
				</div>
        </body>
</html>

`

var rootTmpl = template.Must(template.New("eco-LPC").Parse(rootPage))
