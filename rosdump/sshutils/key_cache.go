package sshutils

import (
	"io/ioutil"
	"sync"
)

type fileCache struct {
	data map[string][]byte
	m    sync.Mutex
}

var idCache = fileCache{
	data: make(map[string][]byte),
}

func ReadIdentityFile(name string) ([]byte, error) {
	idCache.m.Lock()
	defer idCache.m.Unlock()

	if data, ok := idCache.data[name]; ok {
		return data, nil
	}

	data, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}

	idCache.data[name] = data
	return data, nil
}
