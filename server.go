package gn

import (
	"errors"
	"fmt"
	"io"
	"runtime"
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

type event struct {
	FD   int32 // 文件描述符
	Type int32 // 时间类型
}

// Server TCP服务
type Server struct {
	netpoll        netpoll      // 具体操作系统网络实现
	options        *options     // 服务参数
	readBufferPool *sync.Pool   // 读缓存区内存池
	handler        Handler      // 注册的处理
	decoder        Decoder      // 解码器
	ioEventQueues  []chan event // IO事件队列集合
	ioQueueNum     int32        // IO事件队列集合数量
	conns          sync.Map     // TCP长连接管理
	connsNum       int64        // 当前建立的长连接数量
	stop           chan int     // 服务器关闭信号
}

// NewServer 创建server服务器
func NewServer(address string, handler Handler, decoder Decoder, opts ...Option) (*Server, error) {
	options := getOptions(opts...)

	// 初始化读缓存区内存池
	readBufferPool := &sync.Pool{
		New: func() interface{} {
			b := make([]byte, options.readBufferLen)
			return b
		},
	}

	// 初始化epoll网络
	netpoll, err := newNetpoll(address)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	// 初始化io事件队列
	ioEventQueues := make([]chan event, options.ioGNum)
	for i := range ioEventQueues {
		ioEventQueues[i] = make(chan event, options.ioEventQueueLen)
	}

	return &Server{
		netpoll:        netpoll,
		options:        options,
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
	log.Info("gn server run")
	s.startAccept()
	s.startIOConsumer()
	s.checkTimeout()
	s.startIOProducer()
}

// GetConnsNum 获取当前长连接的数量
func (s *Server) GetConnsNum() int64 {
	return atomic.LoadInt64(&s.connsNum)
}

// Stop 启动服务
func (s *Server) Stop() {
	close(s.stop)
	for _, queue := range s.ioEventQueues {
		close(queue)
	}
}

// handleEvent 处理事件
func (s *Server) handleEvent(event event) {
	index := event.FD % s.ioQueueNum
	s.ioEventQueues[index] <- event
}

// StartProducer 启动生产者
func (s *Server) startIOProducer() {
	log.Info("start io producer")
	for {
		select {
		case <-s.stop:
			log.Error("stop producer")
			return
		default:
			events, err := s.netpoll.getEvents()
			if err != nil {
				log.Error(err)
			}
			for i := range events {
				s.handleEvent(events[i])
			}
		}
	}
}

// startAccept 开始接收连接请求
func (s *Server) startAccept() {
	for i := 0; i < s.options.acceptGNum; i++ {
		go s.accept()
	}
	log.Info(fmt.Sprintf("start accept by %d goroutine", s.options.acceptGNum))
}

// accept 接收连接请求
func (s *Server) accept() {
	for {
		select {
		case <-s.stop:
			return
		default:
			nfd, addr, err := s.netpoll.accept()
			if err != nil {
				log.Error(err)
				continue
			}

			fd := int32(nfd)
			conn := newConn(fd, addr, s)
			s.conns.Store(fd, conn)
			atomic.AddInt64(&s.connsNum, 1)
			s.handler.OnConnect(conn)
		}
	}
}

// StartConsumer 启动消费者
func (s *Server) startIOConsumer() {
	for _, queue := range s.ioEventQueues {
		go s.consumeIOEvent(queue)
	}
	log.Info(fmt.Sprintf("start io event consumer by %d goroutine", len(s.ioEventQueues)))
}

// ConsumeIO 消费IO事件
func (s *Server) consumeIOEvent(queue chan event) {
	for event := range queue {
		v, ok := s.conns.Load(event.FD)
		if !ok {
			log.Error("not found in conns,", event.FD)
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

// checkTimeout 定时检查超时的TCP长连接
func (s *Server) checkTimeout() {
	if s.options.timeout == 0 || s.options.timeoutTicker == 0 {
		return
	}
	log.Info(fmt.Sprintf("check timeout goroutine run,check_time:%v,timeout:%v", s.options.timeoutTicker, s.options.timeout))
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
						s.handleEvent(event{FD: c.fd, Type: EventTimeout})
					}
					return true
				})
			}
		}
	}()
}
