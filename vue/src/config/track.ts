// Position-track recording configuration (PRD 002 §11.1, task 082).

/**
 * Seconds between recorded positions.
 *
 * 30 s, decided 2026-08-26: "team are walking, we do not need sub-30 s resolution."
 *
 * It is the right interval, not merely a cheap one — worth knowing so nobody
 * "improves" it later. At walking pace 30 s puts consecutive points **33 m apart at
 * 4 km/h and 50 m at 6 km/h**, while a phone's GPS error under forest canopy at night
 * is **10–30 m**. Below ~30 s the spacing between points is smaller than the error on
 * each point, so the extra samples record receiver noise rather than movement.
 *
 * Cost at this rate: 1,440 points per person for a 12-hour race, ~195 KB per person,
 * ~157 MB per event. Trivial against any plausible quota, including the pre-iOS-17
 * ~1 GB ceiling, so there is no reason to be clever about compaction.
 */
export const TRACK_SAMPLE_SECONDS = 30

/**
 * How old a position may be and still be recorded as the current sample, in ms.
 *
 * This is the whole battery story of this feature. While the map is open it already
 * runs a high-accuracy `watchPosition`, so the recorder reuses that fix and costs
 * *nothing* extra. Only when nothing else is watching does it ask for its own.
 *
 * Slightly less than the sample interval on purpose: at exactly 30 s a fix could be
 * reused for two consecutive samples, recording the same position twice and reporting
 * a stationary walker.
 */
export const TRACK_MAX_FIX_AGE_MS = 25_000

/**
 * How long to wait for a fix before giving up on this sample, in ms.
 *
 * A missed sample is fine — the next one is 30 s away and the track is expected to be
 * fragmented anyway. Hanging is not: a pending geolocation callback holds the radio.
 */
export const TRACK_FIX_TIMEOUT_MS = 20_000

/**
 * Hard ceiling on stored points, as a safety net rather than a policy.
 *
 * 20,000 points is roughly **7 days** of continuous 30 s sampling, against a 12-hour
 * race. Reaching it therefore means something is wrong — a recorder started twice, a
 * clock jumping, an interval firing far too often — and the honest response is to stop
 * writing and say so, not to quietly discard points.
 *
 * Deliberately NOT implemented as "drop the oldest": the oldest points are the ones
 * most likely to matter (they may be the only record of where a team was hours ago),
 * and until task 083 ships there is nothing that has copied them anywhere else.
 */
export const TRACK_MAX_POINTS = 20_000

/**
 * Seconds between upload attempts (PRD 002 §11.1).
 *
 * An attempt is skipped entirely when nothing new has been recorded, which is not a
 * micro-optimisation: a stationary phone — a member asleep at a checkpoint, or one whose GPS
 * has produced no new fix — would otherwise pay for 360 pointless requests over a 12-hour
 * race, on rural mobile data, from a battery that has to last the night.
 */
export const TRACK_UPLOAD_INTERVAL_SECONDS = 120

/**
 * Maximum points in one upload request.
 *
 * Well under the server's 2,000-point limit on purpose. A normal batch is 4 points (2 minutes
 * at 30 s sampling); this bound only matters for a backlog, and there the smaller number is
 * better: a member who was offline for hours is, by definition, somewhere with poor
 * connectivity, and a 30 KB request that succeeds beats a 90 KB one that times out. The
 * backlog then ships as several requests instead of one all-or-nothing attempt.
 */
export const TRACK_UPLOAD_CHUNK = 500

/**
 * How many chunks one upload run will send before yielding until the next interval.
 *
 * Bounds a backlog burst: 4 × 500 = 2,000 points per run, which clears a full 12-hour
 * offline race in one pass while staying far inside the endpoint's 20-requests-per-minute
 * per-user limit (task 084). Without a bound, reconnecting after a long outage would fire as
 * many requests as it took, which is exactly when being rate limited would cost the most.
 */
export const TRACK_UPLOAD_MAX_CHUNKS_PER_RUN = 4

/**
 * How long an already-uploaded point is kept on the device, in hours.
 *
 * Long enough to cover the race it belongs to, so the status page can answer "what did this
 * phone record?" rather than only "what is still waiting". Not longer: the server has these
 * points, and the storage quota is shared with map tiles and portraits, which cannot be
 * re-fetched as cheaply.
 */
export const TRACK_KEEP_UPLOADED_HOURS = 18
