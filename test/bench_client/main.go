package main

import (
	"github.com/alberliu/gn/util"
	"log"
	"net"
	"strconv"
	"time"
)

var codecFactory = util.NewHeaderLenCodecFactory(2, 1024)

func main() {
	var conns []net.Conn
	for i := 0; i < 20000; i++ {
		conn, err := net.Dial("tcp", "172.16.58.235:80")
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
			time.Sleep(time.Millisecond * 3)
			_, err := conns[i].Write(util.Encode([]byte(strconv.FormatInt(time.Now().UnixNano(), 10))))
			if err != nil {
				log.Println("error dialing", err.Error())
			}
		}
	}

	select {}
}

func handleConn(conn net.Conn) {
	codec := codecFactory.NewCodec(conn)
	for {
		_, err := codec.Read()
		if err != nil {
			log.Println("error", err)
			return
		}
		for {
			bytes, err := codec.Read()
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(string(bytes))
		}
	}
}
