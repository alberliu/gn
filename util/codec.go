package util

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/alberliu/gn"
	"net"
)

type headerLenCodecFactory struct {
	headerLen  int // 头部长度
	bodyMaxLen int // 包体最大长度
}

// NewHeaderLenCodecFactory 创建解码器工厂
func NewHeaderLenCodecFactory(headerLen, bodyMaxLen int) *headerLenCodecFactory {
	return &headerLenCodecFactory{
		headerLen:  headerLen,
		bodyMaxLen: bodyMaxLen,
	}
}

// NewCodec 创建解码器
func (h *headerLenCodecFactory) NewCodec(conn net.Conn) *Codec {
	return &Codec{
		factory: h,
		conn:    conn,
		readBuf: gn.NewBuffer(make([]byte, h.bodyMaxLen)),
		unread:  false,
	}
}

// Codec 编解码器，用来处理tcp的拆包粘包
type Codec struct {
	factory *headerLenCodecFactory
	conn    net.Conn
	readBuf *gn.Buffer // 读缓冲
	unread  bool       // 是否有未读数据
}

// Read 读取数据，非多协程安全
func (c *Codec) Read() ([]byte, error) {
	for {
		if c.unread {
			bytes, ok, err := c.decode()
			if err != nil {
				return nil, err
			}
			if ok {
				return bytes, nil
			}
			c.unread = false
		}

		_, err := c.readBuf.ReadFromReader(c.conn)
		if err != nil {
			return nil, err
		}
		c.unread = true
	}
}

// Decode 解码数据
// Package 代表一个解码包
// bool 标识是否还有可读数据
func (c *Codec) decode() ([]byte, bool, error) {
	var err error
	// 读取数据长度
	lenBuf, err := c.readBuf.Seek(c.factory.headerLen)
	if err != nil {
		return nil, false, nil
	}

	// 读取数据内容
	valueLen := int(binary.BigEndian.Uint16(lenBuf))

	// 数据的字节数组长度大于buffer的长度，返回错误
	if valueLen > c.factory.bodyMaxLen {
		fmt.Println("out of max len")
		return nil, false, errors.New("out of max len")
	}

	valueBuf, err := c.readBuf.Read(c.factory.headerLen, valueLen)
	if err != nil {
		return nil, false, nil
	}
	return valueBuf, true, nil
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
