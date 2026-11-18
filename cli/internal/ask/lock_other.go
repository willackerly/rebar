//go:build !(darwin || dragonfly || freebsd || netbsd || openbsd || linux)

package ask

// lockFile: no flock here; callers run unlocked.
func lockFile(string, bool) (func(), bool) { return func() {}, true }
