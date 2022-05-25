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

package proxy

import (
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/hashtool"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	// PolicyRoundRobin is the policy of round-robin.
	PolicyRoundRobin = "roundRobin"
	// PolicyRandom is the policy of random.
	PolicyRandom = "random"
	// PolicyWeightedRandom is the policy of weighted random.
	PolicyWeightedRandom = "weightedRandom"
	// PolicyIPHash is the policy of ip hash.
	PolicyIPHash = "ipHash"
	// PolicyHeaderHash is the policy of header hash.
	PolicyHeaderHash = "headerHash"

	retryTimeout = 3 * time.Second
)

type (
	servers struct {
		poolSpec *PoolSpec
		super    *supervisor.Supervisor

		mutex           sync.Mutex
		serviceRegistry *serviceregistry.ServiceRegistry
		serviceWatcher  serviceregistry.ServiceWatcher
		static          *staticServers
		done            chan struct{}
	}

	staticServers struct {
		count      uint64
		weightsSum int
		servers    []*Server
		lb         LoadBalance
	}

	// Server is proxy server.
	Server struct {
		URL            string   `yaml:"url" jsonschema:"required,format=url"`
		Tags           []string `yaml:"tags" jsonschema:"omitempty,uniqueItems=true"`
		Weight         int      `yaml:"weight" jsonschema:"omitempty,minimum=0,maximum=100"`
		KeepHost       bool     `yaml:"keepHost" jsonschema:"omitempty,default=false`
		addrIsHostName bool
	}

	// LoadBalance is load balance for multiple servers.
	LoadBalance struct {
		Policy        string `yaml:"policy" jsonschema:"required,enum=roundRobin,enum=random,enum=weightedRandom,enum=ipHash,enum=headerHash"`
		HeaderHashKey string `yaml:"headerHashKey" jsonschema:"omitempty"`
	}
)

// String implements the Stringer interface.
func (s *Server) String() string {
	return fmt.Sprintf("%s,%v,%d", s.URL, s.Tags, s.Weight)
}

// checkAddrPattern checks whether the server address is host name or ip:port,
// not all error cases are handled.
func (s *Server) checkAddrPattern() {
	u, err := url.Parse(s.URL)
	if err != nil {
		return
	}
	host := u.Host

	square := strings.LastIndexByte(host, ']')
	colon := strings.LastIndexByte(host, ':')

	// There is a port number, remove it.
	if colon > square {
		host = host[:colon]
	}

	// IPv6
	if square != -1 && host[0] == '[' {
		host = host[1:square]
	}

	s.addrIsHostName = net.ParseIP(host) == nil
}

// Validate validates LoadBalance.
func (lb LoadBalance) Validate() error {
	if lb.Policy == PolicyHeaderHash && len(lb.HeaderHashKey) == 0 {
		return fmt.Errorf("headerHash needs to specify headerHashKey")
	}

	return nil
}

func newServers(super *supervisor.Supervisor, poolSpec *PoolSpec) *servers {
	s := &servers{
		poolSpec: poolSpec,
		super:    super,
		done:     make(chan struct{}),
	}

	s.useStaticServers()

	if poolSpec.ServiceRegistry == "" || poolSpec.ServiceName == "" {
		return s
	}

	s.serviceRegistry = s.super.MustGetSystemController(serviceregistry.Kind).
		Instance().(*serviceregistry.ServiceRegistry)

	s.tryUseService()
	s.serviceWatcher = s.serviceRegistry.NewServiceWatcher(s.poolSpec.ServiceRegistry, s.poolSpec.ServiceName)

	go s.watchService()

	return s
}

func (s *servers) watchService() {
	for {
		select {
		case <-s.done:
			return
		case event := <-s.serviceWatcher.Watch():
			s.handleEvent(event)
		}
	}
}

func (s *servers) handleEvent(event *serviceregistry.ServiceEvent) {
	s.useService(event.Instances)
}

func (s *servers) tryUseService() {
	serviceInstanceSpecs, err := s.serviceRegistry.ListServiceInstances(s.poolSpec.ServiceRegistry, s.poolSpec.ServiceName)
	if err != nil {
		logger.Warnf("first try to use service %s/%s failed(will try again): %v",
			s.poolSpec.ServiceRegistry, s.poolSpec.ServiceName, err)
		s.useStaticServers()
		return
	}

	s.useService(serviceInstanceSpecs)
}

