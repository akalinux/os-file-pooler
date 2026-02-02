package resolver

import (
	"encoding/hex"
	"net"
	"testing"
)

func TestPack(t *testing.T) {
	for _, test := range []string{"google.com", "x.x.x.x", "a.b.c"} {

		t.Logf("Working with fqdn: %s", test)
		dns := &Dns{}
		bytes, _, e := dns.PackFqdnToIp(test, 1, 1)

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
	var ns string
	ns = "192.168.65.7"
	ns = "192.168.1.129"
	//ns = "8.8.8.8"
	ip := net.ParseIP(ns)

	dns, e := NewDnsClient(ip, 53, 6)
	var id uint16 = 1337
	dns.Id = id
	if e != nil {
		t.Fatalf("Error creating client: %v", e)
		return
	}
	defer dns.Close()

	var lookup string
	lookup = "ka1.homenet.ld"
	lookup = "yahoo.com"
	lookup = "google.com"
	payload, e := dns.Send(lookup)

	str := hex.Dump(payload)
	t.Logf("Sent: \n%s", str)
	if e = dns.SetTimeout(2); e != nil {
		t.Fatalf("Failed to force our timeout!")
		return
	}

	payload, e = dns.Recv()
	if e != nil {
		t.Fatalf("Network issue: %e", e)
		return
	}

	res, e := dns.Parse(payload)
	//str := hex.Dump(payload[len(sent):])
	str = hex.Dump(payload)
	t.Logf("Got: \n%s", str)
	if e != nil {
		t.Fatalf("Expected no error and got: %v", e)
		return
	}
	if res.Id != id {
		t.Fatalf("Expected id: %d, got: %d", id, res.Id)
	}

	if res.Type != dns.Type {
		t.Fatalf("Expected Type: %d, got %d", dns.Type, res.Type)
	}
	if res.Class != dns.Class {
		t.Fatalf("Expected Type: %d, got %d", dns.Class, res.Class)
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

	t.Logf("Starting empty string test")

	src = []byte{0}
	res, pos, err = ParseName(src, 0)
	if err != nil {
		t.Fatalf("Something went wrong, got error: %v", err)
		return
	}
	if pos != len(src) {
		t.Fatalf("Expected pos of: %d, got %d", len(src), pos)
		return
	}
	if res != "" {
		t.Fatalf("Expected: [], got: [%s]", res)
	}
	src = []byte{}

	res, pos, err = ParseName(src, 0)
	if err == nil {
		t.Fatalf("Failed to parse section correctly, should get an error")
	}

}
