package storage

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"text/template"
	"time"

	"git.ecadlabs.com/ecad/rostools/rosdump/config"
	"github.com/sirupsen/logrus"
)

type FileStorage struct {
	pathTpl  *template.Template
	compress bool
	logger   *logrus.Logger
}

type fileStorageTx struct {
	f         *FileStorage
	timestamp time.Time
}

func (f *FileStorage) Begin(ctx context.Context) (Tx, error) {
	return &fileStorageTx{
		f:         f,
		timestamp: time.Now(),
	}, nil
}

func (f *fileStorageTx) Add(ctx context.Context, metadata map[string]interface{}, stream io.Reader) (err error) {
	var outPath strings.Builder
	if err = f.f.pathTpl.Execute(&outPath, metadata); err != nil {
		return err
	}

	dir := path.Dir(outPath.String())
	if err = os.MkdirAll(dir, 0777); err != nil {
		return err
	}

	f.f.logger.WithFields(logrus.Fields{
		"file":       outPath.String(),
		"compressed": f.f.compress,
	}).Infoln("writing...")

	fd, err := os.Create(outPath.String())
	if err != nil {
		return err
	}

	defer func() {
		e := fd.Close()
		if err == nil {
			err = e
		}
	}()

	var outFd io.Writer

	if f.f.compress {
		zfd := gzip.NewWriter(fd)
		defer func() {
			e := zfd.Close()
			if err == nil {
				err = e
			}
		}()

		outFd = zfd
	} else {
		outFd = fd
	}

	_, err = io.Copy(outFd, stream)
	return err
}

func (f *fileStorageTx) Timestamp() time.Time { return f.timestamp }

func (f *fileStorageTx) Commit(ctx context.Context) error { return nil }

func NewFileStorage(pathTpl string, compress bool, logger *logrus.Logger) (*FileStorage, error) {
	tpl, err := template.New("path").Parse(pathTpl)
	if err != nil {
		return nil, err
	}

	return &FileStorage{
		pathTpl:  tpl,
		compress: compress,
		logger:   logger,
	}, nil
}

func newFileStorage(options config.Options, logger *logrus.Logger) (Storage, error) {
	path, err := options.GetString("path")
	if err != nil {
		return nil, errors.New("file: path is not specified")
	}

	compress, _ := options.GetBool("compress")

	return NewFileStorage(path, compress, logger)
}

func init() {
	registerStorage("file", newFileStorage)
}
