package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/video"
	"nathejk.dk/nathejk/table/photo"

	"github.com/julienschmidt/httprouter"
)

// Chunked, resumable video upload for the library (PRD 029 §8.4, task 494).
//
// # Why not the photograph route
//
// `POST /api/admin/photos` reads one request body into memory, capped at 32 MB. A phone clip is up to 4 GB and is
// uploaded from a photographer's laptop over whatever connection the camp has, so it has to survive a dropped
// connection without starting over. Hence a session: create it, PUT the bytes in order in chunks, ask for the
// offset after a failure, and complete it.
//
// # Where the bytes live meanwhile
//
// In the blob store's staging directory, which is on the same volume as the objects (`blob.Files`), so completion
// is a rename rather than a 4 GB copy, and outside `original/`, so a half-upload is never in a backup. The session
// is two files: `<id>.part` (the bytes so far, so its size *is* the offset) and `<id>.json` (what was declared).
// Nothing is in memory, so a restart of the API loses no upload.

const (
	// maxVideoUpload bounds one original (PRD 029 §11 Q2).
	maxVideoUpload = 4 << 30
	// maxVideoDuration bounds one clip (PRD 029 §11 Q2).
	maxVideoDuration = 30 * time.Minute
	// maxVideoChunk bounds one PUT. The client sends 8 MiB; the slack is for a client that chooses differently.
	maxVideoChunk = 64 << 20
	// videoUploadChunk is the size the server suggests.
	videoUploadChunk = 8 << 20
	// videoUploadTTL is how long an abandoned session's bytes are kept.
	videoUploadTTL = 24 * time.Hour
	// videoChunkTimeout is the read deadline for one chunk: 8 MiB on a poor uplink is well under this.
	videoChunkTimeout = 5 * time.Minute
)

