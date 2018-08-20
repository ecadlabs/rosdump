package devices

import (
	"bufio"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"sync"
	"time"

	"github.com/e-asphyx/rosdump/config"
	"github.com/e-asphyx/rosdump/sshutils"
	"golang.org/x/crypto/ssh"
)

type SSHCommand struct {
	KeyFunc  sshutils.KeyFunc
	Host     string
	Port     string
	Username string
	Password string
	Command  string
}

type sshCommandResponse struct {
	io.Reader
	session *ssh.Session
	client  *sshutils.Client
}

type readNotifier struct {
	rd         io.Reader
	done       chan<- struct{}
	pendingErr <-chan error
	closed     bool
	err        error
	m          sync.Mutex
}

func (r *readNotifier) close(err error) {
	r.m.Lock()
	defer r.m.Unlock()

	if !r.closed {
		r.err = err
		r.closed = true
		close(r.done)
	}
}

func (r *readNotifier) Read(p []byte) (int, error) {
	r.m.Lock()
	if r.closed {
		defer r.m.Unlock()
		return 0, r.err
	}
	r.m.Unlock()

	n, err := r.rd.Read(p)
	if err != nil {
		select {
		case err = <-r.pendingErr:
		default:
		}

		r.close(err)
	}
	return n, err
}

func (s *sshCommandResponse) Close() (err error) {
	defer func() {
		e := s.client.Close()
		if err == nil {
			err = e
		}
		s.Reader.(*readNotifier).close(io.EOF)
	}()

	if err := s.session.Wait(); err != nil {
		return err
	}

	if err := s.session.Close(); err != nil && err != io.EOF {
		return err
	}

	return nil
}

func (s *SSHCommand) Export(ctx context.Context) (response io.ReadCloser, err error) {
	sshConfig := sshutils.Config{
		KeyFunc:  s.KeyFunc,
		Username: s.Username,
		Password: s.Password,
	}

	port := s.Port
	if port == "" {
		port = "22"
	}

	address := net.JoinHostPort(s.Host, port)

	client, err := sshutils.Dial(ctx, address, &sshConfig)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			client.Close()
		}
	}()

	if d, ok := ctx.Deadline(); ok {
		client.SetDeadline(d)
	}

	now := time.Now()
	readDone := make(chan struct{})

	defer func() {
		if err != nil {
			close(readDone)
		}
	}()

	pendingErr := make(chan error, 1)

	go func() {
		select {
		case <-ctx.Done():
			pendingErr <- ctx.Err()
			client.SetDeadline(now)
		case <-readDone:
		}
	}()

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err = session.Start(s.Command); err != nil {
		return nil, err
	}

	rn := readNotifier{
		rd:         bufio.NewReader(stdout),
		done:       readDone,
		pendingErr: pendingErr,
	}

	res := sshCommandResponse{
		Reader:  &rn,
		client:  client,
		session: session,
	}

	return &res, nil
}

var idFileCache sync.Map

func newSSHCommand(options config.Options) (Exporter, error) {
	var cmd SSHCommand

	cmd.Host, _ = options.GetString("address")
	cmd.Port, _ = options.GetString("port")
	cmd.Username, _ = options.GetString("username")
	cmd.Password, _ = options.GetString("password")
	cmd.Command, _ = options.GetString("command")

	if keyFile, err := options.GetString("identity_file"); err != nil && keyFile != "" {
		var keyData []byte

		if val, ok := idFileCache.Load(keyFile); ok {
			keyData = val.([]byte)
		} else {
			keyData, err = ioutil.ReadFile(keyFile)
			if err != nil {
				return nil, err
			}

			if val, ok := idFileCache.LoadOrStore(keyFile, keyData); ok {
				keyData = val.([]byte)
			}
		}

		cmd.KeyFunc = func() ([]byte, error) {
			return keyData, nil
		}
	}

	if cmd.Host == "" {
		return nil, errors.New("ssh-command: address missing")
	}

	if cmd.Username == "" {
		return nil, errors.New("ssh-command: username missing")
	}

	if cmd.Command == "" {
		return nil, errors.New("ssh-command: command missing")
	}

	return &cmd, nil
}

func init() {
	registerExporter("ssh-command", newSSHCommand)
}
