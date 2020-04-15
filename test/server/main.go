package main

import (
	"github.com/alberliu/gn"
	"log"
	"time"
)

type Handler struct {
}

func (Handler) OnConnect(c *gn.Conn) {
	log.Println("connect:", c.GetFd(), c.GetAddr())
}
func (Handler) OnMessage(c *gn.Conn, bytes []byte) {
	gn.EncodeToFD(c.GetFd(), bytes)
	log.Println("read:", string(bytes))
}
func (Handler) OnClose(c *gn.Conn) {
	log.Println("close:", c.GetFd())
}

func main() {
	server, err := gn.NewServer(8080, &Handler{}, 2, 1024, 1024, 1000)
	if err != nil {
		log.Panicln("err")
		return
	}
	server.SetTimeout(1*time.Second, 5*time.Minute)
	server.Run()
}
