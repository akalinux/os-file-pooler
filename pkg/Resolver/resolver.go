package resolver

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"syscall"
	"time"

	osfp "github.com/akalinux/os-file-pooler"
	"github.com/miekg/dns"
)

const DEFAULT_CONF = "/etc/resolv.conf"

type Resolver struct {
	Conf    *dns.ClientConfig
	Port    int
	id      int
	Util    *osfp.Util
	Timeout int64
	locker  sync.RWMutex
	Servers []ConType
}

type ActiveRequest struct {
	Cb       func(*dns.Msg, error)
	Timeout  int64
	Resolver Resolver

	*osfp.CallBackJob
}

func (s *ActiveRequest) Abort() {
	s.Release()
	syscall.Close(int(s.FdId))
}

func (s *Resolver) NextResolver() ConType {
	s.locker.Lock()
	defer s.locker.Unlock()
	s.id++
	id := s.id % len(s.Servers)
	return s.Servers[id]
}

func (s *Resolver) Request(name string, Cb func(*dns.Msg, error)) (pending *ActiveRequest, e error) {

	var fd int
	dst := s.NextResolver()
	fd, e = syscall.Socket(dst.Type, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if e != nil {
		return
	}
	syscall.Sendto(fd, []byte{}, 0, dst.Addr)
	pending = &ActiveRequest{
		Cb: Cb,
		CallBackJob: &osfp.CallBackJob{
			FdId:    int32(fd),
			Events:  osfp.CAN_READ,
			Timeout: s.Timeout,
		},
	}

	return
}

func ParseServers(servers []string, Port int) (res []ConType, e error) {
	res = make([]ConType, 0, len(servers))

	for _, nameserver := range servers {
		ip := net.ParseIP(nameserver)
		ct := ConType{}

		if ipv4 := ip.To4(); ipv4 != nil {

			ct.Addr = &syscall.SockaddrInet4{
				Port: Port,
				Addr: [4]byte(ipv4.To4()),
			}
			ct.Type = syscall.AF_INET

		} else if ipv6 := ip.To16(); ipv6 != nil {
			ct.Type = syscall.AF_INET6
			ct.Addr = &syscall.SockaddrInet6{
				Addr: [16]byte(ipv6),
				Port: Port,
			}
		} else {
			e = fmt.Errorf("Failed to parse nameserver: %s", nameserver)
			return
		}
		res = append(res, ct)
	}
	return
}
func BuildResolver(util *osfp.Util, file ...string) (resolver *Resolver, e error) {
	conf := DEFAULT_CONF
	dns.Id()
	if len(file) != 0 {
		conf = file[0]
	}

	config, e := dns.ClientConfigFromFile(conf)
	if e == nil && len(config.Servers) == 0 {
		return nil, errors.New("No Name Servers defined")
	}
	var Port int
	if Port, e = strconv.Atoi(config.Port); config.Port == "" || e != nil {
		Port = 53
	}

	var servers []ConType
	if servers, e = ParseServers(config.Servers, Port); e != nil {
		return
	}
	d := 2 * time.Second
	resolver = &Resolver{
		Conf:    config,
		Util:    util,
		Timeout: d.Milliseconds(),
		Port:    Port,
		Servers: servers,
	}

	return
}
