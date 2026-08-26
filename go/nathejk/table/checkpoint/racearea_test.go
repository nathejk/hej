package checkpoint

import (
	"math"
	"testing"
)

// The nine positioned checkpoints of the 2026 event, read off the stream on 2026-08-26.
// Kept as the realistic regression case: measured by hand as hull 220 km², perimeter 60 km,
// extent 22.4 × 15.3 km, buffered area 428 km².
var real2026 = []Point{
	{55.716595, 12.264819},
	{55.852480717942505, 12.19563961029053},
	{55.811699938762914, 12.16813087463379},
	{55.735134, 12.146577},
	{55.84561460967447, 12.09345817565918},
	{55.65158735883139, 12.12321996688843},
	{55.830986, 12.057753},
	{55.81619708037899, 12.301833629608156},
	{55.81365318474995, 12.076635360717775},
}

func TestRealRaceAreaMatchesTheHandMeasurement(t *testing.T) {
	area, ok := ComputeRaceArea(real2026, 12)
	if !ok {
		t.Fatal("the real checkpoint set must produce an area")
	}

	// The *geometric* area — hull plus a 3 km buffer — measures 428 km² by hand. The
	// **published** area is larger because every vertex is snapped outward onto the
	// disclosure grid (see GridLat), which adds up to ~1.1 km on each side: ~476 km², about
	// 11% more. That is the price of not leaking post positions, and it is worth stating
	// numerically rather than hiding — at 0.73 MB/km² it costs roughly 23 MB of tiles.
	//
	// A wide band, because the exact figure depends on where the hull falls relative to the
	// grid. Outside it means the geometry is wrong rather than merely approximate.
	if area.AreaKm2 < 440 || area.AreaKm2 > 520 {
		t.Errorf("area = %.0f km², want ~476 (428 geometric + outward grid snapping)", area.AreaKm2)
	}
	if area.PositionedCount != 9 || area.TotalCount != 12 {
		t.Errorf("counts = %d/%d, want 9/12", area.PositionedCount, area.TotalCount)
	}

	// The bounds must contain every checkpoint with room to spare — that is what the buffer
	// is for, and it is the property the tile cache depends on.
	for _, cp := range real2026 {
		if cp.Lat <= area.SouthWest.Lat || cp.Lat >= area.NorthEast.Lat ||
			cp.Lng <= area.SouthWest.Lng || cp.Lng >= area.NorthEast.Lng {
			t.Errorf("checkpoint %+v is not strictly inside the buffered bounds %+v..%+v",
				cp, area.SouthWest, area.NorthEast)
		}
	}
}

// The buffer must be a real distance on the ground in both directions. Applied in degrees it
// would be ~1.8x wider east-west than north-south at this latitude, and would under-buffer
// exactly where the hull is widest.
func TestBufferIsAppliedInMetresNotDegrees(t *testing.T) {
	// One checkpoint: the buffered area is a disc of radius BufferKm, so the extent in each
	// direction is measurable directly.
	area, ok := ComputeRaceArea([]Point{{55.786, 12.16}}, 1)
	if !ok {
		t.Fatal("a single checkpoint must still produce an area")
	}

	latSpanKm := (area.NorthEast.Lat - area.SouthWest.Lat) * kmPerDegreeLat
	lngSpanKm := (area.NorthEast.Lng - area.SouthWest.Lng) * kmPerDegreeLng(55.786)

	// 2r, plus up to one grid step on each side from outward snapping (~1.1-1.3 km).
	for _, tc := range []struct {
		name string
		got  float64
	}{{"north-south", latSpanKm}, {"east-west", lngSpanKm}} {
		if tc.got < 2*BufferKm*0.95 || tc.got > 2*BufferKm+2.6 {
			t.Errorf("%s span = %.2f km, want between %.1f and %.1f km",
				tc.name, tc.got, 2*BufferKm*0.95, 2*BufferKm+2.6)
		}
	}

	// The real assertion: the two spans must agree with each other in kilometres. If the
	// buffer were applied in degrees they would differ by ~1.8x. Snapping uses slightly
	// different grids per axis (1.11 km vs 1.25 km here), so allow a little more slack than
	// the geometry alone would need.
	ratio := lngSpanKm / latSpanKm
	if ratio < 0.85 || ratio > 1.15 {
		t.Errorf("east-west/north-south span ratio = %.2f, want ~1.0 — the buffer is being "+
			"applied in degrees, not metres", ratio)
	}
}

