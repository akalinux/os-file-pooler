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

type Dns struct {
	Dst     syscall.Sockaddr
	Fd      int
	Id      uint16
	IpPref  *byte
	EDNS0   bool
	Request *DnsRequest
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

func (s *Dns) Send(name string) (e error) {
	id := s.Id
	pref := *s.IpPref

	var offset int
	var payload []byte
	s.Request = &DnsRequest{
		Id: id,
	}
	s.Request.Type = QTYPE_AAAA
	if pref != 0 && pref != 6 {
		s.Request.Type = QTYPE_AA
	}
	payload, offset, e = s.PackFqdnToIp(name)
	if e != nil {
		return
	}
	s.Request.Request = payload
	s.Request.RequestOffset = offset
	s.Id++
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

func (s *Dns) Recv() (e error) {
	payload := make([]byte, 0xffff)
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
	s.Request.Response = payload[0:n]
	return
}

func (s *Dns) PackFqdnToIp(fqdn string) (packed []byte, size int, e error) {

	if s.Request == nil {
		e = ERR_NO_DATA
		return
	}
	size = len(fqdn) + BASE_DNS_PACKET_SIZE
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
	binary.BigEndian.PutUint16(packed[0:2], s.Id)
	offset := 12
	for _, chunk := range chunks {
		cs := len(chunk)
		if cs > 63 || cs < 1 {
			e = fmt.Errorf("Invalid in reuqest: [%s], section: [%s] must be between 1 and 63 bytes long", fqdn, chunk)
			return
		}
		packed[offset] = byte(cs)
		offset += 1
		copy(packed[offset:], []byte(chunk))
		offset += cs
	}
	packed[offset] = 0
	offset += 1

	binary.BigEndian.PutUint16(packed[offset:], s.Request.Type)
	offset += 2
	binary.BigEndian.PutUint16(packed[offset:], 0x0001)

	return
}
