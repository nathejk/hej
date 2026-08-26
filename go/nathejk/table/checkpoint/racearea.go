package checkpoint

import "math"

// Point is a WGS84 coordinate.
type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// RaceArea is the region the offline map cache is scoped to: a polygon enclosing every
// positioned checkpoint with a margin, plus the numbers a client needs to reason about it.
//
// Deliberately does NOT carry the checkpoints. The event area is not fully known to
// participants (PRD 002), so the hull is the most that may leave the server; returning the
// points and letting the client compute the hull would hand out every post position.
type RaceArea struct {
	// Polygon is the buffered hull, in order, without a repeated closing vertex.
	Polygon []Point `json:"polygon"`
	// Bounds is the polygon's bounding box, which is what a tile-cache walk actually
	// iterates over. Provided so the client does not have to derive it.
	SouthWest Point `json:"south_west"`
	NorthEast Point `json:"north_east"`
	// AreaKm2 is the buffered polygon's area, so the client can show a download size and
	// refuse an implausible one.
	AreaKm2 float64 `json:"area_km2"`
	// Checkpoints counts what the area was derived from. Not the positions — just how many
	// there were and how many were usable, so a client or an operator can tell "428 km²
	// from 9 points" from "428 km² from 2 points".
	PositionedCount int `json:"positioned_count"`
	TotalCount      int `json:"total_count"`
}

// BufferKm is the margin added around the checkpoint hull.
//
// 3 km, set by the maintainer. It does two jobs: participants stray from the route, and some
// checkpoints have no recorded position, so the margin also absorbs the ones the hull could
// not see (PRD 002 §11.2).
const BufferKm = 3.0

// MaxPlausibleAreaKm2 bounds the result.
//
// The race area is "roughly the same size every year" and measured at 428 km² for 2026, so
// anything past a few thousand km² means bad input rather than a big event — one checkpoint
// with a mistyped coordinate is enough to stretch the hull across a country. The failure this
// prevents is concrete: the area feeds a tile download, and a hull spanning Europe would try
// to cache tens of gigabytes onto a phone.
//
// Generous rather than tight, because refusing a legitimately larger year would be worse than
// accepting a slightly implausible one: 5,000 km² is ~12× the 2026 area.
const MaxPlausibleAreaKm2 = 5000.0

// kmPerDegreeLat is constant enough for this purpose (it varies by ~0.6% with latitude).
const kmPerDegreeLat = 111.32

// kmPerDegreeLng returns the east–west scale at a given latitude.
//
// This is the whole reason the buffer cannot be applied in degrees. At 55.8°N a degree of
// longitude is ~62 km against ~111 km for a degree of latitude, so a naive "add 0.027° all
// round" buffer would be **1.8× wider east–west than north–south** — and would under-buffer
// in the direction the hull is already widest.
func kmPerDegreeLng(lat float64) float64 {
	return kmPerDegreeLat * math.Cos(lat*math.Pi/180)
}

// GridLat and GridLng are the resolution the published polygon is snapped to.
//
// This exists for disclosure, not for tidiness, and the reason is worth spelling out because
// the flaw it fixes was not obvious.
//
// A buffered hull is *invertible*. Every vertex of the buffer lies on a circle of radius
// BufferKm around a vertex of the original hull — and the original hull's vertices are
// checkpoint positions. Since the buffer distance is published too (the client needs it), an
// unsnapped polygon lets anyone offset inward and recover the outermost posts exactly. An
// earlier version of this code did precisely that: its output contained input latitudes to
// full float precision, because sampling the arc from angle 0 puts a vertex at the centre's
// own latitude.
//
// Snapping to a grid coarser than that inversion's precision destroys it. ~1.1 km is the
// difference between "the event is in this region" — which the client must know, since it has
// to cache tiles for it — and "there is a post at this spot", which is the race's to keep.
//
// Snapping is always **outward**, so the published area is a superset of the true one:
// coverage is never lost, only a little extra cached.
const (
	GridLat = 0.01 // ~1.11 km
	GridLng = 0.02 // ~1.25 km at this latitude
)

