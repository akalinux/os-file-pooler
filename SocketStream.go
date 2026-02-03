package osfp

import (
	"fmt"
	"net"
	"strings"
	"syscall"
)

type SockeStreamtJob struct {
	*CallBackJob
	*SokcetHandlers
	Conn net.Conn
}

type StreamEvent struct {
	*CallbackEvent
	Conn net.Conn
}

type SokcetHandlers struct {
	OnCanRead    func(*StreamEvent)
	OnCanWrite   func(*StreamEvent)
	OnError      func(*StreamEvent)
	OnDisconnect func(*StreamEvent)
	OnConnect    func(*StreamEvent)

	// These options are used at the time of job creation, but not at runtime
	// CAN_READ or CAN_WRITE or CAN_RW
	Wanted  uint32
	Addr    string
	Port    int
	Timeout int64
}

func ResolveAddr(addr string, port int) (dst syscall.Sockaddr, Type int, Addr net.Addr, e error) {
	if strings.HasPrefix(addr, "unix:") {
		path := addr[5:]
		Type = syscall.AF_UNIX
		dst = &syscall.SockaddrUnix{
			Name: path,
		}
		Addr = &net.UnixAddr{Name: path, Net: "unix"}
	} else {
		ip := net.ParseIP(addr)
		if ip == nil {
			e = fmt.Errorf("Failed to parse ip: %s", addr)
			return
		}
		if res := ip.To16(); res != nil {
			dst = &syscall.SockaddrInet6{
				Port: port,
				Addr: [16]byte(res),
			}
			Type = syscall.AF_INET6
			Addr = &net.TCPAddr{IP: ip, Port: port, Zone: ""}
		} else if res := ip.To4(); res != nil {
			dst = &syscall.SockaddrInet4{
				Port: port,
				Addr: [4]byte(res),
			}
			Type = syscall.AF_INET
			Addr = &net.TCPAddr{IP: ip, Port: port}
		} else {
			e = fmt.Errorf("Could not convert [%s] to ipv4 or ip6 address", addr)
		}
	}
	return
}

func NewSocketStreamJob(sh SokcetHandlers) (job *SockeStreamtJob, e error) {
	var fd int
	var dst syscall.Sockaddr
	var Type int
	var Addr net.Addr
	dst, Type, Addr, e = ResolveAddr(sh.Addr, sh.Port)
	if e != nil {
		return
	}

	fd, e = syscall.Socket(Type, syscall.SOCK_STREAM, 0)
	if e != nil {
		return
	}
	e = syscall.Connect(fd, dst)
	if e != nil && e != syscall.EINPROGRESS {
		syscall.Close(fd)
		return
	}
	e = syscall.SetNonblock(fd, true)
	if e != nil {
		syscall.Close(fd)
		return
	}
	if Addr != nil {

	}

	return
}
