package resolver

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"syscall"
)

type ConType struct {
	Type int
	Addr syscall.Sockaddr
}

/*
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

*/

func init() {
	baseRequest = make([]byte, 12)
	// set RD field to true!
	binary.BigEndian.PutUint16(baseRequest[2:], 0x0100)
	// always set request count to 1!
	binary.BigEndian.PutUint16(baseRequest[4:], 0x0001)
	baseRequestType = make([]byte, 4)
	binary.BigEndian.PutUint16(baseRequestType, 0x0001)
	binary.BigEndian.PutUint16(baseRequestType[2:], 0x0001)
}

var seq uint16
var idLock sync.Mutex
var baseRequest []byte
var baseRequestType []byte

func Id() uint16 {
	idLock.Lock()
	defer idLock.Unlock()
	seq++
	return seq
}

func PackFqdnToIp(fqdn string, id uint16) (packed []byte, e error) {

	size := len(fqdn) + 16
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
	copy(packed, baseRequest[:])
	offset := 12
	for _, chunk := range chunks {
		size := len(chunk)
		if size > 255 || size < 1 {
			e = fmt.Errorf("Invalid in reuqest: [%s], section: [%s] must be between 1 and 255 bytes long", fqdn, chunk)
			return
		}
		packed[offset] = byte(size)
		offset += 1
		copy(packed[offset:], []byte(chunk))
		offset += size
	}
	packed[offset] = 0
	offset += 1
	copy(packed[offset:], baseRequestType[:])

	return
}
