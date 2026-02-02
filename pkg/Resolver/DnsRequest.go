package resolver

import (
	"encoding/binary"
	"strings"
)

type DnsRequest struct {
	Id            uint16
	RequestOffset int
	Response      []byte
	Request       []byte
	Type          uint16
	Class         uint16
	Lookup        string
}

func (s *DnsRequest) Parse() (fields *ParsedFeilds, e error) {

	buffer := s.Response
	code := binary.BigEndian.Uint16(buffer[2:4])
	if 0x8000&code != 0x8000 {
		e = ERR_NO_DATA
		return
	}
	if code&0xf != 0 {
		e = ERR_LOOKUP_ERROR
		return
	}
	a := binary.BigEndian.Uint16(buffer[0:2])
	if a != s.Id {
		e = ERR_ID_MISSMATCH
		return
	}
	answers := binary.BigEndian.Uint16(buffer[6:8])
	var ta uint16 = 0
	size := len(buffer)
	offset := s.RequestOffset
	//nscount := binary.BigEndian.Uint16(buffer[8:10])
	//arcount := binary.BigEndian.Uint16(buffer[10:12])
	if answers == 0 {
		e = ERR_NO_DATA
		return
	}

	frame, pos, err := ParseFrame(buffer, offset)
	if err != nil {
		e = err
		return
	}
	ta++
	fields = &ParsedFeilds{
		Name: s.Lookup,
	}
	fields.ConsumeFrame(frame)
	for pos < size && err == nil {
		frame, pos, err = ParseFrame(buffer, pos)

		if err != nil {
			break
		}
		fields.ConsumeFrame(frame)

		ta++
	}

	return
}

func ParseName(buffer []byte, offsetStart int) (res string, pos int, e error) {
	limit := len(buffer)
	if limit == 0 {
		e = ERR_NO_DATA
		return
	}

	if offsetStart > limit {
		e = ERR_PACKET_OUT_OF_BOUNDS
		return
	}
	pos = offsetStart
	var size uint8
	chunks := make([]string, 0, 3)

	size = uint8(buffer[pos])
	if size == 0 {
		pos++
		return
	}
	p := pos

	jump := 0
	for p <= limit {
		if size&IS_COMPRESSED == IS_COMPRESSED {
			b := p
			p = int(binary.BigEndian.Uint16(buffer[p:]) & COMPRESSED_MASK)
			if jump == 0 {
				pos = b + 2
			}
			size = buffer[p]
			jump++
			if jump > MAX_JUMPS {
				e = ERR_IN_FRAME_DECOMPRESSION
				return
			}
			continue
		} else {
			p++
			if jump == 0 {
				pos = p
			}
		}

		end := int(size) + p
		if end > limit {
			e = ERR_PACKET_OUT_OF_BOUNDS
			return
		}
		chunks = append(chunks, string(buffer[p:end]))
		p = end
		size = buffer[p]
		if size == 0 {
			if jump == 0 {
				pos = p + 1
			}
			break
		}
	}
	res = strings.Join(chunks, ".")
	return
}
