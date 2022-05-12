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

package httpserver

import (
	"bytes"
	stdcontext "context"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/lucas-clemente/quic-go/http3"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/graceupdate"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/easemonitor"
	"github.com/megaease/easegress/pkg/util/filterwriter"
	"github.com/megaease/easegress/pkg/util/limitlistener"
)

const (
	defaultKeepAliveTimeout = 60 * time.Second

	checkFailedTimeout = 10 * time.Second

	topNum = 10

	stateNil     stateType = "nil"
	stateFailed  stateType = "failed"
	stateRunning stateType = "running"
	stateClosed  stateType = "closed"
)

var (
	errNil = fmt.Errorf("")
	gnet   = graceupdate.Global
)

type (
	stateType string

	eventCheckFailed struct{}
	eventServeFailed struct {
		startNum uint64
		err      error
	}
	eventReload struct {
		nextSuperSpec *supervisor.Spec
		muxMapper     context.MuxMapper
	}
	eventClose struct{ done chan struct{} }

	runtime struct {
		superSpec *supervisor.Spec
		spec      *Spec
		server    *http.Server
		server3   *http3.Server
		mux       *mux
		startNum  uint64
		eventChan chan interface{}

		// status
		state atomic.Value // stateType
		err   atomic.Value // error

		httpStat      *httpstat.HTTPStat
		topN          *httpstat.TopN
		limitListener *limitlistener.LimitListener
	}

	// Status contains all status generated by runtime, for displaying to users.
	Status struct {
		Name   string `yaml:"name"`
		Health string `yaml:"health"`

		State stateType `yaml:"state"`
		Error string    `yaml:"error,omitempty"`

		*httpstat.Status
		TopN []*httpstat.Item `yaml:"topN"`
	}
)

func newRuntime(superSpec *supervisor.Spec, muxMapper context.MuxMapper) *runtime {
	r := &runtime{
		superSpec: superSpec,
		eventChan: make(chan interface{}, 10),
		httpStat:  httpstat.New(),
		topN:      httpstat.NewTopN(topNum),
	}

	r.mux = newMux(r.httpStat, r.topN, muxMapper)
	r.setState(stateNil)
	r.setError(errNil)

	go r.fsm()
	go r.checkFailed(checkFailedTimeout)

	return r
}

// Close closes runtime.
func (r *runtime) Close() {
	done := make(chan struct{})
	r.eventChan <- &eventClose{done: done}
	<-done
}

// Status returns HTTPServer Status.
func (r *runtime) Status() *Status {
	health := r.getError().Error()

	return &Status{
		Name:   r.superSpec.Name(),
		Health: health,
		State:  r.getState(),
		Error:  r.getError().Error(),
		Status: r.httpStat.Status(),
		TopN:   r.topN.Status(),
	}
}

// FSM is the finite-state-machine for the runtime.
func (r *runtime) fsm() {
	for e := range r.eventChan {
		switch e := e.(type) {
		case *eventCheckFailed:
			r.handleEventCheckFailed(e)
		case *eventServeFailed:
			r.handleEventServeFailed(e)
		case *eventReload:
			r.handleEventReload(e)
		case *eventClose:
			r.handleEventClose(e)
			// NOTE: We don't close hs.eventChan,
			// in case of panic of any other goroutines
			// to send event to it later.
			return
		default:
			logger.Errorf("BUG: unknown event: %T\n", e)
		}
	}
}

func (r *runtime) reload(nextSuperSpec *supervisor.Spec, muxMapper context.MuxMapper) {
	r.superSpec = nextSuperSpec
	r.mux.reload(nextSuperSpec, muxMapper)

	nextSpec := nextSuperSpec.ObjectSpec().(*Spec)

	// r.limitListener is not created just after the process started and the config load for the first time.
	if nextSpec != nil && r.limitListener != nil {
		r.limitListener.SetMaxConnection(nextSpec.MaxConnections)
	}

	// NOTE: Due to the mechanism of supervisor,
	// nextSpec must not be nil, just defensive programming here.
	switch {
	case r.spec == nil && nextSpec == nil:
		logger.Errorf("BUG: nextSpec is nil")
		// Nothing to do.
	case r.spec == nil && nextSpec != nil:
		r.spec = nextSpec
		r.startServer()
	case r.spec != nil && nextSpec == nil:
		logger.Errorf("BUG: nextSpec is nil")
		r.spec = nil
		r.closeServer()
	case r.spec != nil && nextSpec != nil:
		if r.needRestartServer(nextSpec) {
			r.spec = nextSpec
			r.closeServer()
			r.startServer()
		} else {
			r.spec = nextSpec
		}
	}
}

func (r *runtime) setState(state stateType) {
	r.state.Store(state)
}

func (r *runtime) getState() stateType {
	return r.state.Load().(stateType)
}

func (r *runtime) setError(err error) {
	if err == nil {
		r.err.Store(errNil)
	} else {
		// NOTE: For type safe.
		r.err.Store(fmt.Errorf("%v", err))
	}
}

