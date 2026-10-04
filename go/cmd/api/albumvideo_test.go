package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/album"

	"github.com/jrgensen/cqrs/cqrstest"
)

// A library video in a public album (PRD 029, task 496): served as video/mp4, with Range, and rebuilt on a miss.

func videoAlbumApp(t *testing.T) (*application, *httptest.Server, string, string) {
	t.Helper()
	app, store := albumApp(t)
	put := func(body string) string {
		ref, err := app.blobs.PutCache(context.Background(), []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		return ref.String()
	}
	hd, sd := put("0123456789-the-720p-mp4"), put("the-480p-mp4")
	store.albums = append(store.albums, albumStoreEntry{
		album:     album.Album{ID: "al-video", Slug: "film", Title: "Film", SortOrder: 40},
		published: true,
		items: []album.Item{{Ordinal: 0, PhotoID: hd, Ref: hd, ThumbRef: put("poster-thumb"), MediumRef: put("poster-800"),
			Kind: "video", Status: "ready", DurationMs: 22 * 60 * 1000, SdRef: sd, Width: 1280, Height: 720}},
	})
	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	return app, srv, hd, sd
}

func TestAlbumVideoServesMP4WithRange(t *testing.T) {
	_, srv, hd, _ := videoAlbumApp(t)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/public/albums/al-video/media/"+hd, nil)
	req.Header.Set("Range", "bytes=2-5")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusPartialContent || string(body) != "2345" {
		t.Fatalf("want 206 with bytes 2-5, got %d %q", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Content-Type"); got != "video/mp4" {
		t.Errorf("content type %q", got)
	}
	if resp.Header.Get("Accept-Ranges") != "bytes" || resp.Header.Get("ETag") != `"`+hd+`"` {
		t.Errorf("headers %v", resp.Header)
	}
}

func TestAlbumVideoServesTheSDRenditionAndThePoster(t *testing.T) {
	_, srv, hd, _ := videoAlbumApp(t)
	_, body := getPublic(t, srv.URL+"/api/public/albums/al-video/media/"+hd+"?variant=sd", nil)
	if string(body) != "the-480p-mp4" {
		t.Errorf("sd: %q", body)
	}
	resp, body := getPublic(t, srv.URL+"/api/public/albums/al-video/media/"+hd+"?variant=thumb", nil)
	if string(body) != "poster-thumb" || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("thumb: %q %s", body, resp.Header.Get("Content-Type"))
	}
}

// Images keep their 304 path and gain Range for free.
func TestAlbumImageStillAnswers304(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	_, store := app, app.models.Albums.(*albumStore)
	ref := store.albums[0].items[0].Ref
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/public/albums/al-1/media/"+ref, nil)
	req.Header.Set("If-None-Match", `"`+ref+`"`)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("want 304, got %d", resp.StatusCode)
	}
}

func TestAlbumVideoMissingRenditionIsRequeued(t *testing.T) {
	app, srv, hd, _ := videoAlbumApp(t)
	pub := &cqrstest.Publisher{}
	app.commands = commandsWithPublisher(t, pub)
	_ = app.blobs.Delete(context.Background(), blobRefOf(hd))

	for range 3 {
		resp, _ := getPublic(t, srv.URL+"/api/public/albums/al-video/media/"+hd, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("want 404 while rebuilding, got %d", resp.StatusCode)
		}
	}
	s := pub.Subjects()
	if len(s) != 1 || !strings.HasSuffix(s[0], ".videoqueued") {
		t.Errorf("want exactly one requeue for three misses, got %v", s)
	}
}

// The grid shows a video as its poster with a glyph and its length, and loads no video bytes (task 498).
func TestAlbumGridShowsAVideoAsItsPoster(t *testing.T) {
	_, srv, hd, _ := videoAlbumApp(t)
	_, raw := getPublic(t, srv.URL+"/2026/album/film", nil)
	body := string(raw)
	for _, want := range []string{
		`data-kind="video"`, `data-duration-ms="1320000"`, `aria-label="video, 22:00"`,
		`data-sd="/api/public/albums/al-video/media/` + hd + `?variant=sd"`,
		`src="/api/public/albums/al-video/media/` + hd + `?variant=thumb"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the album page lacks %s", want)
		}
	}
	if strings.Contains(body, "<video") {
		t.Error("the grid must not embed video: it would download clips nobody opened")
	}
}
