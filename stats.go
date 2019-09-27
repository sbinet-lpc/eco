// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eco // import "github.com/sbinet-lpc/eco"

import (
	"strings"
	"time"
)

type Stats struct {
	Missions  int               `json:"missions"`
	Start     time.Time         `json:"start"`
	Stop      time.Time         `json:"stop"`
	Countries map[string]int    `json:"countries"`
	Cities    map[string]int    `json:"cities"`
	TransIDs  map[TransID]int   `json:"trans_ids"`
	Dists     map[TransID]int64 `json:"dists"`
}

func NewStats() *Stats {
	return &Stats{
		Countries: make(map[string]int),
		Cities:    make(map[string]int),
		TransIDs:  make(map[TransID]int),
		Dists:     make(map[TransID]int64),
	}
}

func (stats *Stats) Add(m Mission) {
	stats.Missions++
	now := time.Now().UTC()
	if now.After(m.Date) && (stats.Start.After(m.Date) || stats.Start.IsZero()) {
		stats.Start = m.Date
	}
	if now.After(m.Date) && (stats.Stop.Before(m.Date) || stats.Stop.IsZero()) {
		stats.Stop = m.Date
	}
	toks := strings.Split(m.Dest.Name, ",")
	for i, tok := range toks {
		toks[i] = strings.TrimSpace(tok)
	}
	stats.Cities[toks[0]]++
	stats.Countries[toks[len(toks)-1]]++
	stats.TransIDs[m.Trans]++
	stats.Dists[m.Trans] += int64(m.Dist) / 1000 // to kilometers
}
