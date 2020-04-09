package ge

import (
	"io"
	"sync"
	"syscall"
	"time"
)

// Handler Server 注册接口
type Handler interface {
	OnConnect(c *Conn)
	OnMessage(c *Conn, bytes []byte)
	OnClose(c *Conn)
}

// server TCP服务
type server struct {
	epoll         *epoll                  // 系统相关网络模型
	handler       Handler                 // 注册的处理
	eventQueue    chan syscall.EpollEvent // 事件队列
	conns         sync.Map                // TCP长连接管理
	timeoutTicker time.Duration           // 超时时间检查间隔
	timeout       int64                   // 超时时间(单位秒)
	stop          chan int                // 服务器关闭信号
}

// NewServer 创建server服务器
func NewServer(port int, handler Handler, headerLen, readMaxLen, writeLen int) (*server, error) {
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
	return &server{
		epoll:      e,
		handler:    handler,
		eventQueue: make(chan syscall.EpollEvent, 1000),
		timeout:    0,
		stop:       make(chan int),
	}, nil
}

// SetTimeout 设置超时检查时间以及超时时间,默认不进行超时时间检查
func (s *server) SetTimeout(ticker, timeout time.Duration) {
	s.timeoutTicker = ticker
	s.timeout = int64(timeout.Seconds())
}

// Run 启动服务
func (s *server) Run() {
	Log.Info("ge server run")
	s.startConsumer()
	s.checkTimeout()
	s.startProducer()
}

// Run 启动服务
func (s *server) Stop() {
	close(s.stop)
	close(s.eventQueue)
}

// StartProducer 启动生产者
func (s *server) startProducer() {
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
func (s *server) startConsumer() {
	go s.consume()
}

// Consume 消费者
func (s *server) consume() {
	for event := range s.eventQueue {
		// 客户端请求建立连接
		if event.Fd == int32(s.epoll.lfd) {
			nfd, _, err := syscall.Accept(int(event.Fd))
			if err != nil {
				Log.Error(err)
				continue
			}

			err = s.epoll.AddRead(nfd)
			if err != nil {
				Log.Error(err)
				continue
			}
			conn := newConn(int32(nfd), s)
			s.conns.Store(int32(nfd), conn)
			s.handler.OnConnect(conn)
			continue
		}

		v, ok := s.conns.Load(event.Fd)
		if !ok {
			Log.Error("not found in conns,", event.Fd)
			continue
		}
		c := v.(*Conn)

		err := c.Read()
		if err != nil {
			// 客户端关闭连接
			if err == io.EOF {
				c.Close()
				s.handler.OnClose(c)
				continue
			}
			// 服务端关闭连接
			if err == syscall.EBADF {
				continue
			}
			// 其他错误
			Log.Error(err)
		}
	}
}

func (s *server) checkTimeout() {
	if s.timeout == 0 || s.timeoutTicker == 0 {
		return
	}
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
						c.Close()
						s.handler.OnClose(c)
					}
					return true
				})
			}
		}
	}()
}
