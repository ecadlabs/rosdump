package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"strconv"

	"gopkg.in/yaml.v2"
)

const configVersion = "1"

type Devices struct {
	List   []Options `yaml:"list"`
	Common Options   `yaml:"common"`
}

type Config struct {
	Version       string    `yaml:"version"`
	Timeout       string    `yaml:"timeout"`
	MaxGoroutines int       `yaml:"max_goroutines"`
	Interval      string    `yaml:"interval"`
	Devices       Devices   `yaml:"devices"`
	Storage       Options   `yaml:"storage"`
	Filters       []*Filter `yaml:"filters"`
}

type Filter struct {
	Filter  string  `yaml:"filter"`
	Name    string  `yaml:"name"`
	Options Options `yaml:"options"`
}

type Options map[string]interface{}

var ErrOptNotFound = errors.New("Option not found")

func (o Options) GetString(name string) (string, error) {
	v, ok := o[name]
	if !ok {
		return "", ErrOptNotFound
	}

	switch vv := v.(type) {
	case string:
		return vv, nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

func getInt(v interface{}) int64 {
	val := reflect.ValueOf(v)

	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int()

	case reflect.Float32, reflect.Float64:
		return int64(val.Float())
	}

	return 0
}

func (o Options) GetInt(name string) (int64, error) {
	v, ok := o[name]
	if !ok {
		return 0, ErrOptNotFound
	}

	if s, ok := v.(string); ok {
		return strconv.ParseInt(s, 0, 64)
	}

	return getInt(v), nil
}

func (o Options) GetBool(name string) (bool, error) {
	v, ok := o[name]
	if !ok {
		return false, ErrOptNotFound
	}

	switch vv := v.(type) {
	case bool:
		return vv, nil
	case string:
		return strconv.ParseBool(vv)
	default:
		if getInt(v) != 0 {
			return true, nil
		}
		return false, nil
	}
}

func Load(name string) (*Config, error) {
	buf, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}

	var c Config
	if err := yaml.Unmarshal(buf, &c); err != nil {
		return nil, err
	}

	if c.Version != configVersion {
		return nil, fmt.Errorf("Unknown config version: `%s'", c.Version)
	}

	return &c, nil
}
