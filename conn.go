package gn

import (
	"log"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Conn 客户端长连接
type Conn struct {
	rm           sync.Mutex  // read锁
	s            *Server     // 服务器引用
	fd           int         // 文件描述符
	addr         string      // 对端地址
	lastReadTime int64       // 最后一次读取数据的时间
	data         interface{} // 业务自定义数据，用作扩展
}

// newConn 创建tcp链接
func newConn(fd int, addr string, s *Server) *Conn {
	return &Conn{
		s:            s,
		fd:           fd,
		addr:         addr,
		lastReadTime: time.Now().Unix(),
	}
}

// GetFd 获取文件描述符
func (c *Conn) GetFd() int {
	return c.fd
}

// GetAddr 获取客户端地址
func (c *Conn) GetAddr() string {
	return c.addr
}

// Read 读取数据
func (c *Conn) Read() error {
	c.rm.Lock()
	defer c.rm.Unlock()

	c.lastReadTime = time.Now().Unix()
	err := Decode(c)
	if err != nil {
		return err
	}

	return nil
}

// Write 写入数据
func (c *Conn) Write(bytes []byte) (int, error) {
	return syscall.Write(int(c.fd), bytes)
}

// GetData 获取数据
func (c *Conn) GetData() interface{} {
	return c.data
}

// SetData 设置数据
func (c *Conn) SetData(data interface{}) {
	c.data = data
}

// Close 关闭连接
func (c *Conn) Close() error {
	// 从epoll监听的文件描述符中删除
	err := c.s.epoll.RemoveAndClose(int(c.fd))
	if err != nil {
		log.Println(err)
	}

	// 从conns中删除conn
	c.s.conns.Delete(c.fd)
	atomic.AddInt64(&c.s.connsNum, -1)
	return nil
}
