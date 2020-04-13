package client

import (
	"encoding/binary"
	"gn/test/util"
	"log"
	"net"
	"strconv"
	"testing"
	"time"
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

// Encode 编码数据
func Encode(bytes []byte) []byte {
	l := len(bytes)
	buffer := make([]byte, l+2)
	// 将消息长度写入buffer
	binary.BigEndian.PutUint16(buffer[0:2], uint16(l))
	// 将消息内容内容写入buffer
	copy(buffer[2:], bytes)
	return buffer[0 : 2+l]
}

func TestEpoll_Client(t *testing.T) {
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

	for i := 0; i < 100; i++ {
		n, err := conn.Write(Encode([]byte("hello" + strconv.Itoa(i))))
		if err != nil {
			log.Println(err)
			return
		}
		log.Println("write:", n)
	}
	select {}
}

func TestBenchEpoll_Client(t *testing.T) {
	var conns []net.Conn
	for i := 0; i < 10000; i++ {
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
