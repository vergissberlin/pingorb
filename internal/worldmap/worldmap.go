// Package worldmap renders an equirectangular world map onto a character
// grid, using Unicode Braille sub-pixels for extra detail, and projects
// lat/lon coordinates onto that grid so callers can overlay markers.
package worldmap

import "math/rand"

// point is a (longitude, latitude) pair in degrees.
type point struct{ lon, lat float64 }

// ring is a closed polygon boundary: either a landmass's outer coastline or
// a hole in it (e.g. an inland sea).
type ring []point

// landPolygon is one contiguous piece of land: rings[0] is its outer
// coastline, and any further rings are holes cut out of it.
type landPolygon struct {
	rings []ring
}

// dotDensity is the fraction of "land" sub-pixels that get a stipple dot,
// producing the sparse textured look of the reference dashboard instead of
// solid filled continents.
const dotDensity = 0.42

// mapSeed keeps the stipple pattern stable across renders/resizes so the
// map doesn't visibly "shimmer" every refresh tick.
const mapSeed = 42

// Braille characters pack a 2x4 grid of sub-pixels into a single terminal
// cell, giving 8x the effective resolution of a plain "one dot per
// character" map at no extra screen space.
const (
	brailleCols = 2
	brailleRows = 4
	brailleBase = 0x2800
)

// brailleBit maps a sub-pixel's (col, row) position within its character
// cell to the bit it sets in the Braille codepoint.
var brailleBit = [brailleRows][brailleCols]byte{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

// Grid is a rendered map: Cells[row][col] is the character to draw for that
// terminal cell (a Braille glyph, or a space over open ocean).
type Grid struct {
	Width, Height int
	Cells         [][]rune
}

// Generate builds a Grid of the given character dimensions.
func Generate(width, height int) *Grid {
	if width < 2 {
		width = 2
	}
	if height < 2 {
		height = 2
	}

	subW, subH := width*brailleCols, height*brailleRows
	sub := make([][]bool, subH)
	rng := rand.New(rand.NewSource(mapSeed))

	for subRow := 0; subRow < subH; subRow++ {
		sub[subRow] = make([]bool, subW)
		lat := rowToLat(subRow, subH)
		for subCol := 0; subCol < subW; subCol++ {
			lon := colToLon(subCol, subW)
			if isLand(lon, lat) && rng.Float64() < dotDensity {
				sub[subRow][subCol] = true
			}
		}
	}

	g := &Grid{Width: width, Height: height, Cells: make([][]rune, height)}
	for row := 0; row < height; row++ {
		g.Cells[row] = make([]rune, width)
		for col := 0; col < width; col++ {
			var bits byte
			for dy := 0; dy < brailleRows; dy++ {
				for dx := 0; dx < brailleCols; dx++ {
					if sub[row*brailleRows+dy][col*brailleCols+dx] {
						bits |= brailleBit[dy][dx]
					}
				}
			}
			if bits == 0 {
				g.Cells[row][col] = ' '
			} else {
				g.Cells[row][col] = rune(brailleBase + int(bits))
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
	for _, lp := range landPolygons {
		if lp.contains(lon, lat) {
			return true
		}
	}
	return false
}

// contains reports whether (lon, lat) falls inside this landmass: inside
// its outer ring and outside every hole.
func (lp landPolygon) contains(lon, lat float64) bool {
	if len(lp.rings) == 0 || !lp.rings[0].contains(lon, lat) {
		return false
	}
	for _, hole := range lp.rings[1:] {
		if hole.contains(lon, lat) {
			return false
		}
	}
	return true
}

// contains implements a standard ray-casting point-in-polygon test.
func (r ring) contains(lon, lat float64) bool {
	inside := false
	n := len(r)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		pi, pj := r[i], r[j]
		if (pi.lat > lat) != (pj.lat > lat) {
			lonAtLat := (pj.lon-pi.lon)*(lat-pi.lat)/(pj.lat-pi.lat) + pi.lon
			if lon < lonAtLat {
				inside = !inside
			}
		}
	}
	return inside
}
