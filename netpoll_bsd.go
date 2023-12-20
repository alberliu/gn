//go:build darwin || netbsd || freebsd || openbsd || dragonfly
// +build darwin netbsd freebsd openbsd dragonfly

// 实现 mac环境不支持epoll 用kquene替换

package gn

import (
	"fmt"
	"syscall"
)

const (
	EpollRead  = syscall.EV_ADD
	EpollClose = syscall.EV_ADD | syscall.EV_EOF
)

type epoll struct {
	listenFD int
	epollFD  int
	ts       syscall.Timespec
	changes  []syscall.Kevent_t
}

func newNetpoll(address string) (netpoll, error) {
	listenFD, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	err = syscall.SetsockoptInt(listenFD, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	addr, port, err := getIPPort(address)
	if err != nil {
		return nil, err
	}
	err = syscall.Bind(listenFD, &syscall.SockaddrInet4{
		Port: port,
		Addr: addr,
	})
	if err != nil {
		log.Error(err)
		return nil, err
	}
	err = syscall.Listen(listenFD, 1024)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	// !!
	// mac epollFD
	epollFD, err := syscall.Kqueue()
	if err != nil {
		panic(err)
	}
	_, err = syscall.Kevent(epollFD, []syscall.Kevent_t{{
		Ident:  0,
		Filter: syscall.EVFILT_USER,
		Flags:  syscall.EV_ADD | syscall.EV_CLEAR,
	}}, nil, nil)
	if err != nil {
		panic(err)
	}
	//!!

	return &epoll{
		listenFD: listenFD,
		epollFD:  epollFD,
		ts:       syscall.NsecToTimespec(1e9),
	}, nil
}

func (n *epoll) accept() (nfd int, addr string, err error) {
	nfd, sa, err := syscall.Accept(n.listenFD)
	if err != nil {
		return
	}

	//设置为非阻塞状态
	err = syscall.SetNonblock(nfd, true)
	if err != nil {
		return
	}

	n.changes = append(n.changes, syscall.Kevent_t{
		Ident: uint64(nfd), Flags: EpollRead, Filter: syscall.EVFILT_READ,
	})

	s := sa.(*syscall.SockaddrInet4)
	addr = fmt.Sprintf("%d.%d.%d.%d:%d", s.Addr[0], s.Addr[1], s.Addr[2], s.Addr[3], s.Port)
	return
}

func (n *epoll) closeFD(fd int) error {
	// 移除文件描述符的监听
	if len(n.changes) <= 1 {
		n.changes = nil
	} else {
		changes := make([]syscall.Kevent_t, 0, len(n.changes)-1)
		ident := uint64(fd)
		for _, ke := range n.changes {
			if ke.Ident != ident {
				changes = append(changes, ke)
			}
		}
		n.changes = changes
	}

	// 关闭文件描述符
	err := syscall.Close(fd)
	if err != nil {
		return err
	}

	return nil
}

func (n *epoll) getEvents() ([]event, error) {

	epollEvents := make([]syscall.Kevent_t, 100)
	changes := n.changes

retry:
	num, err := syscall.Kevent(n.epollFD, changes, epollEvents, &n.ts)
	if err != nil {
		if err == syscall.EINTR {
			goto retry
		}
		return nil, err
	}

	events := make([]event, 0, len(epollEvents))
	for i := 0; i < num; i++ {
		event := event{
			FD: int32(epollEvents[i].Ident),
		}
		if epollEvents[i].Flags == EpollClose {
			event.Type = EventClose
		} else {
			event.Type = EventIn
		}
		events = append(events, event)
	}

	return events, nil
}

func (n *epoll) closeFDRead(fd int) error {
	_, _, e := syscall.Syscall(syscall.SHUT_RD, uintptr(fd), 0, 0)
	if e != 0 {
		return e
	}
	return nil
}

var _ netpoll = &epoll{}
