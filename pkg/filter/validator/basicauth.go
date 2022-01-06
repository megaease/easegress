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

package validator

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"

	"github.com/megaease/easegress/pkg/cluster"
	httpcontext "github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/httpheader"
)

type (
	// BasicAuthValidatorSpec defines the configuration of Basic Auth validator.
	// Only one of UserFile or UseEtcd should be defined.
	BasicAuthValidatorSpec struct {
		// UserFile is path to file containing encrypted user credentials in apache2-utils/htpasswd format.
		// To add user `userY`, use `sudo htpasswd /etc/apache2/.htpasswd userY`
		// Reference: https://manpages.debian.org/testing/apache2-utils/htpasswd.1.en.html#EXAMPLES
		UserFile string `yaml:"userFile" jsonschema:"omitempty"`
		// When UseEtcd is true, verify user credentials from etcd. Etcd stores them:
		// key: /credentials/{user id}
		// value: {encrypted password}
		UseEtcd bool `yaml:"useEtcd" jsonschema:"omitempty"`
	}

	// AuthorizedUsersCache provides cached lookup for authorized users.
	AuthorizedUsersCache interface {
		GetUser(string) (string, bool)
		WatchChanges() error
		Close()
		Lock() // for testing purposes
		Unlock()
	}

	htpasswdUserCache struct {
		cache        *lru.Cache
		userFile     string
		fileMutex    sync.RWMutex
		syncInterval time.Duration
		cancel       context.CancelFunc
	}

	etcdUserCache struct {
		cache        *lru.Cache
		cluster      cluster.Cluster
		syncInterval time.Duration
		cancel       context.CancelFunc
	}

	// BasicAuthValidator defines the Basic Auth validator
	BasicAuthValidator struct {
		spec                 *BasicAuthValidatorSpec
		authorizedUsersCache AuthorizedUsersCache
	}
)

const credsPrefix = "/credentials/"

func parseCredentials(creds string) (string, string, error) {
	parts := strings.Split(creds, ":")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("bad format")
	}
	return parts[0], parts[1], nil
}

func readPasswordFile(userFile string) (map[string]string, error) {
	credentials := make(map[string]string)
	file, err := os.OpenFile(userFile, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return credentials, err
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := sc.Text()
		userID, password, err := parseCredentials(line)
		if err != nil {
			return credentials, err
		}
		credentials[userID] = password
	}
	return credentials, nil
}

func newHtpasswdUserCache(userFile string, syncInterval time.Duration) *htpasswdUserCache {
	cache, err := lru.New(256)
	if err != nil {
		panic(err)
	}
	return &htpasswdUserCache{
		cache:    cache,
		userFile: userFile,
		// Removed access or updated passwords are updated according syncInterval.
		syncInterval: syncInterval,
	}
}

func (huc *htpasswdUserCache) GetUser(targetUserID string) (string, bool) {
	if val, ok := huc.cache.Get(targetUserID); ok {
		return val.(string), true
	}

	huc.fileMutex.RLock()
	credentials, err := readPasswordFile(huc.userFile)
	huc.fileMutex.RUnlock()
	if err != nil {
		logger.Errorf(err.Error())
		return "", false
	}
	if password, ok := credentials[targetUserID]; ok {
		huc.cache.Add(targetUserID, password)
		return password, true
	}

	logger.Errorf("unauthorized")
	return "", false
}

func (huc *htpasswdUserCache) WatchChanges() error {
	stopCtx, cancel := context.WithCancel(context.Background())
	huc.cancel = cancel

	getFileStat := func() (fs.FileInfo, error) {
		huc.fileMutex.RLock()
		stat, err := os.Stat(huc.userFile)
		huc.fileMutex.RUnlock()
		return stat, err
	}

	initialStat, err := getFileStat()
	if err != nil {
		return err
	}
	for {
		stat, err := getFileStat()
		if err != nil {
			return err
		}
		if stat.Size() != initialStat.Size() || stat.ModTime() != initialStat.ModTime() {
			huc.fileMutex.RLock()
			credentials, err := readPasswordFile(huc.userFile)
			huc.fileMutex.RUnlock()
			if err != nil {
				return err
			}
			updateCache(credentials, huc.cache)

			initialStat, err = getFileStat()
			if err != nil {
				return err
			}
		}
		select {
		case <-time.After(huc.syncInterval):
			continue
		case <-stopCtx.Done():
			return nil
		}
	}
	return nil
}

func (huc *htpasswdUserCache) Close() {
	if huc.cancel != nil {
		huc.cancel()
	}
}

func (huc *htpasswdUserCache) Lock() {
	huc.fileMutex.Lock()
}

func (huc *htpasswdUserCache) Unlock() {
	huc.fileMutex.Unlock()
}

