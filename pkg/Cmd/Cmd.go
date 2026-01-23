package cmd

// https://stackoverflow.com/questions/23031752/start-a-process-in-go-and-detach-from-it
// https://cs.opensource.google/go/go/+/refs/tags/go1.25.5:src/os/exec/exec.go
import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

type Cmd struct {
	Name        string
	Args        []string
	Process     *os.Process
	SysProcAttr *syscall.SysProcAttr
	Stdin       *RwSet
	Stdout      *RwSet
	Stderr      *RwSet
	Dir         string
	Env         []string
}

func NewCmd(name string, args ...string) *Cmd {
	return &Cmd{
		Name: name,
		Args: args,

		Stdin:  &RwSet{},
		Stdout: &RwSet{},
		Stderr: &RwSet{},
	}
}

func (s *Cmd) Start() (proc *os.Process, err error) {
	cmd := exec.Command(s.Name, s.Args...)
	if s.Stdin.Read != nil {
		cmd.Stdin = s.Stdin.Read
	}
	if s.Stdout.Write != nil {
		cmd.Stdout = s.Stdout.Write
	}

	if s.Stderr.Write != nil {
		cmd.Stderr = s.Stderr.Write
	}
	err = cmd.Start()

	if err != nil {
		s.CloseFd()
		return
	}
	proc = cmd.Process
	if s.Stderr.Write != nil {
		s.Stderr.Write.Close()
	}
	if s.Stdout.Write != nil {
		s.Stdout.Write.Close()
	}
	if s.Stdin.Read != nil {
		s.Stdin.Read.Close()
	}
	return
}

func (s *Cmd) CloseFd() {
	if s.Stdin.Write != nil {
		s.Stdin.Write.Close()
		s.Stdin.Read.Close()
	}
	if s.Stdout.Write != nil {
		s.Stdout.Write.Close()
		s.Stdout.Read.Close()
	}
	if s.Stderr.Write != nil {
		s.Stderr.Write.Close()
		s.Stderr.Read.Close()
	}
}

func (s *Cmd) newPs(set *RwSet, side bool) (*os.File, error) {
	if set.Read != nil || set.Write != nil {
		return nil, errors.New("Reader/Writer all ready defined!")
	}
	r, w, e := os.Pipe()
	if e != nil {
		return nil, e
	}
	set.Read = r
	set.Write = w
	if side {
		return w, nil
	}
	return r, nil
}

func (s *Cmd) NewStdin() (*os.File, error) {
	return s.newPs(s.Stdin, true)
}

func (s *Cmd) NewStdout() (*os.File, error) {
	return s.newPs(s.Stdout, false)
}

func (s *Cmd) NewStderr() (*os.File, error) {
	return s.newPs(s.Stderr, false)
}
