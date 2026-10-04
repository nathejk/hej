package main

import (
	"context"
	"strings"
	"testing"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/patrolphoto"
	"nathejk.dk/nathejk/table/photo"
)

// The filename is what the albums sort by, so it has to sort numbers as numbers.
func TestDiplomaFileName(t *testing.T) {
	for _, c := range []struct{ number, contentType, want string }{
		{"7", "image/jpeg", "007.jpg"},
		{"42", "", "042.jpg"},
		{"186", "image/jpeg", "186.jpg"},
		{"1042", "image/jpeg", "1042.jpg"},
		{" 9 ", "image/webp", "009.webp"},
		{"12A", "image/jpeg", "12A.jpg"},
		{"", "image/jpeg", ""},
	} {
		if got := diplomaFileName(c.number, c.contentType); got != c.want {
			t.Errorf("diplomaFileName(%q, %q) = %q, want %q", c.number, c.contentType, got, c.want)
		}
	}
}

// stubLatestPhotos is the patrol photograph read, for the Start/Slut sync.
type stubLatestPhotos struct{ latest []patrolphoto.TeamPhoto }

func (s stubLatestPhotos) Cover(string, string) (patrolphoto.Photo, bool, error) {
	return patrolphoto.Photo{}, false, nil
}
func (s stubLatestPhotos) Teams(string) ([]patrolphoto.Team, error) { return nil, nil }
func (s stubLatestPhotos) Refused(string) ([]string, error)         { return nil, nil }
func (s stubLatestPhotos) Latest(string, string) ([]patrolphoto.TeamPhoto, error) {
	return s.latest, nil
}

// No videos in Start and Slut (PRD 029 §11 Q7, task 501): a video is never filed there, and one a curator put there
// is taken out, while a curator's untagged photograph stays.
func TestStartAndSlutHoldNoVideos(t *testing.T) {
	app, _ := adminApp(t)
	pub := &cqrstest.Publisher{}
	app.commands = commandsWithPublisher(t, pub)

	video, still, filedVideo := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)
	app.models.PatrolPhotos = stubLatestPhotos{latest: []patrolphoto.TeamPhoto{{
		Team:  patrolphoto.Team{TeamID: "team-1", Number: "7"},
		Photo: patrolphoto.Photo{Ref: video, ContentType: "video/mp4"},
	}}}
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{
		{ID: still, Kind: "photo", Status: "ready"},
		{ID: filedVideo, Kind: "video", Status: "ready"},
	}}
	app.models.AlbumCurator = newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-start", Slug: slugifyAlbumTitle("Start"), Title: "Start"},
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: still},
			{Ordinal: 1, PhotoID: filedVideo},
		},
	})

	var res diplomaSyncResult
	if err := app.syncDiplomaAlbum(context.Background(), "2026", "start", "Start", "", &res); err != nil {
		t.Fatal(err)
	}
	if res.Added != 0 || res.Skipped != 1 {
		t.Errorf("the patrol's video must be skipped, not filed: %+v", res)
	}
	var removed []string
	for _, m := range pub.Messages {
		if strings.HasSuffix(m.Subject().Subject(), "."+album.VerbItemRemoved) {
			var ev album.ItemRemoved
			_ = m.Body(&ev)
			removed = append(removed, ev.PhotoID)
		}
	}
	if len(removed) != 1 || removed[0] != filedVideo {
		t.Errorf("want only the filed video removed, got %v (published %v)", removed, pub.Subjects())
	}
}
