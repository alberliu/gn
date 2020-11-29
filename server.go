package gn

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
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

// options Server初始化参数
type options struct {
	readBufferLen   int           // 所读取的客户端包的最大长度，客户端发送的包不能超过这个长度，默认值是1024字节
	acceptGNum      int           // 处理接受请求的goroutine数量
	ioGNum          int           // 处理io的goroutine数量
	ioEventQueueLen int           // io事件队列长度
	timeoutTicker   time.Duration // 超时时间检查间隔
	timeout         time.Duration // 超时时间
}

type Option interface {
	apply(*options)
}

type funcServerOption struct {
	f func(*options)
}

func (fdo *funcServerOption) apply(do *options) {
	fdo.f(do)
}

func newFuncServerOption(f func(*options)) *funcServerOption {
	return &funcServerOption{
		f: f,
	}
}

// WithReadBufferLen 设置缓存区大小
func WithReadBufferLen(len int) Option {
	return newFuncServerOption(func(o *options) {
		if len <= 0 {
			panic("acceptGNum must greater than 0")
		}
		o.readBufferLen = len
	})
}

// WithAcceptGNum 设置建立连接的goroutine数量
func WithAcceptGNum(num int) Option {
	return newFuncServerOption(func(o *options) {
		if num <= 0 {
			panic("acceptGNum must greater than 0")
		}
		o.acceptGNum = num
	})
}

// WithIOGNum 设置处理IO的goroutine数量
func WithIOGNum(num int) Option {
	return newFuncServerOption(func(o *options) {
		if num <= 0 {
			panic("IOGNum must greater than 0")
		}
		o.ioGNum = num
	})
}

// WithIOEventQueueLen 设置IO事件队列长度，默认值是1024
func WithIOEventQueueLen(num int) Option {
	return newFuncServerOption(func(o *options) {
		if num <= 0 {
			panic("ioEventQueueLen must greater than 0")
		}
		o.ioEventQueueLen = num
	})
}

// WithTimeout 设置TCP超时检查的间隔时间以及超时时间
func WithTimeout(timeoutTicker, timeout time.Duration) Option {
	return newFuncServerOption(func(o *options) {
		if timeoutTicker <= 0 {
			panic("timeoutTicker must greater than 0")
		}
		if timeout <= 0 {
			panic("timeoutTicker must greater than 0")
		}

		o.timeoutTicker = timeoutTicker
		o.timeout = timeout
	})
}

func getOptions(opts ...Option) *options {
	cpuNum := runtime.NumCPU()
	options := &options{
		readBufferLen:   1024,
		acceptGNum:      cpuNum,
		ioGNum:          cpuNum,
		ioEventQueueLen: 1024,
	}

	for _, o := range opts {
		o.apply(options)
	}
	return options
}

const (
	EventIn      = 1 // 数据流入
	EventClose   = 2 // 断开连接
	EventTimeout = 3 // 检测到超时
)

type Event struct {
	Fd   int32 // 文件描述符
	Type int32 // 时间类型
}

// server TCP服务
type Server struct {
	options        *options     // 服务参数
	epoll          *epoll       // 系统相关网络模型
	readBufferPool *sync.Pool   // 读缓存区内存池
	handler        Handler      // 注册的处理
	decoder        Decoder      // 解码器
	ioEventQueues  []chan Event // IO事件队列集合
	ioQueueNum     int32        // IO事件队列集合数量
	conns          sync.Map     // TCP长连接管理
	connsNum       int64        // 当前建立的长连接数量
	stop           chan int     // 服务器关闭信号
}

// NewServer 创建server服务器
func NewServer(port int, handler Handler, decoder Decoder, opts ...Option) (*Server, error) {
	options := getOptions(opts...)

	readBufferPool := &sync.Pool{
		New: func() interface{} {
			b := make([]byte, options.readBufferLen)
			return b
		},
	}

	epoll, err := EpollCreate(port)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	ioEventQueues := make([]chan Event, options.ioGNum)
	for i := range ioEventQueues {
		ioEventQueues[i] = make(chan Event, options.ioEventQueueLen)
	}

	return &Server{
		options:        options,
		epoll:          epoll,
		readBufferPool: readBufferPool,
		handler:        handler,
		decoder:        decoder,
		ioEventQueues:  ioEventQueues,
		ioQueueNum:     int32(options.ioGNum),
		conns:          sync.Map{},
		connsNum:       0,
		stop:           make(chan int),
	}, nil
}

