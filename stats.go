// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eco // import "github.com/sbinet-lpc/eco"

import (
	"strings"
)

type Stats struct {
	Missions  int               `json:"missions"`
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

func (st *Stats) Add(m Mission) {
	st.Missions++
	toks := strings.Split(m.Dest.Name, ",")
	for i, tok := range toks {
		toks[i] = strings.TrimSpace(tok)
	}
	st.Cities[toks[0]]++
	st.Countries[toks[len(toks)-1]]++
	st.TransIDs[m.Trans]++
	st.Dists[m.Trans] += int64(m.Dist) / 1000 // to kilometers
}
