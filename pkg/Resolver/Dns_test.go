package resolver

import (
	"encoding/hex"
	"net"
	"testing"
)

func TestPack(t *testing.T) {
	for _, test := range []string{"google.com", "x.x.x.x", "a.b.c"} {

		t.Logf("Working with fqdn: %s", test)
		dns := &Dns{
			Request: &DnsRequest{},
		}
		bytes, _, e := dns.PackFqdnToIp(test)

		if e != nil {
			t.Fatalf("Should not have gotten an error, got %v", e)
			return
		}
		str := hex.Dump(bytes)
		t.Logf("Got the following:\n%s", str)
		var check string
		t.Logf("checking string parsing of: [%s]", test)
		check, _, e = ParseName(bytes, 12)
		if e != nil {
			t.Fatalf("Got an error, expceted none: %v", e)
			return
		}
		if check != test {
			t.Fatalf("Failed to decode string, expected: [%s], got: [%s]", test, check)
		}
	}
}

func TestDnsLookup(t *testing.T) {
	ip := net.ParseIP("192.168.65.7")
	//ip := net.ParseIP("192.168.1.129")
	//ip := net.ParseIP("8.8.8.8")

	dns, e := NewDnsClient(ip, 53, 4)
	dns.Id = 1337
	if e != nil {
		t.Fatalf("Error creating client: %v", e)
		return
	}
	defer dns.Close()

	//_, sent, e = dns.Send("ka1.homenet.ld")
	e = dns.Send("google.com")

	if e = dns.SetTimeout(2); e != nil {
		t.Fatalf("Failed to force our timeout!")
		return
	}

	e = dns.Recv()
	if e != nil {
		t.Fatalf("Network issue: %e", e)
		return
	}
	t.Logf("Sent: \n%s", hex.Dump(dns.Request.Request))

	_, e = dns.Request.Parse()
	//str := hex.Dump(payload[len(sent):])
	str := hex.Dump(dns.Request.Response)
	t.Logf("Got: \n%s", str)
	if e != nil {
		t.Fatalf("Expected no error and got: %v", e)
		return
	}

}

func TestParseName(t *testing.T) {
	src := []byte{3}
	src = append(src, []byte("com")...)
	src = append(src, 0)
	// start after this point
	start := len(src)
	src = append(src, 3)
	src = append(src, []byte("joe")...)
	src = append(src, 0xc0, 0)
	t.Logf("Starting Compressed tests")
	res, pos, err := ParseName(src, start)
	if err != nil {
		t.Fatalf("No errors expected, got: %v", err)
		return
	}
	if res != "joe.com" {
		t.Fatalf("Expected: joe.com, got: %s", res)
	}
	if pos != len(src) {
		t.Fatalf("Expected: %d, got: %d", len(src), pos)
	}

	t.Logf("Starting Uncompressed tests")
	src = []byte{}
	src = append(src, 1)
	src = append(src, []byte("j")...)
	src = append(src, 2)
	src = append(src, []byte("ex")...)
	src = append(src, 3)
	src = append(src, []byte("com")...)
	src = append(src, 0)
	res, pos, err = ParseName(src, 0)
	if err != nil {
		t.Fatalf("Something went wrong, got error: %v", err)
		return
	}
	if pos != len(src) {
		t.Fatalf("Expected pos of: %d, got %d", len(src), pos)
		return
	}
	if res != "j.ex.com" {
		t.Fatalf("Expected: [j.ex.com], got: %s", res)
	}

}