// GetConn 获取Conn
func (s *Server) GetConn(fd int32) (*Conn, bool) {
	value, ok := s.conns.Load(fd)
	if !ok {
		return nil, false
	}
	return value.(*Conn), true
}

// Run 启动服务
func (s *Server) Run() {
	log.Info("ge server run")
	s.startAccept()
	s.startConsumer()
	s.checkTimeout()
	s.startIOProducer()
}

// GetConnsNum 获取当前长连接的数量
func (s *Server) GetConnsNum() int64 {
	return atomic.LoadInt64(&s.connsNum)
}

// Run 启动服务
func (s *Server) Stop() {
	close(s.stop)
	for _, queue := range s.ioEventQueues {
		close(queue)
	}
}

// handleEvent 处理事件
func (s *Server) handleEvent(event Event) {
	index := event.Fd % s.ioQueueNum
	s.ioEventQueues[index] <- event
}

// StartProducer 启动生产者
func (s *Server) startIOProducer() {
	for {
		select {
		case <-s.stop:
			log.Error("stop producer")
			return
		default:
			err := s.epoll.EpollWait(s.handleEvent)
			if err != nil {
				log.Error(err)
			}
		}
	}
}

// startAccept 开始接收连接请求
func (s *Server) startAccept() {
	for i := 0; i < s.options.acceptGNum; i++ {
		go s.accept()
	}
	log.Info("start accept by " + strconv.Itoa(s.options.acceptGNum) + " goroutine")
}

// accept 接收连接请求
func (s *Server) accept() {
	for {
		select {
		case <-s.stop:
			return
		default:
			nfd, sa, err := syscall.Accept(int(s.epoll.lfd))
			if err != nil {
				log.Error(err)
				continue
			}

			// 设置为非阻塞状态
			syscall.SetNonblock(nfd, true)

			fd := int32(nfd)
			conn := newConn(fd, getIPPort(sa), s)
			s.conns.Store(fd, conn)
			atomic.AddInt64(&s.connsNum, 1)
			s.handler.OnConnect(conn)

			err = s.epoll.AddRead(nfd)
			if err != nil {
				log.Error(err)
				continue
			}
		}
	}
}

// StartConsumer 启动消费者
func (s *Server) startConsumer() {
	for _, queue := range s.ioEventQueues {
		go s.consumeIOEvent(queue)
	}
	log.Info("consume io event run by " + strconv.Itoa(s.options.ioGNum) + " goroutine")
}

// ConsumeIO 消费IO事件
func (s *Server) consumeIOEvent(queue chan Event) {
	for event := range queue {
		v, ok := s.conns.Load(event.Fd)
		if !ok {
			log.Error("not found in conns,", event.Fd)
			continue
		}
		c := v.(*Conn)

		if event.Type == EventClose {
			c.Close()
			s.handler.OnClose(c, io.EOF)
			continue
		}
		if event.Type == EventTimeout {
			c.Close()
			s.handler.OnClose(c, ErrReadTimeout)
			continue
		}

		err := c.Read()
		if err != nil {
			// 服务端关闭连接
			if err == syscall.EBADF {
				continue
			}
			c.Close()
			s.handler.OnClose(c, err)

			log.Debug(err)
		}
	}
}

func getIPPort(sa syscall.Sockaddr) string {
	addr := sa.(*syscall.SockaddrInet4)
	return fmt.Sprintf("%d.%d.%d.%d:%d", addr.Addr[0], addr.Addr[1], addr.Addr[2], addr.Addr[3], addr.Port)
}

// checkTimeout 定时检查超时的TCP长连接
func (s *Server) checkTimeout() {
	if s.options.timeout == 0 || s.options.timeoutTicker == 0 {
		return
	}
	log.Info("check timeout goroutine run")
	go func() {
		ticker := time.NewTicker(s.options.timeoutTicker)
		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				s.conns.Range(func(key, value interface{}) bool {
					c := value.(*Conn)

					if time.Now().Sub(c.lastReadTime) > s.options.timeout {
						s.handleEvent(Event{Fd: c.fd, Type: EventTimeout})
					}
					return true
				})
			}
		}
	}()
}
