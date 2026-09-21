package pagereport

import (
	"time"
)

// The report event shape (PRD 011 §6, task 343).
//
// Owned here rather than in internal/, following the album and glimt packages: this app publishes the
// event and this package consumes it, and one struct that both sides share cannot drift the way two
// structs agreeing by convention on JSON tags eventually do.

// Reported is one visitor saying a page should not be up.
type Reported struct {
	ReportID string `json:"reportId"`
	Year     string `json:"year"`

	// Page is which kind of page, from the closed set below; Ref identifies the page within that kind.
	//
	// Two fields rather than a URL, so a report survives a change of URL scheme — and so nothing has to
	// parse a path to answer "which patrol was this about?".
	Page string `json:"page"`
	Ref  string `json:"ref"`

	// Reason is what the reporter wrote, and may be empty.
	//
	// Optional on purpose. Requiring an explanation is a way of receiving fewer reports, and the ones it
	// would filter out are not the ones we can afford to lose: somebody upset enough to want a page about
	// their children taken down should not have to compose a paragraph first.
	Reason string `json:"reason,omitempty"`

	// ReporterPersonID is a sentinel for an anonymous reporter, never an address.
	//
	// The public site has no session, so there is nobody to key on; the alternative identifier is the IP,
	// and PRD 019 already refused it for this exact table shape. See table.sql.
	ReporterPersonID string `json:"reporterPersonId"`

	ReportedAt time.Time `json:"reportedAt"`
}

// The kinds of page that can be reported.
//
// A closed set, as a constant, because the column is indexed and queried by it: a typo'd value would not
// error, it would file the report where no organizer looks.
const (
	// PagePatrol is a patrol's own public page, addressed by its number.
	PagePatrol = "patrol"
)
