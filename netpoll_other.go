//go:build windows && cgo
// +build windows,cgo

package gn

func newNetpoll(address string) (netpoll, error) {
	panic("please run on linux or mac")
}
