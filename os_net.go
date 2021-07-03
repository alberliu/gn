// +build !linux

package gn

func listen(port int) error {
	panic("please run in linux")
}

func accept() (nfd int, addr string, err error) {
	panic("please run in linux")
}

func close(fd int) error {
	panic("please run in linux")
}

func getEvents() ([]event, error) {
}
