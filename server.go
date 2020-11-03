package gn

import (
	"errors"
	"fmt"
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
	headerLen            int           // 数据包头部大小,默认值是2
	readMaxLen           int           // 所读取的客户端包的最大长度，客户端发送的包不能超过这个长度，默认值是1024字节
	writeLen             int           // 服务器发送给客户端包的建议长度，当发送的包小于这个值时，会利用到内存池优化，默认值是1024字节
	acceptGNum           int           // 处理接受请求的goroutine数量
	ioGNum               int           // 处理io的goroutine数量
	timeoutTicker        time.Duration // 超时时间检查间隔
	timeout              time.Duration // 超时时间
	connectEventQueueLen int           // 连接事件队列长度
	ioEventQueueLen      int           // io事件队列长度
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

// WithConnectEventQueueLen 设置连接事件队列长度，默认值是1024
func WithConnectEventQueueLen(num int) Option {
	return newFuncServerOption(func(o *options) {
		if num <= 0 {
			panic("connectEventQueueLen must greater than 0")
		}
		o.connectEventQueueLen = num
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
		headerLen:            2,
		readMaxLen:           1024,
		writeLen:             1024,
		acceptGNum:           cpuNum,
		ioGNum:               cpuNum,
		timeoutTicker:        0,
		timeout:              0,
		connectEventQueueLen: 1024,
		ioEventQueueLen:      1024,
	}

	for _, o := range opts {
		o.apply(options)
	}
	return options
}

const (
	EventConn  = 1 // 请求建立连接
	EventIn    = 2 // 数据流入
	EventClose = 3 // 断开连接
)

type Event struct {
	Fd   int32 // 文件描述符
	Type int32 // 时间类型
}

// server TCP服务
type Server struct {
	options           *options     // 服务参数
	epoll             *epoll       // 系统相关网络模型
	handler           Handler      // 注册的处理
	decoder           Decoder      // 解码器
	connectEventQueue chan Event   // connect事件队列
	ioEventQueues     []chan Event // IO事件队列集合
	ioQueueNum        int32        // IO事件队列集合数量
	conns             sync.Map     // TCP长连接管理
	connsNum          int64        // 当前建立的长连接数量
	stop              chan int     // 服务器关闭信号
}

// NewServer 创建server服务器
func NewServer(port int, handler Handler, decoder Decoder, opts ...Option) (*Server, error) {
	options := getOptions(opts...)

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
		options:           options,
		epoll:             epoll,
		handler:           handler,
		decoder:           decoder,
		connectEventQueue: make(chan Event, options.connectEventQueueLen),
		ioEventQueues:     ioEventQueues,
		ioQueueNum:        int32(options.ioGNum),
		conns:             sync.Map{},
		connsNum:          0,
		stop:              make(chan int),
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
	s.startConsumer()
	s.checkTimeout()
	s.startProducer()
}

// GetConnsNum 获取当前长连接的数量
func (s *Server) GetConnsNum() int64 {
	return atomic.LoadInt64(&s.connsNum)
}

// Run 启动服务
func (s *Server) Stop() {
	close(s.stop)
	close(s.connectEventQueue)
	for _, queue := range s.ioEventQueues {
		close(queue)
	}
}

// HandleEvent
func (s *Server) HandleEvent(event Event) {
	if event.Type == EventConn {
		s.connectEventQueue <- event
		return
	}

	index := event.Fd % s.ioQueueNum
	s.ioEventQueues[index] <- event
}

// StartProducer 启动生产者
func (s *Server) startProducer() {
	for {
		select {
		case <-s.stop:
			log.Error("stop producer")
			return
		default:
			err := s.epoll.EpollWait(s.HandleEvent)
			if err != nil {
				log.Error(err)
			}
		}
	}
}

// StartConsumer 启动消费者
func (s *Server) startConsumer() {
	for i := 0; i < s.options.acceptGNum; i++ {
		go s.consumeConnectEvent()
	}
	log.Info("consume connect event run by " + strconv.Itoa(s.options.acceptGNum) + " goroutine")

	for _, queue := range s.ioEventQueues {
		go s.consumeIOEvent(queue)
	}
	log.Info("consume io event run by " + strconv.Itoa(s.options.ioGNum) + " goroutine")
}

// consumeAcceptEvent 消费连接事件
func (s *Server) consumeConnectEvent() {
	for event := range s.connectEventQueue {
		nfd, sa, err := syscall.Accept(int(event.Fd))
		if err != nil {
			log.Error(err)
			continue
		}

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
						s.HandleEvent(Event{Fd: c.fd, Type: EventClose})
					}
					return true
				})
			}
		}
	}()
}
