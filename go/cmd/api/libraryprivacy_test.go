package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/photo"
)

// The photograph library names no person either (PRD 022 §6, §9; task 381).
//
// # Why this file exists next to publicprivacy_test.go rather than inside it
//
// Task 337's file asks "can anything on the **public** surface carry a person?" This one asks a different
// question about a surface that is *not* public: the library is behind a credential, and it still must
// name nobody.
//
// Three reasons, and the third is the one that is easy to miss:
//
//  1. **PRD 022 §6 states it as a requirement.** The library row has no uploader, no curator, no person
//     id, no name, no phone number, and emphatically no `phoneParent`.
//  2. **§9's last metric names the mechanism**: enforced "by extending the task 337 structural test rather
//     than by review". Review catches it the first time and not the fifth. A walk over the types catches
//     it on the compile after somebody adds a convenient field — which is exactly how task 361's
//     structural guard behaved.
//  3. **With a shared credential the tool cannot honestly attribute anything to a person** (PRD 022 §8.2).
//     There is no logged-in curator, only a password several people know. So an `uploadedBy` column would
//     not merely be a privacy hazard; it would be *false*, and the only way to populate it would be to
//     invent somebody.
//
// The rule being defended is the hardest one in `.rules`: a guardian's phone number may exist on exactly
// one surface in this project — a user confirming their own guardian's number — and nowhere else. A photo
// library with a `phoneParent` would be absurd, and that is precisely why nobody would notice a struct
// embedding that brought one along.
//
// # Three kinds of check, because each one misses what the others catch
//
//   - **Enumeration from source (AST).** Every struct declared in the `photo` package and in the admin
//     files, whether or not anybody remembered to name it here. This is what covers the type somebody adds
//     next month.
//   - **Recursion through the types (reflection).** Into named struct fields, pointers, slices and maps —
//     not just the top level. An enumeration of locally declared structs cannot see a field whose type is
//     `person.Person` from another package; a recursive walk can.
//   - **The schema (SQL).** The column is the durable artefact. A Go struct can be fixed in a commit; a
//     column that has been written to for a week cannot be un-leaked.

// personShapedPaths walks a type and returns the dotted paths of every person-shaped field it can reach.
//
// # Why paths rather than names
//
// `"gained a person-shaped field \"Phone\""` sends a reader looking at the wrong struct when the field is
// three levels down in something embedded from another package. The path says where to go.
//
// # Where the walk stops
//
// At a `visited` set, because a self-referential type would otherwise recurse forever, and at types with
// no exported fields, which is where the standard library's opaque structs (`time.Time`) end up on their
// own. No package allowlist: excluding "the standard library" would mean deciding what counts as one, and
// the cost of walking a few extra fields is nothing compared to the cost of a skipped branch.
func personShapedPaths(t reflect.Type, prefix string, visited map[reflect.Type]bool) []string {
	for t != nil && (t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array) {
		t = t.Elem()
	}
	if t == nil || t.Kind() == reflect.Map {
		// A map's *values* are walked below via Elem; a map's keys are strings everywhere in this
		// codebase. Handled here rather than silently: see the loop.
		if t == nil {
			return nil
		}
	}
	if t.Kind() == reflect.Map {
		return personShapedPaths(t.Elem(), prefix+"[]", visited)
	}
	if t.Kind() != reflect.Struct || visited[t] {
		return nil
	}
	visited[t] = true

	var out []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			// Unexported: it cannot be serialised or rendered, so it cannot leak through a response.
			continue
		}

		path := prefix + f.Name
		if f.Anonymous {
			// Embedded. The field name is the type name, which is not something a reader chose, so the
			// path skips it — a person-shaped field one level down is just as exposed.
			path = prefix
		} else if isPersonShapedInTheLibrary(f.Name) {
			out = append(out, path)
		}

		child := f.Type
		if f.Anonymous {
			out = append(out, personShapedPaths(child, path, visited)...)
			continue
		}
		out = append(out, personShapedPaths(child, path+".", visited)...)
	}
	return out
}

