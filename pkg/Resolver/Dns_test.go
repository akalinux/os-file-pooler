package resolver

import (
	"encoding/hex"
	"net"
	"testing"
)

func TestPack(t *testing.T) {
	for _, test := range []string{"google.com", "x.x.x.x", "a.b.c"} {

		bytes, e := PackFqdnToIp(test, 2, 1)
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
	//ip := net.ParseIP("192.168.65.7")
	ip := net.ParseIP("192.168.1.129")
	//ip := net.ParseIP("8.8.8.8")

	dns, e := NewDnsClient(ip, 53, 4)
	dns.Id = 1337
	if e != nil {
		t.Fatalf("Error creating client: %v", e)
		return
	}
	defer dns.Close()

	var payload []byte
	var sent []byte
	//_, sent, e = dns.Send("ka1.homenet.ld")
	_, sent, e = dns.Send("google.com")

	if e = dns.SetTimeout(2); e != nil {
		t.Fatalf("Failed to force our timeout!")
		return
	}

	payload, e = dns.Recv()
	if e != nil {
		t.Fatalf("Network issue: %e", e)
		return
	}
	t.Logf("Sent: \n%s", hex.Dump(sent))
	_, e = Parse(payload, sent)
	//str := hex.Dump(payload[len(sent):])
	str := hex.Dump(payload[len(sent):])
	t.Logf("Got: \n%s", str)
	if e != nil {
		t.Fatalf("Expected no error and got: %v", e)
		return
	}

}
