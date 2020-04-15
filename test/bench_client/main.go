package main

import (
	"github.com/alberliu/gn/test/util"
	"log"
	"net"
	"strconv"
	"time"
)

// 50000 *5 *60

func main() {
	var conns []net.Conn
	for i := 0; i < 1000; i++ {
		conn, err := net.Dial("tcp", "127.0.0.1:8085")
		if err != nil {
			log.Println("error dialing", err.Error())
			continue
		}

		conns = append(conns, conn)
		go handleConn(conn)
		if i%100 == 0 {
			log.Println(i)
		}
		time.Sleep(time.Millisecond * 1)
	}

	for {
		for i := range conns {
			time.Sleep(time.Millisecond)
			conns[i].Write(util.Encode([]byte(strconv.FormatInt(time.Now().UnixNano(), 10))))
		}
	}

	select {}
}

func handleConn(conn net.Conn) {
	codec := util.NewCodec(conn)
	for {
		_, err := codec.Read()
		if err != nil {
			log.Println("error", err)
			return
		}
		for {
			bytes, ok, err := codec.Decode()
			if err != nil {
				log.Println("error", err)
				return
			}
			if ok {
				log.Println(string(bytes))
				continue
			}
			break
		}
	}
}
