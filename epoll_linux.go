package gn

import (
	"fmt"
	"golang.org/x/sys/unix"
	"syscall"
)

// 对端关闭连接 8193
const (
	EpollRead  = syscall.EPOLLIN | syscall.EPOLLPRI | syscall.EPOLLERR | syscall.EPOLLHUP | unix.EPOLLET | syscall.EPOLLRDHUP
	EpollClose = uint32(syscall.EPOLLIN | syscall.EPOLLRDHUP)
)

var (
	listenFD int
	epollFD  int
)

func listen(address string) error {
	var err error
	listenFD, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		log.Error(err)
		return err
	}
	err = syscall.SetsockoptInt(listenFD, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	if err != nil {
		log.Error(err)
		return err
	}

	addr, port, err := getIPPort(address)
	if err != nil {
		return err
	}
	err = syscall.Bind(listenFD, &syscall.SockaddrInet4{
		Port: port,
		Addr: addr,
	})
	if err != nil {
		log.Error(err)
		return err
	}
	err = syscall.Listen(listenFD, 1024)
	if err != nil {
		log.Error(err)
		return err
	}

	epollFD, err = syscall.EpollCreate1(0)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func accept() (nfd int, addr string, err error) {
	nfd, sa, err := syscall.Accept(int(listenFD))
	if err != nil {
		return
	}

	// 设置为非阻塞状态
	err = syscall.SetNonblock(nfd, true)
	if err != nil {
		return
	}

	err = addRead(nfd)
	if err != nil {
		return
	}
	addr = getAddr(sa)
	return
}

func getAddr(sa syscall.Sockaddr) string {
	addr := sa.(*syscall.SockaddrInet4)
	return fmt.Sprintf("%d.%d.%d.%d:%d", addr.Addr[0], addr.Addr[1], addr.Addr[2], addr.Addr[3], addr.Port)
}

func addRead(fd int) error {
	err := syscall.EpollCtl(epollFD, syscall.EPOLL_CTL_ADD, fd, &syscall.EpollEvent{
		Events: EpollRead,
		Fd:     int32(fd),
	})
	if err != nil {
		return err
	}
	return nil
}

func closeFD(fd int) error {
	// 移除文件描述符的监听
	err := syscall.EpollCtl(epollFD, syscall.EPOLL_CTL_DEL, fd, nil)
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

func closeFDRead(fd int) error {
	_, _, e := syscall.Syscall(syscall.SHUT_RD, uintptr(fd), 0, 0)
	if e != 0 {
		return e
	}
	return nil
}

func getEvents() ([]event, error) {
	epollEvents := make([]syscall.EpollEvent, 100)
	n, err := syscall.EpollWait(epollFD, epollEvents, -1)
	if err != nil {
		return nil, err
	}

	events := make([]event, 0, len(epollEvents))
	for i := 0; i < n; i++ {
		event := event{
			FD: epollEvents[i].Fd,
		}
		if epollEvents[i].Events == EpollClose {
			event.Type = EventClose
		} else {
			event.Type = EventIn
		}
		events = append(events, event)
	}

	return events, nil
}
