package main

import (
	"log"
	"net"
	"time"
)

func main() {
	var conns []net.Conn
	for i := 0; i < 50000; i++ {
		conn, err := net.Dial("tcp", "127.0.0.1:8085")
		if err != nil {
			log.Println("Error dialing", err.Error())
			continue
		}
		conns = append(conns, conn)
		if i%100 == 0 {
			log.Println(i)
		}
		time.Sleep(time.Millisecond * 10)
	}

	select {}
}
