package resolver

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"syscall"
)

const PACKT_READ_SIZE = 0xffff
const BASE_DNS_PACKET_SIZE = 16
const QTYPE_AA uint16 = 1
const QTYPE_AAAA uint16 = 0x001c

var ERR_NO_DATA = fmt.Errorf("No data in response")
var ERR_PACKET_OUT_OF_BOUNDS = fmt.Errorf("Error, oversized packet read")
var ERR_LOOKUP_ERROR = fmt.Errorf("Dns lookup error")
var ERR_ID_MISSMATCH = fmt.Errorf("Response Id does not match the request id")
var ERR_IN_FRAME_DECOMPRESSION = fmt.Errorf("Max packet depth exceeded")

const IS_COMPRESSED = 0xc0
const COMPRESSED_MASK = 0x3fff
const MAX_JUMPS = 50

type ConType struct {
	Type int
	Addr syscall.Sockaddr
}

/*
// TODO https://datatracker.ietf.org/doc/html/rfc6891
// DNS Header
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|0 |1 |2 |3 |4 |5 |6 |7 |0 |1 |2 |3 |4 |5 |6 |7 |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
| ID                                            |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|QR|  Opcode   |AA|TC|RD|RA| Z      |   RCODE   |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
| QDCOUNT                                       |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
| ANCOUNT                                       |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
| NSCOUNT                                       |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
| ARCOUNT                                       |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

// DNS Query
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                                               |
/ QNAME /                                       |
/ /                                             |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
| QTYPE                                         |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
| QCLASS                                        |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                                               |
/ /                                             |
/ NAME /                                        |
|                                               |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
| TYPE |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
| CLASS |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
| TTL |
| |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
| RDLENGTH |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--|
/ RDATA /
/ /
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

*/

func init() {
	baseRequest = make([]byte, 12)
	// set RD field to true!
	binary.BigEndian.PutUint16(baseRequest[2:], 0x0100)
	// always set request count to 1!
	binary.BigEndian.PutUint16(baseRequest[4:], 0x0001)
}

var seq uint16
var idLock sync.Mutex
var baseRequest []byte

func Id() uint16 {
	idLock.Lock()
	defer idLock.Unlock()
	seq++
	return seq
}

func PackFqdnToIp(fqdn string, id uint16, Type uint16) (packed []byte, e error) {

	size := len(fqdn) + BASE_DNS_PACKET_SIZE
	if size == 18 {
		e = fmt.Errorf("String is 0 bytes long")
		return
	}
	chunks := strings.Split(fqdn, ".")
	end := len(chunks) - 1
	if end == 0 {
		e = fmt.Errorf("Invalid fqdn: [%s] must have at least 1 \".\", got: 0", fqdn)
		return
	}

	cs := len(chunks) >> 1
	size += cs + cs&1
	packed = make([]byte, size)
	copy(packed, baseRequest)
	binary.BigEndian.PutUint16(packed[0:2], id)
	offset := 12
	for _, chunk := range chunks {
		size := len(chunk)
		if size > 63 || size < 1 {
			e = fmt.Errorf("Invalid in reuqest: [%s], section: [%s] must be between 1 and 63 bytes long", fqdn, chunk)
			return
		}
		packed[offset] = byte(size)
		offset += 1
		copy(packed[offset:], []byte(chunk))
		offset += size
	}
	packed[offset] = 0
	offset += 1

	binary.BigEndian.PutUint16(packed[offset:], Type)
	offset += 2
	binary.BigEndian.PutUint16(packed[offset:], 0x0001)

	return
}

type Dns struct {
	Dst    syscall.Sockaddr
	Fd     int
	Id     uint16
	IpPref *byte
}

