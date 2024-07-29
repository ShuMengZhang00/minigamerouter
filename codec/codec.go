package codec

const (
	Error MessageType = iota
	Request
	Response
	Event
)

type Codec interface {
	Reader
	Writer
	Close() error
	String() string
}

type Reader interface {
	ReadHeader(*Message, MessageType) error
	ReadBody(interface{}) error
}

type Writer interface {
	Write(*Message, interface{}) error
}

type MessageType int

type Message struct {
	Header map[string]string
	Id     string
	Target string
	Method string
}
