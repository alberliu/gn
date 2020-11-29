# gn
### 简述
gn是一个基于linux下epoll的网络框架，目前只能运行在Linux下环境，gn可以配置处理网络事件的goroutine数量，相比golang原生库，在海量链接下，可以减少goroutine的开销，从而减少系统资源占用。
### 支持功能
1.tcp拆包粘包  
通过调用系统recv_from函数，使用系统读缓存区实现拆包，减少用户态内存分配，使用sync.pool申请读写使用的字节数组，减少内存申请开销以及GC压力。  
2.客户端超时踢出  
可以设置超时时间，gn会定时检测超出超时的TCP连接（在指定时间内没有发送数据的连接）,进行释放。
### 使用方式
```go
package main

import (
	"github.com/alberliu/gn"

	"time"
)

var log = gn.GetLogger()

var server *gn.Server

var encoder = gn.NewHeaderLenEncoder(2, 1024)

type Handler struct {
}

func (Handler) OnConnect(c *gn.Conn) {
	log.Info("connect:", c.GetFd(), c.GetAddr())
}
func (Handler) OnMessage(c *gn.Conn, bytes []byte) {
	encoder.EncodeToFD(c.GetFd(), bytes)
	log.Info("read:", string(bytes))
}
func (Handler) OnClose(c *gn.Conn, err error) {
	log.Info("close:", c.GetFd(), err)
}

func main() {
	var err error
	server, err = gn.NewServer(8080, &Handler{}, gn.NewHeaderLenDecoder(2),
		gn.WithTimeout(1*time.Second, 5*time.Second), gn.WithReadBufferLen(10))
	if err != nil {
		log.Info("err")
		return
	}

	server.Run()
}
```
