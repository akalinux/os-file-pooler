package cmd

// https://stackoverflow.com/questions/23031752/start-a-process-in-go-and-detach-from-it
// https://cs.opensource.google/go/go/+/refs/tags/go1.25.5:src/os/exec/exec.go
import (
	"errors"
	"os"
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
	dir, err := os.Getwd()
	if err != nil {
		dir = "."
	}
	return &Cmd{
		Name:        name,
		Args:        args,
		SysProcAttr: DefaultSysProcAttr(),
		Dir:         dir,

		Env:    os.Environ(),
		Stdin:  &RwSet{},
		Stdout: &RwSet{},
		Stderr: &RwSet{},
	}
}

func (s *Cmd) OsProcAttr() *os.ProcAttr {

	attr := &os.ProcAttr{
		Dir: s.Dir,
		Env: s.Env,
		Files: []*os.File{
			s.Stdin.Read,
			s.Stdout.Write,
			s.Stderr.Write,
		},
		Sys: s.SysProcAttr,
	}

	return attr
}

func (s *Cmd) Start() (*os.Process, error) {
	return os.StartProcess(s.Name, s.Args, s.OsProcAttr())
}

func (s *Cmd) CloseFd() {
	if s.Stdin.Write != nil {
		s.Stdin.Write.Close()
	}
	if s.Stdout.Write != nil {
		s.Stdout.Write.Close()
	}
	if s.Stderr.Write != nil {
		s.Stderr.Write.Close()
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

func DefaultCreds() *syscall.Credential {
	glist, err := os.Getgroups()

	groups := make([]uint32, len(glist))
	if err != nil {
		for i, group := range glist {
			groups[i] = uint32(group)
		}
	}

	return &syscall.Credential{
		Uid:    uint32(os.Getegid()),
		Gid:    uint32(os.Getegid()),
		Groups: groups,
	}
}

// Generates the our default SysProcAttr.
func DefaultSysProcAttr() *syscall.SysProcAttr {

	return &syscall.SysProcAttr{
		Credential: DefaultCreds(),
		Setsid:     true, // detach by default
		Pdeathsig:  syscall.SIGTERM,
		Noctty:     true,
	}
}
