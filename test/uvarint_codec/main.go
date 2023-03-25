package main

import (
	"bytes"
	"fmt"
	"github.com/alberliu/gn"
	"github.com/alberliu/gn/codec"
	"math"
	"net"
	"strconv"
	"time"
)

var (
	decoder = codec.NewUvarintDecoder()
	encoder = codec.NewUvarintEncoder(5)
)

var log = gn.GetLogger()

type Handler struct{}

func (*Handler) OnConnect(c *gn.Conn) {
	log.Info("server:connect:", c.GetFd(), c.GetAddr())
}
func (*Handler) OnMessage(c *gn.Conn, bytes []byte) {
	c.WriteWithEncoder(bytes)
	log.Info("server:read:", string(bytes))
}
func (*Handler) OnClose(c *gn.Conn, err error) {
	log.Info("server:close:", c.GetFd(), err)
}

func startServer() {
	server, err := gn.NewServer(":8080", &Handler{},
		gn.WithDecoder(decoder),
		gn.WithEncoder(encoder),
		gn.WithTimeout(1*time.Second),
		gn.WithReadBufferLen(100))
	if err != nil {
		log.Info("err")
		return
	}

	server.Run()
}

func startClient(i int) {
	var sends, receives [][]byte

	conn, err := net.Dial("tcp", "127.0.0.1:8080")
	if err != nil {
		log.Info(i, "error dialing", err.Error())
		return // 终止程序
	}

	buffer := codec.NewBuffer(make([]byte, 1024))
	var handler = func(bytes []byte) {
		// 需要制作一个拷贝
		c := make([]byte, len(bytes))
		copy(c, bytes)
		receives = append(receives, c)
	}

	go func() {
		for {
			_, err := buffer.ReadFromReader(conn)
			if err != nil {
				log.Error(i, " ", err)

				// 结束时对比
				if !equal(sends, receives) {
					display(sends)
					display(receives)
					panic(fmt.Sprintf("not equal: %d", i))
				} else {
					log.Error("equal", i)
				}
				return
			}

			err = decoder.Decode(buffer, handler)
			if err != nil {
				log.Error(i, " ", err)
				return
			}
		}
	}()

	for i := 0; i < 3; i++ {
		send := []byte("hello, gn " + powAndString(10, i))
		sends = append(sends, send)
		err := encoder.EncodeToWriter(conn, send)
		if err != nil {
			log.Error(err)
			return
		}
	}
}

func equal(a, b [][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

func display(a [][]byte) {
	log.Info("len:", len(a))
	for i := range a {
		log.Info(string(a[i]))
	}
}

func powAndString(x, y int) string {
	r := math.Pow(float64(x), float64(y))
	return strconv.Itoa(int(r))
}

func batchStartClient() {
	for i := 0; i < 20; i++ {
		fmt.Printf("\n\n\n\n")
		go startClient(i)
		time.Sleep(2 * time.Second)
	}
}

func main() {
	go startServer()
	time.Sleep(1 * time.Second)

	go batchStartClient()
	select {}
}
