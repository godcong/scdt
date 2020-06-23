package scdt

// Queue ...
type Queue struct {
	Message  *Message
	cfg      *Config
	callback chan interface{}
}

// NewQueue ...
func NewQueue(msg *Message) *Queue {

	return new(Queue)

}
