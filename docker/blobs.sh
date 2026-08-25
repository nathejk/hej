#!/bin/sh
# Backup, restore and verify the portrait blob store.
#
# WHY ONLY THIS DIRECTORY
#
# Everything else this service stores is a projection of the JetStream event log
# and rebuilds from it on boot (PRD 008 §8) — backing the database up would cost
# effort and add a way to restore a stale read model over a good one. The portrait
# bytes are the exception: the event carries a content hash, not the image, so if
# these files are gone they are gone. This directory is the entire backup scope.
#
# WHY VERIFICATION NEEDS NO MANIFEST
#
# The store is content-addressed: a file's name IS the SHA-256 of its contents
# (see go/internal/blob). So a restore can be checked against nothing but itself —
# recompute each hash and compare it to the filename. A truncated file, a bit flip
# or a half-finished transfer all show up, with no separate checksum list to keep in
# sync and no trust in the backup tooling required.
#
# USAGE
#
#   blobs.sh backup  <src-dir> <dest-dir>   # write a timestamped tar.gz
#   blobs.sh restore <archive>  <dest-dir>  # extract, then verify
#   blobs.sh verify  <dir>                  # check every file against its name
#
# In production <src-dir> is the bind mount from docker-swarm.yml, /srv/hej/blobs,
# on the node labelled hej.storage=true. It can be read straight off the host —
# that is the reason a bind mount was chosen over a named volume.
#
#   ./blobs.sh backup /srv/hej/blobs /var/backups/hej
#
# RETENTION is not implemented here on purpose. How long portraits of minors may be
# kept is an unanswered consent question (PRDs 003/007), and a script that quietly
# deletes them before that is settled would be worse than no script. Add the prune
# step once there is a retention period to implement.

set -eu

usage() {
	sed -n '2,36p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
}

# sha256 of a file, portable between macOS (shasum) and Linux (sha256sum).
sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

verify() {
	dir=$1
	[ -d "$dir" ] || { echo "verify: $dir is not a directory" >&2; exit 1; }

	checked=0
	bad=0
	# Only the two-hex-character bucket directories hold objects; anything else
	# (a stray .tmp-* from an interrupted write, an editor file) is reported rather
	# than silently ignored, because an unexpected file in a content-addressed store
	# means something wrote to it that should not have.
	for path in $(find "$dir" -type f | sort); do
		name=$(basename "$path")
		case $name in
		.tmp-*)
			echo "WARN unexpected temp file: $path" >&2
			bad=$((bad + 1))
			continue
			;;
		esac

		actual=$(sha256_of "$path")
		if [ "$actual" != "$name" ]; then
			echo "CORRUPT $path" >&2
			echo "        name says $name" >&2
			echo "        content is $actual" >&2
			bad=$((bad + 1))
		fi
		checked=$((checked + 1))
	done

	if [ "$bad" -ne 0 ]; then
		echo "verify: FAILED — $bad problem(s) across $checked object(s)" >&2
		exit 1
	fi
	echo "verify: OK — $checked object(s), every hash matches its name"
}

backup() {
	src=$1
	dest=$2
	[ -d "$src" ] || { echo "backup: $src is not a directory" >&2; exit 1; }

	# Verify BEFORE creating the destination or writing anything. Backing up
	# already-corrupt data quietly propagates the corruption into every future
	# restore, and failing without leaving an empty backup directory behind keeps the
	# backup target honest about what actually succeeded.
	verify "$src"

	mkdir -p "$dest"

	stamp=$(date -u +%Y%m%dT%H%M%SZ)
	archive="$dest/hej-blobs-$stamp.tar.gz"

	# -C so paths in the archive are relative to the store root, which makes the
	# archive restorable into a different directory (a staging copy, another node)
	# rather than only into the path it came from.
	tar -czf "$archive" -C "$src" .
	chmod 600 "$archive"

	count=$(find "$src" -type f | wc -l | tr -d ' ')
	echo "backup: wrote $archive ($count object(s))"
	echo "$archive"
}

restore() {
	archive=$1
	dest=$2
	[ -f "$archive" ] || { echo "restore: $archive not found" >&2; exit 1; }

	# 0700: these are photographs of identifiable minors, and go/internal/blob
	# enforces the same mode when it creates the directory itself. A restore must not
	# be the step that widens it.
	mkdir -p "$dest"
	chmod 700 "$dest"

	tar -xzf "$archive" -C "$dest"

	# Verifying here is the point of the whole script: a backup that has never been
	# restored is a hope, not a backup.
	verify "$dest"
	echo "restore: OK — $archive restored into $dest"
}

[ $# -ge 1 ] || usage
cmd=$1
shift

case $cmd in
backup)
	[ $# -eq 2 ] || usage
	backup "$1" "$2"
	;;
restore)
	[ $# -eq 2 ] || usage
	restore "$1" "$2"
	;;
verify)
	[ $# -eq 1 ] || usage
	verify "$1"
	;;
*)
	usage
	;;
esac