// Degenerate hulls are legitimate early in the year, not errors: organizers add checkpoints
// one at a time. Each must still yield a usable area.
func TestDegenerateInputsStillProduceAnArea(t *testing.T) {
	for _, tc := range []struct {
		name    string
		points  []Point
		minArea float64
	}{
		{
			// A disc of radius 3 km ~= 28 km².
			name: "one checkpoint", points: []Point{{55.8, 12.1}}, minArea: 25,
		},
		{
			// A capsule: 2*r*L + pi*r^2. L is ~6.2 km here, so ~65 km².
			name:    "two checkpoints",
			points:  []Point{{55.8, 12.1}, {55.856, 12.1}},
			minArea: 55,
		},
		{
			// Collinear points collapse to a segment; must not become an empty polygon.
			// A 6.7 km segment buffered by 3 km is 2rL + pi r^2 = ~68 km²; the 12-segment
			// arc approximation inscribes the caps, so allow for a few percent under.
			name:    "three collinear checkpoints",
			points:  []Point{{55.8, 12.1}, {55.83, 12.1}, {55.86, 12.1}},
			minArea: 60,
		},
		{
			// Identical points must not produce a zero-area or malformed polygon.
			name:    "duplicate checkpoints",
			points:  []Point{{55.8, 12.1}, {55.8, 12.1}, {55.8, 12.1}},
			minArea: 25,
		},
	} {
		area, ok := ComputeRaceArea(tc.points, len(tc.points))
		if !ok {
			t.Errorf("%s: must produce an area", tc.name)
			continue
		}
		if area.AreaKm2 < tc.minArea {
			t.Errorf("%s: area = %.1f km², want at least %.0f", tc.name, area.AreaKm2, tc.minArea)
		}
		if len(area.Polygon) < 3 {
			t.Errorf("%s: polygon has %d vertices, want a real polygon", tc.name, len(area.Polygon))
		}
		// Every input point must be inside the result.
		for _, p := range tc.points {
			if p.Lat <= area.SouthWest.Lat || p.Lat >= area.NorthEast.Lat {
				t.Errorf("%s: %+v outside bounds", tc.name, p)
			}
		}
	}
}

// No checkpoints is not an error and not an area. The client must fall back rather than
// caching a default region — "cache all of Denmark" is the failure this prevents.
func TestNoCheckpointsYieldsNoArea(t *testing.T) {
	if _, ok := ComputeRaceArea(nil, 0); ok {
		t.Error("no points must not produce an area")
	}
	if _, ok := ComputeRaceArea([]Point{}, 12); ok {
		t.Error("no positioned points must not produce an area, even when checkpoints exist")
	}
}

// One mistyped coordinate is enough to stretch the hull across a continent, and the area
// feeds a tile download. Refusing is the only safe response.
func TestImplausiblyLargeAreaIsRefused(t *testing.T) {
	strays := map[string]Point{
		"a checkpoint in Spain":       {40.4, -3.7},
		"a swapped lat/lng":           {12.16, 55.786},
		"a zero coordinate off Ghana": {0, 0},
	}

	for name, stray := range strays {
		points := append(append([]Point(nil), real2026...), stray)
		if _, ok := ComputeRaceArea(points, len(points)); ok {
			t.Errorf("%s must make the area implausible and be refused", name)
		}
	}

	// And the real set on its own must comfortably pass, or the bound is too tight.
	if _, ok := ComputeRaceArea(real2026, 12); !ok {
		t.Error("the real set must not trip the plausibility bound")
	}
}

