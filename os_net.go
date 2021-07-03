// +build !linux

package gn

func listen(port int) error {
	panic("please run in linux")
}

func accept() (nfd int, addr string, err error) {
	panic("please run in linux")
}

func closeFD(fd int) error {
	panic("please run in linux")
}

func getEvents() ([]event, error) {
	panic("please run in linux")
}
