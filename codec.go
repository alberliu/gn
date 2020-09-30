package gn

import (
	"encoding/binary"
	"io"
	"sync"
	"syscall"
)

// Decoder 解码器
type Decoder interface {
	Decode(c *Conn) error
}

// Encoder 编码器
type Encoder interface {
	EncodeToFD(fd int32, bytes []byte) error
}

type headerLenDecoder struct {
	headerLen      int        // TCP包的头部长度，用来描述这个包的字节长度
	readMaxLen     int        // 所读取的客户端包的最大长度，客户端发送的包不能超过这个长度
	readBufferPool *sync.Pool // 读缓存区内存池
}

// NewHeaderLenDecoder 创建基于头部长度的解码器
// headerLen TCP包的头部内容，用来描述这个包的字节长度
// readMaxLen 所读取的客户端包的最大长度，客户端发送的包不能超过这个长度
func NewHeaderLenDecoder(headerLen, readMaxLen int) Decoder {
	if headerLen <= 0 || readMaxLen <= 0 {
		panic("headerLen or readMaxLen must must greater than 0")
	}

	return &headerLenDecoder{
		headerLen: headerLen,
		readBufferPool: &sync.Pool{
			New: func() interface{} {
				b := make([]byte, readMaxLen)
				return b
			},
		},
	}
}

// Decode 解码
func (d *headerLenDecoder) Decode(c *Conn) error {
	obj := d.readBufferPool.Get()
	defer d.readBufferPool.Put(obj)

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

		if n < d.headerLen {
			return nil
		}

		if n > 1024-d.headerLen {
		}
		valueLen := int(binary.BigEndian.Uint16(bytes[0:d.headerLen]))
		if n < d.headerLen+valueLen {
			return nil
		}
		n, _, err = syscall.Recvfrom(fd, bytes[0:d.headerLen+valueLen], syscall.MSG_DONTWAIT)
		if err != nil {
			return err
		}

		c.s.handler.OnMessage(c, bytes[d.headerLen:d.headerLen+valueLen])
	}
}

type headerLenEncoder struct {
	headerLen       int        // TCP包的头部长度，用来描述这个包的字节长度
	writeBufferLen  int        // 服务器发送给客户端包的建议长度，当发送的包小于这个值时，会利用到内存池优化
	writeBufferPool *sync.Pool // 写缓存区内存池
}

// NewHeaderLenEncoder 创建基于头部长度的编码器
// headerLen TCP包的头部内容，用来描述这个包的字节长度
// writeBufferLen 服务器发送给客户端包的建议长度，当发送的包小于这个值时，会利用到内存池优化
func NewHeaderLenEncoder(headerLen, writeBufferLen int) *headerLenEncoder {
	if headerLen <= 0 || writeBufferLen <= 0 {
		panic("headerLen or writeBufferLen must must greater than 0")
	}

	return &headerLenEncoder{
		headerLen:      headerLen,
		writeBufferLen: writeBufferLen,
		writeBufferPool: &sync.Pool{
			New: func() interface{} {
				b := make([]byte, writeBufferLen)
				return b
			},
		},
	}
}

// EncodeToFD 编码数据,并且写入文件描述符
func (e headerLenEncoder) EncodeToFD(fd int32, bytes []byte) error {
	l := len(bytes)
	var buffer []byte
	if l <= e.writeBufferLen-e.headerLen {
		obj := e.writeBufferPool.Get()
		defer e.writeBufferPool.Put(obj)
		buffer = obj.([]byte)[0 : l+e.headerLen]
	} else {
		buffer = make([]byte, l+e.headerLen)
	}

	// 将消息长度写入buffer
	binary.BigEndian.PutUint16(buffer[0:2], uint16(l))
	// 将消息内容内容写入buffer
	copy(buffer[e.headerLen:], bytes)

	_, err := syscall.Write(int(fd), buffer)
	return err
}
