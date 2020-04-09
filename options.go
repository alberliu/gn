package gn

type Option interface {
	apply(*server)
}
