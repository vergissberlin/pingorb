// Package worldmap renders a coarse equirectangular world map onto a
// character grid and projects lat/lon coordinates onto that grid so callers
// can overlay markers.
package worldmap

import "math/rand"

// dotDensity is the fraction of "land" cells that get a stipple dot,
// producing the sparse textured look of the reference dashboard instead of
// solid filled continents.
const dotDensity = 0.4

// mapSeed keeps the stipple pattern stable across renders/resizes so the
// map doesn't visibly "shimmer" every refresh tick.
const mapSeed = 42

// Grid is a rendered map: Land[row][col] is true where a land stipple dot
// should be drawn.
type Grid struct {
	Width, Height int
	Land          [][]bool
}

// Generate builds a Grid of the given character dimensions.
func Generate(width, height int) *Grid {
	if width < 2 {
		width = 2
	}
	if height < 2 {
		height = 2
	}

	g := &Grid{Width: width, Height: height, Land: make([][]bool, height)}
	rng := rand.New(rand.NewSource(mapSeed))

	for row := 0; row < height; row++ {
		g.Land[row] = make([]bool, width)
		lat := rowToLat(row, height)
		for col := 0; col < width; col++ {
			lon := colToLon(col, width)
			if isLand(lon, lat) && rng.Float64() < dotDensity {
				g.Land[row][col] = true
			}
		}
	}
	return g
}

// Project converts a lat/lon pair into a (col, row) cell on a grid of the
// given dimensions. Out-of-range latitudes/longitudes are clamped.
func Project(lat, lon float64, width, height int) (col, row int) {
	if lon < -180 {
		lon = -180
	}
	if lon > 180 {
		lon = 180
	}
	if lat < -90 {
		lat = -90
	}
	if lat > 90 {
		lat = 90
	}

	col = int((lon + 180) / 360 * float64(width))
	row = int((90 - lat) / 180 * float64(height))

	if col >= width {
		col = width - 1
	}
	if row >= height {
		row = height - 1
	}
	if col < 0 {
		col = 0
	}
	if row < 0 {
		row = 0
	}
	return col, row
}

func colToLon(col, width int) float64 {
	return (float64(col)+0.5)/float64(width)*360 - 180
}

func rowToLat(row, height int) float64 {
	return 90 - (float64(row)+0.5)/float64(height)*180
}

func isLand(lon, lat float64) bool {
	for _, poly := range landmasses {
		if poly.contains(lon, lat) {
			return true
		}
	}
	return false
}

// contains implements a standard ray-casting point-in-polygon test.
func (p polygon) contains(lon, lat float64) bool {
	inside := false
	n := len(p)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		pi, pj := p[i], p[j]
		if (pi.lat > lat) != (pj.lat > lat) {
			lonAtLat := (pj.lon-pi.lon)*(lat-pi.lat)/(pj.lat-pi.lat) + pi.lon
			if lon < lonAtLat {
				inside = !inside
			}
		}
	}
	return inside
}
