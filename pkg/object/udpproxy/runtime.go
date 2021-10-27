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

package udpproxy

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/megaease/easegress/pkg/util/layer4backend"
)

type (
	runtime struct {
		superSpec *supervisor.Spec
		spec      *Spec

		pool       *layer4backend.Pool // backend servers pool
		serverConn *net.UDPConn        // listener
		sessions   map[string]*session

		ipFilters *ipfilter.Layer4IpFilters

		mu   sync.Mutex
		done chan struct{}
	}
)

func newRuntime(superSpec *supervisor.Spec) *runtime {
	spec := superSpec.ObjectSpec().(*Spec)
	r := &runtime{
		superSpec: superSpec,

		pool:      layer4backend.NewPool(superSpec.Super(), spec.Pool, ""),
		ipFilters: ipfilter.NewLayer4IPFilters(spec.IPFilter),

		sessions: make(map[string]*session),
	}

	r.startServer()
	return r
}

// Close notify runtime close
func (r *runtime) Close() {

	close(r.done)
	_ = r.serverConn.Close()

	r.mu.Lock()
	for k, s := range r.sessions {
		delete(r.sessions, k)
		s.Close()
	}
	r.sessions = nil
	r.mu.Unlock()

	r.pool.Close()
}

func (r *runtime) startServer() {
	listenAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", r.spec.Port))
	if err != nil {
		logger.Errorf("parse udp listen addr(%s) failed, err: %+v", r.spec.Port, err)
		return
	}

	r.serverConn, err = net.ListenUDP("udp", listenAddr)
	if err != nil {
		logger.Errorf("create udp listener(%s) failed, err: %+v", r.spec.Port, err)
		return
	}

	var cp *connPool
	if r.spec.HasResponse {
		cp = newConnPool()
	}

	go func() {
		defer cp.close()

		buf := make([]byte, iobufferpool.UDPPacketMaxSize)
		for {
			buf = buf[:0]
			n, downstreamAddr, err := r.serverConn.ReadFromUDP(buf)

			if err != nil {
				select {
				case <-r.done:
					return // detect weather udp server is closed
				default:
				}

				if ope, ok := err.(*net.OpError); ok {
					// not timeout error and not temporary, which means the error is non-recoverable
					if !(ope.Timeout() && ope.Temporary()) {
						logger.Errorf("udp listener(%d) crashed due to non-recoverable error, err: %+v", r.spec.Port, err)
						return
					}
				}
				logger.Errorf("failed to read packet from udp connection(:%d), err: %+v", r.spec.Port, err)
				continue
			}

			if r.ipFilters != nil {
				if !r.ipFilters.AllowIP(downstreamAddr.IP.String()) {
					logger.Debugf("discard udp packet from %s send to udp server(:%d)", downstreamAddr.IP.String(), r.spec.Port)
					continue
				}
			}

			if !r.spec.HasResponse {
				if err := r.sendOneShot(cp, downstreamAddr, buf[0:n]); err != nil {
					logger.Errorf("%s", err.Error())
				}
				continue
			}

			r.proxy(downstreamAddr, buf[0:n])
		}
	}()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-ticker.C:
				r.cleanup()
			case <-r.done:
				ticker.Stop()
				return
			}
		}
	}()
}

func (r *runtime) getUpstreamConn(pool *connPool, downstreamAddr *net.UDPAddr) (net.Conn, string, error) {
	server, err := r.pool.Next(downstreamAddr.IP.String())
	if err != nil {
		return nil, "", fmt.Errorf("can not get upstream addr for udp connection(:%d)", r.spec.Port)
	}

	var upstreamConn net.Conn
	if pool != nil {
		upstreamConn = pool.get(server.Addr)
		if upstreamConn != nil {
			return upstreamConn, server.Addr, nil
		}
	}

	addr, err := net.ResolveUDPAddr("udp", server.Addr)
	if err != nil {
		return nil, server.Addr, fmt.Errorf("parse upstream addr(%s) to udp addr failed, err: %+v", server.Addr, err)
	}

	upstreamConn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, server.Addr, fmt.Errorf("dial to upstream addr(%s) failed, err: %+v", server.Addr, err)
	}
	if pool != nil {
		pool.put(server.Addr, upstreamConn)
	}
	return upstreamConn, server.Addr, nil
}

func (r *runtime) sendOneShot(pool *connPool, downstreamAddr *net.UDPAddr, buf []byte) error {
	upstreamConn, upstreamAddr, err := r.getUpstreamConn(pool, downstreamAddr)
	if err != nil {
		return err
	}

	n, err := upstreamConn.Write(buf)
	if err != nil {
		return fmt.Errorf("sned data to %s failed, err: %+v", upstreamAddr, err)
	}

	if n != len(buf) {
		return fmt.Errorf("failed to send full packet to %s, read %d but send %d", upstreamAddr, len(buf), n)
	}
	return nil
}

func (r *runtime) getSession(downstreamAddr *net.UDPAddr) (*session, error) {
	key := downstreamAddr.String()

	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[key]
	if ok && !s.IsClosed() {
		return s, nil
	}

	if ok {
		go func() { s.Close() }()
	}

	upstreamConn, upstreamAddr, err := r.getUpstreamConn(nil, downstreamAddr)
	if err != nil {
		return nil, err
	}

	s = newSession(downstreamAddr, upstreamAddr, upstreamConn,
		time.Duration(r.spec.UpstreamIdleTimeout)*time.Millisecond, time.Duration(r.spec.DownstreamIdleTimeout)*time.Millisecond)
	s.ListenResponse(r.serverConn)

	r.sessions[key] = s
	return s, nil
}

func (r *runtime) proxy(downstreamAddr *net.UDPAddr, buf []byte) {
	s, err := r.getSession(downstreamAddr)
	if err != nil {
		logger.Errorf("%s", err.Error())
		return
	}

	dup := iobufferpool.UDPBufferPool.Get().([]byte)
	n := copy(dup, buf)
	err = s.Write(&iobufferpool.Packet{Payload: dup, Len: n})
	if err != nil {
		logger.Errorf("write data to udp session(%s) failed, err: %v", downstreamAddr.IP.String(), err)
	}
}

func (r *runtime) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for k, s := range r.sessions {
		if s.IsClosed() {
			delete(r.sessions, k)
		}
	}
}
