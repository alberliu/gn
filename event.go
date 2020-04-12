package gn

const (
	eventConn  = 1 // 请求建立连接
	eventIn    = 2 // 数据流入
	eventClose = 3 // 断开连接
)

type event struct {
	fd    int // 文件描述符
	event int // 事件类型
}