// videoUploadSession is the declared half of a session, stored as `<id>.json`.
type videoUploadSession struct {
	Year      string    `json:"year"`
	FileName  string    `json:"fileName"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

type videoUploadCreateRequest struct {
	FileName string `json:"fileName"`
	Size     int64  `json:"size"`
}

type videoUploadState struct {
	UploadID  string `json:"uploadId"`
	Offset    int64  `json:"offset"`
	Size      int64  `json:"size"`
	ChunkSize int64  `json:"chunkSize"`
}

var uploadIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// videoFiles returns the store's large-object capability, or nil.
func (app *application) videoFiles() blob.Files {
	f, _ := app.blobs.(blob.Files)
	return f
}

// videoUploadDir is the staging subdirectory sessions live in.
func (app *application) videoUploadDir() (string, error) {
	f := app.videoFiles()
	if f == nil {
		return "", errors.New("the blob store cannot hold large files")
	}
	root, err := f.StagingDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, "video-uploads")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// sweepVideoUploads removes sessions untouched for videoUploadTTL. Run on every create, which is often enough:
// the only cost of a stale session is disk, and creating a session is when disk is about to be needed.
func (app *application) sweepVideoUploads(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-videoUploadTTL)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		if rerr := os.Remove(filepath.Join(dir, e.Name())); rerr == nil {
			app.Logger.Info("removed an abandoned video upload", "file", e.Name())
		}
	}
}

// loadVideoUpload resolves the :uploadId parameter to its session and current offset.
func (app *application) loadVideoUpload(r *http.Request) (dir, id string, s videoUploadSession, offset int64, err error) {
	id = httprouter.ParamsFromContext(r.Context()).ByName("uploadId")
	// The id becomes a file name, so it is checked against the shape this file mints before it touches a path.
	if !uploadIDPattern.MatchString(id) {
		return "", "", s, 0, os.ErrNotExist
	}
	if dir, err = app.videoUploadDir(); err != nil {
		return "", "", s, 0, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		return "", "", s, 0, err
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", "", s, 0, err
	}
	// A session belongs to the year it was opened in. Completing it from another year's page would file a
	// clip under the wrong race.
	if s.Year != adminYear(r) {
		return "", "", s, 0, os.ErrNotExist
	}
	info, err := os.Stat(filepath.Join(dir, id+".part"))
	if err != nil {
		return "", "", s, 0, err
	}
	return dir, id, s, info.Size(), nil
}

func (app *application) writeVideoUploadLoadError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, os.ErrNotExist) {
		app.NotFoundResponse(w, r)
		return
	}
	app.ServerErrorResponse(w, r, err)
}

// createVideoUploadHandler opens an upload session.
//
// @Summary      Start a resumable video upload
// @Description  Opens an upload session for one video file in the configured year's library (PRD 029). The body declares the file name and its exact size in bytes; the bytes then follow in order through `PUT /admin/videos/uploads/{uploadId}`. Max 4 GiB. A session untouched for 24 hours is discarded. Requires the admin credential.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        body  body      videoUploadCreateRequest  true  "the file name and size"
// @Success      201   {object}  videoUploadState
// @Failure      400   {object}  map[string]string  "missing or invalid size"
// @Failure      401   "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421   "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      413   {object}  map[string]string  "larger than 4 GiB"
// @Failure      507   {object}  map[string]string  "the blob volume would drop below its free-space floor"
// @Failure      500   {object}  map[string]string
// @Failure      503   {object}  map[string]string  "the blob store cannot hold large files"
// @Router       /admin/videos/uploads [post]
func (app *application) createVideoUploadHandler(w http.ResponseWriter, r *http.Request) {
	var req videoUploadCreateRequest
	if err := app.ReadJSON(w, r, &req); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}
	if req.Size <= 0 {
		app.BadRequestMessageResponse(w, r, "filens størrelse mangler")
		return
	}
	if req.Size > maxVideoUpload {
		app.PayloadTooLargeResponse(w, r, fmt.Errorf("videoen er større end %d GB", maxVideoUpload>>30))
		return
	}
	dir, err := app.videoUploadDir()
	if err != nil {
		app.ServiceUnavailableResponse(w, r, "serveren kan ikke modtage videoer lige nu")
		return
	}
	app.sweepVideoUploads(dir)
	// Checked at the start rather than the end: the end is an hour of somebody's uplink later. Twice the size for
	// the upload's reason (adminupload.go) — the original, and renditions that come to well under it.
	if app.writeAdminDiskResponse(w, r, app.checkAdminDiskFloor(2*req.Size)) {
		return
	}

	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	id := hex.EncodeToString(b[:])
	meta, err := json.Marshal(videoUploadSession{
		Year: adminYear(r), FileName: photo.NormalizeFileName(req.FileName), Size: req.Size,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, id+".part"), nil, 0o600); err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), meta, 0o600); err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	app.Logger.Info("admin started a video upload", "uploadId", id, "bytes", req.Size, "ip", clientIP(r))
	if err := app.WriteJSON(w, http.StatusCreated, videoUploadState{
		UploadID: id, Size: req.Size, ChunkSize: videoUploadChunk,
	}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// showVideoUploadHandler reports how much of an upload has arrived.
//
// @Summary      Resume point of a video upload
// @Description  Returns how many bytes of the session have been received, so a client whose connection dropped continues from there. Requires the admin credential.
// @Tags         admin
// @Produce      json
// @Param        uploadId  path      string  true  "the session id"
// @Success      200       {object}  videoUploadState
// @Failure      400       {object}  map[string]string  "no working year, or one the tool does not know (X-Admin-Year)"
// @Failure      401       "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421       "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      404       {object}  map[string]string  "no such session, or it expired"
// @Failure      500       {object}  map[string]string
// @Router       /admin/videos/uploads/{uploadId} [get]
func (app *application) showVideoUploadHandler(w http.ResponseWriter, r *http.Request) {
	_, id, s, offset, err := app.loadVideoUpload(r)
	if err != nil {
		app.writeVideoUploadLoadError(w, r, err)
		return
	}
	if err := app.WriteJSON(w, http.StatusOK, videoUploadState{
		UploadID: id, Offset: offset, Size: s.Size, ChunkSize: videoUploadChunk,
	}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// parseContentRange reads `bytes start-end/total`.
func parseContentRange(h string) (start, end, total int64, ok bool) {
	spec, found := strings.CutPrefix(strings.TrimSpace(h), "bytes ")
	if !found {
		return 0, 0, 0, false
	}
	rng, tot, found := strings.Cut(spec, "/")
	if !found {
		return 0, 0, 0, false
	}
	a, b, found := strings.Cut(rng, "-")
	if !found {
		return 0, 0, 0, false
	}
	var err error
	if start, err = strconv.ParseInt(a, 10, 64); err != nil {
		return 0, 0, 0, false
	}
	if end, err = strconv.ParseInt(b, 10, 64); err != nil {
		return 0, 0, 0, false
	}
	if total, err = strconv.ParseInt(tot, 10, 64); err != nil {
		return 0, 0, 0, false
	}
	return start, end, total, start >= 0 && end >= start
}

// putVideoChunkHandler appends one chunk.
//
// @Summary      Send one chunk of a video upload
// @Description  Appends the body to the session. `Content-Range: bytes start-end/total` is required, and `start` must equal the bytes received so far: a chunk out of order is refused with 409 and the current offset, from which the client continues. At most 64 MiB per chunk. Requires the admin credential.
// @Tags         admin
// @Accept       application/octet-stream
// @Produce      json
// @Param        uploadId       path      string  true  "the session id"
// @Param        Content-Range  header    string  true  "bytes start-end/total"
// @Success      200            {object}  videoUploadState
// @Failure      400            {object}  map[string]string  "missing or malformed Content-Range, or past the declared size"
// @Failure      401            "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421            "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      404            {object}  map[string]string  "no such session, or it expired"
// @Failure      409            {object}  videoUploadState   "the chunk does not start at the current offset"
// @Failure      413            {object}  map[string]string  "the chunk is larger than 64 MiB"
// @Failure      500            {object}  map[string]string
// @Router       /admin/videos/uploads/{uploadId} [put]
func (app *application) putVideoChunkHandler(w http.ResponseWriter, r *http.Request) {
	if rc := http.NewResponseController(w); rc != nil {
		if err := rc.SetReadDeadline(time.Now().Add(videoChunkTimeout)); err != nil {
			app.Logger.Warn("could not extend the video chunk read deadline", "err", err)
		}
	}
	dir, id, s, offset, err := app.loadVideoUpload(r)
	if err != nil {
		app.writeVideoUploadLoadError(w, r, err)
		return
	}
	start, end, total, ok := parseContentRange(r.Header.Get("Content-Range"))
	if !ok || total != s.Size || end >= s.Size {
		app.BadRequestMessageResponse(w, r, "Content-Range mangler eller passer ikke til filen")
		return
	}
	if end-start+1 > maxVideoChunk {
		app.PayloadTooLargeResponse(w, r, fmt.Errorf("et stykke må højst være %d MB", maxVideoChunk>>20))
		return
	}
	if start != offset {
		// Not an error the client did wrong so much as a fact it lost: tell it where to continue. Written by hand
		// rather than through WriteJSON so the status is a literal the OpenAPI guard can see.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(videoUploadState{
			UploadID: id, Offset: offset, Size: s.Size, ChunkSize: videoUploadChunk,
		})
		return
	}

	f, err := os.OpenFile(filepath.Join(dir, id+".part"), os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		app.writeVideoUploadLoadError(w, r, err)
		return
	}
	want := end - start + 1
	n, cerr := io.Copy(f, io.LimitReader(r.Body, want))
	if closeErr := f.Close(); cerr == nil {
		cerr = closeErr
	}
	if cerr != nil || n != want {
		// A partial chunk is cut back off, so the offset stays a chunk boundary and the retry is the same request.
		_ = os.Truncate(filepath.Join(dir, id+".part"), offset)
		if cerr == nil {
			cerr = fmt.Errorf("the chunk was %d bytes, not %d", n, want)
		}
		app.BadRequestResponse(w, r, cerr)
		return
	}
	if err := app.WriteJSON(w, http.StatusOK, videoUploadState{
		UploadID: id, Offset: offset + n, Size: s.Size, ChunkSize: videoUploadChunk,
	}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// completeVideoUploadHandler turns a finished session into a library item.
//
// @Summary      Finish a video upload
// @Description  Checks the received file with ffprobe — it must contain a video stream and last at most 30 minutes — stores it byte for byte as the item's original, and adds it to the library as a video in `processing`; the transcode worker then produces the playable renditions. The id is the content hash of the file, so uploading the same clip twice yields one item, and a clip a curator has deleted is not restored. Requires the admin credential.
// @Tags         admin
// @Produce      json
// @Param        uploadId  path      string  true  "the session id"
// @Success      200       {object}  adminUploadResponse  "stored, already present, or previously deleted"
// @Failure      400       {object}  map[string]string  "the upload is incomplete, or the file is not a video"
// @Failure      401       "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421       "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      404       {object}  map[string]string  "no such session, or it expired"
// @Failure      413       {object}  map[string]string  "longer than 30 minutes"
// @Failure      500       {object}  map[string]string
// @Failure      503       {object}  map[string]string  "the library or the event stream are unavailable"
// @Router       /admin/videos/uploads/{uploadId}/complete [post]
func (app *application) completeVideoUploadHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.PhotoCurator == nil {
		app.ServiceUnavailableResponse(w, r, "billedarkivet er ikke tilgængeligt lige nu")
		return
	}
	dir, id, s, offset, err := app.loadVideoUpload(r)
	if err != nil {
		app.writeVideoUploadLoadError(w, r, err)
		return
	}
	if offset != s.Size {
		app.BadRequestMessageResponse(w, r, fmt.Sprintf("uploadet er ikke færdigt (%d af %d bytes)", offset, s.Size))
		return
	}
	part := filepath.Join(dir, id+".part")
	discard := func() {
		_ = os.Remove(part)
		_ = os.Remove(filepath.Join(dir, id+".json"))
	}

	probe := app.videoProbe
	if probe == nil {
		probe = video.Probe
	}
	info, err := probe(r.Context(), part)
	if err != nil {
		if errors.Is(err, video.ErrNotVideo) {
			discard()
			app.BadRequestMessageResponse(w, r, "filen er ikke en video vi kan læse")
			return
		}
		app.ServerErrorResponse(w, r, err)
		return
	}
	if time.Duration(info.DurationMs)*time.Millisecond > maxVideoDuration {
		discard()
		app.PayloadTooLargeResponse(w, r,
			fmt.Errorf("videoen er længere end %d minutter", int(maxVideoDuration.Minutes())))
		return
	}

	// Hashed before it is adopted, so a clip a curator deleted is answered without putting 4 GB back on disk.
	ref, err := blob.HashFile(part)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	photoID := ref.String()
	existing, found, err := app.models.PhotoCurator.Photo(s.Year, photoID)
	if err != nil {
		app.ServerErrorResponse(w, r, fmt.Errorf("checking whether the video is already in the library: %w", err))
		return
	}
	resp := adminUploadResponse{PhotoID: photoID, Width: info.Width, Height: info.Height, Bytes: int(s.Size)}
	switch {
	case found && existing.Deleted:
		discard()
		resp.Outcome = adminUploadPreviouslyDeleted
		resp.Message = "Denne video er slettet tidligere og bliver ikke lagt op igen."
		app.Logger.Info("admin video upload skipped: previously deleted", "photoId", photoID, "ip", clientIP(r))

	case found:
		discard()
		resp.Outcome = adminUploadAlreadyPresent
		resp.Message = "Allerede uploadet."

	default:
		stored, err := app.videoFiles().PutFile(r.Context(), part, false)
		if err != nil {
			app.ServerErrorResponse(w, r, err)
			return
		}
		_ = os.Remove(filepath.Join(dir, id+".json"))
		subject, serr := photo.Subject(s.Year, photoID, photo.VerbVideoUploaded)
		if serr != nil {
			app.ServerErrorResponse(w, r, serr)
			return
		}
		if perr := app.commands.Publish(subject, photo.VideoUploaded{
			PhotoID: photoID,
			Year:    s.Year,
			Original: photo.Original{
				Ref: stored.String(), ContentType: video.ContentType(info, s.FileName), Bytes: int(s.Size),
				Width: info.Width, Height: info.Height,
			},
			DurationMs: info.DurationMs,
			FileName:   s.FileName,
			UploadedAt: time.Now().UTC(),
		}); perr != nil {
			// The original is stored and content-addressed, so a retry of the whole upload costs bandwidth and
			// nothing else.
			app.writeAlbumPublishFailure(w, r, perr)
			return
		}
		resp.Outcome = adminUploadStored
		resp.Message = "Uploadet. Videoen behandles."
		app.Logger.Info("admin uploaded a video", "photoId", photoID, "bytes", s.Size,
			"durationMs", info.DurationMs, "ip", clientIP(r))
		app.notifyVideoWorker()
	}

	if err := app.WriteJSON(w, http.StatusOK, resp, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// notifyVideoWorker wakes the transcode worker, if one runs. Never blocks: a wake already pending covers this one.
func (app *application) notifyVideoWorker() {
	if app.videoWake == nil {
		return
	}
	select {
	case app.videoWake <- struct{}{}:
	default:
	}
}
