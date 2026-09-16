import { HttpError } from '@/helpers/fetchWrapper'

// The client half of PRD 017's multiplexed freshness check: what a store does with a version it was
// handed by `GET /api/sync`.
//
// ────────────────────────────────────────────────────────────────────────────────────────────────
// THE CONTRACT, for every store the sync loop dispatches to
// ────────────────────────────────────────────────────────────────────────────────────────────────
//
// 1. **A version is opaque.** Compare it for equality and nothing else. Never parse one, order two,
//    or read a timestamp out of one — the server is free to change how they are derived, and the only
//    promise it makes is that an unchanged dataset keeps its version.
//
// 2. **At most one request.** The whole economy of the sync check is that the common answer costs
//    nothing: same version, no request at all. A store that refetched "just to be sure" would move
//    the cost straight back to where this design removed it from.
//
// 3. **Replace, do not merge** (PRD 009 §6). A refetch replaces the cached copy wholesale, so a map
//    sheet that was reassigned or a contact who left the race stops existing on the device. A delta
//    would need explicit tombstones, or the server's purge is decorative.
//
// 4. **Only record a version we actually hold the data for.** This is the subtle one and it is the
//    reason this file exists rather than six copies of three lines: storing the new version after a
//    *failed* refetch would leave the device holding old data labelled as current, and it would never
//    ask again. That is the silent-stale failure PRD 017 is written to prevent, arriving through the
//    client instead of the server.
//
// 5. **Never throw.** A failed refresh keeps the cached copy and records staleness. Every store here
//    already promises that; this must not become the one path that breaks it.

/**
 * The datasets `GET /api/sync` can report on.
 *
 * A union rather than `string`, so a typo in a dispatch table is a type error instead of a dataset
 * that silently never refreshes — which would look exactly like the feature working.
 */
export type SyncDataset =
  | 'contacts'
  | 'profile'
  | 'scans'
  | 'handouts'
  | 'checkpoints'
  | 'race_area'

/**
 * Compare a held version with the server's and load if they differ.
 *
 * @param held    the version the store holds, or '' when it holds nothing
 * @param incoming the version from `/api/sync`
 * @param load    fetches and replaces the payload; must resolve `true` only when it succeeded
 * @returns whether a refetch happened *and* succeeded, so the caller knows whether to store
 *          `incoming` as held
 *
 * An empty `held` means nothing is cached, and then the version is irrelevant: fetch outright. That
 * makes a first sync indistinguishable from a refresh for every caller, which is what lets the sync
 * loop replace the old quiet-prefetch pass without a special case for "never synced".
 */
export async function versionedRefresh(
  held: string,
  incoming: string,
  load: () => Promise<boolean>,
): Promise<boolean> {
  if (held !== '' && held === incoming) return false
  return await load()
}

/**
 * Whether a failed request means "you may not have this" rather than "we could not fetch it".
 *
 * A 403 is not a transient failure and must not be retried on every foreground. The store's own
 * handling decides what to do with it; this only names the case, so the six stores agree on what a
 * 403 is.
 */
export function isForbidden(err: unknown): boolean {
  return err instanceof HttpError && err.status === 403
}
