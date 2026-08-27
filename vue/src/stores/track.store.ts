import { defineStore } from 'pinia'
import {
  TRACK_FIX_TIMEOUT_MS,
  TRACK_MAX_FIX_AGE_MS,
  TRACK_MAX_POINTS,
  TRACK_SAMPLE_SECONDS,
} from '@/config/track'
import {
  appendPoint,
  countPoints,
  latestTimestamp,
  requestPersistentStorage,
  TrackStorageFullError,
} from '@/helpers/trackDb'
import { useLocationStore } from '@/stores/location.store'
import { useSessionStore } from '@/stores/session.store'

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
      if (!session.user || location.permission !== 'granted') return
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
      if (this.lastPointAt && elapsed >= 0 && elapsed < minGapMs) return

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

      const fresh =
        location.position && Date.now() - location.positionAt <= TRACK_MAX_FIX_AGE_MS
          ? { coords: location.position, at: location.positionAt }
          : await this.acquire()

      if (!fresh) return

      if (this.pointCount >= TRACK_MAX_POINTS) {
        // A safety net, not a policy: this is ~7 days of sampling against a 12-hour
        // race, so reaching it means something is wrong. Stop and surface it instead of
        // discarding points that nothing has copied anywhere else yet.
        this.problem = 'capped'
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
      } catch (err) {
        if (err instanceof TrackStorageFullError) {
          // Handled rather than thrown into the void: a recorder whose writes silently
          // fail looks exactly like a working one.
          this.problem = 'full'
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
            if (err.code === err.PERMISSION_DENIED) {
              location.permission = 'denied'
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
      } catch (err) {
        console.error('failed to count recorded positions', err)
      }
    },
  },
})