func NewDnsClient(ip net.IP, port int, IpPref byte) (client *Dns, e error) {
	if ip == nil || port == 0 {
		e = fmt.Errorf("Ip cannot be nil, and port cannot be 0")
		return
	}
	var dst syscall.Sockaddr
	var Type int
	var fd int
	if res := ip.To16(); res != nil {
		dst = &syscall.SockaddrInet6{
			Port: port,
			Addr: [16]byte(res),
		}
		Type = syscall.AF_INET6
	} else if res := ip.To4(); res != nil {
		dst = &syscall.SockaddrInet4{
			Port: port,
			Addr: [4]byte(res),
		}
		Type = syscall.AF_INET
	} else {
		e = fmt.Errorf("failed ip to address")
	}
	fd, e = syscall.Socket(Type, syscall.SOCK_DGRAM, 0)
	if e != nil {
		return
	}

	client = &Dns{
		Dst:    dst,
		Fd:     fd,
		IpPref: &IpPref,
	}

	return
}

func (s *Dns) Close() {
	syscall.Close(s.Fd)
}

func (s *Dns) Send(name string) (id uint16, payload []byte, e error) {
	id = s.Id
	s.Id++
	pref := *s.IpPref
	Type := QTYPE_AAAA
	if pref != 0 && pref != 6 {
		Type = QTYPE_AA
	}
	payload, e = PackFqdnToIp(name, id, Type)
	if e != nil {
		return
	}
	e = syscall.Sendto(s.Fd, payload, 0, s.Dst)

	return
}

func (s *Dns) SetTimeout(seconds int) error {
	tv := syscall.Timeval{
		Sec:  int64(seconds),
		Usec: 0,
	}
	return syscall.SetsockoptTimeval(s.Fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
}

func (s *Dns) Recv() (payload []byte, e error) {
	payload = make([]byte, 0xffff)
	n, _, e := syscall.Recvfrom(s.Fd, payload, 0)
	if e != nil {
		if e == syscall.EAGAIN || e == syscall.EWOULDBLOCK {
			e = nil
		} else {
			// something went wrong here
			return
		}
	}
	if n <= 0 {
		e = ERR_NO_DATA
		return
	}
	payload = payload[0:n]
	return
}

func (s *ParsedFeilds) ConsumeFrame(frame *FrameRes) {
	if frame.Class == 1 && (frame.Type == QTYPE_AA || frame.Type == QTYPE_AAAA) {
		if s.Ttl != 0 {
			s.Ipv4 = nil
			s.Ipv6 = nil
		}
		ip := net.IP(frame.Rdata)
		if ip.To4() != nil {
			s.Ipv4 = append(s.Ipv4, ip)
		} else {
			s.Ipv6 = append(s.Ipv6, ip)
		}
		s.Ttl = frame.Ttl
	}
}

func Parse(buffer []byte, query []byte) (fields *ParsedFeilds, e error) {

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
	b := binary.BigEndian.Uint16(query[0:2])
	if a != b {
		e = ERR_ID_MISSMATCH
		return
	}
	count := binary.BigEndian.Uint16(buffer[4:6])
	if count == 0 {
		e = ERR_NO_DATA
		return
	}
	size := len(buffer)

	frame, pos, err := ParseFrame(buffer, len(query))
	if err != nil {
		e = err
		return
	}
	fields = &ParsedFeilds{}
	fields.ConsumeFrame(frame)
	for pos < size && err == nil {
		frame, pos, err = ParseFrame(buffer, pos)
		if err != nil {
			break
		}
		fields.ConsumeFrame(frame)
	}

	return
}

func ParseName(buffer []byte, offsetStart int) (res string, pos int, e error) {
	limit := len(buffer)

	if offsetStart > limit {
		e = ERR_PACKET_OUT_OF_BOUNDS
		return
	}
	pos = offsetStart
	var size uint8
	chunks := make([]string, 0, 3)

	size = uint8(buffer[pos])
	p := pos

	jump := 0
	for p <= limit {
		if size&IS_COMPRESSED != 0 {
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
				pos++
			}
			break
		}
	}
	res = strings.Join(chunks, ".")
	return
}

type FrameRes struct {
	Name  string
	Type  uint16
	Class uint16
	RdLen uint16
	Rdata []byte
	Ttl   uint32
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

	if limit <= pos+10 {
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
	pos += int(frame.RdLen) + 1
	if frame.Class == 1 && (frame.Type == QTYPE_AAAA || frame.Type == QTYPE_AA) {
		// TODO
	}
	return
}
