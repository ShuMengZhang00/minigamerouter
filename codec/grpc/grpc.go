package grpc

import (
	"gamerouter/codec"
	"io"
)

type Codec struct {
	Conn        io.ReadWriteCloser
	ContentType string
}

func (c *Codec) ReadHeader(message *codec.Message, messageType codec.MessageType) error {
	//TODO implement me
	panic("implement me")
}

func (c *Codec) ReadBody(i interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (c *Codec) Write(message *codec.Message, i interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (c *Codec) Close() error {
	//TODO implement me
	panic("implement me")
}

func (c *Codec) String() string {
	//TODO implement me
	panic("implement me")
}
