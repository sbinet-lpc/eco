// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main // import "github.com/sbinet-lpc/eco/cmd/eco-srv"

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	log.SetPrefix("eco-srv: ")
	log.SetFlags(0)

	var (
		addrFlag = flag.String("addr", ":80", "[host]:port to serve")
		dbFlag   = flag.String("db", "eco.db", "path to lpc-eco database")
	)

	flag.Parse()

	srv, err := newServer(*dbFlag)
	if err != nil {
		log.Fatalf("could not create eco server: %+v", err)
	}
	defer srv.Close()

	log.Printf("serving %q...", *addrFlag)

	http.HandleFunc("/", srv.rootHandle)
	http.HandleFunc("/api/last-id", srv.apiLastID)
	http.HandleFunc("/api/stats", srv.apiStats)
	http.HandleFunc("/api/update-db", srv.apiUpdateDB)
	http.HandleFunc("/plot/co2", srv.plotCO2)

	log.Fatalf("error serving eco-srv: %+v", http.ListenAndServe(*addrFlag, nil))
}
