package gn

import (
	"sync/atomic"
	"syscall"
	"time"
)

// Conn 客户端长连接
type Conn struct {
	server *Server     // 服务器引用
	fd     int32       // 文件描述符
	addr   string      // 对端地址
	buffer *Buffer     // 读缓存区
	timer  *time.Timer // 连接超时定时器
	data   interface{} // 业务自定义数据，用作扩展
}

// newConn 创建tcp链接
func newConn(fd int32, addr string, server *Server) *Conn {
	var timer *time.Timer
	if server.options.timeout != 0 {
		timer = time.AfterFunc(server.options.timeout, func() {
			server.handleTimeoutEvent(fd)
		})
	}

	return &Conn{
		server: server,
		fd:     fd,
		addr:   addr,
		buffer: NewBuffer(server.readBufferPool.Get().([]byte)),
		timer:  timer,
	}
}

// GetFd 获取文件描述符
func (c *Conn) GetFd() int32 {
	return c.fd
}

// GetAddr 获取客户端地址
func (c *Conn) GetAddr() string {
	return c.addr
}

// GetBuffer 获取客户端地址
func (c *Conn) GetBuffer() *Buffer {
	return c.buffer
}

// Read 读取数据
func (c *Conn) read() error {
	if c.server.options.timeout != 0 {
		c.timer.Reset(c.server.options.timeout)
	}

	fd := int(c.GetFd())
	for {
		err := c.buffer.ReadFromFD(fd)
		if err != nil {
			// 缓存区暂无数据可读
			if err == syscall.EAGAIN {
				return nil
			}
			return err
		}

		if c.server.options.decoder == nil {
			c.server.handler.OnMessage(c, c.buffer.ReadAll())
		} else {
			var handle = func(bytes []byte) {
				c.server.handler.OnMessage(c, bytes)
			}
			err = c.server.options.decoder.Decode(c.buffer, handle)
			if err != nil {
				return err
			}
		}
	}
}

// WriteWithEncoder 使用编码器写入
func (c *Conn) WriteWithEncoder(bytes []byte) error {
	return c.server.options.encoder.EncodeToWriter(c, bytes)
}

// Write 写入数据
func (c *Conn) Write(bytes []byte) (int, error) {
	return syscall.Write(int(c.fd), bytes)
}

// Close 关闭连接
func (c *Conn) Close() {
	// 从epoll监听的文件描述符中删除
	err := c.server.netpoll.closeFD(int(c.fd))
	if err != nil {
		log.Error(err)
	}
	// stop timer
	c.timer.Stop()

	// 从conns中删除conn
	c.server.conns.Delete(c.fd)
	// 归还缓存区
	c.server.readBufferPool.Put(c.buffer.GetBuf())
	// 连接数减一
	atomic.AddInt64(&c.server.connsNum, -1)
}

// CloseRead 关闭连接
func (c *Conn) CloseRead() error {
	err := c.server.netpoll.closeFDRead(int(c.fd))
	if err != nil {
		log.Error(err)
	}
	return nil
}

// GetData 获取数据
func (c *Conn) GetData() interface{} {
	return c.data
}

// SetData 设置数据
func (c *Conn) SetData(data interface{}) {
	c.data = data
}
