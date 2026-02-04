package osfp

import (
	"net"
	"os"
)

type StreamEvent struct {
	AsyncEvent
	conn *AsyncConn
	File *os.File
}

func (s *StreamEvent) Conn() net.Conn {
	return s.conn
}

func NewStreamEvent(e AsyncEvent, file *os.File, conn *AsyncConn) *StreamEvent {
	return &StreamEvent{
		AsyncEvent: &ThreadSafeCallbackEvent{AsyncEvent: e},
		conn:       conn,
		File:       file,
	}
}