// snapOutward moves every vertex away from the centroid onto the grid.
//
// Outward rather than nearest, so the result contains the input polygon. For a convex polygon
// with the centroid inside it, moving each vertex away from the centroid cannot shrink the
// shape.
func snapOutward(poly []Point) []Point {
	if len(poly) == 0 {
		return nil
	}

	var cLat, cLng float64
	for _, p := range poly {
		cLat += p.Lat
		cLng += p.Lng
	}
	cLat /= float64(len(poly))
	cLng /= float64(len(poly))

	away := func(v, centre, grid float64) float64 {
		if v >= centre {
			return math.Ceil(v/grid) * grid
		}
		return math.Floor(v/grid) * grid
	}

	out := make([]Point, 0, len(poly))
	for _, p := range poly {
		out = append(out, Point{
			Lat: away(p.Lat, cLat, GridLat),
			Lng: away(p.Lng, cLng, GridLng),
		})
	}
	// Snapping can collide neighbouring vertices onto the same grid point.
	return dedupe(out)
}

// ComputeRaceArea returns the buffered convex hull of the given points.
//
// ok is false when there is nothing usable to derive an area from, or when the result is
// implausibly large. Callers must treat that as "no area" and fall back, rather than caching
// a default region — see MaxPlausibleAreaKm2.
//
// The projection is a local equirectangular one centred on the points: kilometres per degree
// of longitude is taken once at the mean latitude and used throughout. Across a 25 km span
// that is accurate to well under the 3 km buffer, and the alternative — a real geodesic
// buffer — would add a dependency and precision nobody can use. PRD 002 §11.2 records that
// the precision required here is low.
func ComputeRaceArea(points []Point, total int) (RaceArea, bool) {
	if len(points) == 0 {
		return RaceArea{}, false
	}

	var sumLat float64
	for _, p := range points {
		sumLat += p.Lat
	}
	latRef := sumLat / float64(len(points))
	kmLng := kmPerDegreeLng(latRef)
	if kmLng <= 0 {
		// Only reachable at a pole, where the whole model breaks down.
		return RaceArea{}, false
	}

	// To local kilometres, so the hull and buffer are computed on a plane.
	local := make([]Point, len(points))
	for i, p := range points {
		local[i] = Point{Lat: p.Lat * kmPerDegreeLat, Lng: p.Lng * kmLng}
	}

	hull := convexHull(local)
	buffered := bufferPolygon(hull, BufferKm)

	// Back to degrees.
	polygon := make([]Point, len(buffered))
	for i, p := range buffered {
		polygon[i] = Point{Lat: p.Lat / kmPerDegreeLat, Lng: p.Lng / kmLng}
	}

	// Snapped before publication so the buffer cannot be inverted to recover checkpoint
	// positions — see GridLat. Done in degrees, after the geometry, so the grid is a fixed
	// lattice rather than something that moves with the event's location.
	polygon = snapOutward(polygon)

	// Area measured on the snapped polygon, which is what the client will actually cache, so
	// the reported size matches the download. Converted back to kilometres for the shoelace.
	localSnapped := make([]Point, len(polygon))
	for i, p := range polygon {
		localSnapped[i] = Point{Lat: p.Lat * kmPerDegreeLat, Lng: p.Lng * kmLng}
	}
	area := polygonAreaKm2(localSnapped)
	if area > MaxPlausibleAreaKm2 {
		return RaceArea{}, false
	}

	sw, ne := bounds(polygon)
	return RaceArea{
		Polygon:         polygon,
		SouthWest:       sw,
		NorthEast:       ne,
		AreaKm2:         area,
		PositionedCount: len(points),
		TotalCount:      total,
	}, true
}

