package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/internal/commands"
	"nathejk.dk/nathejk/table/pagereport"
)

// The takedown route on a patrol's public page (task 343).
//
// Most of this file is about what the route must *not* do: hide the page, store an address, or answer
// differently for a patrol whose page is closed than for one that does not exist.

// reportApp is the patrol page app with a recording publisher installed.
func reportApp(t *testing.T) (*application, *cqrstest.Publisher, *httptest.Server) {
	t.Helper()

	app, _, srv := patrolPageApp(t)

	pub := &cqrstest.Publisher{}
	holder := commands.NewPublisherHolder()
	holder.Set(pub)
	app.commands = commands.New(holder)

	return app, pub, srv
}

// postReport submits the form the way a browser with no JavaScript does, and does **not** follow the
// redirect — the redirect itself is part of what is being asserted.
func postReport(t *testing.T, base, number, reason string) *http.Response {
	t.Helper()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.PostForm(base+"/2026/patrulje/"+number+"/anmeld",
		url.Values{"reason": {reason}})
	if err != nil {
		t.Fatalf("POST report: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func decodeReport(t *testing.T, msg interface{ Body(any) error }) pagereport.Reported {
	t.Helper()
	var out pagereport.Reported
	if err := msg.Body(&out); err != nil {
		t.Fatalf("decoding the report event: %v", err)
	}
	return out
}

func TestPatrolReport_PublishesAReportAndRedirectsBack(t *testing.T) {
	_, pub, srv := reportApp(t)

	resp := postReport(t, srv.URL, "42", "Ruten er ikke vores")

	// **303, not 200.** POST-redirect-GET, so a reload does not file the report again.
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", resp.StatusCode)
	}
	if got, want := resp.Header.Get("Location"), "/2026/patrulje/42?anmeldt=1"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}

	if len(pub.Messages) != 1 {
		t.Fatalf("published %d events, want 1 (%v)", len(pub.Messages), pub.Subjects())
	}
	if got, want := pub.Subjects()[0], "NATHEJK.2026.pagereport."; !strings.HasPrefix(got, want) {
		t.Errorf("subject = %q, want a %s… subject", got, want)
	}

	body := decodeReport(t, pub.Messages[0])
	if body.Page != pagereport.PagePatrol {
		t.Errorf("page = %q, want %q", body.Page, pagereport.PagePatrol)
	}
	// The **number**, which is what the reporter was looking at and what an organizer searches for — not
	// the internal team id, which would need a join to be useful.
	if body.Ref != "42" {
		t.Errorf("ref = %q, want the patrol number \"42\"", body.Ref)
	}
	if body.Reason != "Ruten er ikke vores" {
		t.Errorf("reason = %q", body.Reason)
	}
	if body.ReportID == "" {
		t.Error("a report with no id cannot be addressed or deduplicated")
	}
	if body.ReportedAt.IsZero() {
		t.Error("reportedAt must be stamped by the publisher, or a replay dates every report to the deploy")
	}
	if time.Since(body.ReportedAt) > time.Minute {
		t.Errorf("reportedAt = %v, which is not now", body.ReportedAt)
	}
}

// **No address, anywhere in the event.** The one identifier this surface could collect about somebody
// outside the app is the reporter's IP, and PRD 019 refused it for this table shape. Asserted against the
// *serialised* event rather than the struct, so a field added later is covered too.
func TestPatrolReport_RecordsASentinelAndNoAddress(t *testing.T) {
	_, pub, srv := reportApp(t)

	postReport(t, srv.URL, "42", "noget galt")

	body := decodeReport(t, pub.Messages[0])
	if body.ReporterPersonID != pagereport.ReporterSentinel {
		t.Errorf("reporter = %q, want the sentinel %q", body.ReporterPersonID, pagereport.ReporterSentinel)
	}

	// Serialised rather than field-by-field, so a field added to the event later is covered too.
	raw, merr := json.Marshal(pub.Messages[0].RawBody())
	if merr != nil {
		t.Fatalf("marshalling the event body: %v", merr)
	}
	for _, forbidden := range []string{"127.0.0.1", "::1", "ip", "userAgent", "referer"} {
		if strings.Contains(strings.ToLower(string(raw)), strings.ToLower(forbidden)) {
			t.Errorf("the report event contains %q\n%s", forbidden, raw)
		}
	}
}

