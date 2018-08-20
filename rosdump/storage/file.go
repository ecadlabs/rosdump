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

	"github.com/e-asphyx/rosdump/config"
)

type FileStorage struct {
	pathTpl  *template.Template
	compress bool
}

func (f *FileStorage) Begin(ctx context.Context) (Tx, error) {
	return f, nil
}

func (f *FileStorage) Add(ctx context.Context, metadata map[string]interface{}, stream io.Reader) (err error) {
	var outPath strings.Builder
	if err = f.pathTpl.Execute(&outPath, metadata); err != nil {
		return err
	}

	dir := path.Dir(outPath.String())
	if err = os.MkdirAll(dir, 0777); err != nil {
		return err
	}

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

	if f.compress {
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

func (f *FileStorage) Commit(ctx context.Context) error { return nil }

func NewFileStorage(pathTpl string, compress bool) (*FileStorage, error) {
	tpl, err := template.New("path").Parse(pathTpl)
	if err != nil {
		return nil, err
	}

	return &FileStorage{
		pathTpl:  tpl,
		compress: compress,
	}, nil
}

func newFileStorage(options config.Options) (Storage, error) {
	path, err := options.GetString("path")
	if err != nil {
		return nil, errors.New("file: path is not specified")
	}

	compress, _ := options.GetBool("compress")

	return NewFileStorage(path, compress)
}

func init() {
	registerStorage("file", newFileStorage)
}
