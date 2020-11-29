// +build linux

package gn

import (
	"golang.org/x/sys/unix"
	"syscall"
)

// 对端关闭连接 8193
const (
	EpollRead  = syscall.EPOLLIN | syscall.EPOLLPRI | syscall.EPOLLERR | syscall.EPOLLHUP | unix.EPOLLET | syscall.EPOLLRDHUP
	EpollClose = uint32(syscall.EPOLLIN | syscall.EPOLLRDHUP)
)

type epoll struct {
	fd  int
	lfd int32
}

func EpollCreate(port int) (*epoll, error) {
	lfd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	err = syscall.Bind(lfd, &syscall.SockaddrInet4{Port: port})
	if err != nil {
		log.Error(err)
		return nil, err
	}
	err = syscall.Listen(lfd, 1024)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	fd, err := syscall.EpollCreate1(0)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	return &epoll{
		fd:  fd,
		lfd: int32(lfd),
	}, nil
}

func (e *epoll) AddRead(fd int) error {
	err := syscall.EpollCtl(e.fd, syscall.EPOLL_CTL_ADD, fd, &syscall.EpollEvent{
		Events: EpollRead,
		Fd:     int32(fd),
	})
	if err != nil {
		return err
	}
	return nil
}

func (e *epoll) RemoveAndClose(fd int) error {
	// 移除文件描述符的监听
	err := syscall.EpollCtl(e.fd, syscall.EPOLL_CTL_DEL, fd, nil)
	if err != nil {
		return err
	}

	// 关闭文件描述符
	err = syscall.Close(fd)
	if err != nil {
		return err
	}

	return nil
}

func (e *epoll) EpollWait(handler func(event Event)) error {
	events := make([]syscall.EpollEvent, 100)
	n, err := syscall.EpollWait(e.fd, events, -1)
	if err != nil {
		return err
	}

	for i := 0; i < n; i++ {
		if events[i].Events == EpollClose {
			handler(Event{
				Fd:   events[i].Fd,
				Type: EventClose,
			})
		} else {
			handler(Event{
				Fd:   events[i].Fd,
				Type: EventIn,
			})
		}
	}

	return nil
}
