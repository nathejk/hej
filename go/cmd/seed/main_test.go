package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jrgensen/cqrs"
)

// The guard that matters most. This tool publishes to a broker shared with every other
// service and cannot un-publish, so a mistyped -year must not be able to inject synthetic
// members into a live event's projection.
func TestRealYearsAreRefused(t *testing.T) {
	for _, year := range []string{"2025", "2026", "2027"} {
		if !realYears[year] {
			t.Errorf("year %q must be refused", year)
		}
	}
	for _, year := range []string{"9999", "1901", "0000"} {
		if realYears[year] {
			t.Errorf("sentinel year %q must be allowed", year)
		}
	}
}

// No case may hardcode a year: -year has to be honoured everywhere, or a case leaks into
// whatever year its author happened to type.
func TestNoCaseHardcodesAYear(t *testing.T) {
	const year = "1901"

	for _, c := range allCases() {
		for _, e := range c.events(year) {
			parts := strings.Split(e.subject, ".")
			if len(parts) < 2 {
				t.Fatalf("%s: malformed subject %q", c.name, e.subject)
			}
			if parts[1] != year {
				t.Errorf("%s: subject %q uses year %q, not the requested %q",
					c.name, e.subject, parts[1], year)
			}
		}
	}
}

// Every subject must be well-formed enough for the projection's matcher: a domain, a
// year, an entity kind, an id and a verb. A subject with too few parts is silently
// ignored by the consumer, so the case would appear to seed and do nothing.
func TestSubjectsAreWellFormed(t *testing.T) {
	for _, c := range allCases() {
		for _, e := range c.events("9999") {
			if got := cqrs.SubjectFromStr(e.subject).Subject(); got == "" {
				t.Errorf("%s: %q is not a usable subject", c.name, e.subject)
			}
			if n := len(strings.Split(e.subject, ".")); n < 5 {
				t.Errorf("%s: subject %q has %d parts, want at least 5 "+
					"(domain.year.kind.id.verb)", c.name, e.subject, n)
			}
			if !strings.HasPrefix(e.subject, "NATHEJK.") {
				t.Errorf("%s: subject %q is not in the NATHEJK domain", c.name, e.subject)
			}
		}
	}
}

// Every seeded phone number shares a recognisable prefix.
//
// Not a correctness requirement — the year scoping already makes collision with a real
// member impossible — but a legibility one. The whole point of this tool's naming is that
// somebody staring at one row can tell it is synthetic, and a phone number is often all
// that is on screen (a login form, an SMS log line).
func TestSeededPhoneNumbersAreObviouslySynthetic(t *testing.T) {
	const prefix = "+4599"
	checked := 0

	for _, c := range allCases() {
		for _, e := range c.events("9999") {
			for _, phone := range phonesIn(e.body) {
				// Guardian fields deliberately carry unusable junk in one case; that
				// is the point of it, so exempt anything that is not a plain number.
				if !strings.HasPrefix(phone, "+45") {
					continue
				}
				checked++
				if !strings.HasPrefix(phone, prefix) {
					t.Errorf("%s: phone %q does not start with %s, so it does not read "+
						"as synthetic", c.name, phone, prefix)
				}
			}
		}
	}

	// Without this the test passes when the extractor finds nothing at all, which is the
	// most likely way for it to break.
	if checked < 15 {
		t.Fatalf("only found %d phone numbers to check; the extractor is probably broken", checked)
	}
}

// phonesIn pulls phone-ish strings out of a body without knowing its concrete type.
func phonesIn(body any) []string {
	return fieldsMatching(body, "phone", "phonecontact")
}

func namesIn(body any) []string {
	return fieldsMatching(body, "name")
}

// fieldsMatching returns the string values of the named fields (case-insensitively) from
// a struct or a map[string]any.
//
// Reflection rather than a type switch per message: the catalogue mixes shared-go structs
// with maps, the string-ish fields are named types (types.PhoneNumber and friends) rather
// than plain strings, and a switch would need extending every time a case is added — which
// is exactly when the check would be silently skipped.
func fieldsMatching(body any, names ...string) []string {
	wanted := map[string]bool{}
	for _, n := range names {
		wanted[n] = true
	}

	var out []string
	v := reflect.ValueOf(body)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Map:
		for _, key := range v.MapKeys() {
			if key.Kind() != reflect.String || !wanted[strings.ToLower(key.String())] {
				continue
			}
			if s, ok := asString(v.MapIndex(key)); ok && s != "" {
				out = append(out, s)
			}
		}
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			if !wanted[strings.ToLower(t.Field(i).Name)] {
				continue
			}
			if s, ok := asString(v.Field(i)); ok && s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// asString unwraps a value that is a string or a named string type, and also an `any`
// holding one — which is what a map[string]any yields.
func asString(v reflect.Value) (string, bool) {
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	if v.Kind() != reflect.String {
		return "", false
	}
	return v.String(), true
}

// Names must be unmistakably fake. This is the other half of the legibility rule, and it
// is easy to forget when adding a case in a hurry.
func TestSeededNamesAreObviouslyFake(t *testing.T) {
	checked := 0

	for _, c := range allCases() {
		for _, e := range c.events("9999") {
			for _, name := range namesIn(e.body) {
				checked++
				if !strings.HasPrefix(name, "TEST ") {
					t.Errorf("%s: name %q does not start with \"TEST \"; realistic-looking "+
						"fake data gets mistaken for real", c.name, name)
				}
			}
		}
	}

	if checked < 12 {
		t.Fatalf("only found %d names to check; the extractor is probably broken", checked)
	}
}

func TestCaseNamesAreUniqueAndSorted(t *testing.T) {
	cases := allCases()
	seen := map[string]bool{}
	prev := ""

	for _, c := range cases {
		if seen[c.name] {
			t.Errorf("duplicate case name %q — -case would only reach one of them", c.name)
		}
		seen[c.name] = true
		if c.name < prev {
			t.Errorf("case %q is out of order after %q; -list should be stable", c.name, prev)
		}
		prev = c.name
		if c.what == "" {
			t.Errorf("case %q has no description, so -list cannot explain it", c.name)
		}
		if len(c.events("9999")) == 0 {
			t.Errorf("case %q publishes nothing", c.name)
		}
	}
}
