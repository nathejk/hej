package main

import (
	"errors"
	"net/http"

	"nathejk.dk/internal/commands"
	"nathejk.dk/nathejk/table/album"
)

// Publishing album events (task 383).
//
// # Why these two functions have their own file
//
// They were written for the dev fixture (`devalbum.go`, task 333) because that was the only thing in the
// app that created an album. PRD 022's curator tool is now the thing that creates albums, and the fixture
// is gone — so these moved out rather than being deleted with it.
//
// Their own file rather than a corner of `adminalbum.go`, for one reason: the curator's tool is not the
// only writer that will ever exist. `albumremove.go`'s in-app takedown publishes too, and the next writer
// will be some surface nobody has thought of. A shared helper sitting inside the admin file would read as
// admin-owned and get a second copy made, which is the shape PRD 022 \u00a78.9 argues against for blob purging
// and the argument is no different here.

// publishAlbum publishes one album event.
//
// One helper so the subject is built in one place: `album.Subject` validates the tokens, and an id with
// a dot in it would publish successfully while quietly never matching the per-album patterns again.
//
// The year is a parameter rather than the configured one since task 392: the curator may be working in another
// year, and an event whose subject names one year while its body names another would fold into neither cleanly.
func (app *application) publishAlbum(year, verb, albumID string, body any) error {
	subject, err := album.Subject(year, albumID, verb)
	if err != nil {
		return err
	}
	return app.commands.Publish(subject, body)
}

// writeAlbumPublishFailure answers a publish error.
//
// A missing broker is a 503 rather than a 500: it is a configuration state, not a fault, and the
// distinction is what tells a developer to start the broker rather than read a stack trace.
func (app *application) writeAlbumPublishFailure(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, commands.ErrNoPublisher) {
		app.ServiceUnavailableResponse(w, r, "eventstrømmen er ikke tilgængelig")
		return
	}
	app.ServerErrorResponse(w, r, err)
}
