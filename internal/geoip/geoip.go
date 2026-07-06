// Package geoip resolves a rough lat/lon for a host, used only as a
// convenience fallback when the user doesn't pass --lat/--lon explicitly.
package geoip

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type lookupResponse struct {
	Status  string  `json:"status"`
	Message string  `json:"message"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

// Lookup queries a public IP-geolocation API for an approximate position of
// host. It sends only the hostname/IP - no other data leaves the machine.
// Callers should treat failures as non-fatal (position stays 0,0).
func Lookup(ctx context.Context, host string) (lat, lon float64, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	endpoint := "http://ip-api.com/json/" + url.PathEscape(host) + "?fields=status,message,lat,lon"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var out lookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, 0, err
	}
	if out.Status != "success" {
		return 0, 0, fmt.Errorf("geoip lookup failed for %s: %s", host, out.Message)
	}
	return out.Lat, out.Lon, nil
}
