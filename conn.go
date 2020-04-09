package ge

import (
	"log"
	"sync"
	"syscall"
	"time"
)

type Conn struct {
	rm           sync.Mutex  // read锁
	s            *server     // 服务器引用
	fd           int32       // 文件描述符
	lastReadTime int64       // 最后一次读取数据的时间
	extra        interface{} // 扩展字段
}

func newConn(fd int32, s *server) *Conn {
	return &Conn{
		s:            s,
		fd:           fd,
		lastReadTime: time.Now().Unix(),
	}
}

// 关闭连接
func (c *Conn) GetFd() int32 {
	return c.fd
}

// 关闭连接
func (c *Conn) GetRemoteIP() int32 {
	return c.fd
}

func (c *Conn) Read() error {
	c.rm.Lock()
	defer c.rm.Unlock()

	err := Decode(c)
	if err != nil {
		return err
	}

	return nil
}

func (c *Conn) Write(bytes []byte) (int, error) {
	return syscall.Write(int(c.fd), bytes)
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
	return nil
}
