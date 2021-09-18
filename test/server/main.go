package main

import (
	"github.com/alberliu/gn"

	"time"
)

var log = gn.GetLogger()

var server *gn.Server

var encoder = gn.NewHeaderLenEncoder(2, 1024)

type Handler struct{}

func (*Handler) OnConnect(c *gn.Conn) {
	log.Info("connect:", c.GetFd(), c.GetAddr())
}
func (*Handler) OnMessage(c *gn.Conn, bytes []byte) {
	encoder.EncodeToFD(c.GetFd(), bytes)
	log.Info("read:", string(bytes))
}
func (*Handler) OnClose(c *gn.Conn, err error) {
	log.Info("close:", c.GetFd(), err)
}

func main() {
	var err error
	server, err = gn.NewServer(":8080", &Handler{},
		gn.NewHeaderLenDecoder(2),
		gn.WithTimeout(1*time.Second, 5*time.Second),
		gn.WithReadBufferLen(10))
	if err != nil {
		log.Info("err")
		return
	}

	server.Run()
}
