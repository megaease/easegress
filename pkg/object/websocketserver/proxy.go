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

package websocketserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	xForwardedFor   = "X-Forwarded-For"
	xForwardedHost  = "X-Forwarded-Host"
	xForwardedProto = "X-Forwarded-Proto"
)

// headersToSkip are gorilla library's request headers, our websocket proxy should not set.
var headersToSkip = map[string]struct{}{
	"Upgrade":                  {},
	"Connection":               {},
	"Sec-Websocket-Key":        {},
	"Sec-Websocket-Version":    {},
	"Sec-Websocket-Extensions": {},
	"Sec-Websocket-Protocol":   {},
}

var (
	// defaultUpgrader specifies the parameters for upgrading an HTTP
	// connection to a WebSocket connection.
	defaultUpgrader = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	// defaultDialer is a dialer with all fields set to the default zero values.
	defaultDialer = websocket.DefaultDialer

	// defaultInterval is the default interval for polling websocket client and server which is
	//  200ms right now.
	defaultInterval = 200 * time.Millisecond
)

// Proxy is a handler that takes an incoming WebSocket
// connection and proxies it to the backend server.
type Proxy struct {
	// server is the HTTPServer
	server    http.Server
	superSpec *supervisor.Spec

	// backendURL URL is the URL of target websocket server.
	backendURL *url.URL

	// upgrader specifies the parameters for upgrading an incoming HTTP
	// connection to a WebSocket connection.
	upgrader *websocket.Upgrader

	//  dialer contains options for connecting to the backend WebSocket server.
	dialer *websocket.Dialer

	// done is the channel for shutdowning this proxy.
	done chan struct{}
}

// NewProxy returns a new Websocket proxy.
func newProxy(superSpec *supervisor.Spec) *Proxy {
	proxy := &Proxy{
		superSpec: superSpec,
		done:      make(chan struct{}),
	}
	go proxy.run()
	return proxy
}

// buildRequestURL builds an URL with backend in spec and original HTTP request.
func (p *Proxy) buildRequestURL(r *http.Request) *url.URL {
	u := *p.backendURL
	u.Fragment = r.URL.Fragment
	u.Path = r.URL.Path
	u.RawQuery = r.URL.RawQuery
	return &u
}

// passMsg passes websocket message from src to dst.
func (p *Proxy) passMsg(src, dst *websocket.Conn, errc chan error, stop chan struct{}) {
	handle := func() bool {
		msgType, msg, err := src.ReadMessage()
		if err != nil {
			m := websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("%v", err))
			if e, ok := err.(*websocket.CloseError); ok {
				if e.Code != websocket.CloseNoStatusReceived {
					m = websocket.FormatCloseMessage(e.Code, e.Text)
				}
			}
			dst.WriteMessage(websocket.CloseMessage, m)
			errc <- err
			return false
		}
		err = dst.WriteMessage(msgType, msg)
		if err != nil {
			errc <- err
			return false
		}
		return true
	}

	for {
		select {
		// this request handling is stopped due to some error or websocketserver shutdown.
		case <-stop:
			return
		case <-time.After(defaultInterval):
			if !handle() {
				return
			}
		}
	}
}

// run runs the websocket proxy.
func (p *Proxy) run() {
	spec := p.superSpec.ObjectSpec().(*Spec)
	backendURL, err := url.Parse(spec.Backend)
	if err != nil {
		logger.Errorf("BUG: %s get invalid websocketserver backend URL: %s",
			p.superSpec.Name(), spec.Backend)
		return
	}

	p.backendURL = backendURL
	dialer := defaultDialer
	if strings.HasPrefix(spec.Backend, "wss") {
		tlsConfig, err := spec.wssTLSConfig()
		if err != nil {
			logger.Errorf("%s gen websocketserver backend tls failed: %v, spec :%#v",
				p.superSpec.Name(), spec)
			return
		}
		dialer.TLSClientConfig = tlsConfig
	}
	p.dialer = dialer
	p.upgrader = defaultUpgrader

	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handle)
	addr := fmt.Sprintf(":%d", spec.Port)

	p.server.Addr = addr
	p.server.Handler = mux

	if spec.HTTPS {
		tlsConfig, err := spec.tlsConfig()
		if err != nil {
			logger.Errorf("%s gen websocketserver's httpserver tlsConfig: %#v, failed: %v",
				p.superSpec.Name(), spec, err)
		}
		p.server.TLSConfig = tlsConfig
	}

	if p.server.TLSConfig != nil {
		if err := p.server.ListenAndServeTLS("", ""); err != nil {
			logger.Errorf("%s websocketserver ListenAndServeTLS failed: %v", p.superSpec.Name(), err)
		}
	} else {
		if err := p.server.ListenAndServe(); err != nil {
			logger.Errorf("%s websocketserver ListenAndServe failed: %v", p.superSpec.Name(), err)
		}
	}
}

