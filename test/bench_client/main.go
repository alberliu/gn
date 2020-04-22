package main

import (
	"github.com/alberliu/gn/test/util"
	"log"
	"net"
	"strconv"
	"time"
)

// 20000 *5 / 300

func main() {
	var conns []net.Conn
	for i := 0; i < 20000; i++ {
		conn, err := net.Dial("tcp", "172.16.186.92:80")
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
	codec := util.NewCodec(conn)
	for {
		_, err := codec.Read()
		if err != nil {
			log.Println("error", err)
			return
		}
		for {
			_, ok, err := codec.Decode()
			if err != nil {
				log.Println("error", err)
				return
			}
			if ok {
				//log.Println(string(bytes))
				continue
			}
			break
		}
	}
}
