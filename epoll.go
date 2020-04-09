// +build linux

package ge

import (
	"golang.org/x/sys/unix"
	"syscall"
)

const (
	EpollListener = syscall.EPOLLIN | syscall.EPOLLPRI | syscall.EPOLLERR | syscall.EPOLLHUP | unix.EPOLLET
	EpollRead     = syscall.EPOLLIN | syscall.EPOLLPRI | syscall.EPOLLERR | syscall.EPOLLHUP | unix.EPOLLET
)

type epoll struct {
	fd  int
	lfd int
}

func EpollCreate() (*epoll, error) {
	fd, err := syscall.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &epoll{
		fd: fd,
	}, nil
}

func (e *epoll) AddListener(fd int) error {
	err := syscall.EpollCtl(e.fd, syscall.EPOLL_CTL_ADD, fd, &syscall.EpollEvent{
		Events: EpollListener,
		Fd:     int32(fd),
	})
	if err != nil {
		return err
	}
	e.lfd = fd
	return nil
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

	err = syscall.Close(fd)
	if err != nil {
		return err
	}

	return nil
}

func (e *epoll) EpollWait(eventQueue chan syscall.EpollEvent) error {
	events := make([]syscall.EpollEvent, 100)
	n, err := syscall.EpollWait(e.fd, events, -1)
	if err != nil {
		return err
	}

	for i := 0; i < n; i++ {
		eventQueue <- events[i]
	}
	return nil
}