func (s *servers) useService(serviceInstanceSpecs map[string]*serviceregistry.ServiceInstanceSpec) {
	var servers []*Server
	for _, instance := range serviceInstanceSpecs {
		servers = append(servers, &Server{
			URL:    instance.URL(),
			Tags:   instance.Tags,
			Weight: instance.Weight,
		})
	}
	if len(servers) == 0 {
		logger.Warnf("%s/%s: empty service instance",
			s.poolSpec.ServiceRegistry, s.poolSpec.ServiceName)
		s.useStaticServers()
		return
	}

	dynamicServers := newStaticServers(servers, s.poolSpec.ServersTags, s.poolSpec.LoadBalance)
	if dynamicServers.len() == 0 {
		logger.Warnf("%s/%s: no service instance satisfy tags: %v",
			s.poolSpec.ServiceRegistry, s.poolSpec.ServiceName, s.poolSpec.ServersTags)
		s.useStaticServers()
	}

	logger.Infof("use dynamic service: %s/%s", s.poolSpec.ServiceRegistry, s.poolSpec.ServiceName)

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.static = dynamicServers
}

func (s *servers) useStaticServers() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.static = newStaticServers(s.poolSpec.Servers, s.poolSpec.ServersTags, s.poolSpec.LoadBalance)
}

func (s *servers) snapshot() *staticServers {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.static
}

func (s *servers) len() int {
	static := s.snapshot()

	return static.len()
}

func (s *servers) next(ctx context.HTTPContext) (*Server, error) {
	static := s.snapshot()

	if static.len() == 0 {
		return nil, fmt.Errorf("no server available")
	}

	return static.next(ctx), nil
}

func (s *servers) close() {
	close(s.done)

	if s.serviceWatcher != nil {
		s.serviceWatcher.Stop()
	}
}

func newStaticServers(servers []*Server, tags []string, lb *LoadBalance) *staticServers {
	if servers == nil {
		servers = make([]*Server, 0)
	}

	ss := &staticServers{}
	if lb == nil {
		ss.lb.Policy = PolicyRoundRobin
	} else {
		ss.lb = *lb
	}

	defer ss.prepare()

	if len(tags) == 0 {
		ss.servers = servers
		return ss
	}

	chosenServers := make([]*Server, 0)
	for _, server := range servers {
		for _, tag := range tags {
			if stringtool.StrInSlice(tag, server.Tags) {
				chosenServers = append(chosenServers, server)
				break
			}
		}
	}
	ss.servers = chosenServers

	return ss
}

func (ss *staticServers) prepare() {
	for _, server := range ss.servers {
		server.checkAddrPattern()
		ss.weightsSum += server.Weight
	}
}

func (ss *staticServers) len() int {
	return len(ss.servers)
}

func (ss *staticServers) next(ctx context.HTTPContext) *Server {
	switch ss.lb.Policy {
	case PolicyRoundRobin:
		return ss.roundRobin(ctx)
	case PolicyRandom:
		return ss.random(ctx)
	case PolicyWeightedRandom:
		return ss.weightedRandom(ctx)
	case PolicyIPHash:
		return ss.ipHash(ctx)
	case PolicyHeaderHash:
		return ss.headerHash(ctx)
	}

	logger.Errorf("BUG: unknown load balance policy: %s", ss.lb.Policy)

	return ss.roundRobin(ctx)
}

func (ss *staticServers) roundRobin(ctx context.HTTPContext) *Server {
	count := atomic.AddUint64(&ss.count, 1)
	// NOTE: start from 0.
	count--
	return ss.servers[int(count)%len(ss.servers)]
}

func (ss *staticServers) random(ctx context.HTTPContext) *Server {
	return ss.servers[rand.Intn(len(ss.servers))]
}

func (ss *staticServers) weightedRandom(ctx context.HTTPContext) *Server {
	randomWeight := rand.Intn(ss.weightsSum)
	for _, server := range ss.servers {
		randomWeight -= server.Weight
		if randomWeight < 0 {
			return server
		}
	}

	logger.Errorf("BUG: weighted random can't pick a server: sum(%d) servers(%+v)",
		ss.weightsSum, ss.servers)

	return ss.random(ctx)
}

func (ss *staticServers) ipHash(ctx context.HTTPContext) *Server {
	sum32 := int(hashtool.Hash32(ctx.Request().RealIP()))
	return ss.servers[sum32%len(ss.servers)]
}

func (ss *staticServers) headerHash(ctx context.HTTPContext) *Server {
	value := ctx.Request().Header().Get(ss.lb.HeaderHashKey)
	sum32 := int(hashtool.Hash32(value))
	return ss.servers[sum32%len(ss.servers)]
}
