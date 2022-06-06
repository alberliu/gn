package main

import (
	"github.com/alberliu/gn/codec"
	"log"
	"net"
	"strconv"
	"time"
)

var (
	decoder = codec.NewHeaderLenDecoder(2)
	encoder = codec.NewHeaderLenEncoder(2, 1024)
)

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
			err := encoder.EncodeToWriter(conns[i], []byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
			if err != nil {
				log.Println("error dialing", err.Error())
			}
		}
	}

	select {}
}

func handleConn(conn net.Conn) {
	buffer := codec.NewBuffer(make([]byte, 1024))
	var handler = func(bytes []byte) {
		log.Println(string(bytes))
	}

	for {
		_, err := buffer.ReadFromReader(conn)
		if err != nil {
			log.Println("error", err)
			return
		}

		err = decoder.Decode(buffer, handler)
		if err != nil {
			log.Println(err)
			return
		}
	}
}