// copyHeader copies headers from the incoming request to the dialer and forward them to
// the destination.
func (p *Proxy) copyHeader(req *http.Request) http.Header {
	// Based on https://docs.oracle.com/en-us/iaas/Content/Balance/Reference/httpheaders.htm
	// For load balancer, we add following key-value pairs to headers
	// X-Forwarded-For: <original_client>, <proxy1>, <proxy2>
	// X-Forwarded-Host: www.example.com:8080
	// X-Forwarded-Proto: https

	// New client connection is created using [Gorilla websocket library](https://github.com/gorilla/websocket), which takes care of some of the headers.
	// Let's copy copy all headers from the incoming request, except the ones gorilla will set.

	requestHeader := http.Header{}
	for k, values := range req.Header {
		if _, ok := headersToSkip[k]; ok {
			continue
		}
		for _, v := range values {
			requestHeader.Add(k, v)
		}
	}

	xff := requestHeader.Get(xForwardedFor)
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		if xff == "" {
			requestHeader.Set(xForwardedFor, clientIP)
		} else {
			requestHeader.Set(xForwardedFor, fmt.Sprintf("%s, %s", xff, clientIP))
		}
	}

	xfh := requestHeader.Get(xForwardedHost)
	if xfh == "" && req.Host != "" {
		requestHeader.Set(xForwardedHost, req.Host)
	}

	requestHeader.Set(xForwardedProto, "http")
	if req.TLS != nil {
		requestHeader.Set(xForwardedProto, "https")
	}
	return requestHeader
}

// upgradeRspHeader passes only selected headers as return.
func (p *Proxy) upgradeRspHeader(resp *http.Response) http.Header {
	upgradeHeader := http.Header{}
	if hdr := resp.Header.Get("Sec-Websocket-Protocol"); hdr != "" {
		upgradeHeader.Set("Sec-Websocket-Protocol", hdr)
	}
	if hdr := resp.Header.Get("Set-Cookie"); hdr != "" {
		upgradeHeader.Set("Set-Cookie", hdr)
	}
	return upgradeHeader
}

// handle implements the http.Handler that proxies WebSocket connections.
func (p *Proxy) handle(rw http.ResponseWriter, req *http.Request) {
	connBackend, resp, err := p.dialer.Dial(p.buildRequestURL(req).String(), p.copyHeader(req))
	if err != nil {
		logger.Errorf("%s dials %s failed: %v", p.superSpec.Name(), p.backendURL.String(), err)
		if resp != nil {
			// Handle WebSocket handshake failed scenario.
			// Should send back a non-nil *http.Response for callers to handle
			// `redirects`, `authentication` operations and so on.
			if err := copyResponse(rw, resp); err != nil {
				logger.Errorf("%s writes response failed at remote backend: %s handshake: %v",
					p.superSpec.Name(), p.backendURL.String(), err)
			}
		} else {
			http.Error(rw, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		}
		return
	}
	defer connBackend.Close()

	// Upgrade the incoming request to a WebSocket connection(Protocol Switching).
	// Also pass the header from the Dial handshake.
	connClient, err := p.upgrader.Upgrade(rw, req, p.upgradeRspHeader(resp))
	if err != nil {
		logger.Errorf("%s upgrades req: %#v failed: %s", p.superSpec.Name(), err)
		return
	}
	defer connClient.Close()

	errClient := make(chan error, 1)
	errBackend := make(chan error, 1)
	stop := make(chan struct{})

	defer close(stop)

	// pass msg from backend to client via WebSocket protocol.
	go p.passMsg(connBackend, connClient, errBackend, stop)
	// pass msg from client to backend via WebSocket protocol.
	go p.passMsg(connClient, connBackend, errClient, stop)

	var errMsg string
	select {
	case err = <-errBackend:
		errMsg = "%s passes msg from backend: %s to client failed: %v"
	case err = <-errClient:
		errMsg = "%s passes msg client to backend: %s failed: %v"
	case <-p.done:
		logger.Debugf("shutdown websocketserver in request handling")
		return
	}

	if e, ok := err.(*websocket.CloseError); !ok || e.Code == websocket.CloseAbnormalClosure {
		logger.Errorf(errMsg, p.superSpec.Name(), p.backendURL.String(), err)
	}
	// other error type is expected, not need to log
}

// Close closes websocket proxy.
func (p *Proxy) Close() {
	close(p.done)

	ctx, cancelFunc := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFunc()
	err := p.server.Shutdown(ctx)
	if err != nil {
		logger.Warnf("%s shutdowns http server failed: %v",
			p.superSpec.Name(), err)
	}
}

func copyResponse(rw http.ResponseWriter, resp *http.Response) error {
	for k, vv := range resp.Header {
		for _, v := range vv {
			rw.Header().Add(k, v)
		}
	}
	rw.WriteHeader(resp.StatusCode)
	defer resp.Body.Close()

	_, err := io.Copy(rw, resp.Body)
	return err
}
