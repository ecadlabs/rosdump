package sshutils

/*
import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type scpFileInfo struct {
	name string
	perm uint32
	size int64
}

func (s *scpFileInfo) Name() string       { return s.name }
func (s *scpFileInfo) Size() int64        { return s.size }
func (s *scpFileInfo) Mode() os.FileMode  { return os.FileMode(s.perm) }
func (s *scpFileInfo) ModTime() time.Time { return time.Time{} }
func (s *scpFileInfo) IsDir() bool        { return false }
func (s *scpFileInfo) Sys() interface{}   { return nil }

func SCPGet(ctx context.Context, client *ssh.Client, name string) (os.FileInfo, io.ReadCloser, error) {
	session, err := NewSession(ctx, client)
	if err != nil {
		return nil, nil, err
	}

	var (
		hdr       *scpFileInfo
		cancelled bool
	)

	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, nil, err
	}

	rd := bufio.NewReader(stdout)
	ch := make(chan struct{})

	go func() {
		defer func() {
			close(ch)

			// Close timeouted session in the background
			if cancelled || err != nil {
				session.Close()
			}
		}()

		if err = session.Start("scp -f " + name); err != nil {
			return
		}

		var ackByte [1]byte

		if _, err = stdin.Write(ackByte[:]); err != nil {
			return
		}

		var code byte
		if code, err = rd.ReadByte(); err != nil {
			return
		}

		var headerStr string
		if headerStr, err = rd.ReadString('\n'); err != nil {
			return
		}

		if code == 1 {
			// Error response
			err = fmt.Errorf("scp: %s", headerStr)
			return
		} else if code != 'C' {
			err = fmt.Errorf("scp: unknown response: %c%s", code, headerStr)
			return
		}

		f := strings.Fields(headerStr)
		if len(f) != 3 {
			err = fmt.Errorf("scp: unknown response header: %s", headerStr)
			return
		}

		// Mode
		var perm uint64
		if perm, err = strconv.ParseUint(f[0], 8, 32); err != nil {
			return
		}

		// Size
		var sz int64
		if sz, err = strconv.ParseInt(f[1], 10, 64); err != nil {
			return
		}

		hdr = &scpFileInfo{
			size: sz,
			perm: uint32(perm),
			name: f[2],
		}

		// Ready to receive
		_, err = stdin.Write(ackByte[:])
	}()

	select {
	case <-ctx.Done():
		cancelled = true
		return nil, nil, ctx.Err()

	case <-ch:
		if err != nil {
			return nil, nil, err
		}
	}

	res := sshSessionResponse{
		Reader:  io.LimitReader(rd, hdr.size),
		session: session,
		closer:  stdin,
		rd:      rd,
	}

	return hdr, &res, nil
}

type sshSessionResponse struct {
	io.Reader
	session *ssh.Session
	closer  io.Closer
	rd      io.Reader
}

func (s *sshSessionResponse) Close() error {
	// Read acknowledge byte
	var ackByte [1]byte
	if _, err := s.rd.Read(ackByte[:]); err != nil {
		return err
	}

	if err := s.closer.Close(); err != nil {
		return err
	}

	if err := s.session.Wait(); err != nil {
		return err
	}

	if err := s.session.Close(); err != nil && err != io.EOF {
		return err
	}

	return nil
}
*/
