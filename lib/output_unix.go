//go:build unix

package lib

import (
	"io/fs"
	"os"
	"syscall"
)

func links(fi fs.FileInfo) uint64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Nlink)
	}
	return 1
}

// takeOwner gives f the owner and group of old, which writing old directly
// would have kept.
func takeOwner(f *os.File, old fs.FileInfo) error {
	want, ok := old.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	have, ok := fi.Sys().(*syscall.Stat_t)
	if ok && have.Uid == want.Uid && have.Gid == want.Gid {
		return nil
	}
	return f.Chown(int(want.Uid), int(want.Gid))
}
