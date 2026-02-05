package resolver

import (
	"fmt"
	"net"
	"syscall"
)

type DnsTester struct {
	*Dns
	Fd         int
	Dst        syscall.Sockaddr
	SocketType int
}

func (s *DnsTester) Recv() (payload []byte, e error) {
	payload = make([]byte, 0xffff)
	n, _, e := syscall.Recvfrom(s.Fd, payload, 0)
	payload = payload[0:n]
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

	return
}
func NewDnsClient(ip net.IP, port int, IpPref byte) (client *DnsTester, e error) {
	if ip == nil || port == 0 {
		e = fmt.Errorf("Ip cannot be nil, and port cannot be 0")
		return
	}

	if IpPref != 4 && IpPref != 6 {
		e = ERR_NO_IP_TYPE
		return
	}

	var dst syscall.Sockaddr
	var Type int
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
	}
	if IpPref != 4 && IpPref != 6 {
		e = ERR_NO_IP_TYPE
		return
	}
	var RType uint16
	if IpPref == 4 {
		RType = QTYPE_AA
	} else {
		RType = QTYPE_AAAA
	}

	client = &DnsTester{
		Dst:        dst,
		SocketType: Type,
		Dns: &Dns{
			IpPref:     &IpPref,
			EDNS0:      ENABLE_EDNSO,
			Class:      CLASS_A,
			Type:       RType,
			CheckNames: true,
		},
	}

	return
}

func (s *DnsTester) SetupSocket() (e error) {
	var fd int

	fd, e = syscall.Socket(s.SocketType, syscall.SOCK_DGRAM, 0)
	if e != nil {
		return
	}
	s.Fd = fd
	return
}

func (s *DnsTester) Close() error {
	if s.Fd == 0 {
		// was never opened
		return nil
	}
	return syscall.Close(s.Fd)
}

func (s *DnsTester) Send(name string) (payload []byte, e error) {
	if len(name) == 0 {
		e = ERR_INVALID_QUERY_STRING
		return
	}

	if s.CheckNames {
		for _, c := range name {
			if uint8(c) > 128 || string(c) == "_" {
				e = ERR_INVALID_QUERY_STRING
				return
			}

		}
	}

	payload, _, e = s.PackFqdnToIp(name, s.Type, s.Class)
	if e != nil {
		return
	}
	s.Id++
	e = syscall.Sendto(s.Fd, payload, 0, s.Dst)

	return
}

func (s *DnsTester) SetTimeout(seconds int) error {
	tv := syscall.Timeval{
		Sec:  int64(seconds),
		Usec: 0,
	}
	return syscall.SetsockoptTimeval(s.Fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
}
