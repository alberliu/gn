package gn

import (
	"encoding/binary"
	"errors"
	"fmt"
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
	headerLen int // TCP包的头部长度，用来描述这个包的字节长度
}

// NewHeaderLenDecoder 创建基于头部长度的解码器
// headerLen TCP包的头部内容，用来描述这个包的字节长度
// readMaxLen 所读取的客户端包的最大长度，客户端发送的包不能超过这个长度
func NewHeaderLenDecoder(headerLen int) Decoder {
	if headerLen <= 0 {
		panic("headerLen or readMaxLen must must greater than 0")
	}

	return &headerLenDecoder{
		headerLen: headerLen,
	}
}

// Decode 解码
func (d *headerLenDecoder) Decode(c *Conn) error {
	buffer := c.GetBuffer()
	for {
		header, err := buffer.Seek(d.headerLen)
		if err == ErrNotEnough {
			return nil
		}
		bodyLen := int(binary.BigEndian.Uint16(header))
		// 检查valueLen合法性
		if bodyLen > buffer.Cap()-d.headerLen {
			return errors.New(fmt.Sprintf("illegal body length %d", bodyLen))
		}

		body, err := buffer.Read(d.headerLen, bodyLen)
		if err == ErrNotEnough {
			return nil
		}

		c.server.handler.OnMessage(c, body)
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

// EncodeToWriter 编码数据,并且写入Writer
func (e headerLenEncoder) EncodeToWriter(w io.Writer, bytes []byte) error {
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

	_, err := w.Write(buffer)
	return err
}
