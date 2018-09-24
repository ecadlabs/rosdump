package devices

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/ecadlabs/rosdump/config"
	"github.com/ecadlabs/rosdump/sshutils"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type SSHCommand struct {
	KeyFunc        sshutils.KeyFunc
	Name           string
	Host           string
	Port           string
	Username       string
	Password       string
	Command        string
	Logger         *logrus.Logger
	ExportMetadata Metadata
	DeviceMetadata Metadata
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

func (s *SSHCommand) Export(ctx context.Context) (response io.ReadCloser, metadata Metadata, err error) {
	sshConfig := sshutils.Config{
		KeyFunc:  s.KeyFunc,
		Username: s.Username,
		Password: s.Password,
	}

	port := s.Port
	if port == "" {
		port = "22"
	}

	command := s.Command
	if command == "" {
		command = "export"
	}

	address := net.JoinHostPort(s.Host, port)

	l := s.Logger.WithFields(logrus.Fields{
		"name":    s.Name,
		"address": address,
	})

	l.Info("establishing SSH connection...")

	client, err := sshutils.Dial(ctx, address, &sshConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("ssh-command: %v", err)
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
		return nil, nil, fmt.Errorf("ssh-command: new session: %v", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	l.Infof("issuing `%s' command...", command)

	if err = session.Start(command); err != nil {
		return nil, nil, fmt.Errorf("ssh-command: session start: %v", err)
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

	return &res, s.ExportMetadata, nil
}

func (s *SSHCommand) Metadata() Metadata {
	return s.DeviceMetadata
}

func newSSHCommand(options config.Options, logger *logrus.Logger) (Exporter, error) {
	cmd := SSHCommand{
		Logger: logger,
	}

	cmd.Name, _ = options.GetString("name")
	cmd.Host, _ = options.GetString("host")
	cmd.Port, _ = options.GetString("port")
	cmd.Username, _ = options.GetString("username")
	cmd.Password, _ = options.GetString("password")
	cmd.Command, _ = options.GetString("command")

	if cmd.Host == "" {
		return nil, errors.New("ssh-command: address missing")
	}

	if cmd.Username == "" {
		return nil, errors.New("ssh-command: user name missing")
	}

	/*
		if cmd.Command == "" {
			return nil, errors.New("ssh-command: command missing")
		}
	*/

	if keyFile, err := options.GetString("identity_file"); err == nil && keyFile != "" {
		keyData, err := sshutils.ReadIdentityFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("ssh-command: %v", err)
		}

		cmd.KeyFunc = func() ([]byte, error) {
			return keyData, nil
		}
	}

	// Filter out password
	metadata := make(Metadata, len(options))
	for k, v := range options {
		if k != "password" {
			metadata[k] = v
		}
	}

	cmd.ExportMetadata = metadata
	cmd.DeviceMetadata = Metadata{
		"name":   cmd.Name,
		"host":   cmd.Host,
		"device": "ssh-command",
	}

	return &cmd, nil
}

func init() {
	registerExporter("ssh-command", newSSHCommand)
}
