/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mqttproxy

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/cluster"
	etcderror "go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
)

type (
	storage interface {
		get(key string) (*string, error)
		getPrefix(prefix string, keysOnly bool) (map[string]string, error)
		put(key, value string) error
		delete(key string) error
		watchDelete(prefix string) (<-chan map[string]*string, func(), error)
	}

	mockStorage struct {
		mu        sync.RWMutex
		store     map[string]string
		watchCh   chan map[string]*string
		watchFlag int32
	}

	clusterStorage struct {
		cls cluster.Cluster
	}
)

var (
	_ storage = (*mockStorage)(nil)
	_ storage = (*clusterStorage)(nil)
)

func newStorage(cls cluster.Cluster) storage {
	if cls != nil {
		return &clusterStorage{
			cls: cls,
		}
	}
	return &mockStorage{
		store:   make(map[string]string),
		watchCh: make(chan map[string]*string),
	}
}

func (m *mockStorage) get(key string) (*string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if val, ok := m.store[key]; ok {
		return &val, nil
	}
	return nil, etcderror.ErrKeyNotFound
}

func (m *mockStorage) getPrefix(prefix string, keysOnly bool) (map[string]string, error) {
	m.mu.RLock()
	out := make(map[string]string)
	for k, v := range m.store {
		if strings.HasPrefix(k, prefix) {
			if keysOnly {
				out[k] = ""
			} else {
				out[k] = v
			}
		}
	}
	m.mu.RUnlock()
	return out, nil
}

func (m *mockStorage) put(key, value string) error {
	m.mu.Lock()
	m.store[key] = value
	m.mu.Unlock()
	return nil
}

func (m *mockStorage) delete(key string) error {
	m.mu.Lock()
	delete(m.store, key)
	if m.watched() {
		go func() {
			ans := make(map[string]*string)
			ans[key] = nil
			m.watchCh <- ans
		}()
	}
	m.mu.Unlock()
	return nil
}

func (m *mockStorage) watched() bool {
	flag := atomic.LoadInt32(&m.watchFlag)
	return flag == 1
}

func (m *mockStorage) watchDelete(prefix string) (<-chan map[string]*string, func(), error) {
	atomic.StoreInt32(&m.watchFlag, 1)
	return m.watchCh, func() {}, nil
}

func (cs *clusterStorage) get(key string) (*string, error) {
	return cs.cls.Get(key)
}

func (cs *clusterStorage) getPrefix(prefix string, keysOnly bool) (map[string]string, error) {
	if keysOnly {
		return cs.cls.GetWithOp(prefix, cluster.OpPrefix, cluster.OpKeysOnly)
	}
	return cs.cls.GetPrefix(prefix)
}

func (cs *clusterStorage) put(key, value string) error {
	return cs.cls.Put(key, value)
}

func (cs *clusterStorage) delete(key string) error {
	return cs.cls.Delete(key)
}

func (cs *clusterStorage) watchDelete(prefix string) (<-chan map[string]*string, func(), error) {
	watcher, err := cs.cls.Watcher()
	if err != nil {
		return nil, nil, err
	}
	ch, err := watcher.WatchWithOp(prefix, cluster.OpPrefix, cluster.OpNotWatchPut)
	if err != nil {
		watcher.Close()
		return nil, nil, err
	}
	return ch, watcher.Close, nil
}
