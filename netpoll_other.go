//go:build !linux
// +build !linux

package gn

func newNetpoll(address string) (netpoll, error) {
	panic("please run on linux")
}
