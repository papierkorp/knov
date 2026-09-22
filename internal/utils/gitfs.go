package utils

import (
	"os"
	"runtime"

	"github.com/go-git/go-billy/v5"
)

// DotGitFilesystem returns the billy filesystem for root's ".git" directory, for
// building a go-git Storer. Equivalent to what git.PlainInit/git.PlainOpen/git.PlainOpen
// build internally, except on Android: there, it's wrapped so every file it hands out is
// lock-free (see noRWFilesystem/noLockFile below) - Android's app-sandbox seccomp policy
// rejects flock with ENOSYS ("function not implemented"), which otherwise breaks every git
// init/commit in the Android wrapper app (see android/). Safe to skip locking here since
// each knov instance is the only process ever touching its own data dir.
//
// Shared by configmanager and git (an import cycle rules out either package
// owning it, since git already imports configmanager) - keep both callers
// routed through here rather than reintroducing separate copies.
func DotGitFilesystem(root billy.Filesystem) (billy.Filesystem, error) {
	dot, err := root.Chroot(".git")
	if err != nil {
		return nil, err
	}
	if runtime.GOOS == "android" {
		return &noRWFilesystem{dot}, nil
	}
	return dot, nil
}

// noRWFilesystem hides ReadAndWriteCapability so go-git's dotgit package takes its
// lock-free ref-write path (dotgit.setRefNorwfs) instead of calling File.Lock() - but
// that capability check only gates that one code path (see dotgit.setRef). Other
// call sites, e.g. dotgit.openAndLockPackedRefs (rewriting packed-refs, hit when
// deleting a ref that's been packed), call File.Lock() unconditionally regardless of
// capabilities. Every file this filesystem hands out is wrapped in noLockFile so
// Lock()/Unlock() are no-ops everywhere, closing that gap for any go-git code path -
// current or future - rather than only the ones already traced through its source.
type noRWFilesystem struct {
	billy.Filesystem
}

func (fs *noRWFilesystem) Capabilities() billy.Capability {
	return billy.Capabilities(fs.Filesystem) &^ billy.ReadAndWriteCapability
}

func (fs *noRWFilesystem) Create(filename string) (billy.File, error) {
	f, err := fs.Filesystem.Create(filename)
	return noLock(f), err
}

func (fs *noRWFilesystem) Open(filename string) (billy.File, error) {
	f, err := fs.Filesystem.Open(filename)
	return noLock(f), err
}

func (fs *noRWFilesystem) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	f, err := fs.Filesystem.OpenFile(filename, flag, perm)
	return noLock(f), err
}

func (fs *noRWFilesystem) TempFile(dir, prefix string) (billy.File, error) {
	f, err := fs.Filesystem.TempFile(dir, prefix)
	return noLock(f), err
}

func noLock(f billy.File) billy.File {
	if f == nil {
		return nil
	}
	return &noLockFile{f}
}

type noLockFile struct {
	billy.File
}

func (f *noLockFile) Lock() error   { return nil }
func (f *noLockFile) Unlock() error { return nil }
