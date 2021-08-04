// +build !linux

package gn

func listen(port string) error {
	panic("please run in linux")
}

func accept() (nfd int, addr string, err error) {
	panic("please run in linux")
}

func closeFD(fd int) error {
	panic("please run in linux")
}

func closeFDRead(fd int) error {
	panic("please run in linux")
}

func getEvents() ([]event, error) {
	panic("please run in linux")
}
