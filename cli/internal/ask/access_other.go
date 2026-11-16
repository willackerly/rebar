//go:build !(darwin || dragonfly || freebsd || netbsd || openbsd || linux)

package ask

func accessOK(string) bool { return true }