func (r *runtime) getError() error {
	err := r.err.Load()
	if err == nil {
		return nil
	}
	return err.(error)
}

func (r *runtime) needRestartServer(nextSpec *Spec) bool {
	x := *r.spec
	y := *nextSpec

	// The change of options below need not restart the HTTP server.
	x.MaxConnections, y.MaxConnections = 0, 0
	x.CacheSize, y.CacheSize = 0, 0
	x.XForwardedFor, y.XForwardedFor = false, false
	x.Tracing, y.Tracing = nil, nil
	x.IPFilter, y.IPFilter = nil, nil
	x.Rules, y.Rules = nil, nil

	// The update of rules need not to shutdown server.
	return !reflect.DeepEqual(x, y)
}

func (r *runtime) startServer() {
	keepAliveTimeout := defaultKeepAliveTimeout
	if r.spec.KeepAliveTimeout != "" {
		t, _ := time.ParseDuration(r.spec.KeepAliveTimeout)
		keepAliveTimeout = t
	}

	fw := filterwriter.New(os.Stderr, func(p []byte) bool {
		return !bytes.Contains(p, []byte("TLS handshake error"))
	})
	srv := &http.Server{
		Addr:        fmt.Sprintf(":%d", r.spec.Port),
		Handler:     r.mux,
		IdleTimeout: keepAliveTimeout,
		ErrorLog:    log.New(fw, "", log.LstdFlags),
	}
	srv.SetKeepAlivesEnabled(r.spec.KeepAlive)

	if r.spec.HTTPS {
		tlsConfig, _ := r.spec.tlsConfig()
		srv.TLSConfig = tlsConfig
	}

	r.server = srv
	r.startNum++
	r.setState(stateRunning)
	r.setError(nil)

	if r.spec.HTTP3 {
		r.server3 = &http3.Server{
			Server: r.server,
		}
		go r.runHTTP3Server(r.startNum)
	} else {
		listener, err := gnet.Listen("tcp", fmt.Sprintf(":%d", r.spec.Port))
		if err != nil {
			r.setState(stateFailed)
			r.setError(err)

			return
		}

		limitListener := limitlistener.NewLimitListener(listener, r.spec.MaxConnections)
		r.limitListener = limitListener
		go r.runHTTP1And2Server(limitListener, r.spec.HTTPS, r.startNum)
	}
}

func (r *runtime) runHTTP3Server(startNum uint64) {
	err := r.server3.ListenAndServe()
	if err != http.ErrServerClosed {
		r.eventChan <- &eventServeFailed{
			err:      err,
			startNum: startNum,
		}
	}
}

func (r *runtime) runHTTP1And2Server(limitListener *limitlistener.LimitListener, https bool, startNum uint64) {
	var err error
	if https {
		err = r.server.ServeTLS(limitListener, "", "")
	} else {
		err = r.server.Serve(limitListener)
	}
	if err != http.ErrServerClosed {
		r.eventChan <- &eventServeFailed{
			err:      err,
			startNum: startNum,
		}
	}
}

func (r *runtime) closeServer() {
	if r.server3 != nil {
		err := r.server3.Close()
		if err != nil {
			logger.Warnf("shutdown http3 server %s failed: %v", r.superSpec.Name(), err)
		}
		return
	}

	if r.server != nil {
		// NOTE: It's safe to shutdown serve failed server.
		ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 30*time.Second)
		defer cancel()
		err := r.server.Shutdown(ctx)
		if err != nil {
			logger.Warnf("shutdown http1/2 server %s failed: %v",
				r.superSpec.Name(), err)
		}
	}
}

func (r *runtime) checkFailed(timeout time.Duration) {
	ticker := time.NewTicker(timeout)
	for range ticker.C {
		state := r.getState()
		if state == stateFailed {
			r.eventChan <- &eventCheckFailed{}
		} else if state == stateClosed {
			ticker.Stop()
			return
		}
	}
}

func (r *runtime) handleEventCheckFailed(e *eventCheckFailed) {
	if r.getState() == stateFailed {
		r.startServer()
	}
}

func (r *runtime) handleEventServeFailed(e *eventServeFailed) {
	if r.startNum > e.startNum {
		return
	}
	r.setState(stateFailed)
	r.setError(e.err)
}

func (r *runtime) handleEventReload(e *eventReload) {
	r.reload(e.nextSuperSpec, e.muxMapper)
}

func (r *runtime) handleEventClose(e *eventClose) {
	r.closeServer()
	r.mux.close()
	close(e.done)
}

// ToMetrics implements easemonitor.Metricer.
func (s *Status) ToMetrics(service string) []*easemonitor.Metrics {
	var results []*easemonitor.Metrics

	if s.Status != nil {
		results = s.Status.ToMetrics(service)
		for _, m := range results {
			m.Resource = "SERVER"
		}
	}

	for _, item := range s.TopN {
		metrics := item.ToMetrics(service)
		for _, m := range metrics {
			m.Resource = "SERVER_TOPN"
			m.URL = item.Path
		}
		results = append(results, metrics...)
	}

	return results
}
