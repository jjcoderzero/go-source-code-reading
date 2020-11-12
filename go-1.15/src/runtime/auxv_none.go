// +build !linux
// +build !darwin
// +build !dragonfly
// +build !freebsd
// +build !netbsd
// +build !openbsd !arm64
// +build !solaris

package runtime

func sysargs(argc int32, argv **byte) {
}