// **The report does not hide the page.** The decision recorded in patrolreport.go: a patrol page is the
// record of a whole patrol's night, and one anonymous request must not remove it from the open web. So the
// page is unchanged afterwards, and nothing but the report event is published.
func TestPatrolReport_HidesNothing(t *testing.T) {
	_, pub, srv := reportApp(t)

	_, before := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	postReport(t, srv.URL, "42", "tag den ned")
	afterResp, after := getPublic(t, srv.URL+"/2026/patrulje/42", nil)

	if afterResp.StatusCode != http.StatusOK {
		t.Fatalf("the page answered %d after a report; it must not be hidden", afterResp.StatusCode)
	}
	if !strings.Contains(string(after), "Ørnene") {
		t.Error("the patrol's page lost its content after a report")
	}
	if len(before) != len(after) {
		t.Error("the page changed after a report; a report notifies, it does not act")
	}

	// One event, and it is the report. Nothing that could hide or alter the patrol.
	if len(pub.Messages) != 1 {
		t.Fatalf("published %v, want only the report", pub.Subjects())
	}
	if strings.Contains(pub.Subjects()[0], "hidden") || strings.Contains(pub.Subjects()[0], "patrulje") {
		t.Errorf("the report published %q, which is not a report", pub.Subjects()[0])
	}
}

// **A closed patrol answers exactly like a number that does not exist.** Otherwise this route becomes the
// discovery oracle the whole surface is built to avoid being: "has 43 finished?" answered by whether the
// report was accepted.
func TestPatrolReport_ClosedAndUnknownAnswerAlike(t *testing.T) {
	_, pub, srv := reportApp(t)

	closedResp := postReport(t, srv.URL, "43", "hvorfor")
	unknownResp := postReport(t, srv.URL, "999", "hvorfor")
	rubbishResp := postReport(t, srv.URL, "ikke-et-nummer", "hvorfor")

	for _, resp := range []*http.Response{closedResp, unknownResp, rubbishResp} {
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want the not-yet page's 200", resp.StatusCode)
		}
	}
	if got := len(pub.Messages); got != 0 {
		t.Errorf("published %d events for pages that are not open (%v)", got, pub.Subjects())
	}
}

func TestPatrolReport_IsRateLimitedByIP(t *testing.T) {
	app, _, srv := reportApp(t)
	app.publicReportLimiter = limiterOrNil(2, time.Hour)

	for i := 0; i < 2; i++ {
		if resp := postReport(t, srv.URL, "42", "igen"); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("report %d: status = %d, want 303", i+1, resp.StatusCode)
		}
	}
	if resp := postReport(t, srv.URL, "42", "igen"); resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 past the limit", resp.StatusCode)
	}
}

// An over-long reason is **truncated, not refused**: losing somebody's complaint to protect a column width
// is the wrong trade, and the form is the one place on this surface a stranger can type freely.
func TestPatrolReport_TruncatesAnOverlongReason(t *testing.T) {
	_, pub, srv := reportApp(t)

	long := strings.Repeat("æ", maxPatrolReportReason+500)
	if resp := postReport(t, srv.URL, "42", long); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 — an over-long report is still a report", resp.StatusCode)
	}

	body := decodeReport(t, pub.Messages[0])
	if got := len([]rune(body.Reason)); got != maxPatrolReportReason {
		t.Errorf("reason is %d runes, want it truncated to %d", got, maxPatrolReportReason)
	}
}

// An empty reason is accepted. Requiring an explanation is a way of receiving fewer reports, and the ones
// it filters out are not the ones we can afford to lose.
func TestPatrolReport_AcceptsNoReason(t *testing.T) {
	_, pub, srv := reportApp(t)

	if resp := postReport(t, srv.URL, "42", ""); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", resp.StatusCode)
	}
	if len(pub.Messages) != 1 {
		t.Fatalf("a wordless report was not filed")
	}
}

// **The form is on the page and needs no JavaScript**: a method=post form with a real action, and the
// acknowledgement arrives as a query parameter after the redirect rather than from anything stored.
func TestPatrolPage_CarriesAWorkingTakedownForm(t *testing.T) {
	_, _, srv := reportApp(t)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	page := string(body)

	for _, want := range []string{
		`<form method="post" action="/2026/patrulje/42/anmeld">`,
		`name="reason"`,
		`<button type="submit">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the takedown form is missing %s", want)
		}
	}
	// No acknowledgement before anything was reported.
	if strings.Contains(page, "Tak. Vi har fået din besked") {
		t.Error("the page thanks a visitor who reported nothing")
	}

	_, reported := getPublic(t, srv.URL+"/2026/patrulje/42?anmeldt=1", nil)
	if !strings.Contains(string(reported), "Tak. Vi har fået din besked") {
		t.Error("the page does not acknowledge a report after the redirect")
	}
}

// The promise in the footer (task 331) and the form must not contradict each other: neither may say the
// page comes down immediately, because it does not.
func TestPatrolPage_TakedownCopyDoesNotPromiseImmediateRemoval(t *testing.T) {
	_, _, srv := reportApp(t)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42?anmeldt=1", nil)
	page := strings.ToLower(string(body))

	for _, forbidden := range []string{"med det samme", "straks", "fjernet nu"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the copy promises %q, but a report notifies rather than removing", forbidden)
		}
	}
}
