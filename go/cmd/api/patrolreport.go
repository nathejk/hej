package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/commands"
	"nathejk.dk/nathejk/table/pagereport"
)

// The takedown route on a patrol's public page (PRD 011 §6, task 343).
//
// # Why a public page about children needs a way to write back
//
// Not because the design is in doubt: PRD 011 §0b.1 settled that a patrol's merged route may be public and
// the right to publish it was obtained. This exists because a page can be **wrong** — a scan attributed to
// the wrong patrol, a track that is not theirs, a name that changed — and because a patrol may have a
// reason nobody anticipated. The footer has promised "Skriv til os, så tager vi det ned" since task 331;
// until this handler there was nothing behind the promise.
//
// # Notify, not auto-hide. The decision and why
//
// A reported **glimt** is hidden immediately, before any human looks (PRD 019). A reported **patrol page**
// is not, and the asymmetry is deliberate:
//
//  1. *The subject is different.* A glimt is one photograph from one member and hiding it costs that member
//     one photograph. A patrol page is the record of eight people's night, and it is also the page their
//     grandparents were sent a link to.
//  2. *The abuse is trivial and the cost asymmetric.* Anyone can POST here. Auto-hide would mean one
//     anonymous stranger can remove any patrol's page from the open web, repeatedly, with no account and
//     nothing to revoke — and the patrol whose page vanished has no way to tell that it was not us.
//  3. *There is no undo path.* Un-hiding a glimt is a moderation action the app already has a surface for.
//     Un-hiding a patrol page is not, so an auto-hide would be a one-way door with an anonymous handle.
//  4. *The urgency is lower than it looks.* The page names no person (task 337). The harm a report is most
//     likely to describe — a wrong attribution — is a correction, not an exposure, and a correction that
//     takes an hour is fine.
//
// **What makes notify-only acceptable is that somebody reads it.** Organizers read `public_page_report`
// out of band; if that stops being true, this decision stops being defensible and auto-hide becomes the
// lesser evil. That is the condition to watch, and it is a process condition rather than a code one.
//
// # No CSRF token, deliberately
//
// There is no session here, so there is no authority for a forged request to borrow: a cross-site POST can
// file a report that anyone could have filed by hand. A token would also break the one property that
// matters most on this form — that it works with JavaScript off — for no gain.

// maxPatrolReportReason bounds the free text.
//
// Generous, because somebody explaining why a page about their children should come down should not hit a
// limit mid-sentence. Bounded at all because it is an anonymous write into an append-only table.
const maxPatrolReportReason = 2000

// reportPatrolPageHandler receives a takedown report from a patrol's public page.
//
// @Summary      Report a patrol's public page (no login)
// @Description  Files an anonymous report that a patrol's public page should not be up — a wrong attribution, a route that is not theirs, or a reason nobody anticipated. A **form POST**, so it works with JavaScript disabled, and it answers with a redirect back to the page rather than JSON. Unlike a reported glimt this **does not hide anything**: a patrol page is the record of a whole patrol's night, and letting one anonymous request remove it would be trivially abusable — organizers read the reports instead. The reporter's IP is not stored; a sentinel stands in for them. Rate-limited by IP. A report about a patrol whose page is not open answers exactly like one about a number that does not exist.
// @Tags         public-site
// @Accept       x-www-form-urlencoded
// @Produce      html
// @Param        number  path      string  true   "patrol number"
// @Param        reason  formData  string  false  "what is wrong"
// @Success      303  "redirect back to the page"
// @Failure      429  {object}  map[string]string  "report rate limit, by IP"
// @Failure      503  {object}  map[string]string  "event stream unavailable — retry"
// @Router       /offentligt/patrulje/{number}/anmeld [post]
func (app *application) reportPatrolPageHandler(w http.ResponseWriter, r *http.Request) {
	// By IP, generously, and for the reason PRD 019 recorded: the cost of a spurious report is a
	// moderator's glance, while the cost of a throttled one is a page somebody objected to staying up.
	if app.publicReportLimiter != nil && !app.publicReportLimiter.Allow(clientIP(r)) {
		app.RateLimitMessageResponse(w, r, "For mange anmeldelser. Prøv igen om lidt.")
		return
	}

	number, ok := normalizePatrolNumber(httprouter.ParamsFromContext(r.Context()).ByName("number"))
	if !ok {
		app.renderPatrolNotYet(w)
		return
	}

	// **The same gate as the page, for the same reason.** Reporting is only possible where the page is
	// visible — otherwise this route would answer differently for a real closed patrol than for a number
	// that does not exist, and become the discovery oracle the whole surface avoids being (task 330).
	patrol, _, open := app.openPatrol(number)
	if !open {
		app.renderPatrolNotYet(w)
		return
	}

	reason := strings.TrimSpace(r.FormValue("reason"))
	if len([]rune(reason)) > maxPatrolReportReason {
		// Truncated rather than refused. An over-long report is still a report, and answering 400 to
		// somebody who wrote too much would lose the complaint to protect a column width.
		reason = string([]rune(reason)[:maxPatrolReportReason])
	}

	reportID := uuid.NewString()
	subject, serr := pagereport.Subject(app.config.eventYear, reportID, pagereport.VerbReported)
	if serr != nil {
		app.ServerErrorResponse(w, r, serr)
		return
	}

	if perr := app.commands.Publish(subject, pagereport.Reported{
		ReportID: reportID,
		Year:     app.config.eventYear,
		Page:     pagereport.PagePatrol,
		// The **number**, not the team id. The number is what the reporter was looking at and what an
		// organizer will search for, and the id is an internal handle that would need a join to be useful.
		Ref:    patrol.Number,
		Reason: reason,
		// A sentinel, never `clientIP(r)`. See table.sql: an IP here would put a personal identifier of
		// somebody outside the app into an append-only table, to solve a duplicate-counting problem that
		// does not matter.
		ReporterPersonID: pagereport.ReporterSentinel,
		ReportedAt:       time.Now().UTC(),
	}); perr != nil {
		if errors.Is(perr, commands.ErrNoPublisher) {
			app.ServiceUnavailableResponse(w, r, "anmeldelsen kunne ikke gemmes, prøv igen")
			return
		}
		app.ServerErrorResponse(w, r, perr)
		return
	}

	// **303 back to the page**, which is the POST-redirect-GET every form should do: a reload must not
	// re-file the report, and the acknowledgement belongs on the page the visitor was reading rather than
	// on a bare confirmation screen they then have to navigate away from.
	http.Redirect(w, r, "/offentligt/patrulje/"+patrol.Number+"?anmeldt=1", http.StatusSeeOther)
}