func newEtcdUserCache(cluster cluster.Cluster) *etcdUserCache {
	cache, err := lru.New(256)
	if err != nil {
		panic(err)
	}
	return &etcdUserCache{
		cache:   cache,
		cluster: cluster,
		// cluster.Syncer updates changes (removed access or updated passwords) immediately.
		// syncInterval defines data consistency check interval.
		syncInterval: 30 * time.Minute,
	}
}

func (euc *etcdUserCache) GetUser(targetUserID string) (string, bool) {
	if val, ok := euc.cache.Get(targetUserID); ok {
		return val.(string), true
	}

	password, err := euc.cluster.Get(credsPrefix + targetUserID)
	if err != nil {
		logger.Errorf(err.Error())
		return "", false
	}
	if password == nil {
		logger.Errorf("unauthorized")
		return "", false
	}

	euc.cache.Add(targetUserID, *password)
	return *password, true
}

// updateCache updates the intersection of kvs map and cache and removes other keys from cache.
func updateCache(kvs map[string]string, cache *lru.Cache) {
	intersection := make(map[string]string)
	for key, password := range kvs {
		userID := strings.TrimPrefix(key, credsPrefix)
		if oldPassword, ok := cache.Peek(userID); ok {
			intersection[userID] = password
			if password != oldPassword {
				cache.Add(userID, password) // update password
			}
		}
	}
	// delete cache items that were not in kvs
	for _, cacheKey := range cache.Keys() {
		if _, exists := intersection[cacheKey.(string)]; !exists {
			cache.Remove(cacheKey)
		}
	}
}

func (euc *etcdUserCache) WatchChanges() error {
	stopCtx, cancel := context.WithCancel(context.Background())
	euc.cancel = cancel
	var (
		syncer *cluster.Syncer
		err    error
		ch     <-chan map[string]string
	)

	for {
		syncer, err = euc.cluster.Syncer(20 * time.Minute)
		if err != nil {
			logger.Errorf("failed to create syncer: %v", err)
		} else if ch, err = syncer.SyncPrefix(credsPrefix); err != nil {
			logger.Errorf("failed to sync prefix: %v", err)
			syncer.Close()
		} else {
			break
		}

		select {
		case <-time.After(10 * time.Second):
		case <-stopCtx.Done():
			return nil
		}
	}
	defer syncer.Close()

	for {
		select {
		case <-stopCtx.Done():
			return nil
		case kvs := <-ch:
			updateCache(kvs, euc.cache)
		}
	}
	return nil
}

func (euc *etcdUserCache) Close() {
	if euc.cancel != nil {
		euc.cancel()
	}
}

func (euc *etcdUserCache) Lock()   {}
func (euc *etcdUserCache) Unlock() {}

// NewBasicAuthValidator creates a new Basic Auth validator
func NewBasicAuthValidator(spec *BasicAuthValidatorSpec, supervisor *supervisor.Supervisor) *BasicAuthValidator {
	var cache AuthorizedUsersCache
	if spec.UseEtcd {
		if supervisor == nil || supervisor.Cluster() == nil {
			logger.Errorf("BasicAuth validator : failed to read data from etcd")
		} else {
			cache = newEtcdUserCache(supervisor.Cluster())
		}
	} else if spec.UserFile != "" {
		cache = newHtpasswdUserCache(spec.UserFile, 1*time.Minute)
	} else {
		logger.Errorf("BasicAuth validator spec unvalid.")
		return nil
	}
	go cache.WatchChanges()
	bav := &BasicAuthValidator{
		spec:                 spec,
		authorizedUsersCache: cache,
	}
	return bav
}

func parseBasicAuthorizationHeader(hdr *httpheader.HTTPHeader) (string, error) {
	const prefix = "Basic "

	tokenStr := hdr.Get("Authorization")
	if !strings.HasPrefix(tokenStr, prefix) {
		return "", fmt.Errorf("unexpected authorization header: %s", tokenStr)
	}
	return strings.TrimPrefix(tokenStr, prefix), nil
}

// Validate validates the Authorization header of a http request
func (bav *BasicAuthValidator) Validate(req httpcontext.HTTPRequest) error {
	hdr := req.Header()
	base64credentials, err := parseBasicAuthorizationHeader(hdr)
	if err != nil {
		return err
	}
	credentialBytes, err := base64.StdEncoding.DecodeString(base64credentials)
	if err != nil {
		return fmt.Errorf("error occured during base64 decode: %s", err.Error())
	}
	credentials := string(credentialBytes)
	userID, token, err := parseCredentials(credentials)
	if err != nil {
		return fmt.Errorf("unauthorized")
	}

	if expectedToken, ok := bav.authorizedUsersCache.GetUser(userID); ok && expectedToken == token {
		return nil
	}
	return fmt.Errorf("unauthorized")
}

// Close closes authorizedUsersCache.
func (bav *BasicAuthValidator) Close() {
	bav.authorizedUsersCache.Close()
}
