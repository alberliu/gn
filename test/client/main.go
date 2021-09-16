package main

import (
	"github.com/alberliu/gn/util"
	"log"
	"net"
	"strconv"
)

var codecFactory = util.NewHeaderLenCodecFactory(2, 1024)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func main() {
	go start()
	select {}
}

func start() {
	log.Println("start")
	conn, err := net.Dial("tcp", "127.0.0.1:8085")
	if err != nil {
		log.Println("Error dialing", err.Error())
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
			log.Println(string(bytes))
		}
	}()

	for i := 0; i < 1000; i++ {
		_, err := conn.Write(util.Encode([]byte("hello" + strconv.Itoa(i))))
		if err != nil {
			log.Println(err)
			return
		}
		//log.Println("write:", n)
	}
}
