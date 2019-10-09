// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eco // import "github.com/sbinet-lpc/eco"

import (
	"strings"
	"time"
)

type Summary struct {
	Start     time.Time      `json:"start"`
	Stop      time.Time      `json:"stop"`
	Countries map[string]int `json:"countries"`
	Cities    map[string]int `json:"cities"`
	All       Stats          `json:"all_missions"`
	Planned   Stats          `json:"planned_missions"`
	Executed  Stats          `json:"executed_missions"`
}

func NewSummary() *Summary {
	return &Summary{
		Countries: make(map[string]int),
		Cities:    make(map[string]int),
		All:       NewStats(),
		Planned:   NewStats(),
		Executed:  NewStats(),
	}
}

func (summ *Summary) Add(m Mission) {
	now := time.Now().UTC()
	if now.After(m.Date) && (summ.Start.After(m.Date) || summ.Start.IsZero()) {
		summ.Start = m.Date
	}
	if now.After(m.Date) && (summ.Stop.Before(m.Date) || summ.Stop.IsZero()) {
		summ.Stop = m.Date
	}

	summ.All.Add(m)
	planned := now.Before(m.Date)
	switch {
	case planned:
		summ.Planned.Add(m)
	default:
		summ.Executed.Add(m)
	}

	toks := strings.Split(m.Dest.Name, ",")
	for i, tok := range toks {
		toks[i] = strings.TrimSpace(tok)
	}
	summ.Cities[toks[0]]++
	summ.Countries[toks[len(toks)-1]]++
}

type Stats struct {
	N        int               `json:"missions"`
	TransIDs map[TransID]int   `json:"trans_ids"`
	Dists    map[TransID]int64 `json:"dists"`
}

func NewStats() Stats {
	return Stats{
		TransIDs: make(map[TransID]int),
		Dists:    make(map[TransID]int64),
	}
}

func (stats *Stats) Add(m Mission) {
	stats.N++
	stats.TransIDs[m.Trans]++
	stats.Dists[m.Trans] += int64(m.Dist) / 1000 // to kilometers
}
