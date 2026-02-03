package resolver

import (
	"net"
	"strings"
)

type ParsedFeilds struct {
	Ipv4 []net.IP
	Pos4 int
	Ipv6 []net.IP
	Pos6 int
	Ttl  uint32
	Name string
}

func (s *ParsedFeilds) Validate() bool {
	return len(s.Ipv4)+len(s.Ipv6) > 0
}

func (s *ParsedFeilds) Ipv4Ok() bool {
	return len(s.Ipv4) > 0
}

func (s *ParsedFeilds) Ipv6Ok() bool {
	return len(s.Ipv6) > 0
}

func (s *ParsedFeilds) ConsumeFrame(frame *FrameRes) {
	if frame.Class == 1 && (frame.Type == QTYPE_AA || frame.Type == QTYPE_AAAA) && strings.ToLower(frame.Name) == s.Name {

		ip := net.IP(frame.Rdata)
		if ip == nil {
			return
		}
		if s.Ttl == 0 || s.Ttl > frame.Ttl {
			s.Ttl = frame.Ttl
		}
		s.Name = frame.Name
		if ip.To4() != nil {
			s.Ipv4 = append(s.Ipv4, ip)
		} else {
			s.Ipv6 = append(s.Ipv6, ip)
		}
		s.Ttl = frame.Ttl
	}
}

func (s *ParsedFeilds) Resolve(IpPref byte, cb func(net.IP, error)) {
	var pref int = int(IpPref)
	if pref == 0 {
		pref = int(IP_PREF)
	}
	for ; pref > 2; pref -= 2 {
		switch pref {
		case 4:
			size := len(s.Ipv4)
			if size != 0 {
				cb(s.Ipv4[s.Pos4%size], nil)
				s.Pos4++
				if s.Pos4 < 0 {
					s.Pos4 = 0
				}
				return
			}
		case 6:
			size := len(s.Ipv4)
			if size != 0 {
				cb(s.Ipv6[s.Pos6%size], nil)
				s.Pos6++
				if s.Pos6 < 0 {
					s.Pos6 = 0
				}
				return
			}

		}

	}
	cb(nil, nil)
}
