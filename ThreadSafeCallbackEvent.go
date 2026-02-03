package osfp

import (
	"sync"
	"time"
)

type ThreadSafeCallbackEvent struct {
	sync.RWMutex
	*CallbackEvent
}

func (s *ThreadSafeCallbackEvent) Error() error {
	s.RLock()
	defer s.RUnlock()
	return s.CallbackEvent.Error()
}

func (s *ThreadSafeCallbackEvent) InError() bool {
	s.RLock()
	defer s.RUnlock()
	return s.CallbackEvent.InError()
}

func (s *ThreadSafeCallbackEvent) InTimeout() bool {
	s.RLock()
	defer s.RUnlock()
	return s.CallbackEvent.InTimeout()
}

func (s *ThreadSafeCallbackEvent) IsRead() bool {
	s.RLock()
	defer s.RUnlock()
	return s.CallbackEvent.IsRead()
}

func (s *ThreadSafeCallbackEvent) IsWrite() bool {
	s.RLock()
	defer s.RUnlock()
	return s.CallbackEvent.IsWrite()
}

func (s *ThreadSafeCallbackEvent) IsRW() bool {
	s.RLock()
	defer s.RUnlock()
	return s.CallbackEvent.IsRW()
}

func (s *ThreadSafeCallbackEvent) GetTimeout() int64 {
	s.Lock()
	defer s.Unlock()
	return s.CallbackEvent.GetTimeout()
}

func (s *ThreadSafeCallbackEvent) GetNow() time.Time {
	s.RLock()
	defer s.RUnlock()
	return s.CallbackEvent.GetNow()
}

func (s *ThreadSafeCallbackEvent) SetTimeout(t int64) {
	s.Lock()
	defer s.Unlock()
	s.CallbackEvent.SetTimeout(t)
}

func (s *ThreadSafeCallbackEvent) Release() {
	s.Lock()
	defer s.Unlock()
	s.CallbackEvent.Release()
}

func (s *ThreadSafeCallbackEvent) PollRead() {
	s.Lock()
	defer s.Unlock()
	s.CallbackEvent.PollRead()
}

func (s *ThreadSafeCallbackEvent) PollWrite() {
	s.Lock()
	defer s.Unlock()
	s.CallbackEvent.PollWrite()
}

func (s *ThreadSafeCallbackEvent) PollReadWrite() {
	s.Lock()
	defer s.Unlock()
	s.CallbackEvent.PollReadWrite()
}

func (s *ThreadSafeCallbackEvent) SetError(e error) {
	s.Lock()
	defer s.Unlock()
	s.CallbackEvent.SetError(e)
}
