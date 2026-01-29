package osfp

import (
	"context"
	"testing"
	"time"
)

func TestPool(t *testing.T) {
	p, e := NewPool(1, 1)
	if e != nil {
		t.Fatalf("Got an eror creating our pool %v", e)
	}
	p.Start()
	defer p.Stop()

	u := p.NewUtil()
	count := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	u.SetTimeout(func(event *CallbackEvent) {
		count++
		t.Logf("runtime completed, ending context")
		cancel()
	}, 1)

	<-ctx.Done()
	t.Logf("calling stop")
	p.Stop()
	if count != 1 {
		t.Fatalf("expected a count of: 1, got: %d", count)
	}

}
