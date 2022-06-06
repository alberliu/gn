package gn

import (
	"github.com/alberliu/gn/codec"
	"runtime"
	"time"
)

// options Server初始化参数
type options struct {
	decoder         codec.Decoder // 解码器
	encoder         codec.Encoder // 编码器
	readBufferLen   int           // 所读取的客户端包的最大长度，客户端发送的包不能超过这个长度，默认值是1024字节
	acceptGNum      int           // 处理接受请求的goroutine数量
	ioGNum          int           // 处理io的goroutine数量
	ioEventQueueLen int           // io事件队列长度
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

// WithDecoder 设置解码器
func WithDecoder(decoder codec.Decoder) Option {
	return newFuncServerOption(func(o *options) {
		o.decoder = decoder
	})
}

// WithEncoder 设置解码器
func WithEncoder(encoder codec.Encoder) Option {
	return newFuncServerOption(func(o *options) {
		o.encoder = encoder
	})
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
func WithTimeout(timeout time.Duration) Option {
	return newFuncServerOption(func(o *options) {
		if timeout <= 0 {
			panic("timeoutTicker must greater than 0")
		}

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
