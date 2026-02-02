package resolver

import (
	"encoding/binary"
)

type FrameRes struct {
	Name  string
	Type  uint16
	Class uint16
	RdLen uint16
	Rdata []byte
	Ttl   uint32
	Id    uint16
}

func ParseFrame(buffer []byte, offsetStart int) (frame *FrameRes, pos int, e error) {
	pos = offsetStart
	limit := len(buffer)
	if pos >= limit {
		e = ERR_PACKET_OUT_OF_BOUNDS
		return
	}

	frame = &FrameRes{}
	size := uint8(buffer[pos])
	if size&IS_COMPRESSED != 0 {
		sPos := binary.BigEndian.Uint16(buffer[pos:]) & COMPRESSED_MASK

		chunk, _, err := ParseName(buffer, int(sPos))
		frame.Name = chunk
		if err != nil {
			e = err
			return
		}

		pos += 2
		size = uint8(buffer[pos])
	} else {
		chunk, p, err := ParseName(buffer, pos)
		if err != nil {
			e = err
			return
		}
		frame.Name = chunk
		pos = p
	}

	if limit < pos+10 {
		e = ERR_PACKET_OUT_OF_BOUNDS
		return
	}
	frame.Type = binary.BigEndian.Uint16(buffer[pos:])
	pos += 2
	frame.Class = binary.BigEndian.Uint16(buffer[pos:])
	pos += 2
	frame.Ttl = binary.BigEndian.Uint32(buffer[pos:])
	pos += 4
	frame.RdLen = binary.BigEndian.Uint16(buffer[pos:])
	pos += 2
	final := pos + int(frame.RdLen)
	if final > limit {
		e = ERR_PACKET_OUT_OF_BOUNDS
		return
	}
	frame.Rdata = buffer[pos:final]

	pos = final

	return
}