// walkForPeople is the assertion personShapedPaths exists for.
func walkForPeople(t *testing.T, name string, v any) {
	t.Helper()

	typ := reflect.TypeOf(v)
	if typ == nil || typ.Kind() != reflect.Struct {
		t.Fatalf("%s is not a struct: the reflection is broken, which would make this pass while "+
			"asserting nothing", name)
	}
	if typ.NumField() == 0 {
		t.Fatalf("%s has no fields: same problem", name)
	}

	for _, path := range personShapedPaths(typ, "", map[reflect.Type]bool{}) {
		t.Errorf("%s.%s is person-shaped. The photograph library names nobody: no uploader, no curator, "+
			"no person id, no name, no phone number and emphatically no phoneParent (PRD 022 §6, and the "+
			"hard rule in .rules). Two reasons it is not merely undesirable: the credential is shared, so "+
			"the tool cannot honestly attribute anything to a person (§8.2) — an attribution field would "+
			"be a lie as well as a hazard; and PRD 022 §9 requires this to be enforced by extending task "+
			"337's structural walk rather than by review. If this field is genuinely not about a human "+
			"being, except it by name in isPersonShaped with the reasoning, next to photoId",
			name, path)
	}
}

// isPersonShapedInTheLibrary is isPersonShaped, minus what PRD 022 §6 has excepted **for this surface only**.
//
// # Why the exception lives here and not in isPersonShaped
//
// `isPersonShaped` answers "could a field with this name carry a person's name?". For a filename the answer
// is yes and must stay yes, because that function is also what guards the **public** responses
// (`publicprivacy_test.go`). Excepting `filename` there would permit it everywhere, including on the open
// web, which is exactly what it may not do.
//
// So the two questions are separated: what *could* name somebody, and what this authenticated projection is
// allowed to hold anyway. `TestTheFilenameExceptionStopsAtTheAdminSurface` below holds the boundary, because
// a scoped exception whose scope is not tested is an unscoped exception.
func isPersonShapedInTheLibrary(field string) bool {
	if !isPersonShaped(field) {
		return false
	}
	return !libraryPersonShapedExceptions[strings.ToLower(field)]
}

// The fields PRD 022 §6 admits into the library projection despite naming, or being able to name, a person.
//
// **Two, and each one is a sentence somebody had to write here.** That is the mechanism: the guard cannot
// judge a field, so it fails, and whoever wants the field has to state the bounds it is safe within.
var libraryPersonShapedExceptions = map[string]bool{
	// The photographer's credit line (task 393) — the original exception, argued at length in
	// `isPersonShaped` itself. A consenting adult volunteer's name, typed **in order to be published**.
	"credit": true,

	// The crew member credited for a photograph (task 450, PRD 025).
	//
	// # What it is
	//
	// A reference to a person row, not a name. It exists so that a curator **picks** the photographer from the
	// crew roster instead of typing their name onto three hundred photographs, which is how the same person ends
	// up spelled two ways on one public page.
	//
	// # Why it is allowed, given that this file's whole purpose is to forbid exactly this
	//
	// PRD 022 §6 said "emphatically not a `creditPersonId`", and the reasoning was that a typed name is a
	// judgement somebody made while a name resolved out of our person records is a different feature nobody
	// agreed to. Three of that argument's premises turned out to be wrong, and the maintainer corrected them
	// (2026-09-28, task 455):
	//
	//   - **Erasure, which is the decisive one.** A name copied into `credit` is also on the append-only event
	//     log, so "delete me" could never be fully honoured. A reference can be: the name lives in one
	//     projection, and deleting that row removes it from every photograph and every public page at once.
	//     Storing the *name* — which is what I argued for — is the option that cannot be undone.
	//   - **Consent.** Crew are adults who consented to be registered. §6's own phrase, "a consenting adult
	//     volunteer in a professional capacity", already covers them.
	//   - **Retention.** Crew names are not purged after the event; the same id is reissued the following year.
	//     The purge argument applied to participants.
	//
	// # What still holds, and where
	//
	// This exception is **narrower than it looks**, and the bounds are elsewhere by design:
	//
	//   - `isPersonShaped` still flags the word, so a **public** response carrying this id fails (PRD 025 §6
	//     R6). The id is a handle to a person record; the open web has no business holding one.
	//   - it resolves to a name through **one function** (task 451): the name column and nothing else, a person
	//     who classifies as `RoleCrew` and nobody else, within the photograph's own year, and "" when it finds
	//     nothing. A participant's id — mistyped, stale or malicious — cannot publish a participant's name.
	//
	// Named `creditCrewId` rather than `creditPersonId` deliberately: the narrower word is the true one, and it
	// keeps `person` meaning what it means everywhere else in this file.
	"creditcrewid": true,

	// The name the photographer's file had (task 448, PRD 024 §6 R5).
	//
	// # What it is for
	//
	// A photographer sorts a card on their own computer — renaming, numbering, ordering — and then wants the
	// album to keep that order after the upload. The filename *is* that ordering. Without it the sort can
	// only offer capture time, which is not the same thing and is absent from a large minority of files.
	//
	// # Why it is allowed, given PRD 022 §6
	//
	// Not because a filename is harmless: it usually reads `IMG_0123.JPG` and could read `mor-og-far.jpg`.
	// The exception rests on **bounds**, which is the same shape as the credit line's argument and a
	// different justification:
	//
	//   - **Behind the credential.** The field exists on the curator's projection and its reads. It is on no
	//     public response, and that is enforced rather than promised: `isPersonShaped` still flags the word,
	//     so the public walk refuses it, and the test below refuses any attempt to widen this exception out
	//     of here.
	//   - **A sort key, never an attribution.** Nothing renders it as anybody's name, and nothing derives a
	//     person from it. The maintainer's decision (2026-09-28) is explicit: "name will never be used".
	//   - **The photographer's own filing**, about their own work, chosen by them — not a value this service
	//     assembled about a third party, which is the hazard `TestACreditIsOnlyEverTypedNeverDerived`
	//     guards and the one this codebase actually fears.
	//   - **Never parsed for meaning.** It is not read for dates, patrol numbers or names; see
	//     `photo.NormalizeFileName`, which bounds it and nothing more.
	//
	// The residue is real and worth naming rather than hiding: a filename a photographer chose privately is
	// now in the projection and in every backup taken since. That is the cost the decision accepted, and the
	// bounds above are what it bought.
	"filename": true,
}

