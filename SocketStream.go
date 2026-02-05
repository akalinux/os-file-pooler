package osfp

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

type SockeStreamtJob struct {
	*CallBackJob
	*SokcetHandlers
	file      *os.File
	conn      *AsyncConn
	connected bool
}

func (s *SockeStreamtJob) OnEventCallBack(event AsyncEvent) {

}

type SokcetHandlers struct {
	OnCanRead    func(*StreamEvent)
	OnCanWrite   func(*StreamEvent)
	OnError      func(*StreamEvent)
	OnDisconnect func(*StreamEvent)
	OnConnect    func(*StreamEvent)

	// These options are used at the time of job creation, but not at runtime
	Timeout int64

	Addr  string
	Proto string
	Port  int
}

func ResolveAddr(addr string, port int, proto string) (dst syscall.Sockaddr, Type int, Addr net.Addr, e error) {
	if proto == "unix" {
		Type = syscall.AF_UNIX
		dst = &syscall.SockaddrUnix{
			Name: addr,
		}
		Addr = &net.UnixAddr{Name: addr, Net: proto}
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
			switch proto {
			case "tcp":
				Addr = &net.TCPAddr{IP: ip, Port: port, Zone: ""}
			case "udp":
				Addr = &net.UDPAddr{IP: ip, Port: port, Zone: ""}
			}
		} else if res := ip.To4(); res != nil {
			dst = &syscall.SockaddrInet4{
				Port: port,
				Addr: [4]byte(res),
			}
			Type = syscall.AF_INET
			switch proto {
			case "tcp":
				Addr = &net.TCPAddr{IP: ip, Port: port}
			case "udp":
				Addr = &net.UDPAddr{IP: ip, Port: port}
			}
		} else {
			e = fmt.Errorf("Unable to parse ip: %s", addr)
		}
	}
	if Addr == nil {
		e = fmt.Errorf("Unable to resolve protocol from string: %s", proto)
	}
	return
}

func NewSocketStreamJob(sh SokcetHandlers) (job *SockeStreamtJob, e error) {
	var fd int
	var dst syscall.Sockaddr
	var Type int
	var Addr net.Addr
	dst, Type, Addr, e = ResolveAddr(sh.Addr, sh.Port, sh.Proto)
	if e != nil {
		return
	}

	var CType int
	var stream bool
	connected := false
	if sh.Proto == "tcp" || sh.Proto == "unix" {
		stream = true
		CType = syscall.SOCK_STREAM
	} else {
		CType = syscall.SOCK_DGRAM
		connected = true
	}
	fd, e = syscall.Socket(Type, CType, 0)
	if e != nil {
		return
	}
	if stream {
		e = syscall.Connect(fd, dst)
		syscall.Close((fd))
		if e != nil {
			return
		}
	}
	if e != nil && e != syscall.EINPROGRESS {
		syscall.Close(fd)
		return
	}
	e = syscall.SetNonblock(fd, true)
	if e != nil {
		syscall.Close(fd)
		return
	}

	conn := &AsyncConn{
		Remote: Addr,
	}

	file := os.NewFile(uintptr(fd), fmt.Sprintf("[%s]:%d %s", sh.Addr, sh.Port, sh.Proto))

	job = &SockeStreamtJob{
		SokcetHandlers: &sh,
		conn:           conn,
		file:           file,
		connected:      connected,
	}

	job.CallBackJob = &CallBackJob{
		Events:          CAN_READ,
		FdId:            int32(fd),
		OnEventCallBack: job.OnEventCallBack,
	}

	return
}
