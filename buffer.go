package gn

import (
	"errors"
	"io"
	"syscall"
)

var ErrNotEnough = errors.New("not enough")

// Buffer 读缓冲区,每个tcp长连接对应一个读缓冲区
type Buffer struct {
	Buf   []byte // 应用内缓存区
	start int    // 有效字节开始位置
	end   int    // 有效字节结束位置
}

// NewBuffer 创建一个缓存区
func NewBuffer(bytes []byte) *Buffer {
	return &Buffer{Buf: bytes, start: 0, end: 0}
}

func (b *Buffer) Len() int {
	return b.end - b.start
}

// reset 重新设置缓存区（将有用字节前移）
func (b *Buffer) reset() {
	if b.start == 0 {
		return
	}
	copy(b.Buf, b.Buf[b.start:b.end])
	b.end -= b.start
	b.start = 0
}

// ReadFromFD 从文件描述符里面读取数据
func (b *Buffer) ReadFromFD(fd int) error {
	b.reset()

	n, err := syscall.Read(fd, b.Buf[b.end:])
	if err != nil {
		return err
	}
	if n == 0 {
		return syscall.EAGAIN
	}
	b.end += n
	return nil
}

// ReadFromReader 从reader里面读取数据，如果reader阻塞，会发生阻塞
func (b *Buffer) ReadFromReader(reader io.Reader) (int, error) {
	b.reset()
	n, err := reader.Read(b.Buf[b.end:])
	if err != nil {
		return n, err
	}
	b.end += n
	return n, nil
}

// Seek 返回n个字节，而不产生移位，如果没有足够字节，返回错误
func (b *Buffer) Seek(len int) ([]byte, error) {
	if b.end-b.start >= len {
		buf := b.Buf[b.start : b.start+len]
		return buf, nil
	}
	return nil, ErrNotEnough
}

// Read 舍弃offset个字段，读取n个字段,如果没有足够的字节，返回错误
func (b *Buffer) Read(offset, limit int) ([]byte, error) {
	if b.Len() < offset+limit {
		return nil, ErrNotEnough
	}
	b.start += offset
	buf := b.Buf[b.start : b.start+limit]
	b.start += limit
	return buf, nil
}
