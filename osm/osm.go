// Copyright 2019 The lpc-eco Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package osm // import "github.com/sbinet-lpc/eco/osm"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Place describes a place location from Nominatim's OpenStreetMap database.
type Place struct {
	ID          int64    `json:"place_id"`
	Rank        int64    `json:"place_rank"`
	Licence     string   `json:"licence"`
	OsmType     string   `json:"osm_type"`
	OsmID       int64    `json:"osm_id"`
	Boundingbox []string `json:"boundingbox"`
	Lat         string   `json:"lat"`
	Lng         string   `json:"lon"`
	DisplayName string   `json:"display_name"`
	Category    string   `json:"category"`
	Type        string   `json:"type"`
	Importance  float64  `json:"importance"`
	Address     Address  `json:"address"`
}

// Address gives (optional) additional informations about a Place.
type Address struct {
	City          string `json:"city"`
	StateDistrict string `json:"state_district"`
	State         string `json:"state"`
	Postcode      string `json:"postcode"`
	Country       string `json:"country"`
	CountryCode   string `json:"country_code"`
}

// Search queries the OpenStreetMap Nominatim service for a given place, using
// the default client.
func Search(query string) ([]Place, error) {
	return DefaultClient.Search(query)
}

const (
	UserAgent = "OpenStreetMap_Go_Client/0.1" // Default UserAgent used by osm queries.
	apiSearch = "https://nominatim.openstreetmap.org/search"
)

var DefaultClient = Client{
	UserAgent: UserAgent,
}

type Client struct {
	UserAgent       string
	AddressDetails  bool
	AcceptLanguages []string
}

// Search queries the OpenStreetMap Nominatim service for a given place.
func (c *Client) Search(query string) ([]Place, error) {
	var cli http.Client

	req, err := http.NewRequest(http.MethodGet, apiSearch, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create HTTP request: %w", err)
	}

	req.Header.Set("User-Agent", c.UserAgent)

	form := make(url.Values)
	form.Add("q", query)
	form.Add("format", "jsonv2")
	switch c.AddressDetails {
	case true:
		form.Add("addressdetails", "1")
	default:
		form.Add("addressdetails", "0")
	}
	if c.AcceptLanguages != nil {
		form.Add("accept-language", strings.Join(c.AcceptLanguages, ","))
	}
	req.URL.RawQuery = form.Encode()

	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not send request to OpenStreetMap: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		out := new(bytes.Buffer)
		io.Copy(out, resp.Body)
		return nil, fmt.Errorf(
			"invalid status code %s (%d):\n%s",
			resp.Status, resp.StatusCode, out.String(),
		)
	}

	var places []Place
	err = json.NewDecoder(resp.Body).Decode(&places)
	if err != nil {
		return nil, fmt.Errorf("could not decode JSON reply from %q: %w", req, err)
	}

	return places, nil
}
