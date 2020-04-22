package gn

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var (
	ErrReadTimeout = errors.New("tcp read timeout")
)

// Handler Server 注册接口
type Handler interface {
	OnConnect(c *Conn)               // OnConnect 当TCP长连接建立成功是回调
	OnMessage(c *Conn, bytes []byte) // OnMessage 当客户端有数据写入是回调
	OnClose(c *Conn, err error)      // OnClose 当客户端主动断开链接或者超时时回调,err返回关闭的原因
}

// server TCP服务
type Server struct {
	epoll         *epoll        // 系统相关网络模型
	handler       Handler       // 注册的处理
	eventQueue    chan event    // 事件队列
	gNum          int           // 处理事件goroutine数量
	conns         sync.Map      // TCP长连接管理
	timeoutTicker time.Duration // 超时时间检查间隔
	timeout       int64         // 超时时间(单位秒)
	stop          chan int      // 服务器关闭信号
}

// NewServer 创建server服务器
func NewServer(port int, handler Handler, headerLen, readMaxLen, writeLen, gNum int) (*Server, error) {
	if headerLen <= 0 {
		return nil, errors.New("headerLen must be greater than 0")
	}
	if readMaxLen <= 0 {
		return nil, errors.New("readMaxLen must be greater than 0")
	}
	if writeLen <= 0 {
		return nil, errors.New("writeLen must be greater than 0")
	}
	if gNum <= 0 {
		return nil, errors.New("gNum must be greater than 0")
	}

	InitCodec(headerLen, readMaxLen, writeLen)
	lfd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		Log.Error(err)
		return nil, err
	}

	err = syscall.Bind(lfd, &syscall.SockaddrInet4{Port: port})
	if err != nil {
		Log.Error(err)
		return nil, err
	}

	err = syscall.Listen(lfd, 1024)
	if err != nil {
		Log.Error(err)
		return nil, err
	}

	e, err := EpollCreate()
	if err != nil {
		Log.Error(err)
		return nil, err
	}

	e.AddListener(lfd)
	if err != nil {
		Log.Error(err)
		return nil, err
	}

	Log.Info("ge server init,listener port:", port)
	return &Server{
		epoll:      e,
		handler:    handler,
		eventQueue: make(chan event, 1000),
		gNum:       gNum,
		timeout:    0,
		stop:       make(chan int),
	}, nil
}

// SetTimeout 设置超时检查时间以及超时时间,默认不进行超时时间检查
func (s *Server) SetTimeout(ticker, timeout time.Duration) {
	s.timeoutTicker = ticker
	s.timeout = int64(timeout.Seconds())
}

// GetConn 获取Conn
func (s *Server) GetConn(fd int) (*Conn, bool) {
	value, ok := s.conns.Load(fd)
	if !ok {
		return nil, false
	}
	return value.(*Conn), true
}

// Run 启动服务
func (s *Server) Run() {
	Log.Info("ge server run")
	s.startConsumer()
	s.checkTimeout()
	s.startProducer()
}

// Run 启动服务
func (s *Server) Stop() {
	close(s.stop)
	close(s.eventQueue)
}

// StartProducer 启动生产者
func (s *Server) startProducer() {
	for {
		select {
		case <-s.stop:
			Log.Error("stop producer")
			return
		default:
			err := s.epoll.EpollWait(s.eventQueue)
			if err != nil {
				Log.Error(err)
			}
		}
	}
}

// StartConsumer 启动消费者
func (s *Server) startConsumer() {
	for i := 0; i < s.gNum; i++ {
		go s.consume()
	}
	Log.Info("consumer run by " + strconv.Itoa(s.gNum) + " goroutine")
}

// Consume 消费者
func (s *Server) consume() {
	for event := range s.eventQueue {
		// 客户端请求建立连接
		if event.event == eventConn {
			nfd, sa, err := syscall.Accept(event.fd)
			if err != nil {
				Log.Error(err)
				continue
			}

			err = s.epoll.AddRead(nfd)
			if err != nil {
				Log.Error(err)
				continue
			}
			conn := newConn(nfd, getIPPort(sa), s)
			s.conns.Store(nfd, conn)
			s.handler.OnConnect(conn)
			continue
		}

		v, ok := s.conns.Load(event.fd)
		if !ok {
			Log.Error("not found in conns,", event.fd)
			continue
		}
		c := v.(*Conn)

		if event.event == eventClose {
			c.Close()
			s.handler.OnClose(c, ErrReadTimeout)
			return
		}

		err := c.Read()
		if err != nil {
			// 服务端关闭连接
			if err == syscall.EBADF {
				continue
			}
			c.Close()
			s.handler.OnClose(c, err)

			Log.Debug(err)
		}
	}
}

func getIPPort(sa syscall.Sockaddr) string {
	addr := sa.(*syscall.SockaddrInet4)
	return fmt.Sprintf("%d.%d.%d.%d:%d", addr.Addr[0], addr.Addr[1], addr.Addr[2], addr.Addr[3], addr.Port)
}

// checkTimeout 定时检查超时的TCP长连接
func (s *Server) checkTimeout() {
	if s.timeout == 0 || s.timeoutTicker == 0 {
		return
	}
	Log.Info("check timeout goroutine run")
	go func() {
		ticker := time.NewTicker(s.timeoutTicker)
		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				s.conns.Range(func(key, value interface{}) bool {
					c := value.(*Conn)

					if time.Now().Unix()-c.lastReadTime > s.timeout {
						s.eventQueue <- event{fd: int(c.fd), event: eventClose}
					}
					return true
				})
			}
		}
	}()
}
