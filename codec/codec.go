package codec

import (
	"io"
)

// Decoder 解码器
type Decoder interface {
	Decode(*Buffer, func([]byte)) error
}

// Encoder 编码器
type Encoder interface {
	EncodeToWriter(w io.Writer, bytes []byte) error
}
