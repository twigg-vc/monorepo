package iterator

// Helper struct to implement the Mapper interface with a function
type fMapper[IterT any, OutputT any] struct {
	mapFunc func(i IterT) (OutputT, error)
}

func (f fMapper[IterT, OutputT]) Map(i IterT) (OutputT, error) {
	return f.mapFunc(i)
}
