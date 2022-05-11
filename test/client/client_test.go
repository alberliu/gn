package main

import (
	"github.com/alberliu/gn/util"
	"log"
	"net"
	"strconv"
	"testing"
)

var codecFactory = util.NewHeaderLenCodecFactory(2, 1024)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func TestClient(t *testing.T) {
	for i := 0; i < 1; i++ {
		go startWithCodec(i)
	}
	select {}
}

func start(i int) {
	log.Println(i, "start")
	conn, err := net.Dial("tcp", "127.0.0.1:8085")
	if err != nil {
		log.Println(i, "Error dialing", err.Error())
		return // 终止程序
	}

	go func() {
		for {
			bytes := make([]byte, 0, 50)
			n, err := conn.Read(bytes)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(i, string(bytes[0:n]))
		}
	}()

	for i := 0; i < 10; i++ {
		_, err := conn.Write([]byte("hello" + strconv.Itoa(i)))
		if err != nil {
			log.Println(err)
			return
		}
	}
}

func TestClientWithCodec(t *testing.T) {
	for i := 0; i < 1; i++ {
		go start(i)
	}
	select {}
}

func startWithCodec(i int) {
	log.Println(i, "start")
	conn, err := net.Dial("tcp", "127.0.0.1:8085")
	if err != nil {
		log.Println(i, "Error dialing", err.Error())
		return // 终止程序
	}

	codec := codecFactory.NewCodec(conn)

	go func() {
		for {
			bytes, err := codec.Read()
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(i, string(bytes))
		}
	}()

	for i := 0; i < 10; i++ {
		_, err := conn.Write(util.Encode([]byte("hello" + strconv.Itoa(i))))
		if err != nil {
			log.Println(err)
			return
		}
	}
}
