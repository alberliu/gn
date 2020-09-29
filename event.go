package gn

const (
	EventConn  = 1 // 请求建立连接
	EventIn    = 2 // 数据流入
	EventClose = 3 // 断开连接
)

type Event struct {
	Fd   int32 // 文件描述符
	Type int32 // 时间类型
}
