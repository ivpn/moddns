package channel

// CollectorChannel defines a generic channel interface that all collector channels must implement.
type CollectorChannel interface {
	Send(data any) error
	Receive() (any, error)
}
