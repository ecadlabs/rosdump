package filter

import (
	"bufio"
	"fmt"
	"io"
	"regexp"

	"github.com/ecadlabs/rosdump/config"
	"github.com/sirupsen/logrus"
)

type Regexp struct {
	Regexp  *regexp.Regexp
	Replace string
	Logger  *logrus.Logger
}

func (r *Regexp) Start(dst io.WriteCloser, src io.Reader) error {
	go func() {
		s := bufio.NewScanner(src)

		for s.Scan() {
			str := r.Regexp.ReplaceAllString(s.Text(), r.Replace)
			if _, err := fmt.Fprintln(dst, str); err != nil {
				r.Logger.Errorf("regexp: %v", err)
			}
		}

		if err := s.Err(); err != nil {
			if closer, ok := dst.(closerWithError); ok {
				// Propagate error
				if e := closer.CloseWithError(err); e != nil {
					r.Logger.Errorf("regexp: %v", e)
				}
				return
			}
		}

		if err := dst.Close(); err != nil {
			r.Logger.Errorf("regexp: %v", err)
		}
	}()

	return nil
}

func newRegexpFilter(options config.Options, logger *logrus.Logger) (Filter, error) {
	expr, _ := options.GetString("expr")
	var err error

	re := Regexp{
		Logger: logger,
	}

	re.Regexp, err = regexp.Compile(expr)
	if err != nil {
		return nil, err
	}

	re.Replace, _ = options.GetString("replace")

	return &re, nil
}

func init() {
	registerFilter("regexp", newRegexpFilter)
}
