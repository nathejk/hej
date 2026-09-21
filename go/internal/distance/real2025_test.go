package distance

import (
	"testing"
	"time"
)

// A real patrol from the 2025 event (task 339's sanity check).
//
// # Why production data is pinned into a test
//
// Because the arithmetic agreeing with itself proves nothing. These nine scans are one patrol's actual
// night, taken from the `scan` projection, and the expected figure was computed **independently** by
// MariaDB's own `ST_Distance_Sphere` — a different implementation of the same geometry, in a different
// language, written by other people.
//
// That cross-check is the only thing standing between this package and a haversine with a plausible bug
// in it: a factor-of-two error, a swapped latitude and longitude, or degrees where radians belong would
// all produce numbers that look like distances.
//
// The ids are omitted. This is a patrol's route, and while task 337's rules are about *people*, there is
// no reason for a test fixture to name a team.
func realPatrol2025() []Scan {
	at := func(uts int64) time.Time { return time.Unix(uts, 0).UTC() }
	p := func(lat, lng float64, uts int64) Scan {
		return Scan{Lat: f(lat), Lng: f(lng), At: at(uts)}
	}
	return []Scan{
		p(55.55395858833152, 11.708988072361526, 1758328210),
		p(55.53456469706661, 11.673666919288424, 1758334607),
		p(55.543703004697775, 11.709085047018005, 1758357143),
		p(55.528140856051785, 11.784539241479505, 1758361650),
		p(55.5595947, 11.8001041, 1758369204),
		p(55.549549549549546, 11.816799783584827, 1758372177),
		p(55.555751, 11.8308003, 1758372785),
		p(55.5429487, 11.8310824, 1758377512),
		p(55.532128313569544, 11.848950811538963, 1758378921),
	}
}

// The figure MariaDB computed for this patrol, in kilometres, with the same 7 km/h filter applied:
//
//	SELECT SUM(CASE WHEN hrs>0 AND km/hrs<7 THEN km ELSE 0 END)
//	FROM (… ST_Distance_Sphere(POINT(lng,lat), POINT(LAG(lng) OVER w, LAG(lat) OVER w))/1000 …)
//
// 19.953 km over 8 legs.
const realPatrol2025Km = 19.953

func TestAgainstRealScansFrom2025(t *testing.T) {
	est := Compute(realPatrol2025(), nil)

	// A 30 m tolerance over ~20 km. The two implementations use slightly different earth radii
	// (`ST_Distance_Sphere` defaults to 6,370,986 m against our 6,371,000), which is ~2 ppm — so
	// anything outside this is a real disagreement rather than a rounding difference.
	if !closeTo(est.Km, realPatrol2025Km, 0.03) {
		t.Errorf("got %.3f km, MariaDB's ST_Distance_Sphere says %.3f \u2014 the geometry disagrees",
			est.Km, realPatrol2025Km)
	}

	if est.Legs != 8 {
		t.Errorf("want 8 legs, got %d", est.Legs)
	}
	if est.VehicleLegs != 0 {
		t.Errorf("this patrol walked its whole route; %d legs were read as vehicles", est.VehicleLegs)
	}
	if est.Incomplete {
		t.Error("every scan is positioned, so the figure is not incomplete")
	}
}

// And the number this patrol would have been shown.
//
// ~20 km over a night is a real Nathejk figure and is the point of the whole exercise: a plausible number
// that a patrol would recognise, produced without claiming a route was measured.
func TestTheLabelARealPatrolWouldHaveSeen(t *testing.T) {
	est := Compute(realPatrol2025(), nil)

	if got, want := Label(est), "mindst ~19 km"; got != want {
		t.Errorf("Label = %q, want %q", got, want)
	}
}

// **The 2025 distribution, as a guard rather than a note.**
//
// Task 339's sanity check found that summing every leg gave the event a 157.9 km maximum and a 45.9 km
// median, and that excluding vehicle legs brought those to 41 km and 41.3 km. The whole event cannot be
// replayed in a unit test, but the *shape* of the correction can be pinned: a route with one long
// transfer in it must land near the walked distance and nowhere near the total.
func TestATransferDoesNotInflateANightsWalk(t *testing.T) {
	// Four posts of ~4 km, walked over four hours each — a 12 km night.
	base := time.Unix(1758328210, 0).UTC()
	at := func(h float64) time.Time { return base.Add(time.Duration(h * float64(time.Hour))) }
	p := func(lat, lng float64, h float64) Scan { return Scan{Lat: f(lat), Lng: f(lng), At: at(h)} }

	walked := []Scan{
		p(55.530, 11.700, 0),
		p(55.566, 11.700, 4),
		p(55.602, 11.700, 8),
		p(55.638, 11.700, 12),
	}
	// Then a 40 km transfer taking 45 minutes, and one more 4 km leg.
	withTransfer := append(walked,
		p(55.998, 11.700, 12.75),
		p(56.034, 11.700, 16.75),
	)

	walkedOnly := Compute(walked, nil)
	withBus := Compute(withTransfer, nil)

	if withBus.VehicleLegs != 1 {
		t.Fatalf("want the transfer recognised as one vehicle leg, got %d", withBus.VehicleLegs)
	}
	// The bus leg is gone; the extra walked leg is counted. So the figure grows by one leg's walk, not
	// by forty kilometres.
	grew := withBus.Km - walkedOnly.Km
	if grew < 3 || grew > 5 {
		t.Errorf("the figure grew by %.1f km; want ~4 (the extra walked leg, not the 40 km transfer)", grew)
	}
	if withBus.Km > 20 {
		t.Errorf("a 16 km walk with a bus ride in it reported %.1f km \u2014 the transfer leaked in", withBus.Km)
	}
}
