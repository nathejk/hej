import { defineStore } from 'pinia'
import {
  TRACK_FIX_TIMEOUT_MS,
  TRACK_KEEP_UPLOADED_HOURS,
  TRACK_MAX_FIX_AGE_MS,
  TRACK_MAX_POINTS,
  TRACK_SAMPLE_SECONDS,
  TRACK_UPLOAD_CHUNK,
  TRACK_UPLOAD_INTERVAL_SECONDS,
  TRACK_UPLOAD_MAX_CHUNKS_PER_RUN,
} from '@/config/track'
import {
  appendPoint,
  countPending,
  countPoints,
  latestTimestamp,
  logEvent,
  markUploaded,
  pendingPoints,
  pruneUploaded,
  requestPersistentStorage,
  TrackStorageFullError,
} from '@/helpers/trackDb'
import { fetchWrapper, HttpError, NetworkError } from '@/helpers'
import { useLocationStore } from '@/stores/location.store'
import { useSessionStore } from '@/stores/session.store'
import { useAppStore } from '@/stores/app.store'

// track.store records where the user has been, into local persistent storage
// (PRD 002 §11.1, task 082). Uploading is task 083; nothing here talks to the network.
//
// WHERE THIS RUNS, AND WHY IT MATTERS MOST. The recorder is started at app level (see
// App.vue), not by the map view. MapsView stops its geolocation watch on
// `document.hidden` and on unmount, which is right for a map marker and wrong for a
// recorder: tying the track to that lifecycle would mean recording only while the user
// is *looking at the map*. Since the goal the maintainer set is "track as much as
// possible", moving the recorder up to the app is the single change that most increases
// coverage — it turns "recorded while looking at the map" into "recorded whenever the
// app is open at all". Drawing the live marker and recording the track are separate
// concerns that happen to share a data source.
//
// WHAT IT CANNOT DO. A web app cannot record while backgrounded, on any platform. The
// document does not run when suspended; Screen Wake Lock is released when the document
// becomes inactive (and a lit screen for 12 hours is both a battery and a
// light-discipline problem in a night race); Periodic Background Sync can wake a
// service worker but service workers have no Geolocation API. So the deliverable is
// coverage of everywhere the member was *while the app was open*, not a continuous
// route. Accepted by the maintainer 2026-08-26.
export const useTrackStore = defineStore('track', {
  state: () => ({
    recording: false,
    /** How many points are stored on this device, across users. Observability. */
    pointCount: 0,
    /** Whether the browser promised not to evict our storage. Answered, not assumed. */
    persisted: false,
    /** Epoch ms of the last point written, or 0. */
    lastPointAt: 0,
    /**
     * Why recording stopped, when it stopped for a reason the user might care about.
     * Empty while healthy.
     */
    problem: '' as '' | 'full' | 'capped',
    timer: null as ReturnType<typeof setInterval> | null,
    /**
     * Set synchronously while start() is in progress.
     *
     * Not decoration: start() awaits (persistence request, count, last timestamp) before
     * it can set `recording`, and it is called from two places — App.vue's onMounted and
     * the permission/session watcher. Without a guard taken *before* the first await,
     * both calls sail past `if (this.recording)`, and the app ends up with two intervals
     * writing two sets of points. That showed up in testing only as a wrong sampling
     * cadence, which is a very quiet symptom for a doubled recorder.
     */
    starting: false,
    /** Set while a sample is in flight, so two never overlap (see start()'s note). */
    sampling: false,

    // ---- upload (task 083) ---------------------------------------------------------
    /** How many of this user's points have not been accepted by the server yet. */
    pendingCount: 0,
    /** Epoch ms of the last upload the server accepted, or 0. */
    lastUploadAt: 0,
    /** Human-readable reason the last upload attempt failed, or '' when healthy. */
    uploadError: '',
    /**
     * Set when the uploader has stopped and will not retry on its own.
     *
     * Only a 400 does this. Everything else — offline, 5xx, timeout, 429 — is temporary and
     * retried on the next interval. A 400 means the client is sending something the server
     * will never accept, i.e. a bug in this app; retrying forever would block every later
     * point behind it, and silently discarding the batch to get unstuck would throw away a
     * member's track to hide our own defect. So it stops and says so.
     */
    uploadBlocked: false,
    uploading: false,
    uploadTimer: null as ReturnType<typeof setInterval> | null,
  }),
  actions: {
    // start begins recording, if it should be. Safe to call repeatedly — the app-level
    // watcher calls it on every permission or session change.
    async start() {
      const session = useSessionStore()
      const location = useLocationStore()

      // Nothing is recorded before permission is granted, and nothing is recorded
      // without a signed-in person to attribute it to: a point with no owner could not
      // be uploaded (task 083 resolves the person from the session) and could not be
      // erased on request either.
      //
      // Logged when it refuses. The first device run showed `optager nu: false` with no
      // explanation anywhere, and the reason (WebKit reporting `prompt` for a granted
      // permission) took a report and a code read to work out. A refusal is exactly the
      // thing a diagnostic needs to state out loud.
      if (!session.user || location.permission !== 'granted') {
        // Only logged when there IS a signed-in person: App.vue calls start() on mount,
        // before the session has resolved, so "no user yet" is the normal startup order
        // rather than a refusal worth reporting. Logging it filled the diagnostic with
        // noise that looked like a fault.
        if (session.user) {
          void logEvent('nostart', `permission=${location.permission}`)
        }
        return
      }
      if (this.recording || this.starting || this.problem) return
      this.starting = true

      try {
        // Requested once per recording session, and the answer is kept so it can be
        // shown rather than assumed. An evicted track cannot be re-fetched from
        // anywhere — unlike map tiles, which is why this matters more here than there.
        this.persisted = await requestPersistentStorage()
        await this.refreshCount()
        // Recover the cadence across page loads: every full navigation remounts the app
        // and calls start(), so without this a handful of reloads would each take an
        // immediate fix and pile up points seconds apart.
        try {
          this.lastPointAt = await latestTimestamp(session.user.userId)
        } catch (err) {
          console.error('failed to read the last recorded position', err)
        }

        // Permission or session may have changed while we awaited.
        if (!session.user || location.permission !== 'granted') return

        this.recording = true
        void logEvent('start', `points=${this.pointCount} persisted=${this.persisted}`)
        // Belt and braces: never leave an orphaned interval behind.
        if (this.timer !== null) clearInterval(this.timer)
        // Sample immediately — but sample() skips if anything was recorded within the
        // last interval, so a short session still records something without a reload
        // becoming a way to oversample.
        void this.sample()
        this.timer = setInterval(() => void this.sample(), TRACK_SAMPLE_SECONDS * 1000)
      } finally {
        this.starting = false
      }
    },

    stop() {
      if (this.timer !== null) {
        clearInterval(this.timer)
        this.timer = null
      }
      if (this.recording) void logEvent('stop')
      this.recording = false
    },

    // sample records one position.
    //
    // ACQUISITION MODE, decided deliberately (task 082's criterion). Not a continuous
    // high-accuracy `watchPosition` of its own: that keeps the GPS receiver hot for
    // twelve hours to use one fix in sixty, and battery over a night race is the
    // deciding factor. Instead:
    //
    //   1. reuse the current fix if it is younger than TRACK_MAX_FIX_AGE_MS — while the
    //      map is open its watch supplies these, so recording costs nothing extra;
    //   2. otherwise ask for a single fix, which lets the radio sleep in between.
    //
    // A missed sample is acceptable (the track is expected to be fragmented); a hung
    // one is not, hence the timeout.
    async sample() {
      const location = useLocationStore()
      const session = useSessionStore()

      if (!this.recording) return
      if (!session.user || location.permission !== 'granted') {
        // Permission revoked mid-session, or signed out. Stop rather than spin.
        this.stop()
        return
      }

      // Don't sample if the track already has a recent point. Guards the two ways the
      // cadence can be undercut: a page load taking an immediate fix, and a recorder
      // accidentally started twice. The 80% margin keeps a scheduled sample that fires a
      // fraction early from being skipped.
      //
      // `elapsed >= 0` matters: lastPointAt is a *fix* timestamp, and some receivers
      // report a clock that runs ahead of the device's. A future timestamp would
      // otherwise suppress recording until real time caught up with it — an hour of
      // silence from a one-off clock error.
      const minGapMs = TRACK_SAMPLE_SECONDS * 1000 * 0.8
      const elapsed = Date.now() - this.lastPointAt
      if (this.lastPointAt && elapsed >= 0 && elapsed < minGapMs) {
        void logEvent('skip', `${Math.round(elapsed / 1000)}s since last point`)
        return
      }

      // Acquiring a fix takes up to TRACK_FIX_TIMEOUT_MS, which is most of a sample
      // interval, so the next tick can arrive while this one is still waiting. Two
      // concurrent acquisitions would cost two GPS fixes to store one point.
      if (this.sampling) return
      this.sampling = true
      try {
        await this.record()
      } finally {
        this.sampling = false
      }
    },

    // record takes one fix and stores it. Split out of sample() only so the in-flight
    // guard there has a single exit point.
    async record() {
      const location = useLocationStore()
      const session = useSessionStore()
      if (!session.user) return

      // Whether this used a fix the map's watch already had (free) or cost us our own.
      // Recorded in the event log because it is the whole battery argument for this
      // design, and on a real phone it is the number worth knowing.
      const reused =
        !!location.position && Date.now() - location.positionAt <= TRACK_MAX_FIX_AGE_MS
      const fresh = reused
        ? { coords: location.position!, at: location.positionAt }
        : await this.acquire()

      if (!fresh) {
        void logEvent('nofix')
        return
      }

      if (this.pointCount >= TRACK_MAX_POINTS) {
        // A safety net, not a policy: this is ~7 days of sampling against a 12-hour
        // race, so reaching it means something is wrong. Stop and surface it instead of
        // discarding points that nothing has copied anywhere else yet.
        this.problem = 'capped'
        void logEvent('capped', `${this.pointCount} points`)
        this.stop()
        return
      }

      try {
        await appendPoint({
          userId: session.user.userId,
          // The fix's own timestamp, not "now": it is when the position was true, and
          // it is half the primary key, so using a later clock reading would let the
          // same position be stored twice under two keys.
          ts: fresh.at,
          lat: fresh.coords.lat,
          lng: fresh.coords.lng,
          accuracy: fresh.coords.accuracy,
          uploaded: 0,
        })
        this.lastPointAt = fresh.at
        this.pointCount += 1
        void logEvent('point', `acc=${Math.round(fresh.coords.accuracy)}m reused=${reused ? 1 : 0}`)
      } catch (err) {
        if (err instanceof TrackStorageFullError) {
          // Handled rather than thrown into the void: a recorder whose writes silently
          // fail looks exactly like a working one.
          this.problem = 'full'
          void logEvent('full')
          this.stop()
          return
        }
        // Anything else (a closed connection, a transaction abort) is per-sample and
        // may well succeed next time, so keep recording.
        console.error('failed to record position', err)
      }
    },

    // acquire reads a single position. Resolves null on error or timeout — never
    // rejects, because one failed sample must not stop the recorder.
    acquire(): Promise<{ coords: { lat: number; lng: number; accuracy: number }; at: number } | null> {
      if (typeof navigator === 'undefined' || !('geolocation' in navigator)) {
        return Promise.resolve(null)
      }
      const location = useLocationStore()
      return new Promise((resolve) => {
        navigator.geolocation.getCurrentPosition(
          (pos) => {
            const coords = {
              lat: pos.coords.latitude,
              lng: pos.coords.longitude,
              accuracy: pos.coords.accuracy,
            }
            const at = pos.timestamp || Date.now()
            // Share the fix: the map gets a free position update out of it, and the
            // freshness check above will reuse it if a sample lands close behind.
            location.position = coords
            location.positionAt = at
            resolve({ coords, at })
          },
          (err) => {
            void logEvent('geoerror', `code=${err.code} ${err.message}`)
            if (err.code === err.PERMISSION_DENIED) {
              // Clears the remembered grant too, which is what makes a revocation in iOS
              // Settings self-correcting rather than a permanent wrong belief.
              location.markDenied(err.message)
              this.stop()
            }
            resolve(null)
          },
          {
            enableHighAccuracy: true,
            timeout: TRACK_FIX_TIMEOUT_MS,
            // maximumAge: 0 — no cached fix. The platform's cache would buy nothing
            // here, because the cheap path is already covered one level up: if a recent
            // fix exists at all it came from the map's watch, and record() reuses that
            // without calling this. What a cached fix WOULD cost is timestamp accuracy: a
            // point's `ts` is when the position was true and is half its primary key, so
            // accepting a 25 s-old fix lets a 30 s cadence store points 5 s apart in the
            // track. Reaching acquire() means nothing recent is known, which is exactly
            // when a fresh fix is the right thing to ask for.
            maximumAge: 0,
          },
        )
      })
    },

    async refreshCount() {
      try {
        this.pointCount = await countPoints()
        const session = useSessionStore()
        if (session.user) this.pendingCount = await countPending(session.user.userId)
      } catch (err) {
        console.error('failed to count recorded positions', err)
      }
    },

    // ---- upload (task 083) ---------------------------------------------------------

    // startUploading begins the 2-minute upload cycle.
    //
    // Deliberately independent of recording. Points can be pending with recording stopped —
    // permission revoked, storage full, or simply signed in after a session where the phone
    // recorded and never got signal — and in every one of those cases the backlog still has
    // to ship. Tying the uploader to the recorder would strand exactly the data that is
    // hardest to reproduce.
    startUploading() {
      const session = useSessionStore()
      if (!session.user || this.uploadTimer !== null || this.uploadBlocked) return

      // Attempt immediately: on iOS the app does not run while backgrounded (task 082), so
      // being foregrounded is the moment a backlog can move, and waiting out a fresh
      // 2-minute interval first would waste the window the user is actually giving us.
      void this.flush()
      this.uploadTimer = setInterval(() => void this.flush(), TRACK_UPLOAD_INTERVAL_SECONDS * 1000)
    },

    stopUploading() {
      if (this.uploadTimer !== null) {
        clearInterval(this.uploadTimer)
        this.uploadTimer = null
      }
    },

    // flush uploads pending points in bounded chunks.
    //
    // Points are marked uploaded ONLY after the server has answered 2xx, so a failure of any
    // kind leaves them pending and the next interval retries them.
    //
    // WHERE DUPLICATES ARE REMOVED, decided here rather than left to whoever notices
    // (task 083 asks for this in writing): **at the reader**, keyed by (person, timestamp).
    // A request that times out *after* the server published it is indistinguishable from one
    // that never arrived, so the client must retry, and that retry republishes points the
    // stream already has. The client cannot know; the endpoint cannot know without keeping
    // state it is forbidden to keep (it writes no SQL, PRD 008 §8); so the stream can contain
    // the same point twice and the reader must collapse it. That is safe rather than merely
    // tolerable, because a point is immutable: the same (person, timestamp) always carries
    // the same position, so last-write-wins and first-write-wins agree. Task 086 and any
    // other consumer must key on (person, timestamp) — this is the contract.
    async flush() {
      const session = useSessionStore()
      const app = useAppStore()

      // No upload without a signed-in person: the endpoint resolves the person from the
      // session, so an unauthenticated attempt is a guaranteed 401 and a pointless request.
      if (!session.user || this.uploading || this.uploadBlocked) return

      this.uploading = true
      try {
        for (let chunk = 0; chunk < TRACK_UPLOAD_MAX_CHUNKS_PER_RUN; chunk++) {
          const points = await pendingPoints(session.user.userId, TRACK_UPLOAD_CHUNK)
          if (points.length === 0) {
            // Nothing new. This is the common case for a stationary phone and costs no
            // request at all — the "only when changed" condition PRD 002 §11.1 asks for.
            this.pendingCount = 0
            break
          }

          // Send only the wire fields. `userId` and `uploaded` are local bookkeeping, and
          // the endpoint rejects unknown fields (task 084) — so forwarding the stored row
          // as-is would be a 400 on every batch.
          const payload = points.map((p) => ({
            ts: p.ts,
            lat: p.lat,
            lng: p.lng,
            accuracy: p.accuracy,
          }))

          const accepted = await this.send(payload)
          if (!accepted) return

          await markUploaded(
            session.user.userId,
            points.map((p) => p.ts),
          )
          this.lastUploadAt = Date.now()
          this.uploadError = ''
          app.setOnline(true)
          void logEvent('upload', `${points.length} points`)

          // A short chunk means the queue is drained; stop rather than spend another
          // round-trip discovering it is empty.
          if (points.length < TRACK_UPLOAD_CHUNK) break
        }

        await this.refreshCount()
        // Uploaded points stop earning their storage once the race they belong to is over.
        const cutoff = Date.now() - TRACK_KEEP_UPLOADED_HOURS * 60 * 60 * 1000
        await pruneUploaded(session.user.userId, cutoff)
      } catch (err) {
        // Anything unexpected (an IndexedDB failure mid-flush) must not leave the uploader
        // wedged: the points are still pending, so the next interval retries.
        this.uploadError = err instanceof Error ? err.message : 'ukendt fejl'
        console.error('track upload failed', err)
      } finally {
        this.uploading = false
      }
    },

    // send posts one batch. Returns true when the server accepted it.
    //
    // Classifying the failure is the whole job here: the difference between "retry in two
    // minutes" and "stop, this will never work" decides whether a member's track survives.
    async send(points: { ts: number; lat: number; lng: number; accuracy: number }[]): Promise<boolean> {
      const app = useAppStore()
      try {
        await fetchWrapper.post<{ accepted: number; dropped: number }>('/api/track', { points })
        return true
      } catch (err) {
        if (err instanceof NetworkError) {
          // No signal. The normal case during the race, and not worth an error message.
          app.setOnline(false)
          this.uploadError = 'ingen forbindelse'
          void logEvent('uploadfail', 'offline')
          return false
        }
        if (err instanceof HttpError) {
          if (err.status === 401) {
            // The session expired. Stop the cycle rather than hammer a dead session; the
            // points stay pending and ship after the next sign-in.
            this.uploadError = 'ikke logget ind'
            this.stopUploading()
            void logEvent('uploadfail', '401')
            return false
          }
          if (err.status === 400) {
            // A batch the server will never accept — our bug. See uploadBlocked.
            this.uploadBlocked = true
            this.uploadError = `afvist af serveren: ${err.message}`
            this.stopUploading()
            void logEvent('uploadfail', `400 ${err.message}`)
            return false
          }
          // 413 (too large), 429 (rate limited), 5xx, 503 (broker down) — all temporary or
          // self-correcting. 413 in particular should be impossible, since the chunk is a
          // quarter of the server's bound; if it happens the retry is harmless and the
          // message says what to look at.
          this.uploadError = `serverfejl (${err.status})`
          void logEvent('uploadfail', String(err.status))
          return false
        }
        throw err
      }
    },
  },
})
