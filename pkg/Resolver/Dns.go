package resolver

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"syscall"
)

const PACKT_READ_SIZE = 0xffff
const BASE_DNS_PACKET_SIZE = 16
const QTYPE_AA uint16 = 1
const QTYPE_AAAA uint16 = 0x001c
const CLASS_A uint16 = 1
const CLASS_ANY uint16 = 0x00ff
const EDNSO_SIZE = 1232
const ENABLE_EDNSO = true

var ERR_INVALID_QUERY_STRING = fmt.Errorf("Invalid Query String")
var ERR_NO_DATA = fmt.Errorf("No data in response")
var ERR_PACKET_OUT_OF_BOUNDS = fmt.Errorf("Error, oversized packet read")
var ERR_LOOKUP_ERROR = fmt.Errorf("Dns lookup error")
var ERR_ID_MISSMATCH = fmt.Errorf("Response Id does not match the request id")
var ERR_IN_FRAME_DECOMPRESSION = fmt.Errorf("Max packet depth exceeded")
var ERR_NO_IP_TYPE = fmt.Errorf("IpPref must either be 4 for ipv4 or 6 for ipv6")

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
	// TURN THIS BACK ON WHEN DONE TESTING!
	binary.BigEndian.PutUint16(baseRequest[2:], 0x0100)

	// always set request count to 1!
	binary.BigEndian.PutUint16(baseRequest[4:], 0x0001)
	ednsoPayload = []byte{
		0,
		0, 0x29,
		0x04, 0xd0, // Size 1232
		0, 0, 0, 0, // TTL
		0, 0,
	}

}

var seq uint16
var baseRequest []byte
var ednsoPayload []byte

type Dns struct {
	Id         uint16
	SocketType int
	IpPref     *byte
	EDNS0      bool
	Class      uint16
	Type       uint16
	pending    map[int16]func(net.IP, error)
	CheckNames bool
	// if not 0, add search domains if there are not at least this many dots
	Dots int
}

func (s *Dns) Parse(buffer []byte) (response *DnsRequest, e error) {

	response = &DnsRequest{}
	e = response.Parse(buffer)

	return
}

func (s *Dns) PackFqdnToIp(fqdn string, Type, Class uint16) (packed []byte, size int, e error) {

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

	binary.BigEndian.PutUint16(packed[offset:], Type)
	offset += 2
	binary.BigEndian.PutUint16(packed[offset:], Class)
	packed = s.Ednso(packed)
	return
}

func (s *Dns) Ednso(buffer []byte) []byte {
	if !s.EDNS0 {
		return buffer
	}
	// set the extended field size
	binary.BigEndian.PutUint16(buffer[10:], binary.BigEndian.Uint16(buffer[10:])+1)
	buffer = append(buffer, ednsoPayload...)
	return buffer

}
