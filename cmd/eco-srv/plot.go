// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main // import "github.com/sbinet-lpc/eco/cmd/eco-srv"

import (
	"fmt"
	"image/color"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/sbinet-lpc/eco"
	"go-hep.org/x/hep/hplot"
	"go.etcd.io/bbolt"
	"golang.org/x/xerrors"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

func (srv *server) plotCO2(w http.ResponseWriter, r *http.Request) {
	srv.mu.RLock()
	defer srv.mu.RUnlock()

	var ms []eco.Mission
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
			ms = append(ms, m)
			return nil
		})
	})
	if err != nil {
		err = xerrors.Errorf("could not process missions: %w", err)
		log.Printf("%+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tp := hplot.NewTiledPlot(draw.Tiles{
		Cols: 2, Rows: 2,
		PadX: 1 * vg.Centimeter,
		PadY: 1 * vg.Centimeter,
	})
	tp.Plots[0] = makeTIDPlot(eco.Train, ms)
	tp.Plots[1] = makeTIDPlot(eco.Bus, ms)
	tp.Plots[2] = makeTIDPlot(eco.Car, ms)
	tp.Plots[3] = makeTIDPlot(eco.Plane, ms)

	c := &vgimg.PngCanvas{vgimg.New(2*15*vg.Centimeter, 2*10*vg.Centimeter)}
	tp.Draw(draw.New(c))

	w.Header().Set("Content-Type", "image/png")
	_, err = c.WriteTo(w)
	if err != nil {
		err = xerrors.Errorf("could not write PNG canvas: %w", err)
		log.Printf("%+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func makeTIDPlot(tid eco.TransID, ms []eco.Mission) *hplot.Plot {
	var (
		now  = time.Now().UTC()
		xmin = ms[0].Date
		xmax = now
	)

	p := hplot.New()
	sort.Slice(ms, func(i, j int) bool {
		mi := ms[i]
		mj := ms[j]
		switch {
		case mi.Date.Equal(mj.Date):
			return mi.ID < mj.ID
		default:
			return mi.Date.Before(mj.Date)
		}
	})

	data := make([]eco.Mission, 0, len(ms))
	for _, m := range ms {
		if m.Date.Before(xmin) {
			xmin = m.Date
		}
		if m.Trans != tid {
			continue
		}
		if m.Date.After(now) {
			continue
		}
		data = append(data, m)
	}

	total := 0.0
	pts := make(plotter.XYs, len(data))
	for i, m := range data {
		total += m.Dist
		pts[i].X = float64(m.Date.Unix())
		pts[i].Y = total / 1000
	}

	cost := eco.CostOf(tid, total)
	p.Title.Text = fmt.Sprintf("%s: %3.2f tCO2e", strings.Title(tid.String()), cost/1000)
	p.Y.Label.Text = "Cumulative distance [km]"

	// xticks defines how we convert and display time.Time values.
	xticks := plot.TimeTicks{Format: "2006-01-02"}
	p.X.Tick.Marker = xticks
	p.X.Min = float64(xmin.Unix())
	p.X.Max = float64(xmax.Unix())

	line, err := hplot.NewLine(pts)
	if err != nil {
		panic(xerrors.Errorf("could not create line plot for %v: %w", tid, err))
	}
	line.StepStyle = plotter.PreStep
	line.LineStyle.Color = color.RGBA{0, 0, 255, 255}

	p.Add(line)
	p.Add(hplot.NewGrid())
	return p
}