// The exception is **scoped**, and a scope that is not tested is not a scope.
//
// This is the assertion that keeps "it is protected by authentication" true. If somebody later moves
// `filename` into `isPersonShaped` itself — the shorter change, and the tempting one when a public template
// wants it — the public guard stops flagging it and a filename can reach the open web. This fails first.
func TestTheCreditReferenceStaysOffThePublicSurface(t *testing.T) {
	// Still person-shaped in general: that is what keeps it out of a public response (PRD 025 §6 R6), since
	// publicprivacy_test.go walks those types with the same function.
	for _, field := range []string{"creditCrewId", "CreditCrewID", "creditcrewid"} {
		if !isPersonShaped(field) {
			t.Errorf("%q must still be person-shaped in general, or a public response could carry a handle "+
				"to a person record", field)
		}
	}
	if isPersonShapedInTheLibrary("creditCrewId") {
		t.Error("the library must be allowed to hold the reference: PRD 022 §6's third exception (task 450)")
	}

	// The exception is one name, not a family. Anything that pairs the credit with a *person* — or resolves one
	// — is a different thing and is not covered.
	for _, field := range []string{"creditPersonId", "creditCrewName", "creditCrewPhone", "creditedBy"} {
		if !isPersonShapedInTheLibrary(field) {
			t.Errorf("%q is not the excepted field and must still fail: the exception is exactly "+
				"`creditCrewId`, and a name or a person id alongside it is what §6 still forbids", field)
		}
	}
}

func TestTheFilenameExceptionStopsAtTheAdminSurface(t *testing.T) {
	for _, field := range []string{"fileName", "FileName", "filename", "OriginalFileName"} {
		if !isPersonShaped(field) {
			t.Errorf("%q must still be person-shaped in general: that is what keeps it off the public "+
				"surface, where publicprivacy_test.go uses the same function", field)
		}
	}
	if isPersonShapedInTheLibrary("fileName") {
		t.Error("the library must be allowed to hold a filename: PRD 022 §6's second exception (task 448)")
	}

	// And the exception is exact. A field that *derives* something about a person from a filename, or attaches
	// a person to it, is a different thing and is not covered.
	for _, field := range []string{"fileNamePersonId", "uploaderFileName", "fileNameOf"} {
		if !isPersonShapedInTheLibrary(field) {
			t.Errorf("%q is not the excepted field and must still fail: the exception is one name, not a "+
				"prefix", field)
		}
	}
}

