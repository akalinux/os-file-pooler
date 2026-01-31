package resolver

import (
	"encoding/hex"
	"testing"
)

func TestPack(t *testing.T) {
	for _, test := range []string{"google.com", "x.x.x.x", "a.b.c"} {

		bytes, e := PackFqdnToIp(test, Id())
		if e != nil {
			t.Fatalf("Should not have gotten an error, got %v", e)
			return
		}
		str := hex.Dump(bytes)
		t.Logf("Got the following:\n%s", str)
	}
}