// The hull must not carry the checkpoints themselves out of the package.
//
// Two distinct properties, and the second is the one an earlier version of this code got
// wrong. Every vertex must be at least ~BufferKm from every checkpoint — and no vertex may
// share a coordinate *exactly* with a checkpoint, because a buffered hull is invertible: its
// vertices sit on circles of known radius around the original hull's vertices, which are
// checkpoint positions. Snapping to a grid (GridLat/GridLng) is what breaks that inversion.
func TestPolygonDoesNotRevealCheckpointPositions(t *testing.T) {
	area, ok := ComputeRaceArea(real2026, 12)
	if !ok {
		t.Fatal("want an area")
	}

	for _, v := range area.Polygon {
		for _, cp := range real2026 {
			dLat := (v.Lat - cp.Lat) * kmPerDegreeLat
			dLng := (v.Lng - cp.Lng) * kmPerDegreeLng(cp.Lat)
			if d := math.Hypot(dLat, dLng); d < BufferKm*0.9 {
				t.Errorf("polygon vertex %+v is only %.1f km from checkpoint %+v — the "+
					"buffer should keep every vertex at least %.0f km away", v, d, cp, BufferKm)
			}

			// No exact coordinate sharing. This is what made the buffer invertible.
			if v.Lat == cp.Lat {
				t.Errorf("vertex latitude %v is exactly checkpoint %+v's — the buffer can be "+
					"inverted to recover the post", v.Lat, cp)
			}
			if v.Lng == cp.Lng {
				t.Errorf("vertex longitude %v is exactly checkpoint %+v's — the buffer can be "+
					"inverted to recover the post", v.Lng, cp)
			}
		}
	}
}

// Every published vertex sits on the disclosure grid, so inverting the buffer yields a ~1 km
// region rather than a post.
func TestPolygonIsSnappedToTheGrid(t *testing.T) {
	area, ok := ComputeRaceArea(real2026, 12)
	if !ok {
		t.Fatal("want an area")
	}

	for _, v := range area.Polygon {
		if r := math.Abs(math.Remainder(v.Lat, GridLat)); r > 1e-9 {
			t.Errorf("vertex latitude %v is not on the %v grid (off by %v)", v.Lat, GridLat, r)
		}
		if r := math.Abs(math.Remainder(v.Lng, GridLng)); r > 1e-9 {
			t.Errorf("vertex longitude %v is not on the %v grid (off by %v)", v.Lng, GridLng, r)
		}
	}
}

// Snapping must be outward: the published area has to remain a superset of the true one, or
// it would quietly stop covering checkpoints near the edge.
func TestSnappingNeverLosesCoverage(t *testing.T) {
	unsnapped, _ := ComputeRaceArea(real2026, 12)

	// Every checkpoint, plus a 3 km margin around it, must be inside the published bounds.
	for _, cp := range real2026 {
		marginLat := BufferKm / kmPerDegreeLat
		marginLng := BufferKm / kmPerDegreeLng(cp.Lat)
		if cp.Lat-marginLat < unsnapped.SouthWest.Lat ||
			cp.Lat+marginLat > unsnapped.NorthEast.Lat ||
			cp.Lng-marginLng < unsnapped.SouthWest.Lng ||
			cp.Lng+marginLng > unsnapped.NorthEast.Lng {
			t.Errorf("checkpoint %+v plus its %0.f km buffer is not inside the published "+
				"bounds %+v..%+v", cp, BufferKm, unsnapped.SouthWest, unsnapped.NorthEast)
		}
	}
}

// Same input, same output: the area is recomputed on every request, so an unstable hull would
// make the client re-download tiles for a region that had not changed.
func TestComputeIsDeterministic(t *testing.T) {
	a, _ := ComputeRaceArea(real2026, 12)
	b, _ := ComputeRaceArea(real2026, 12)

	if a.AreaKm2 != b.AreaKm2 || len(a.Polygon) != len(b.Polygon) {
		t.Fatalf("unstable result: %.4f/%d vs %.4f/%d",
			a.AreaKm2, len(a.Polygon), b.AreaKm2, len(b.Polygon))
	}
	for i := range a.Polygon {
		if a.Polygon[i] != b.Polygon[i] {
			t.Errorf("vertex %d differs between runs", i)
		}
	}
}

// Input order must not change the result either — the querier orders by checkpointId, but a
// hull that depended on insertion order would still be a latent bug.
func TestOrderIndependent(t *testing.T) {
	forward, _ := ComputeRaceArea(real2026, 12)

	reversed := make([]Point, len(real2026))
	for i, p := range real2026 {
		reversed[len(real2026)-1-i] = p
	}
	backward, _ := ComputeRaceArea(reversed, 12)

	if math.Abs(forward.AreaKm2-backward.AreaKm2) > 0.001 {
		t.Errorf("area depends on input order: %.4f vs %.4f", forward.AreaKm2, backward.AreaKm2)
	}
}
