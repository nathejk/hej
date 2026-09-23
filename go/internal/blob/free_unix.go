//go:build unix

package blob

import (
	"fmt"
	"syscall"
)

// Free space on the volume holding the objects (PRD 022 §8.11, task 384).
//
// # Why this is here and not a capacity column somewhere
//
// The blob store is the only non-rebuildable data in this service (PRD 008 §8): every projection replays
// from the log, the objects do not. PRD 022 §8.11 calls the capacity question *the largest operational risk
// in the PRD* and is explicit that it is not a code risk — but one part of it is, and it is this: the
// failure mode to prevent is a photographer invited to upload a full card, **the disk filling mid-batch**,
// and half a hand-in left behind with the photographer's card already wiped.
//
// A byte quota cannot prevent that honestly. The glimt ceiling can be a quota because retention frees space
// on a schedule everybody was told about (PRD 019); here PRD 022 §11 Q2 resolved that photographs are
// **never purged**, so a quota would eventually refuse a legitimate hand-in with no way through except an
// operator raising a number. What actually needs checking is a property of the volume, not of the feature.
//
// So the store reports free space and the upload path decides. Asking the filesystem rather than summing a
// projection is also the only measurement that counts the *other* tenants of the volume — portraits, glimt
// media, and on the production bind mount whatever else shares the disk.
//
// The `FreeSpacer` interface itself lives in blob.go, because this file does not exist on every platform and
// callers must still compile where it does not.

// FreeBytes reports the space available on the filesystem holding the object root.
//
// `Bavail`, not `Bfree`: the difference is the reserved blocks only root may use, and this process is not
// root — counting them would promise room that a write cannot have. On the one occasion that distinction
// matters, it matters completely.
func (s *FileStore) FreeBytes() (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(s.root, &st); err != nil {
		return 0, fmt.Errorf("blob: statfs %s: %w", s.root, err)
	}
	// Both fields are widened rather than assumed: Bsize is int64 on Linux and uint32 on Darwin, and this
	// repository is developed on one and deployed on the other.
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
