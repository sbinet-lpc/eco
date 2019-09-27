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
	"sync"
	"time"

	"github.com/sbinet-lpc/eco"
	"go.etcd.io/bbolt"
	"golang.org/x/xerrors"
)

var (
	bucketEco = []byte("eco")
	bucketOSM = []byte("osm")
)

type server struct {
	mu  sync.RWMutex
	db  *bbolt.DB
	mid int32 // last mission id
}

func newServer(name string) (*server, error) {
	db, err := bbolt.Open("eco.db", 0644, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, xerrors.Errorf("could not open eco db: %w", err)
	}

	srv := &server{db: db}
	err = srv.init()
	if err != nil {
		return nil, xerrors.Errorf("could not initialize eco server: %w", err)
	}

	return srv, nil
}

func (srv *server) init() error {
	err := srv.db.Update(func(tx *bbolt.Tx) error {
		eco, err := tx.CreateBucketIfNotExists(bucketEco)
		if err != nil {
			return xerrors.Errorf("could not create %q bucket: %w", bucketEco, err)
		}
		if eco == nil {
			return xerrors.Errorf("could not create %q bucket", bucketEco)
		}

		osm, err := tx.CreateBucketIfNotExists(bucketOSM)
		if err != nil {
			return xerrors.Errorf("could not create %q bucket: %w", bucketOSM, err)
		}
		if osm == nil {
			return xerrors.Errorf("could not create %q bucket", bucketOSM)
		}

		return nil
	})
	if err != nil {
		return xerrors.Errorf("could not setup eco db buckets: %w", err)
	}

	err = srv.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(bucketEco)
		if bkt == nil {
			return xerrors.Errorf("could not find %q bucket", bucketEco)
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
		return xerrors.Errorf("could not find last mission id: %w", err)
	}

	return nil
}

func (srv *server) Close() error {
	err := srv.db.Close()
	if err != nil {
		return xerrors.Errorf("could not close eco db: %w", err)
	}

	return nil
}

func (srv *server) rootHandle(w http.ResponseWriter, r *http.Request) {
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
			xerrors.Errorf("could not encode last-mission ID: %w", err).Error(),
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

	stats := eco.NewStats()
	err := srv.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(bucketEco)
		if bkt == nil {
			return xerrors.Errorf("could not find bucket %q", bucketEco)
		}
		return bkt.ForEach(func(k, v []byte) error {
			var m eco.Mission
			err := m.UnmarshalBinary(v)
			if err != nil {
				return xerrors.Errorf("could not unmarshal mission: %w", err)
			}

			stats.Add(m)

			return nil
		})
	})
	if err != nil {
		err = xerrors.Errorf("could not process missions: %w", err)
		log.Printf("%+v", err)
		http.Error(
			w,
			xerrors.Errorf("could not encode last-mission ID: %w", err).Error(),
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(stats)
	if err != nil {
		log.Printf("could not encode last-mission ID: %+v", err)
		http.Error(
			w,
			xerrors.Errorf("could not encode last-mission ID: %w", err).Error(),
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
			return xerrors.Errorf("could not access %q bucket", bucketEco)
		}

		id := make([]byte, 4)
		for _, m := range ms {
			binary.LittleEndian.PutUint32(id, uint32(m.ID))
			buf, err := m.MarshalBinary()
			if err != nil {
				return xerrors.Errorf("could not marshal mission %v: %w", m, err)
			}

			err = bkt.Put(id, buf)
			if err != nil {
				return xerrors.Errorf("could not store mission %v: %w", m, err)
			}
			if m.ID > srv.mid {
				srv.mid = m.ID
			}
		}
		return nil
	})

	if err != nil {
		err = xerrors.Errorf("could not update eco db buckets: %w", err)
		log.Printf("+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("updated eco db with %d missions (%d -> %d)", len(ms),
		ms[0].ID,
		ms[len(ms)-1].ID,
	)
}
