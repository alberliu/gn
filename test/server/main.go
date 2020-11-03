package main

import (
	"github.com/alberliu/gn"
	"log"
	"time"
)

var server *gn.Server

type Handler struct {
}

func (Handler) OnConnect(c *gn.Conn) {
	//log.Println("connect:", c.GetFd(), c.GetAddr())
}
func (Handler) OnMessage(c *gn.Conn, bytes []byte) {
	encoder.EncodeToFD(c.GetFd(), bytes)
	//log.Println("read:", string(bytes))
}
func (Handler) OnClose(c *gn.Conn, err error) {
	//log.Println("close:", c.GetFd())
}

var encoder = gn.NewHeaderLenEncoder(2, 1024)

func main() {
	var err error
	server, err = gn.NewServer(8080, &Handler{}, gn.NewHeaderLenDecoder(2, 1024), gn.WithTimeout(1*time.Second, 5*time.Second))
	if err != nil {
		log.Panicln("err")
		return
	}

	server.Run()
}
