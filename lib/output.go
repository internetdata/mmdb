package lib

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// outFile is a database being written to path, through a new file beside it
// that commit renames over it where it can, so a failed write leaves path whole
// and a reader that has it open keeps its copy.
type outFile struct {
	*os.File
	path   string
	tmp    string
	backup string
}

// backupName is where a backup of out is kept: out with suffix.
func backupName(out, suffix string) (string, error) {
	if suffix == "" {
		return "", errors.New("--backup needs a suffix, such as .bak")
	}
	if strings.ContainsAny(suffix, "/"+string(filepath.Separator)) {
		return "", errors.New("backup suffix must not contain a path separator")
	}
	return out + suffix, nil
}

// checkBackup fails, before any input is read, where the regular file at out
// could not be kept as backup, or keeping it would replace a file still needed.
func checkBackup(out, backup, in string) error {
	o, err := os.Stat(out)
	if err != nil || !o.Mode().IsRegular() {
		return nil
	}
	b, err := os.Lstat(backup)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("could not back up %v: %w", out, err)
	}
	why := ""
	if b.IsDir() {
		why = "a directory"
	} else if os.SameFile(o, b) {
		why = "another name for " + out
	} else if i, err := os.Stat(in); err == nil && os.SameFile(i, b) {
		why = "the input"
	}
	if why != "" {
		return fmt.Errorf("could not back up %v to %v, which is %v", out, backup, why)
	}
	return nil
}

// createOut writes beside path only where a rename leaves what writing path
// would: a new path, or a regular file with one name that this process may
// write, whose mode, owner and group the new file takes. Elsewhere, and when
// the directory refuses a new file, it copies path to backup, then writes it.
func createOut(path, backup string) (*outFile, error) {
	if tmp, err := createBeside(path); err == nil {
		return &outFile{File: tmp, path: path, tmp: tmp.Name(), backup: backup}, nil
	}
	if backup != "" {
		if err := copyToBackup(path, backup); err != nil {
			return nil, err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("could not create %v: %w", path, err)
	}
	return &outFile{File: f, path: path}, nil
}

// createBeside creates a file in path's directory under a name no other file
// has, with the mode, owner and group of the file at path, if there is one.
func createBeside(path string) (*os.File, error) {
	if _, base := filepath.Split(path); base == "" {
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

	var f *os.File
	name, err := beside(path, func(name string) (err error) {
		f, err = os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
		return err
	})
	if err != nil {
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

// beside calls create with names in path's directory until one is not taken,
// and returns that name. They are hidden, and this process's own.
func beside(path string, create func(name string) error) (string, error) {
	dir, base := filepath.Split(path)
	for i := range 100 {
		name := filepath.Join(dir, fmt.Sprintf(".%s.%d-%d.tmp", base, os.Getpid(), i))
		if err := create(name); !errors.Is(err, fs.ErrExist) {
			return name, err
		}
	}
	return "", errors.New("no free name beside " + path)
}

// commit keeps the file at path as the backup, then renames a file written
// beside path over it, or copies it in where a rename cannot, as on a mount point.
func (o *outFile) commit() error {
	err := o.Close()
	if err != nil || o.tmp == "" {
		return err
	}
	tmp := o.tmp
	o.tmp = ""
	linked := false
	if o.backup != "" {
		linked, err = keep(o.path, o.backup)
		if err != nil {
			os.Remove(tmp)
			return err
		}
	}
	if os.Rename(tmp, o.path) == nil {
		return nil
	}
	// Copying in writes through every name the file has, so a backup that is
	// one of them becomes a copy first.
	if linked {
		err = copyToBackup(o.path, o.backup)
	}
	if err == nil {
		err = copyFile(tmp, o.path)
	}
	os.Remove(tmp)
	return err
}

// keep links the file at path to backup, in place of whatever had that name,
// or copies it there where the filesystem has no hard links.
func keep(path, backup string) (bool, error) {
	name, err := beside(path, func(name string) error {
		return os.Link(path, name)
	})
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, copyToBackup(path, backup)
	}
	if err := os.Rename(name, backup); err != nil {
		os.Remove(name)
		return false, fmt.Errorf("could not back up %v to %v: %w", path, backup, err)
	}
	return true, nil
}

// copyToBackup copies the regular file at path, through a symlink, to a new
// file that then replaces backup. Anything else at path has nothing to keep.
func copyToBackup(path, backup string) (err error) {
	fi, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) || err == nil && !fi.Mode().IsRegular() {
		return nil
	}
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not back up %v to %v: %w", path, backup, err)
		}
	}()
	if err != nil {
		return err
	}
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	var dst *os.File
	name, err := beside(path, func(name string) (err error) {
		dst, err = os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fi.Mode().Perm())
		return err
	})
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	if err == nil {
		// The umask may have taken bits off the mode. The owner and group are
		// kept where this process may give them, as root can.
		err = dst.Chmod(fi.Mode().Perm())
		takeOwner(dst, fi)
	}
	if cerr := dst.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, backup)
	}
	if err != nil {
		os.Remove(name)
	}
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
