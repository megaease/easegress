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

package cluster

import (
	"bytes"
	"context"
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/megaease/easegress/pkg/logger"
)

// Syncer syncs data from Etcd, it uses an Etcd watcher to receive update.
// The syncer keeps a full copy of data, and keeps apply changes onto it when an
// update event is received from the watcher, and then send out the full data copy.
// The syncer also pulls full data from Etcd at a configurable pull interval, this
// is to ensure data consistency, as Etcd watcher may be cancelled if it cannot catch
// up with the key-value store.
type Syncer struct {
	sync.WaitGroup
	cluster      *cluster
	client       *clientv3.Client
	pullInterval time.Duration
	done         chan struct{}
}

func (c *cluster) Syncer(pullInterval time.Duration) (*Syncer, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return &Syncer{
		WaitGroup:    sync.WaitGroup{},
		cluster:      c,
		client:       client,
		pullInterval: pullInterval,
		done:         make(chan struct{}),
	}, nil
}

func (s *Syncer) pull(key string, prefix bool) (map[string]*mvccpb.KeyValue, error) {
	if prefix {
		result, err := s.cluster.GetRawPrefix(key)
		if err != nil {
			logger.Errorf("failed to pull data for prefix %s: %v", key, err)
		}
		return result, err
	}

	kv, err := s.cluster.GetRaw(key)
	if err != nil {
		logger.Errorf("failed to pull data for key %s: %v", key, err)
		return nil, err
	}

	result := make(map[string]*mvccpb.KeyValue)
	if kv != nil {
		result[string(kv.Key)] = kv
	}

	return result, nil
}

func (s *Syncer) watch(key string, prefix bool) (clientv3.Watcher, clientv3.WatchChan) {
	opts := make([]clientv3.OpOption, 0, 1)
	if prefix {
		opts = append(opts, clientv3.WithPrefix())
	}
	watcher := clientv3.NewWatcher(s.client)
	watchChan := watcher.Watch(context.Background(), key, opts...)
	logger.Debugf("watcher created for key %s (prefix: %v)", key, prefix)
	return watcher, watchChan
}

func isDataEqual(data1 map[string]*mvccpb.KeyValue, data2 map[string]*mvccpb.KeyValue) bool {
	if len(data1) != len(data2) {
		return false
	}

	for k1, kv1 := range data1 {
		kv2, exists := data2[k1]
		if !exists {
			return false
		}

		if !isKeyValueEqual(kv1, kv2) {
			return false
		}
	}

	return true
}

func isKeyValueEqual(kv1, kv2 *mvccpb.KeyValue) bool {
	switch {
	case kv1 == nil && kv2 == nil:
		return true
	case kv1 == nil && kv2 != nil:
		return false
	case kv1 != nil && kv2 == nil:
		return false
	case kv1 != nil && kv2 != nil:
		return bytes.Equal(kv1.Key, kv2.Key) && bytes.Equal(kv1.Value, kv2.Value)
	}

	return false
}

func (s *Syncer) run(key string, prefix bool, send func(data map[string]*mvccpb.KeyValue)) {
	watcher, watchChan := s.watch(key, prefix)
	defer watcher.Close()

	ticker := time.NewTicker(s.pullInterval)
	defer ticker.Stop()

	data := make(map[string]*mvccpb.KeyValue)

	pullCompareSend := func() {
		newData, err := s.pull(key, prefix)
		if err != nil {
			logger.Errorf("pull data for key %s (prefix: %v) failed: %v", key, prefix, err)
			return
		}
		if !isDataEqual(data, newData) {
			data = newData
			send(data)
		}
	}

	pullCompareSend()

	for {
		select {
		case <-s.done:
			return

		case <-ticker.C:
			pullCompareSend()

		case resp := <-watchChan:
			if resp.Canceled {
				// Etcd cancels a watcher when it cannot catch up with the progress of
				// the key-value store. And no matter what happens, we restart the watcher.
				logger.Debugf("watch key %s canceled: %v", key, resp.Err())
				watcher.Close()
				watcher, watchChan = s.watch(key, prefix)
				continue
			}
			if resp.IsProgressNotify() {
				continue
			}

			pullCompareSend()
		}
	}
}

// Sync syncs a given Etcd key's value through the returned channel.
func (s *Syncer) Sync(key string) (<-chan *string, error) {
	ch := make(chan *string, 10)

	fn := func(data map[string]*mvccpb.KeyValue) {
		if kv := data[key]; kv == nil {
			ch <- nil
		} else {
			value := string(kv.Value)
			ch <- &value
		}
	}

	s.Add(1)
	go func() {
		defer s.Done()
		defer close(ch)
		s.run(key, false, fn)
	}()

	return ch, nil
}

// SyncRaw syncs a given Etcd key's raw Etcd mvccpb structure through the returned channel.
func (s *Syncer) SyncRaw(key string) (<-chan *mvccpb.KeyValue, error) {
	ch := make(chan *mvccpb.KeyValue, 10)

	fn := func(data map[string]*mvccpb.KeyValue) {
		ch <- data[key]
	}

	s.Add(1)
	go func() {
		defer s.Done()
		defer close(ch)
		s.run(key, false, fn)
	}()

	return ch, nil
}

// SyncPrefix syncs Etcd keys' values with the same prefix through the returned channel.
func (s *Syncer) SyncPrefix(prefix string) (<-chan map[string]string, error) {
	ch := make(chan map[string]string, 10)

	fn := func(data map[string]*mvccpb.KeyValue) {
		m := make(map[string]string, len(data))
		for k, v := range data {
			m[k] = string(v.Value)
		}
		ch <- m
	}

	s.Add(1)
	go func() {
		defer s.Done()
		defer close(ch)
		s.run(prefix, true, fn)
	}()

	return ch, nil
}

// SyncRawPrefix syncs Etcd keys' values with the same prefix in raw Etcd mvccpb structure format through the returned channel.
func (s *Syncer) SyncRawPrefix(prefix string) (<-chan map[string]*mvccpb.KeyValue, error) {
	ch := make(chan map[string]*mvccpb.KeyValue, 10)

	fn := func(data map[string]*mvccpb.KeyValue) {
		// make a copy of data as it may be modified after the function returns
		m := make(map[string]*mvccpb.KeyValue, len(data))
		for k, v := range data {
			m[k] = v
		}
		ch <- m
	}

	s.Add(1)
	go func() {
		defer s.Done()
		defer close(ch)
		s.run(prefix, true, fn)
	}()

	return ch, nil
}

// Close closes the syncer.
func (s *Syncer) Close() {
	close(s.done)
	s.Wait()
}
