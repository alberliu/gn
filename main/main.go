package main

import (
	"gn"
	"log"
	"time"
)

type Handler struct {
}

func (Handler) OnConnect(c *gn.Conn) {
	log.Println("connect:", c.GetFd())
}
func (Handler) OnMessage(c *gn.Conn, bytes []byte) {
	log.Println("read:", string(bytes))
}
func (Handler) OnClose(c *gn.Conn) {
	log.Println("close:", c.GetFd())
}

func main() {
	server, err := gn.NewServer(8080, &Handler{}, 2, 1024, 1024)
	if err != nil {
		log.Panicln("err")
		return
	}
	server.SetTimeout(1*time.Minute, 5*time.Minute)
	server.Run()
}
