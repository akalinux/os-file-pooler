package osfp

import "golang.org/x/sys/unix"

type PidJob struct {
	*CallBackJob
	pid int
	fd  *int
}

func (s *PidJob) Release() error {
	s.Lock.Lock()
	if *s.fd == -1 {
		s.Lock.Unlock()
		return nil
	}
	_ = unix.Close(*s.fd)
	*s.fd = -1
	s.Lock.Unlock()
	return nil

}
