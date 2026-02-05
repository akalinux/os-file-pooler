package resolver

import (
	"encoding/binary"
	"fmt"
	"strings"
)

var ERR_FORMAT_ERROR = fmt.Errorf("Request format error, Code: 1")
var ERR_SERVER_FAILURE = fmt.Errorf("Server Error, Code: 2")
var ERR_NAME_ERROR = fmt.Errorf("Name Error, Code: 3")
var ERR_NOT_IMPLEMENTED = fmt.Errorf("Not Implemented, Code: 4")
var ERR_REFUSED = fmt.Errorf("Request was refused by the server, Code: 5")

type Question struct {
	Name  string
	Type  uint16
	Class uint16
}

type DnsRequest struct {
	Id        uint16
	Response  []byte
	Type      uint16
	Class     uint16
	Lookup    string
	Questions []*Question
	QCount    uint16
	ACount    uint16
	NsCount   uint16
	ArCount   uint16
	Responses []*FrameRes
	*ParsedFeilds
}

func (s *DnsRequest) Parse(buffer []byte) (e error) {
	size := len(buffer)
	if size < 12 {
		return ERR_PACKET_OUT_OF_BOUNDS
	}
	code := binary.BigEndian.Uint16(buffer[2:4])
	s.Response = buffer
	if 0x8000&code != 0x8000 {
		e = ERR_NO_DATA
		return
	}
	if msg := code & 0xf; msg != 0 {
		switch msg {
		case 1:
			e = ERR_FORMAT_ERROR
		case 2:
			e = ERR_SERVER_FAILURE
		case 3:
			e = ERR_NAME_ERROR
		case 4:
			e = ERR_NOT_IMPLEMENTED
		case 5:
			e = ERR_REFUSED
		default:
			e = fmt.Errorf("Unknown error, Code; %d", msg)
		}
		return
	}
	s.Id = binary.BigEndian.Uint16(buffer[0:2])

	answers := binary.BigEndian.Uint16(buffer[6:8])
	nscount := binary.BigEndian.Uint16(buffer[8:10])
	arcount := binary.BigEndian.Uint16(buffer[10:12])
	total := answers + nscount + arcount
	if total == 0 {
		e = ERR_NO_DATA
		return

	}
	questions := binary.BigEndian.Uint16(buffer[4:6])

	var pos int = 12
	if questions != 0 {

		var res string
		s.Questions = make([]*Question, questions)
		for i := range questions {
			q := &Question{}

			if res, pos, e = ParseName(buffer, pos); e == nil {
				q.Name = res
			} else {
				return
			}

			if size >= pos+3 {
				q.Type = binary.BigEndian.Uint16(buffer[pos:])
				pos += 2
				q.Class = binary.BigEndian.Uint16(buffer[pos:])
				pos += 2
			} else {
				e = ERR_PACKET_OUT_OF_BOUNDS
				return
			}
			if i == 0 {
				s.Lookup = q.Name
				s.Type = q.Type
				s.Class = q.Class
			}

			s.Questions[i] = q
		}
	}

	frame, pos, e := ParseFrame(buffer, pos)
	if e != nil {
		return
	}
	var tf uint16 = 1
	fields := &ParsedFeilds{
		Name: s.Lookup,
	}
	s.Responses = make([]*FrameRes, total)
	s.Responses[0] = frame
	fields.ConsumeFrame(frame)

	for pos < size && tf < total {
		frame, pos, e = ParseFrame(buffer, pos)

		if e != nil {
			return
		}
		s.Responses[tf] = frame
		fields.ConsumeFrame(frame)

		tf++
	}
	if tf != total {
		e = ERR_PACKET_OUT_OF_BOUNDS
		return
	}
	s.ParsedFeilds = fields
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
