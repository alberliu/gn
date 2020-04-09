package ge

type Option interface {
	apply(*server)
}
