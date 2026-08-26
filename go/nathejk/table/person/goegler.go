package person

import "github.com/nathejk/shared-go/types"

// Gøgler message shapes, declared here because they exist nowhere else.
//
// # Why these are not imported
//
// Every other population in this projection reads a struct from
// github.com/nathejk/shared-go/messages. Gøglere have none: a search of shared-go
// turns up no gøgler message, no gøgler entity and no gøgler table. hq keeps this
// population in its own local `personnel` table, which this module cannot import.
//
// So the wire shapes below were read off the live stream rather than copied from a
// published contract. That is worth being explicit about, because it means these
// structs are an *observation*, not an agreement: nothing upstream fails if tilmelding
// changes them.
//
// # The duplication this creates
//
// Projecting gøglere here makes this the **second** projection of the same population
// in a second repo, and the two can silently disagree — hq's `personnel` and this
// table are both derived from the same events but by different code, with different
// classification and different notions of deletion.
//
// That is a knowing trade, not an oversight. The alternatives were worse: reading hq's
// projection would mean one service calling another's API (forbidden — this app's api
// is strictly a BFF), and promoting hq's slice to shared-go is a cross-repo change that
// blocks this app's login on another repo's release. PRD 006 §11 Q4 tracks the promotion
// as the intended end state; until then, this file is the duplication.
//
// # Identity
//
// One person, two spellings. `signedup` keys them as `teamId` and `updated` as
// `userId`, and both equal the subject's entity id — verified on the stream for a
// person carrying both events. Neither handler trusts the body alone.

// NathejkGoeglerSignedUp is the signup event.
//
// It carries only the four fields the signup form collects, and it is **not**
// redundant with the updated event below. On the 2026 data, 31 of 99 gøglere have a
// `signedup` and no `updated` at all (26 of 125 in 2025) — roughly a third of the
// population exists nowhere else. This is the opposite of the crew case, where
// `crew.*.signedup` is a strict subset of `crewmember.*.updated` and is deliberately
// not consumed. The two looked alike enough that copying the crew decision would have
// silently dropped a third of the gøglere; the counts are why this one is consumed.
type NathejkGoeglerSignedUp struct {
	// TeamID is the person's id despite the name, mirroring crew.*.signedup.
	TeamID types.TeamID       `json:"teamId"`
	Name   string             `json:"name,omitempty"`
	Phone  types.PhoneNumber  `json:"phone,omitempty"`
	Email  types.EmailAddress `json:"email,omitempty"`
}

// NathejkGoeglerUpdated is the richer profile event.
//
// Only the fields this projection stores are declared. The real body also carries
// hqAccess, pincode, department, medlemnr, klan, tshirtsize, corps and an `additionals`
// map (diet, car, days, ...) — none of which the directory has a column for, and one of
// which (pincode) it should deliberately never touch.
type NathejkGoeglerUpdated struct {
	UserID types.UserID       `json:"userId"`
	Name   string             `json:"name,omitempty"`
	Phone  types.PhoneNumber  `json:"phone,omitempty"`
	Email  types.EmailAddress `json:"email,omitempty"`
	// Group is the person's scout group, e.g. "Strandboerne Vallensbæk". It is the only
	// affiliation a gøgler has — see handleGoeglerUpdated for why it is stored.
	Group string `json:"group,omitempty"`
}
