package ge

import (
	"encoding/binary"
	"io"
	"sync"
	"syscall"
)

var (
	codecHeaderLen  int
	codecReadMaxLen int
	codecWriteLen   int
)

var (
	readBufferPool  *sync.Pool
	writeBufferPool *sync.Pool
)

func InitCodec(headerLen, readMaxLen, writeLen int) {
	codecHeaderLen = headerLen
	codecReadMaxLen = readMaxLen
	codecWriteLen = writeLen

	readBufferPool = &sync.Pool{
		New: func() interface{} {
			b := make([]byte, codecReadMaxLen)
			return b
		},
	}
	writeBufferPool = &sync.Pool{
		New: func() interface{} {
			b := make([]byte, codecWriteLen)
			return b
		},
	}
}

// Encode 编码数据
func Encode(bytes []byte) []byte {
	l := len(bytes)
	buffer := make([]byte, l+codecHeaderLen)
	// 将消息长度写入buffer
	binary.BigEndian.PutUint16(buffer[0:2], uint16(l))
	// 将消息内容内容写入buffer
	copy(buffer[codecHeaderLen:], bytes)
	return buffer[0 : codecHeaderLen+l]
}

// EncodeToFD 编码数据,并且写入文件描述符
func EncodeToFD(fd int, bytes []byte) error {
	l := len(bytes)
	var buffer []byte
	if l <= codecWriteLen-codecHeaderLen {
		obj := writeBufferPool.Get()
		defer writeBufferPool.Put(obj)
		buffer = obj.([]byte)[0 : l+codecHeaderLen]
	} else {
		buffer = make([]byte, l+codecHeaderLen)
	}

	// 将消息长度写入buffer
	binary.BigEndian.PutUint16(buffer[0:2], uint16(l))
	// 将消息内容内容写入buffer
	copy(buffer[codecHeaderLen:], bytes)

	_, err := syscall.Write(fd, bytes)
	return err
}

// Decode 解码
func Decode(c *Conn) error {
	obj := readBufferPool.Get()
	defer readBufferPool.Put(obj)

	var bytes = obj.([]byte)
	fd := int(c.GetFd())
	for {
		n, _, err := syscall.Recvfrom(fd, bytes, syscall.MSG_PEEK|syscall.MSG_DONTWAIT)
		if err != nil {
			// 缓存区暂无数据可读
			if err == syscall.EAGAIN {
				return nil
			}
			return err
		}
		// 客户端关闭连接
		if n == 0 {
			return io.EOF
		}

		if n < codecHeaderLen {
			return nil
		}

		if n > 1024-codecHeaderLen {
		}
		valueLen := int(binary.BigEndian.Uint16(bytes[0:codecHeaderLen]))
		if n < codecHeaderLen+valueLen {
			return nil
		}
		n, _, err = syscall.Recvfrom(fd, bytes[0:codecHeaderLen+valueLen], syscall.MSG_DONTWAIT)
		if err != nil {
			return err
		}

		c.s.handler.OnMessage(c, bytes[codecHeaderLen:codecHeaderLen+valueLen])
	}
}
