package codec

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

type uvarintDecoder struct {
}

// NewUvarintDecoder 创建基于头部长度的解码器
func NewUvarintDecoder() Decoder {
	return &uvarintDecoder{}
}

// Decode 解码
func (d *uvarintDecoder) Decode(buffer *Buffer, handle func([]byte)) error {
	for {
		bytes := buffer.GetBytes()
		bodyLen, headerLen := binary.Uvarint(bytes)
		if headerLen == 0 {
			return nil
		}

		// 检查valueLen合法性
		if int(bodyLen)+headerLen > buffer.Cap() {
			return errors.New(fmt.Sprintf("illegal body length %d", bodyLen))
		}
		body, err := buffer.Read(headerLen, int(bodyLen))
		if err == ErrNotEnough {
			return nil
		}
		handle(body)
	}
}

type uvarintEncoder struct {
	writeBufferLen  int        // 服务器发送给客户端包的建议长度，当发送的包小于这个值时，会利用到内存池优化
	writeBufferPool *sync.Pool // 写缓存区内存池
}

// NewUvarintEncoder 创建基于Uvarint的编码器
// writeBufferLen 服务器发送给客户端包的建议长度，当发送的包小于这个值时，会利用到内存池优化
func NewUvarintEncoder(writeBufferLen int) *uvarintEncoder {
	if writeBufferLen <= 0 {
		panic("headerLen or writeBufferLen must must greater than 0")
	}

	return &uvarintEncoder{
		writeBufferLen: writeBufferLen,
		writeBufferPool: &sync.Pool{
			New: func() interface{} {
				b := make([]byte, writeBufferLen)
				return b
			},
		},
	}
}

func getUvarintLen(x uint64) int {
	i := 0
	for x >= 0x80 {
		x >>= 7
		i++
	}
	return i + 1
}

// EncodeToWriter 编码数据,并且写入Writer
func (e uvarintEncoder) EncodeToWriter(w io.Writer, bytes []byte) error {
	bytesLen := uint64(len(bytes))
	uvarintLen := getUvarintLen(bytesLen)

	var buffer []byte
	l := uvarintLen + len(bytes)
	if l <= e.writeBufferLen {
		obj := e.writeBufferPool.Get()
		defer e.writeBufferPool.Put(obj)
		buffer = obj.([]byte)[0:l]
	} else {
		buffer = make([]byte, l)
	}

	// 将消息长度写入buffer
	binary.PutUvarint(buffer, bytesLen)
	// 将消息内容内容写入buffer
	copy(buffer[uvarintLen:], bytes)

	_, err := w.Write(buffer)
	return err
}