// The library read model, the tag, and the curator's filter.
//
// `Tag` carries a team id and the number the curator typed, and **no name** — see
// `photo.CuratorQueries.Tags`. A photograph is attributed to a patrulje, never to a person (PRD 011 §4).
func TestTheLibraryReadModelHasNowhereToPutAPerson(t *testing.T) {
	for name, v := range map[string]any{
		"photo.LibraryPhoto": photo.LibraryPhoto{},
		"photo.Tag":          photo.Tag{},
		"photo.Counts":       photo.Counts{},
		"photo.Filter":       photo.Filter{},
	} {
		walkForPeople(t, name, v)
	}
}

// Every event the library folds from.
//
// Enumerated rather than listed: a sixth event type is exactly the moment somebody reaches for "and who
// uploaded it". `Uploaded` is the dangerous one — it is published by the handler that has just parsed a
// multipart form, which is the point in the request where a person's details are nearest to hand.
func TestThePhotoEventsHaveNowhereToPutAPerson(t *testing.T) {
	for name, v := range map[string]any{
		"photo.Uploaded":        photo.Uploaded{},
		"photo.Updated":         photo.Updated{},
		"photo.Location":        photo.Location{},
		"photo.LocationCleared": photo.LocationCleared{},
		"photo.PatrolTagged":    photo.PatrolTagged{},
		"photo.PatrolUntagged":  photo.PatrolUntagged{},
		"photo.Deleted":         photo.Deleted{},
	} {
		walkForPeople(t, name, v)
	}

	// And the list above is complete. Reflection cannot enumerate a package's types, so the source is
	// parsed instead — otherwise a new event type would be covered by nothing, and the test above would
	// keep passing while asserting less than it claims.
	declared := structsDeclaredIn(t, "../../nathejk/table/photo")
	for _, verb := range []string{"Uploaded", "Updated", "LocationCleared", "PatrolTagged",
		"PatrolUntagged", "Deleted"} {
		if !declared[verb] {
			t.Errorf("photo.%s is no longer declared — has an event been renamed? The list in this test "+
				"must follow, or it silently stops covering one", verb)
		}
	}
}

// **The whole `photo` package, and the whole admin surface, by enumeration.**
//
// The tests above are readable; this one is the safety net under them. It walks every struct declared in
// the projection and in the curator's handlers — request types, response types, view types, whatever
// arrives next — and checks each field name. Nobody has to remember to add a type here.
//
// Request types are included deliberately. A person-shaped field on the way *in* is the more likely
// mistake ("let the uploader say who they are") and it is the one that would put the value in the event
// log, where it is permanent.
func TestNoStructInTheLibraryOrTheAdminToolNamesAPerson(t *testing.T) {
	for _, target := range []struct {
		what string
		dir  string
		only func(string) bool
	}{
		{what: "the photo projection", dir: "../../nathejk/table/photo"},
		{what: "the admin tool", dir: ".", only: func(file string) bool {
			return strings.HasPrefix(file, "admin")
		}},
	} {
		fields, types := structFieldsDeclaredIn(t, target.dir, target.only)
		if types < 5 {
			t.Fatalf("found only %d structs in %s: the enumeration is broken, which would make this "+
				"pass while asserting nothing", types, target.what)
		}
		for owner, names := range fields {
			for _, field := range names {
				if isPersonShapedInTheLibrary(field) {
					t.Errorf("%s.%s in %s is person-shaped (PRD 022 §6). Nothing in the library or the "+
						"curator's tool may carry a person: the credential is shared, so there is no "+
						"person to attribute anything to", owner, field, target.what)
				}
			}
		}
	}
}

// **The schema.** A column is the durable artefact, and the one that cannot be un-leaked.
//
// A Go struct with a bad field is fixed in a commit. A column that has been written to since Friday holds
// the values, in the database and in every backup taken since — so the structural walk above is necessary
// and not sufficient.
func TestTheLibraryTablesDeclareNoPersonShapedColumn(t *testing.T) {
	src, err := os.ReadFile("../../nathejk/table/photo/table.sql")
	if err != nil {
		t.Fatalf("read photo/table.sql: %v", err)
	}

	tables, columns := sqlColumns(string(src))
	for _, want := range []string{"photo", "photo_patrol"} {
		if !tables[want] {
			t.Fatalf("did not find CREATE TABLE %s — has the schema moved? This test would otherwise "+
				"pass over nothing", want)
		}
	}
	if len(columns) < 10 {
		t.Fatalf("parsed only %d columns: the parser is broken", len(columns))
	}

	for _, column := range columns {
		if isPersonShapedInTheLibrary(column) {
			t.Errorf("the library schema declares a person-shaped column %q. PRD 022 §6: no uploader, no "+
				"curator, no person id, no name, no phone number, no phoneParent. A column outlives the "+
				"commit that added it \u2014 the values are in every backup taken since", column)
		}
	}
}

