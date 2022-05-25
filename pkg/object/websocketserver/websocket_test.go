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
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testUpgrader = &websocket.Upgrader{}

func init() {
	logger.InitNop()
}

func upgrade(w http.ResponseWriter, r *http.Request) {
	conn, err := testUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		err = conn.WriteMessage(mt, msg)
		if err != nil {
			break
		}
	}
}

func checkStart(w http.ResponseWriter, r *http.Request) {
}

func getTestServer(t *testing.T, addr string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", upgrade)
	mux.HandleFunc("/start", checkStart)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	go server.ListenAndServe()

	started := false
	for i := 0; i < 10; i++ {
		req, err := http.NewRequest(http.MethodGet, "http://"+addr+"/start", nil)
		require.Nil(t, err)
		_, err = http.DefaultClient.Do(req)
		if err == nil {
			started = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.Equal(t, true, started, "server not started")
	return server
}

func getWebSocket(t *testing.T, yamlStr string, checkURL string) *WebSocketServer {
	super := supervisor.NewDefaultMock()
	superSpec, err := super.NewSpec(yamlStr)
	ws := &WebSocketServer{}
	require.Nil(t, err)
	ws.Init(superSpec)
	assert.Nil(t, ws.Validate())

	started := false
	for i := 0; i < 10; i++ {
		ws, _, err := websocket.DefaultDialer.Dial(checkURL, nil)
		if err == nil {
			started = true
			ws.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.Equal(t, true, started)
	return ws
}

func doClient(t *testing.T, wg *sync.WaitGroup, url, cid string) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		ws, _, err := websocket.DefaultDialer.Dial(url, nil)
		require.Nil(t, err)
		defer ws.Close()

		for j := 0; j < 10; j++ {
			val := fmt.Sprintf("%s_%d", cid, j)
			err := ws.WriteMessage(websocket.TextMessage, []byte(val))
			assert.Nil(t, err)
			_, p, err := ws.ReadMessage()
			assert.Nil(t, err)
			assert.Equal(t, val, string(p))
		}
	}()
}

func TestWebSocket(t *testing.T) {
	testSrv := getTestServer(t, "127.0.0.1:8000")
	defer testSrv.Close()

	wsYaml := `
kind: WebSocketServer
name: websocket-demo
port: 10081
https: false
backend: ws://127.0.0.1:8000`
	ws := getWebSocket(t, wsYaml, "ws://127.0.0.1:10081")

	clientNum := 50
	wg := &sync.WaitGroup{}
	for i := 0; i < clientNum; i++ {
		doClient(t, wg, "ws://127.0.0.1:10081", strconv.Itoa(i))
	}
	wg.Wait()

	// test inherit
	newWs := WebSocketServer{}
	newWs.Inherit(ws.superSpec, ws)
	newWs.Status()
	time.Sleep(100 * time.Millisecond)
	newWs.Close()
}

func TestWebSocketTLS(t *testing.T) {
	testSrv := getTestServer(t, "127.0.0.1:8000")
	defer testSrv.Close()

	cert := base64.StdEncoding.EncodeToString([]byte(certPem))
	key := base64.StdEncoding.EncodeToString([]byte(keyPem))
	wsYaml := `
kind: WebSocketServer
name: websocket-demo
port: 10081
https: true
backend: wss://127.0.0.1:8000
certBase64: %v
keyBase64: %v
wssCertBase64: %v
wssKeyBase64: %v
`
	wsYaml = fmt.Sprintf(wsYaml, cert, key, cert, key)

	super := supervisor.NewDefaultMock()
	superSpec, err := super.NewSpec(wsYaml)
	require.Nil(t, err)

	ws := &WebSocketServer{}
	ws.Init(superSpec)
	assert.Nil(t, ws.Validate())
	time.Sleep(50 * time.Millisecond)
	ws.Close()
}

func TestWebSocketEmptyTLS(t *testing.T) {
	testSrv := getTestServer(t, "127.0.0.1:8000")
	defer testSrv.Close()

	wsYaml := `
kind: WebSocketServer
name: websocket-demo
port: 10081
https: true
backend: wss://127.0.0.1:8000
certBase64: 1234
keyBase64: 2234
wssCertBase64: 3234
wssKeyBase64: 4234
`
	super := supervisor.NewDefaultMock()
	superSpec, err := super.NewSpec(wsYaml)
	require.Nil(t, err)

	_, err = superSpec.ObjectSpec().(*Spec).tlsConfig()
	assert.NotNil(t, err)
	_, err = superSpec.ObjectSpec().(*Spec).wssTLSConfig()
	assert.NotNil(t, err)

	ws := &WebSocketServer{}
	ws.Init(superSpec)
	assert.Nil(t, ws.Validate())
	time.Sleep(50 * time.Millisecond)
	ws.Close()
}
