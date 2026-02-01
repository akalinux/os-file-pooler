package resolver

import (
	"bufio"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
)

var lineRe = regexp.MustCompile(`\s+`)
var clearComment = regexp.MustCompile("#.*$")

// default ip pref
const IP_PREF byte = 6

type ParsedHostsFile struct {
	LastMod  time.Time
	Resolved map[string]*ParsedFeilds
	// if set,
	Pref *byte
}

func (s *ParsedHostsFile) Resolve(lookup string, cb func(net.IP, error)) {
	if ip := net.ParseIP(lookup); ip != nil {
		cb(ip, nil)
		return
	}
	var ipset *ParsedFeilds
	var ok bool
	if ipset, ok = s.Resolved[strings.ToLower(lookup)]; !ok {
		cb(nil, nil)
		return
	}
	ipset.Resolve(*s.Pref, cb)

}

const BUFFER_SIZE = 0xffff

func ParseHostsFile(hostfile string, opts ...*ParsedHostsFile) (hosts *ParsedHostsFile, e error) {

	stat, e := os.Stat(hostfile)
	if e != nil {
		return
	}
	ts := stat.ModTime()
	if len(opts) > 0 {
		if ts.Compare(opts[0].LastMod) == 0 {
			hosts = opts[0]
			return
		}
	}
	fh, e := os.OpenFile(hostfile, os.O_RDONLY, 0)
	if e != nil {
		return
	}
	defer fh.Close()
	scanner := bufio.NewScanner(fh)
	res := make(map[string]*ParsedFeilds)
	pref := IP_PREF
	hosts = &ParsedHostsFile{
		LastMod:  ts,
		Resolved: res,
		Pref:     &pref,
	}

SCAN_CTRL:
	for scanner.Scan() {
		line := scanner.Text()
		list := lineRe.Split(line, -1)
		var ip net.IP
		for id, value := range list {
			clean := string(clearComment.ReplaceAll([]byte(value), []byte{}))
			if len(clean) == 0 {
				break
			} else if id == 0 {
				ip = net.ParseIP(clean)
				if ip == nil {
					continue SCAN_CTRL
				}
			} else {
				lc := strings.ToLower(clean)
				var f *ParsedFeilds
				var ok bool
				if f, ok = res[lc]; !ok {
					f = &ParsedFeilds{}
					res[lc] = f
				}
				if ip.To4() != nil {

					f.Ipv4 = append(f.Ipv4, ip)
				} else {
					f.Ipv6 = append(f.Ipv6, ip)
				}
			}

			if len(clean) != len(value) {
				// we had to clear a comment
				break
			}
		}
	}

	return
}