// convexHull returns the hull in counter-clockwise order (monotone chain).
//
// Degenerate inputs are returned as-is rather than rejected: one point is a point, two are a
// segment, and collinear points are a segment. All three are legitimate early in the year,
// and bufferPolygon turns each into a usable area — a single checkpoint becomes a 3 km disc,
// which is exactly the right answer when that is all that is known.
func convexHull(pts []Point) []Point {
	if len(pts) < 3 {
		return dedupe(pts)
	}

	sorted := append([]Point(nil), pts...)
	sortPoints(sorted)
	sorted = dedupe(sorted)
	if len(sorted) < 3 {
		return sorted
	}

	cross := func(o, a, b Point) float64 {
		return (a.Lng-o.Lng)*(b.Lat-o.Lat) - (a.Lat-o.Lat)*(b.Lng-o.Lng)
	}

	var lower []Point
	for _, p := range sorted {
		for len(lower) >= 2 && cross(lower[len(lower)-2], lower[len(lower)-1], p) <= 0 {
			lower = lower[:len(lower)-1]
		}
		lower = append(lower, p)
	}
	var upper []Point
	for i := len(sorted) - 1; i >= 0; i-- {
		p := sorted[i]
		for len(upper) >= 2 && cross(upper[len(upper)-2], upper[len(upper)-1], p) <= 0 {
			upper = upper[:len(upper)-1]
		}
		upper = append(upper, p)
	}

	hull := append(lower[:len(lower)-1], upper[:len(upper)-1]...)
	if len(hull) < 3 {
		// Every point was collinear; the sorted extremes are the segment.
		return []Point{sorted[0], sorted[len(sorted)-1]}
	}
	return hull
}

// bufferPolygon grows a hull outward by r kilometres.
//
// Approximated by offsetting each vertex along its outward bisector and rounding corners with
// a fixed number of segments, rather than a true Minkowski sum. For a convex hull that is
// close enough at this scale, and it keeps the result a simple polygon a client can iterate.
//
// A segment (two points) and a single point are handled by the same code path: each vertex
// contributes an arc, so a point becomes a disc and a segment becomes a capsule. That is why
// the degenerate hulls above do not need special-casing here.
func bufferPolygon(hull []Point, r float64) []Point {
	const arcSegments = 12

	if len(hull) == 0 {
		return nil
	}
	if len(hull) == 1 {
		return circle(hull[0], r, arcSegments*4)
	}

	var out []Point
	n := len(hull)
	for i := 0; i < n; i++ {
		// A full circle at every vertex, unioned by the convex hull below. Cruder than
		// offsetting edges and joining them, and correct for a convex input: the hull of
		// all the vertex circles *is* the buffered polygon.
		out = append(out, circle(hull[i], r, arcSegments)...)
	}
	return convexHull(out)
}

func circle(c Point, r float64, segments int) []Point {
	pts := make([]Point, 0, segments)
	for i := 0; i < segments; i++ {
		a := 2 * math.Pi * float64(i) / float64(segments)
		pts = append(pts, Point{Lat: c.Lat + r*math.Sin(a), Lng: c.Lng + r*math.Cos(a)})
	}
	return pts
}

// polygonAreaKm2 is the shoelace area of a polygon already in kilometre coordinates.
func polygonAreaKm2(poly []Point) float64 {
	if len(poly) < 3 {
		return 0
	}
	var sum float64
	for i := range poly {
		j := (i + 1) % len(poly)
		sum += poly[i].Lng*poly[j].Lat - poly[j].Lng*poly[i].Lat
	}
	return math.Abs(sum) / 2
}

func bounds(poly []Point) (sw, ne Point) {
	sw = Point{Lat: math.Inf(1), Lng: math.Inf(1)}
	ne = Point{Lat: math.Inf(-1), Lng: math.Inf(-1)}
	for _, p := range poly {
		sw.Lat = math.Min(sw.Lat, p.Lat)
		sw.Lng = math.Min(sw.Lng, p.Lng)
		ne.Lat = math.Max(ne.Lat, p.Lat)
		ne.Lng = math.Max(ne.Lng, p.Lng)
	}
	return sw, ne
}

func sortPoints(p []Point) {
	for i := 1; i < len(p); i++ {
		for j := i; j > 0 && less(p[j], p[j-1]); j-- {
			p[j], p[j-1] = p[j-1], p[j]
		}
	}
}

func less(a, b Point) bool {
	if a.Lng != b.Lng {
		return a.Lng < b.Lng
	}
	return a.Lat < b.Lat
}

func dedupe(p []Point) []Point {
	out := make([]Point, 0, len(p))
	for _, q := range p {
		dup := false
		for _, r := range out {
			if r == q {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, q)
		}
	}
	return out
}
