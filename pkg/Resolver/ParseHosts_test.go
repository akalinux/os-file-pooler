package resolver

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestNegative(t *testing.T) {
	_, e := ParseHostsFile("/dev/IamNotCrook")
	if e == nil {
		t.Fatalf("Expected the parse to be nil!")
	}
}

func TestParse(t *testing.T) {
	wd, _ := os.Getwd()
	file := filepath.Join(wd, "testdata", "hosts.txt")

	res, e := ParseHostsFile(file)
	if e != nil {
		t.Fatalf("Should have parsed without error, got %v", e)
		return
	}

	if res == nil {
		t.Fatalf("Expected result to be non negative!")
		return
	}
	var ip net.IP
	cb := func(i net.IP, err error) {
		ip = i
		e = err
	}
	res.Resolve("LocalHost", cb)
	if e != nil {
		t.Fatalf("Expected no error?")
		return
	}
	if ip == nil {
		t.Fatalf("Should have resolved localhost!")
	}
	if ip.String() != "::1" {
		t.Fatalf("Expected loopback to be: [::1], got [%s]", ip.String())
	}
	res.Resolve("127.0.0.1", cb)
	if ip.String() != "127.0.0.1" {
		t.Fatalf("Expected loopback to be: [127.0.0.1], got [%s]", ip.String())
	}
	var pref byte = 4
	res.Pref = &pref
	res.Resolve("LocalHost", cb)
	if ip.String() != "127.0.0.1" {
		t.Fatalf("Expected loopback to be: [127.0.0.1], got [%s]", ip.String())
	}

	// prove an ip being passed through should come back!
	res.Resolve("::1", cb)
	if ip.String() != "::1" {
		t.Fatalf("Expected loopback to be: [::1], got [%s]", ip.String())
	}

	for id := range 4 {
		cmp := fmt.Sprintf("127.0.0.%d", id%3+1)
		res.Resolve("no.where.com", cb)
		t.Logf("In Set: %d, Expecting: %s", id, cmp)
		if ip.String() != cmp {
			t.Fatalf("In id: %d, Expected [no.where.com] to be: [%s], got [%s]", id, cmp, ip.String())
		}
	}
	pref = 6
	for id := range 4 {
		cmp := fmt.Sprintf("::%d", id%3+1)
		res.Resolve("no.where.com", cb)
		t.Logf("In Set: %d, Expecting: %s", id, cmp)
		if ip.String() != cmp {
			t.Fatalf("In id: %d, Expected [no.where.com] to be: [%s], got [%s]", id, cmp, ip.String())
		}
	}
	res.Resolve("Nope", cb)
	if ip != nil {
		t.Fatalf("Should fail our lookup!")
	}
	const UintSize = 32 << (^uint(0) >> 32 & 1) // 32 or 64
	const MaxInt = 1<<(UintSize-1) - 1
	res.Resolved["no.where.com"].Pos4 = MaxInt
	res.Resolved["no.where.com"].Pos6 = MaxInt
	t.Logf("MaxInt is: %d", MaxInt)
	remap := []byte{2, 1, 2, 3}
	for i, str := range []string{"127.0.0.", "::"} {
		// Prove wrapping code works
		pref = 4 + byte(i)*2
		for set, id := range remap {
			cmp := fmt.Sprintf("%s%d", str, id)
			res.Resolve("no.where.com", cb)

			t.Logf("Ipv%d Rollover max int roll over verification, In Set: %d, Expecting: %s", pref, set, cmp)
			if ip.String() != cmp {
				t.Fatalf("In id: %d, Expected [no.where.com] to be: [%s], got [%s]", seq, cmp, ip.String())
			}
		}
	}

}
