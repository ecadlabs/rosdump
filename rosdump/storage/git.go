package storage

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type GitStorageConfig struct {
	Repository    string
	Destination   string
	Name          string
	Email         string
	CommitMessage string
	Logger        *logrus.Logger
}

type GitStorage struct {
	repo    *git.Repository
	conf    *GitStorageConfig
	destTpl *template.Template
	msgTpl  *template.Template
	mtx     sync.Mutex
}

func NewGitStorage(conf *GitStorageConfig) (*GitStorage, error) {
	repo, err := git.PlainOpen(conf.Repository)
	if err != nil {
		return nil, fmt.Errorf("git: %v", err)
	}

	destTpl, err := template.New("destination").Parse(conf.Destination)
	if err != nil {
		return nil, fmt.Errorf("git: %v", err)
	}

	msgTpl, err := template.New("message").Parse(conf.CommitMessage)
	if err != nil {
		return nil, fmt.Errorf("git: %v", err)
	}

	return &GitStorage{
		repo:    repo,
		conf:    conf,
		destTpl: destTpl,
		msgTpl:  msgTpl,
	}, nil
}

type gitStorageTx struct {
	wt        *git.Worktree
	g         *GitStorage
	timestamp time.Time
}

func (g *GitStorage) Begin(ctx context.Context) (Tx, error) {
	wt, err := g.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("git: %v", err)
	}

	return &gitStorageTx{
		g:         g,
		wt:        wt,
		timestamp: time.Now(),
	}, nil
}

func (g *gitStorageTx) Add(ctx context.Context, metadata map[string]interface{}, stream io.Reader) error {
	var dest strings.Builder
	if err := g.g.destTpl.Execute(&dest, metadata); err != nil {
		return fmt.Errorf("git: %v", err)
	}

	out := dest.String()

	g.g.mtx.Lock()
	defer g.g.mtx.Unlock()

	// Use underlying FS abstraction
	fs := g.wt.Filesystem

	dir := path.Dir(out)
	if err := fs.MkdirAll(dir, 0777); err != nil {
		return fmt.Errorf("git: %v", err)
	}

	g.g.conf.Logger.WithField("file", out).Infoln("writing...")

	fd, err := fs.Create(out)
	if err != nil {
		return fmt.Errorf("git: %v", err)
	}

	if _, err := io.Copy(fd, stream); err != nil {
		return fmt.Errorf("git: %v", err)
	}

	if err := fd.Close(); err != nil {
		return fmt.Errorf("git: %v", err)
	}

	if _, err := g.wt.Add(out); err != nil {
		return fmt.Errorf("git: %v", err)
	}

	return nil
}

func (g *gitStorageTx) Timestamp() time.Time { return g.timestamp }

func (g *gitStorageTx) Commit(ctx context.Context) error {
	tdata := map[string]interface{}{
		"time": g.timestamp,
	}

	var msg strings.Builder
	if err := g.g.msgTpl.Execute(&msg, tdata); err != nil {
		return fmt.Errorf("git: %v", err)
	}

	g.g.mtx.Lock()
	defer g.g.mtx.Unlock()

	commit, err := g.wt.Commit(msg.String(), &git.CommitOptions{
		Author: &object.Signature{
			Name:  g.g.conf.Name,
			Email: g.g.conf.Email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("git: %v", err)
	}

	g.g.conf.Logger.WithFields(logrus.Fields{
		"hash": commit.String(),
		"msg":  msg.String(),
	}).Infoln("commit object created")

	obj, err := g.g.repo.CommitObject(commit)
	if err != nil {
		return fmt.Errorf("git: %v", err)
	}

	g.g.conf.Logger.Infoln(obj)

	return nil
}
