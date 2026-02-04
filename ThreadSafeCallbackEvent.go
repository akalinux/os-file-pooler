package osfp

import (
	"sync"
	"time"
)

type ThreadSafeCallbackEvent struct {
	sync.RWMutex
	AsyncEvent
}

func NewThreadSafeEvent(e AsyncEvent) *ThreadSafeCallbackEvent {
	return &ThreadSafeCallbackEvent{AsyncEvent: e}
}

func (s *ThreadSafeCallbackEvent) Error() error {
	s.RLock()
	defer s.RUnlock()
	return s.AsyncEvent.Error()
}

func (s *ThreadSafeCallbackEvent) InError() bool {
	s.RLock()
	defer s.RUnlock()
	return s.AsyncEvent.InError()
}

func (s *ThreadSafeCallbackEvent) InTimeout() bool {
	s.RLock()
	defer s.RUnlock()
	return s.AsyncEvent.InTimeout()
}

func (s *ThreadSafeCallbackEvent) IsRead() bool {
	s.RLock()
	defer s.RUnlock()
	return s.AsyncEvent.IsRead()
}

func (s *ThreadSafeCallbackEvent) IsWrite() bool {
	s.RLock()
	defer s.RUnlock()
	return s.AsyncEvent.IsWrite()
}

func (s *ThreadSafeCallbackEvent) IsRW() bool {
	s.RLock()
	defer s.RUnlock()
	return s.AsyncEvent.IsRW()
}

func (s *ThreadSafeCallbackEvent) GetTimeout() int64 {
	s.Lock()
	defer s.Unlock()
	return s.AsyncEvent.GetTimeout()
}

func (s *ThreadSafeCallbackEvent) GetNow() time.Time {
	s.RLock()
	defer s.RUnlock()
	return s.AsyncEvent.GetNow()
}

func (s *ThreadSafeCallbackEvent) SetTimeout(t int64) {
	s.Lock()
	defer s.Unlock()
	s.AsyncEvent.SetTimeout(t)
}

func (s *ThreadSafeCallbackEvent) Release() {
	s.Lock()
	defer s.Unlock()
	s.AsyncEvent.Release()
}

func (s *ThreadSafeCallbackEvent) PollRead() {
	s.Lock()
	defer s.Unlock()
	s.AsyncEvent.PollRead()
}

func (s *ThreadSafeCallbackEvent) PollWrite() {
	s.Lock()
	defer s.Unlock()
	s.AsyncEvent.PollWrite()
}

func (s *ThreadSafeCallbackEvent) PollReadWrite() {
	s.Lock()
	defer s.Unlock()
	s.AsyncEvent.PollReadWrite()
}

func (s *ThreadSafeCallbackEvent) SetError(e error) {
	s.Lock()
	defer s.Unlock()
	s.AsyncEvent.SetError(e)
}