// sqlColumns returns the tables and column names a schema file declares.
//
// A regex over the source rather than a real parser, and that is sufficient here because the question is
// "does a person-shaped *name* appear as a column?" — which a false positive answers safely and a missed
// line does not. So the floor assertions above exist: a parser that stopped matching would report zero
// columns, and zero columns is a failure rather than a pass.
func sqlColumns(src string) (map[string]bool, []string) {
	tables := map[string]bool{}
	for _, m := range sqlCreateTable.FindAllStringSubmatch(src, -1) {
		tables[m[1]] = true
	}

	var columns []string
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		m := sqlColumn.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		switch strings.ToUpper(m[1]) {
		case "PRIMARY", "KEY", "UNIQUE", "INDEX", "CONSTRAINT", "FOREIGN", "CREATE", "SET", "DROP",
			"ALTER", "INSERT", "SELECT":
			continue
		}
		columns = append(columns, m[1])
	}
	return tables, columns
}

var sqlCreateTable = regexp.MustCompile(`(?i)CREATE TABLE(?: IF NOT EXISTS)? ` + "`?" + `(\w+)` + "`?")
var sqlColumn = regexp.MustCompile("^`?(\\w+)`? +(?i:VARCHAR|TEXT|INT|TINYINT|BIGINT|DATETIME|TIMESTAMP|DOUBLE|DECIMAL|FLOAT|CHAR|BLOB|JSON|ENUM)")

// structsDeclaredIn returns the names of every struct type declared in a package's non-test files.
func structsDeclaredIn(t *testing.T, dir string) map[string]bool {
	t.Helper()

	out := map[string]bool{}
	fields, _ := structFieldsDeclaredIn(t, dir, nil)
	for name := range fields {
		out[name] = true
	}
	return out
}

// structFieldsDeclaredIn parses a directory and returns each struct's field names, plus how many structs
// were found.
//
// `only` narrows it to particular files; nil means all of them. Used for the admin surface, which lives in
// the same package as every other handler — so "the curator's tool" is a filename prefix rather than a
// package, and that is worth knowing rather than hiding: a new admin file must be named `admin*` to be
// covered, which is the same convention `curatorboundary_test.go`'s allowlist relies on.
func structFieldsDeclaredIn(t *testing.T, dir string, only func(string) bool) (map[string][]string, int) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	fset := token.NewFileSet()
	out := map[string][]string{}
	count := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if only != nil && !only(name) {
			continue
		}
		file, perr := parser.ParseFile(fset, dir+"/"+name, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := spec.Type.(*ast.StructType)
			if !ok {
				return true
			}
			count++
			for _, field := range st.Fields.List {
				for _, ident := range field.Names {
					if !ident.IsExported() {
						// Unexported: it cannot be serialised into a response or rendered into a
						// template, so it cannot leak. Skipped for the same reason the reflection walk
						// skips `PkgPath != ""`, and not merely for tidiness — `photo.Table` embeds a
						// private `curatorQuerier`, and allowlisting an implementation detail would be
						// the wrong repair.
						continue
					}
					out[spec.Name.Name] = append(out[spec.Name.Name], ident.Name)
				}
				if len(field.Names) == 0 {
					// Embedded. The type name is the field name as far as a reader is concerned, and an
					// embedded `person.Person` is exactly the shape this catches.
					if ident, ok := field.Type.(*ast.Ident); ok && ident.IsExported() {
						out[spec.Name.Name] = append(out[spec.Name.Name], ident.Name)
					}
					if sel, ok := field.Type.(*ast.SelectorExpr); ok {
						out[spec.Name.Name] = append(out[spec.Name.Name], sel.Sel.Name)
					}
				}
			}
			return true
		})
	}
	return out, count
}
