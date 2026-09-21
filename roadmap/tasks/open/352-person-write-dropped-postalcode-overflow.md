# 352 — A person write is silently dropped on every replay: postalCode overflows

**Status:** open
**Priority:** high
**Created:** 2026-09-21

## Description

Found by rebuilding every projection from the stream (task 350's experiment). The `deadletter` table holds
one unresolved row, written **during the replay** — so this is not a historical blip, it happens on every
boot that rebuilds:

```
Error 1406 (22001): Data too long for column 'postalCode' at row 1

INSERT INTO person (address, appRole, city, …, postalCode, …) VALUES
  ("", "spejder", "", …, "Drejøgade 28, 6. dør 603 2100 København ø", …)
  ON DUPLICATE KEY UPDATE …
```

`person.postalCode` is `VARCHAR(32)`. Upstream sent **the whole address** — street, postcode and city
concatenated, 40 characters — in that one field, with `address` and `city` empty. MariaDB rejects the
statement, `cqrs`'s dead-letter writer records it, and **the person is not written**.

### Why nobody noticed

That member *is* in the directory, because a **later** event for the same person carried the split form
(`address: "Drejøgade 28, 6. dør 603"`, `postalCode: "2100 København ø"`) and its upsert succeeded. So the
symptom was invisible: the projection is complete by luck, not by correctness. A member whose only event is
the concatenated variant would simply be absent — no error visible anywhere a human looks.

### It is not one row

159 of 4,230 people (**3.8%**) have a `postalCode` longer than 10 characters, because upstream routinely packs
`"2100 København ø"` into it. Those fit in 32 characters and are stored; they are still not postal codes, which
matters for anything that formats an address. The failure only becomes a *dropped write* when the packed value
exceeds 32.

### Two things to fix, and they are separable

1. **The column is too narrow for what upstream sends.** Widening it (or storing the value as an opaque
   "address line" and parsing on read) stops the write being lost. `person/table.go`'s additive-column list is
   the mechanism; note that *narrowing* a column later needs a real migration, so pick a width once.
2. **Nothing surfaces a dead letter.** The table exists precisely for this and is read by nobody: no startup
   log line, no health field, no test. I found this row by looking at the table on a hunch. A projection that
   drops a write silently is worse than one that fails loudly, and the dead-letter table was supposed to be
   the loud part.

## Acceptance Criteria

- [ ] A `person` event carrying a 40-character `postalCode` is stored rather than rejected — verified by
      replaying the stream and finding **no** new dead letter.
- [ ] The decision between "widen the column" and "normalise on the way in" is recorded here, including what
      happens to the 159 rows that already hold a postcode-plus-city string.
- [ ] Unresolved dead letters are **visible without anybody looking for them**: a startup count in the log at
      minimum, and a decision recorded about whether they belong in the health endpoint.
- [ ] A test covers the over-long value at the projection level, so the fix cannot regress quietly.
- [ ] Check the other projections' string columns against what upstream actually sends — this is unlikely to be
      the only column sized from an assumption.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-21 — Found while rebuilding all projections from the stream for task 350. Worth recording *how* it
  surfaced: not from a bug report, and not from the dead-letter table being monitored, but because a replay was
  run for an unrelated reason and the table happened to be non-empty.
