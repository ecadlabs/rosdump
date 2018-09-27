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

	"github.com/ecadlabs/rosdump/config"
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

type fileWriter struct {
	io.Writer
	fd  *os.File
	zfd *gzip.Writer
}

func (f *fileWriter) Close() error {
	if f.zfd != nil {
		if err := f.zfd.Close(); err != nil {
			return err
		}
	}

	return f.fd.Close()
}

func (f *fileStorageTx) Add(ctx context.Context, metadata map[string]interface{}) (io.WriteCloser, error) {
	var outPath strings.Builder
	if err := f.f.pathTpl.Execute(&outPath, metadata); err != nil {
		return nil, err
	}

	dir := path.Dir(outPath.String())
	if err := os.MkdirAll(dir, 0777); err != nil {
		return nil, err
	}

	f.f.logger.WithFields(logrus.Fields{
		"file":       outPath.String(),
		"compressed": f.f.compress,
	}).Infoln("writing...")

	fd, err := os.Create(outPath.String())
	if err != nil {
		return nil, err
	}

	res := fileWriter{
		Writer: fd,
		fd:     fd,
	}

	if f.f.compress {
		zfd := gzip.NewWriter(fd)

		res.Writer = zfd
		res.zfd = zfd
	}

	return &res, err
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

func newFileStorage(ctx context.Context, options config.Options, logger *logrus.Logger) (Storage, error) {
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
