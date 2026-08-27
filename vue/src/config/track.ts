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
