package osfp

import "golang.org/x/sys/unix"

type WaitPidJob struct {
	*callBackJob
	pid int
	fd  *int
}

func (s *WaitPidJob) Release() error {
	s.Lock.Lock()
	if *s.fd == -1 {
		s.Lock.Unlock()
		return nil
	}

	defer s.closeFd()
	s.Lock.Unlock()
	return s.callBackJob.Release()

}

func (s *WaitPidJob) closeFd() {
	if *s.fd == -1 {
		return
	}
	unix.Close(*s.fd)
	*s.fd = -1
}
