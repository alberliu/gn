package gn

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"syscall"
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
	ioEventQueues  []chan event // IO事件队列集合
	ioQueueNum     int32        // IO事件队列集合数量
	conns          sync.Map     // TCP长连接管理
	connsNum       int64        // 当前建立的长连接数量
	stop           chan int     // 服务器关闭信号
}

// NewServer 创建server服务器
func NewServer(address string, handler Handler, opts ...Option) (*Server, error) {
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

		err := c.read()
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

func (s *Server) handleTimeoutEvent(fd int32) {
	s.handleEvent(event{FD: fd, Type: EventTimeout})
}
