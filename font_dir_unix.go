// go:build aix || dragonfly || freebsd || (js && wasm) || nacl || linux || netbsd || openbsd || solaris
//go:build aix || dragonfly || freebsd || (js && wasm) || nacl || linux || netbsd || openbsd || solaris
// +build aix dragonfly freebsd js,wasm nacl linux netbsd openbsd solaris

package main

import (
	"os"
	"path/filepath"
)

func userFontDir() string {
	home, _ := os.UserHomeDir()
	// default $HOME/.local/share/fonts
	return filepath.Join(home, ".local/share/fonts")
}

func systemFontDir() string {
	return "/usr/local/share/fonts"
}
