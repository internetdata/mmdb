package lib

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// outFile is a database being written to path, through a new file beside it
// that commit renames over it where it can, so a failed write leaves path whole
// and a reader that has it open keeps its copy.
type outFile struct {
	*os.File
	path string
	tmp  string
}

// createOut writes beside path only where a rename leaves what writing path
// would: a new path, or a regular file with one name that this process may
// write, whose mode, owner and group the new file takes. Elsewhere, and when
// the directory refuses a new file, it writes path directly.
func createOut(path string) (*outFile, error) {
	if tmp, err := createBeside(path); err == nil {
		return &outFile{File: tmp, path: path, tmp: tmp.Name()}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &outFile{File: f, path: path}, nil
}

// createBeside creates a file in path's directory under a name no other file
// has, with the mode, owner and group of the file at path, if there is one.
func createBeside(path string) (*os.File, error) {
	dir, base := filepath.Split(path)
	if base == "" {
		return nil, errors.New("no file name")
	}
	perm := fs.FileMode(0666)
	old, err := os.Lstat(path)
	if err == nil {
		if !old.Mode().IsRegular() || links(old) > 1 {
			return nil, errors.New("not a regular file with one name")
		}
		f, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return nil, err
		}
		f.Close()
		perm = old.Mode().Perm()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	for i := range 100 {
		name := filepath.Join(dir, fmt.Sprintf(".%s.%d-%d.tmp", base, os.Getpid(), i))
		f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
		if errors.Is(err, fs.ErrExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		if old != nil {
			// The umask may have taken bits off the mode.
			err = f.Chmod(perm)
			if err == nil {
				err = takeOwner(f, old)
			}
			if err != nil {
				f.Close()
				os.Remove(name)
				return nil, err
			}
		}
		return f, nil
	}
	return nil, errors.New("no free name beside " + path)
}

// commit renames a file written beside path over it, or copies it in where a
// rename cannot replace path, as on a mount point.
func (o *outFile) commit() error {
	err := o.Close()
	if err != nil || o.tmp == "" {
		return err
	}
	tmp := o.tmp
	o.tmp = ""
	if os.Rename(tmp, o.path) == nil {
		return nil
	}
	err = copyFile(tmp, o.path)
	os.Remove(tmp)
	return err
}

// abort removes a file written beside path, leaving path as it was.
func (o *outFile) abort() {
	o.Close()
	if o.tmp != "" {
		os.Remove(o.tmp)
		o.tmp = ""
	}
}

func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(to)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	if cerr := dst.Close(); err == nil {
		err = cerr
	}
	return err
}
