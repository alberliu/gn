package main

import (
	"github.com/alberliu/gn/test/util"
	"log"
	"net"
	"strconv"
)

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

	codec := util.NewCodec(conn)

	go func() {
		for {
			_, err = codec.Read()
			if err != nil {
				log.Println(err)
				return
			}
			for {
				bytes, ok, err := codec.Decode()
				// 解码出错，需要中断连接
				if err != nil {
					log.Println(err)
					return
				}
				if ok {
					log.Println(string(bytes))
					continue
				}
				break
			}
		}
	}()

	for i := 0; i < 10; i++ {
		_, err := conn.Write(util.Encode([]byte("hello" + strconv.Itoa(i))))
		if err != nil {
			log.Println(err)
			return
		}
		//log.Println("write:", n)
	}
}
