// Package worldmap renders an equirectangular world map onto a character
// grid, using Unicode Braille sub-pixels for extra detail, and projects
// lat/lon coordinates onto that grid so callers can overlay markers.
//
// The rendering technique (Braille sub-pixel canvas, coastlines drawn as
// plotted line segments rather than filled polygons) follows the approach
// used by satnogs-monitor (https://github.com/wose/satnogs-monitor) and its
// tui-rs Canvas widget.
package worldmap

import "math"

// point is a (longitude, latitude) pair in degrees.
type point struct{ lon, lat float64 }

// ring is a closed coastline loop: a continent or island outline, or the
// shoreline of a large inland body of water. It's drawn as an outline, not
// filled, so there's no distinction between "outer" and "hole" rings.
type ring []point

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

// Generate builds a Grid of the given character dimensions by plotting
// every coastline as a sequence of line segments onto a Braille sub-pixel
// buffer, then packing that buffer into one rune per character cell.
func Generate(width, height int) *Grid {
	if width < 2 {
		width = 2
	}
	if height < 2 {
		height = 2
	}

	subW, subH := width*brailleCols, height*brailleRows
	sub := make([][]bool, subH)
	for i := range sub {
		sub[i] = make([]bool, subW)
	}

	for _, r := range coastlines {
		n := len(r)
		for i := 0; i < n; i++ {
			p1, p2 := r[i], r[(i+1)%n]
			plotLine(sub, subW, subH, p1.lat, p1.lon, p2.lat, p2.lon)
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

// subPixelCoord converts a lat/lon pair into continuous (fractional)
// sub-pixel coordinates for a buffer of the given sub-pixel dimensions.
// Out-of-range latitudes/longitudes are clamped.
func subPixelCoord(lat, lon float64, subW, subH int) (x, y float64) {
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
	x = (lon + 180) / 360 * float64(subW)
	y = (90 - lat) / 180 * float64(subH)
	return x, y
}

// plotPoint sets the sub-pixel nearest to (lat, lon).
func plotPoint(sub [][]bool, subW, subH int, lat, lon float64) {
	x, y := subPixelCoord(lat, lon, subW, subH)
	col, row := int(x), int(y)
	if col >= subW {
		col = subW - 1
	}
	if row >= subH {
		row = subH - 1
	}
	if col < 0 {
		col = 0
	}
	if row < 0 {
		row = 0
	}
	sub[row][col] = true
}

// plotLine draws a segment between two lat/lon points by interpolating a
// number of steps sized from the buffer's current sub-pixel resolution, so
// the line looks continuous (no gaps wider than ~1 sub-pixel) regardless of
// terminal size. This is the primitive coastline drawing uses today, and
// the one a future traceroute hop-path overlay would reuse to connect hops.
func plotLine(sub [][]bool, subW, subH int, lat1, lon1, lat2, lon2 float64) {
	x1, y1 := subPixelCoord(lat1, lon1, subW, subH)
	x2, y2 := subPixelCoord(lat2, lon2, subW, subH)

	dx, dy := x2-x1, y2-y1
	if math.Abs(dx) > float64(subW)/2 {
		return // likely an antimeridian wrap-around edge; skip rather than streak across the map
	}

	steps := int(math.Ceil(math.Max(math.Abs(dx), math.Abs(dy))))
	if steps < 1 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		plotPoint(sub, subW, subH, lat1+(lat2-lat1)*t, lon1+(lon2-lon1)*t)
	}
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
