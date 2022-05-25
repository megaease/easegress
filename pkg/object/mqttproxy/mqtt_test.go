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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/eclipse/paho.mqtt.golang/packets"
	_ "github.com/megaease/easegress/pkg/filter/mqttclientauth"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/openzipkin/zipkin-go/propagation/b3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func init() {
	logger.InitNop()
	pipeline.Register(&pipeline.MockMQTTFilter{})
	pipeline.Register(&MockKafka{})
	// pipeline.Register(&authentication.Authentication{})
}

var (
	publishPipeline = "publish-pipeline"
	connectPipeline = "connect-pipeline"
)

func getMQTTClient(t *testing.T, clientID, userName, password string, cleanSession bool) paho.Client {
	opts := paho.NewClientOptions().AddBroker("tcp://0.0.0.0:1883").SetClientID(clientID).SetUsername(userName).SetPassword(password).SetCleanSession(cleanSession)
	c := paho.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		t.Errorf("basic connect error for client <%s> with <%s>", clientID, token.Error())
	}
	return c
}

func getDefaultMQTTClient(t *testing.T, clientID string, cleanSession bool) paho.Client {
	return getMQTTClient(t, clientID, "test", "test", cleanSession)
}

func getUnConnectClient(clientID, userName, password string, cleanSession bool) paho.Client {
	opts := paho.NewClientOptions().AddBroker("tcp://0.0.0.0:1883").SetClientID(clientID).SetUsername(userName).SetPassword(password).SetCleanSession(cleanSession)
	c := paho.NewClient(opts)
	return c
}

func getMQTTSubscribeHandler(ch chan CheckMsg) paho.MessageHandler {
	handler := func(client paho.Client, message paho.Message) {
		msg := CheckMsg{
			topic:   message.Topic(),
			payload: string(message.Payload()),
			qos:     int(message.Qos()),
		}
		ch <- msg
	}
	return handler
}

func getDefaultSpec() *Spec {
	spec := &Spec{
		Name:   "test",
		EGName: "test",
		Port:   1883,
		Rules: []*Rule{
			{
				When: &When{
					PacketType: Publish,
				},
				Pipeline: publishPipeline,
			},
		},
	}
	return spec
}

func getBrokerFromSpec(spec *Spec) *Broker {
	store := newStorage(nil)
	broker := newBroker(spec, store, func(s, ss string) ([]string, error) {
		m := map[string]string{
			"test":  "http://localhost:8888/mqtt",
			"test1": "http://localhost:8889/mqtt",
		}
		urls := []string{}
		for k, v := range m {
			if k != s {
				urls = append(urls, v)
			}
		}
		return urls, nil
	})
	return broker
}

func getDefaultBroker() *Broker {
	spec := getDefaultSpec()
	return getBrokerFromSpec(spec)
}

func getConnectPipeline(t *testing.T) *pipeline.Pipeline {
	yamlStr := `
name: %s
kind: Pipeline
protocol: MQTT
filters:
- name: connect
  kind: MockMQTTFilter
  userName: test
  password: test
`
	yamlStr = fmt.Sprintf(yamlStr, connectPipeline)

	super := supervisor.NewDefaultMock()
	superSpec, err := super.NewSpec(yamlStr)
	require.Nil(t, err)
	pipe := &pipeline.Pipeline{}
	pipe.Init(superSpec)
	return pipe
}

func getPublishPipeline(t *testing.T) (*pipeline.Pipeline, *MockKafka) {
	yamlStr := `
name: %s
kind: Pipeline
protocol: MQTT
filters:
- name: publish
  kind: MockKafka
`
	yamlStr = fmt.Sprintf(yamlStr, publishPipeline)

	super := supervisor.NewDefaultMock()
	superSpec, err := super.NewSpec(yamlStr)
	require.Nil(t, err)
	pipe := &pipeline.Pipeline{}
	pipe.Init(superSpec)

	filter := pipeline.MockGetFilter(pipe, "publish")
	require.NotNil(t, filter)
	backend := filter.(*MockKafka)
	return pipe, backend
}

func checkSessionStore(broker *Broker, cid, topic string) error {
	checkFn := func() error {
		sessStr, err := broker.sessMgr.store.get(sessionStoreKey(cid))
		if err != nil {
			return err
		}
		if topic == "" {
			return nil
		}
		sess := Session{
			info: &SessionInfo{},
		}
		sess.decode(*sessStr)

		for t := range sess.info.Topics {
			if topic == t {
				return nil
			}
		}
		return fmt.Errorf("topic %v not in session %v", topic, cid)
	}

	for i := 0; i < 20; i++ {
		if checkFn() == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("session %v with topic %v not been stored", cid, topic)
}

func TestConnection(t *testing.T) {
	spec := getDefaultSpec()
	spec.Rules = append(spec.Rules, &Rule{
		When: &When{
			PacketType: Connect,
		},
		Pipeline: connectPipeline,
	})
	broker := getBrokerFromSpec(spec)
	defer broker.close()

	pipe := getConnectPipeline(t)
	defer pipe.Close()

	c1 := getDefaultMQTTClient(t, "test", true)
	c1.Disconnect(200)

	c2 := getUnConnectClient("test", "fakeuser", "fakepasswd", true)
	if token := c2.Connect(); token.Wait() && token.Error() == nil {
		t.Errorf("non auth user should fail%s", token.Error())
	}
	c2.Disconnect(200)
}

func TestPublish(t *testing.T) {
	broker := getDefaultBroker()
	defer broker.close()

	pipe, backend := getPublishPipeline(t)
	defer pipe.Close()

	client := getMQTTClient(t, "test", "test", "test", true)

	for i := 0; i < 5; i++ {
		topic := "go-mqtt/sample"
		text := fmt.Sprintf("qos0 msg #%d!", i)
		token := client.Publish(topic, 0, false, text)
		token.Wait()
		p := backend.get()
		if p.TopicName != topic || string(p.Payload) != text {
			t.Errorf("get wrong publish")
		}
	}

	for i := 0; i < 5; i++ {
		topic := "go-mqtt/sample"
		text := fmt.Sprintf("qos1 msg #%d!", i)
		token := client.Publish(topic, 1, false, text)
		token.Wait()
		if token.Error() != nil {
			t.Errorf("should support qos1")
		}
		p := backend.get()
		if p.TopicName != topic || string(p.Payload) != text {
			t.Errorf("get wrong publish")
		}
	}
	client.Disconnect(200)
}

func TestSubUnsub(t *testing.T) {
	broker := getDefaultBroker()
	defer broker.close()

	client := getMQTTClient(t, "test", "test", "test", true)
	if token := client.Subscribe("go-mqtt/qos0", 0, nil); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos0 error %s", token.Error())
	}
	if token := client.Subscribe("go-mqtt/qos1", 1, nil); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos1 error %s", token.Error())
	}
	if token := client.Unsubscribe("go-mqtt/sample"); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos1 error %s", token.Error())
	}
	client.Disconnect(200)
}

func TestCleanSession(t *testing.T) {
	broker := getDefaultBroker()
	defer broker.close()

	// client that set cleanSession
	cid := "cleanSessionClient"
	client := getMQTTClient(t, cid, "test", "test", true)
	if err := checkSessionStore(broker, cid, ""); err != nil {
		t.Fatal(err)
	}
	if token := client.Subscribe("test/cleanSession/0", 0, nil); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos0 error %s", token.Error())
	}
	if err := checkSessionStore(broker, cid, "test/cleanSession/0"); err != nil {
		t.Fatal(err)
	}

	client.Disconnect(200)

	c := broker.getClient(cid)
	if c != nil {
		c.closeAndDelSession()
	}
	subscribers, err := broker.topicMgr.findSubscribers("test/cleanSession/0")
	if err != nil {
		t.Errorf("findSubscribers for topic test/cleanSession/0 failed, %v", err)
	}
	if _, ok := subscribers[cid]; ok {
		t.Errorf("topicMgr is not cleaned when client %v close connection", subscribers)
	}
	_, err = broker.sessMgr.store.get(sessionStoreKey(cid))
	if err == nil {
		t.Errorf("not clean DB when cleanSession is set, potential resource wasted")
	}

	// client that not set cleanSession, getMQTTClient last parameter set to false
	cid = "notCleanSessionClient"
	client = getMQTTClient(t, cid, "test", "test", false)
	if token := client.Subscribe("test/cleanSession/0", 0, nil); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos0 error %s", token.Error())
	}

	// make sure before disconnect, session and subscribe topic has been stored in storage
	for i := 0; i < 10; i++ {
		time.Sleep(30 * time.Millisecond)
		yamlStr, err := broker.sessMgr.store.get(sessionStoreKey(cid))
		if err != nil {
			continue
		}
		session := &Session{
			info: &SessionInfo{},
		}
		session.decode(*yamlStr)
		if len(session.info.Topics) == 1 {
			break
		}
	}
	client.Disconnect(200)

	c = broker.getClient(cid)
	if c != nil {
		c.closeAndDelSession()
	}
	subscribers, err = broker.topicMgr.findSubscribers("test/cleanSession/0")
	if err != nil {
		t.Errorf("findSubscribers for topic test/cleanSession/0 failed, %v", err)
	}
	if _, ok := subscribers[cid]; ok {
		t.Errorf("topicMgr is not cleaned when client %v close connection", subscribers)
	}
	_, err = broker.sessMgr.store.get(sessionStoreKey(cid))
	if err != nil {
		t.Errorf("clean DB when cleanSession is not set")
	}

	// check not clean session work when client come.
	client = getMQTTClient(t, cid, "test", "test", false)
	// publish topic to make sure client read loop start
	token := client.Publish("topic", 1, false, "text")
	token.Wait()
	if token.Error() != nil {
		t.Errorf("client publish message failed %v", token.Error())
	}
	subscribers, err = broker.topicMgr.findSubscribers("test/cleanSession/0")
	if err != nil {
		t.Errorf("findSubscribers for topic test/cleanSession/0 failed, %v", err)
	}
	if _, ok := subscribers[cid]; !ok {
		t.Errorf("topicMgr should contain topic test/cleanSession/0 when client reconnect, but got %v", subscribers)
	}
	_, err = broker.sessMgr.store.get(sessionStoreKey(cid))
	if err != nil {
		t.Errorf("clean DB when cleanSession is not set")
	}
	client.Disconnect(200)
}

func TestMultiClientPublish(t *testing.T) {
	broker := getDefaultBroker()
	defer broker.close()

	pipe, backend := getPublishPipeline(t)
	defer pipe.Close()

	var wg sync.WaitGroup

	clientNum := 30
	msgNum := 50
	for i := 0; i < clientNum; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cid := fmt.Sprintf("test_%d", i)
			client := getMQTTClient(t, cid, "test", "test", true)
			for j := 0; j < msgNum; j++ {
				topic := cid
				text := fmt.Sprintf("%d", j)
				token := client.Publish(topic, 1, false, text)
				token.Wait()
				if token.Error() != nil {
					t.Errorf("client <%d> publish failed, <%s>", i, token.Error())
				}
			}
			client.Disconnect(200)
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		ans := make(map[string]int)
		for i := 0; i < clientNum*msgNum; i++ {
			p := backend.get()
			num, _ := strconv.Atoi(string(p.Payload))
			if val, ok := ans[p.TopicName]; ok {
				if num != val+1 {
					t.Errorf("received publish not in order")
				}
				ans[p.TopicName] = num
			} else {
				if num != 0 {
					t.Errorf("received publish not in order")
				}
				ans[p.TopicName] = num
			}
		}
	}()

	wg.Wait()
}

func TestSession(t *testing.T) {
	broker := getDefaultBroker()
	defer broker.close()

	client := getDefaultMQTTClient(t, "test", true)

	// sub go-mqtt/qos0 and check
	if token := client.Subscribe("go-mqtt/qos0", 0, nil); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos0 error %s", token.Error())
	}
	subs, qoss, _ := broker.sessMgr.get("test").allSubscribes()
	if subs[0] != "go-mqtt/qos0" || qoss[0] != 0 {
		t.Errorf("session error")
	}

	// sub go-mqtt/qos1 and check
	if token := client.Subscribe("go-mqtt/qos1", 1, nil); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos1 error %s", token.Error())
	}
	subs, qoss, _ = broker.sessMgr.get("test").allSubscribes()
	if len(subs) != 2 || len(qoss) != 2 {
		t.Errorf("session error")
	}

	// unsub go-mqtt/qos0 and check
	if token := client.Unsubscribe("go-mqtt/qos0"); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos1 error %s", token.Error())
	}
	subs, qoss, _ = broker.sessMgr.get("test").allSubscribes()
	if subs[0] != "go-mqtt/qos1" || qoss[0] != 1 {
		t.Errorf("session error")
	}

	sessMgr := broker.sessMgr
	sess := Session{
		info: &SessionInfo{
			EGName:    "eg",
			Name:      "mqtt",
			ClientID:  "checkStoreGet",
			Topics:    map[string]int{"a": 1},
			CleanFlag: false,
		},
	}
	str, _ := sess.encode()
	sessMgr.store.put(sessionStoreKey("checkStoreGet"), str)
	newSess := sessMgr.get("checkStoreGet")
	if !reflect.DeepEqual(newSess.info, sess.info) {
		t.Errorf("sessMgr get wrong sess %v, %v", sess.info, newSess.info)
	}
	client.Disconnect(200)
}

func TestSpec(t *testing.T) {
	yamlStr := `
    port: 1883
    auth:
      - userName: test
        passBase64: dGVzdA==
      - userName: admin
        passBase64: YWRtaW4=
    useTLS: true
    certificate:
      - name: cert1
        cert: balabala
        key: keyForbalabala
      - name: cert2
        cert: foo
        key: bar
`
	got := Spec{}
	err := yaml.Unmarshal([]byte(yamlStr), &got)
	if err != nil {
		t.Errorf("yaml unmarshal failed, err:%v", err)
	}

	want := Spec{
		Port:   1883,
		UseTLS: true,
		Certificate: []Certificate{
			{"cert1", "balabala", "keyForbalabala"},
			{"cert2", "foo", "bar"},
		},
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("got:<%#v>\n want:<%#v>\n", got, want)
	}
}

type CheckMsg struct {
	topic   string
	payload string
	qos     int
}

func TestSendMsgBack(t *testing.T) {
	clientNum := 10
	msgNum := 100
	subscribeCh := make(chan CheckMsg, 100)
	var wg sync.WaitGroup

	// make broker
	broker := getDefaultBroker()
	defer broker.close()

	pipe, backend := getPublishPipeline(t)
	defer pipe.Close()

	// subscribe handler
	handler := getMQTTSubscribeHandler(subscribeCh)
	// make client and subscribe different topic
	clients := []paho.Client{}
	for i := 0; i < clientNum; i++ {
		cid := fmt.Sprintf("test_%d", i)
		c := getMQTTClient(t, cid, "test", "test", true)
		if token := c.Subscribe(cid, 1, handler); token.Wait() && token.Error() != nil {
			t.Errorf("subscribe qos1 error %s", token.Error())
		}
		clients = append(clients, c)
	}
	fmt.Printf("TestSendMsgBack: finished get clients\n")

	// publish msg and send msg back
	for i := 0; i < clientNum; i++ {
		wg.Add(2)
		go func(c paho.Client) {
			defer wg.Done()
			r := c.OptionsReader()
			for j := 0; j < msgNum; j++ {
				topic := r.ClientID()
				text := fmt.Sprintf("%d", j)
				token := c.Publish(topic, 1, false, text)
				token.Wait()
				if token.Error() != nil {
					t.Errorf("client <%d> publish failed, <%s>", i, token.Error())
				}
			}
			fmt.Printf("client %s finished publish\n", r.ClientID())
		}(clients[i])

		go func(c paho.Client) {
			defer wg.Done()
			r := c.OptionsReader()
			for j := 0; j < msgNum; j++ {
				topic := r.ClientID()
				text := fmt.Sprintf("sub %d", j)
				broker.sendMsgToClient(nil, topic, []byte(text), QoS1)
			}
		}(clients[i])
	}
	fmt.Printf("TestSendMsgBack: finished publish and send msg back\n")

	wg.Add(2)
	// receive msg
	go func() {
		defer func() {
			fmt.Printf("all msg received!\n")
			wg.Done()
		}()
		ans := make(map[string]int)
		for i := 0; i < clientNum*msgNum; i++ {
			p := backend.get()
			num, _ := strconv.Atoi(string(p.Payload))
			if val, ok := ans[p.TopicName]; ok {
				if num != val+1 {
					t.Errorf("received publish not in order")
				}
				ans[p.TopicName] = num
			} else {
				if num != 0 {
					t.Errorf("received publish not in order")
				}
				ans[p.TopicName] = num
			}
		}
	}()

	// check get enough msg, there may be resend
	checkAns := func(ans map[string]map[string]struct{}) bool {
		if len(ans) < clientNum {
			return false
		}
		for _, v := range ans {
			if len(v) < msgNum {
				return false
			}
		}
		return true
	}

	// receive subscribes
	go func() {
		defer func() {
			fmt.Printf("all subscribes received!\n")
			wg.Done()
		}()
		ans := make(map[string]map[string]struct{})

		for {
			if checkAns(ans) {
				break
			}
			msg := <-subscribeCh
			if _, ok := ans[msg.topic]; ok {
				ans[msg.topic][msg.payload] = struct{}{}
			} else {
				ans[msg.topic] = make(map[string]struct{})
				ans[msg.topic][msg.payload] = struct{}{}
			}
		}

		for _, v := range ans {
			if len(v) != msgNum {
				t.Errorf("not receive right number of msg")
			}
		}
	}()

	fmt.Printf("TestSendMsgBack: wait received all msg and subscribes\n")
	// close
	wg.Wait()
	fmt.Printf("TestSendMsgBack: received all msg and subscribes\n")

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-subscribeCh:
			}
		}
	}()

	for _, c := range clients {
		go c.Disconnect(200)
	}
	close(done)
}

func TestYamlEncodeDecode(t *testing.T) {
	broker := getDefaultBroker()
	defer broker.close()

	// old session
	s := &Session{
		broker: broker,
		info: &SessionInfo{
			Topics:    map[string]int{"abc": 1, "def": 2},
			ClientID:  "test",
			CleanFlag: true,
		},
	}
	str, err := s.encode()
	if err != nil {
		t.Errorf("%s", err.Error())
	}

	// new session
	newS := Session{
		info: &SessionInfo{},
	}
	if err := newS.decode(str); err != nil {
		t.Errorf("yaml Decode error")
	}

	if !reflect.DeepEqual(newS.info.Topics, s.info.Topics) || newS.info.ClientID != s.info.ClientID || newS.info.CleanFlag != s.info.CleanFlag {
		t.Errorf("yaml encode decode error")
	}
}

type testServer struct {
	mux  *http.ServeMux
	srv  http.Server
	addr string
}

func newServer(addr string) *testServer {
	mux := http.NewServeMux()
	srv := http.Server{Addr: addr, Handler: mux}
	ts := &testServer{
		mux:  mux,
		srv:  srv,
		addr: addr,
	}
	return ts
}

func (ts *testServer) addHandlerFunc(pattern string, f http.HandlerFunc) {
	ts.mux.HandleFunc(pattern, f)
}

func (ts *testServer) start() error {
	go ts.srv.ListenAndServe()
	// Poll server until it is ready
	for t := 0; t < 25; t++ {
		time.Sleep(50 * time.Millisecond)
		req, _ := http.NewRequest(http.MethodGet, "http://localhost"+ts.addr, nil)
		_, err := http.DefaultClient.Do(req)
		if err == nil {
			return nil
		}
	}
	return errors.New("Server did not respond")
}

func (ts *testServer) shutdown() {
	ts.srv.Shutdown(context.Background())
}

func topicsPublish(t *testing.T, data HTTPJsonData) int {
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Errorf("json marshal error")
	}
	req, err := http.NewRequest(http.MethodPost, "http://localhost:8888/mqtt", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Errorf("request mqtt error")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("mqtt client do error, %s", err.Error())
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestHTTPRequest(t *testing.T) {
	broker := getDefaultBroker()
	defer broker.close()

	srv := newServer(":8888")
	srv.addHandlerFunc("/mqtt", broker.httpTopicsPublishHandler)
	if err := srv.start(); err != nil {
		t.Errorf("couldn't start server: %s", err)
	}

	// GET request should fail
	req, _ := http.NewRequest(http.MethodGet, "http://localhost:8888/mqtt", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("client do req error %s", err)
	}
	if resp.StatusCode == http.StatusOK {
		t.Errorf("http GET should fail, only POST permitted")
	}
	resp.Body.Close()

	// non json data should fail
	req, err = http.NewRequest(http.MethodPost, "http://localhost:8888/mqtt", bytes.NewBuffer([]byte("non json")))
	if err != nil {
		t.Errorf("request mqtt error")
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("mqtt client do error")
	}
	if resp.StatusCode == http.StatusOK {
		t.Errorf("http post should put json data")
	}
	resp.Body.Close()

	// err qos should fail
	data := HTTPJsonData{
		Topic:   "topic",
		QoS:     10,
		Payload: "data",
		Base64:  false,
	}
	statusCode := topicsPublish(t, data)
	if statusCode == http.StatusOK {
		t.Errorf("qos must be 0 or 1 or 2")
	}

	// base64 flag
	data = HTTPJsonData{
		Topic:   "topic",
		QoS:     10,
		Payload: "data",
		Base64:  true,
	}
	statusCode = topicsPublish(t, data)
	if statusCode == http.StatusOK {
		t.Errorf("not use base64 but set base64 true")
	}

	// success
	data = HTTPJsonData{
		Topic:   "topic",
		QoS:     1,
		Payload: "data",
		Base64:  false,
	}
	statusCode = topicsPublish(t, data)
	if statusCode != http.StatusOK {
		t.Errorf("request should success")
	}
	data = HTTPJsonData{
		Topic:   "topic",
		QoS:     1,
		Payload: base64.StdEncoding.EncodeToString([]byte("data")),
		Base64:  true,
	}
	statusCode = topicsPublish(t, data)
	if statusCode != http.StatusOK {
		t.Errorf("request should success")
	}

	srv.shutdown()
}

func TestHTTPPublish(t *testing.T) {
	broker := getDefaultBroker()
	defer broker.close()

	srv := newServer(":8888")
	srv.addHandlerFunc("/mqtt", broker.httpTopicsPublishHandler)
	srv.start()

	subscribeCh := make(chan CheckMsg)
	handler := getMQTTSubscribeHandler(subscribeCh)
	client := getMQTTClient(t, "test", "test", "test", true)
	if token := client.Subscribe("test", 1, handler); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos1 error %s", token.Error())
	}

	numMsg := 200
	go func() {
		for i := 0; i < numMsg; i++ {
			data := HTTPJsonData{
				Topic:   "test",
				QoS:     1,
				Payload: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", i))),
				Base64:  true,
			}
			func(data HTTPJsonData) {
				code := topicsPublish(t, data)
				if code != http.StatusOK {
					t.Errorf("wrong status code return")
				}
			}(data)
		}
	}()

	ans := map[string]struct{}{}
	for len(ans) < numMsg {
		msg := <-subscribeCh
		if msg.topic != "test" {
			t.Errorf("wrong topic received")
		}
		ans[msg.payload] = struct{}{}
	}

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-subscribeCh:
			}
		}
	}()

	client.Disconnect(200)
	srv.shutdown()
	close(done)
}

func TestHTTPTransfer(t *testing.T) {
	broker0 := getDefaultBroker()

	srv0 := newServer(":8888")
	srv0.addHandlerFunc("/mqtt", broker0.httpTopicsPublishHandler)
	srv0.start()

	spec := getDefaultSpec()
	spec.EGName = "test1"
	spec.Name = "test1"
	spec.Port = 1884
	broker1 := getBrokerFromSpec(spec)

	srv1 := newServer(":8889")
	srv1.addHandlerFunc("/mqtt", broker1.httpTopicsPublishHandler)
	srv1.start()

	// auto connect to broker0, not broker1
	ch := make(chan CheckMsg)
	handler := getMQTTSubscribeHandler(ch)
	mqttClient := getMQTTClient(t, "client", "test", "test", true)
	if token := mqttClient.Subscribe("client", 1, handler); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos1 error %s", token.Error())
	}

	// set data to broker1
	data := HTTPJsonData{
		Topic:   "client",
		QoS:     1,
		Payload: "data",
		Base64:  false,
	}
	jsonData, _ := json.Marshal(data)
	req, _ := http.NewRequest(http.MethodPost, "http://localhost:8889/mqtt", bytes.NewBuffer(jsonData))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("mqtt client do error")
	}
	defer resp.Body.Close()

	// get data from client who connect to broker0
	msg := <-ch
	if msg.topic != "client" || msg.payload != "data" {
		t.Errorf("wrong msg")
	}
	go func() {
		msg = <-ch
		t.Errorf("receive data again, may transfer data for multi times")
	}()

	broker0.close()
	broker1.close()
	srv0.shutdown()
	srv1.shutdown()

	go func() {
		for {
			<-ch
		}
	}()
	mqttClient.Disconnect(200)
}

func TestSplitTopic(t *testing.T) {
	tests := []struct {
		topic  string
		levels []string
		ans    bool
	}{
		{"#", []string{"#"}, true},
		{"#/sport", nil, false},
		{"sport#", nil, false},
		{"/#/sport", nil, false},
		{"#+", nil, false},
		{"/+/sport/+/player1/+/score", []string{"", "+", "sport", "+", "player1", "+", "score"}, true},
		{"/+/++/+finance", nil, false},
		{"/sport/ball/player1/score", []string{"", "sport", "ball", "player1", "score"}, true},
		{"finance/+/bank/#", []string{"finance", "+", "bank", "#"}, true},
		{"/sport/ball/player1/score/+/wimbledon/#", []string{"", "sport", "ball", "player1", "score", "+", "wimbledon", "#"}, true},
		{"/sport/ball/player2/score/+/game2/#", []string{"", "sport", "ball", "player2", "score", "+", "game2", "#"}, true},
		{"/", []string{"", ""}, true},
		{"finance/", []string{"finance", ""}, true},
		{"//", []string{"", "", ""}, true},
		{"/finance/bank", []string{"", "finance", "bank"}, true},
		{"/finance/bank/", []string{"", "finance", "bank", ""}, true},
		{"sport/tennis/player1/#", []string{"sport", "tennis", "player1", "#"}, true},
		{"sport/tennis/player1", []string{"sport", "tennis", "player1"}, true},
		{"sport/tennis/player1/ranking", []string{"sport", "tennis", "player1", "ranking"}, true},
		{"sport/tennis/player1/score/wimbledon", []string{"sport", "tennis", "player1", "score", "wimbledon"}, true},
		{"sport/#", []string{"sport", "#"}, true},
		{"sport/tennis/#", []string{"sport", "tennis", "#"}, true},
		{"sport/tennis#", nil, false},
		{"sport/tennis/#/ranking", nil, false},
		{"sport/tennis/+", []string{"sport", "tennis", "+"}, true},
		{"sport/tennis/player1", []string{"sport", "tennis", "player1"}, true},
		{"+", []string{"+"}, true},
		{"+/tennis/#", []string{"+", "tennis", "#"}, true},
		{"sport+", nil, false},
		{"sport/+/player1", []string{"sport", "+", "player1"}, true},
		{"/finance", []string{"", "finance"}, true},
	}
	for _, tt := range tests {
		levels, ans := splitTopic(tt.topic)
		if !reflect.DeepEqual(levels, tt.levels) {
			t.Errorf("topic:<%s> levels got:%v, want:%v", tt.topic, levels, tt.levels)
		}
		if ans != tt.ans {
			t.Errorf("topic:<%s>, got:%v, want:%v", tt.topic, ans, tt.ans)
		}
	}
}

func TestWildCard(t *testing.T) {
	mgr := newTopicManager(10000)
	mgr.subscribe([]string{"a/+", "b/d"}, []byte{0, 1}, "A")
	mgr.subscribe([]string{"+/+"}, []byte{1}, "B")
	mgr.subscribe([]string{"+/fin"}, []byte{1}, "C")
	mgr.subscribe([]string{"a/fin"}, []byte{1}, "D")
	mgr.subscribe([]string{"#"}, []byte{2}, "E")
	mgr.subscribe([]string{"a/#"}, []byte{2}, "F")

	subscribers, _ := mgr.findSubscribers("a/fin")
	want := map[string]byte{"A": 0, "B": 1, "C": 1, "D": 1, "E": 2, "F": 2}
	if !reflect.DeepEqual(subscribers, want) {
		t.Errorf("test wild card want:%v, got:%v\n", want, subscribers)
	}

	subscribers, _ = mgr.findSubscribers("b/d")
	want = map[string]byte{"A": 1, "B": 1, "E": 2}
	if !reflect.DeepEqual(subscribers, want) {
		t.Errorf("test wild card want:%v, got:%v\n", want, subscribers)
	}

	subscribers, _ = mgr.findSubscribers("b/d/f")
	want = map[string]byte{"E": 2}
	if !reflect.DeepEqual(subscribers, want) {
		t.Errorf("test wild card want:%v, got:%v\n", want, subscribers)
	}

	err := mgr.subscribe([]string{"++", "##"}, []byte{0, 1}, "A")
	if err == nil {
		t.Errorf("subscribe invalid topic should return error")
	}
	err = mgr.unsubscribe([]string{"++", "##", "/a/b/++"}, "A")
	if err == nil {
		t.Errorf("unsubscribe invalid topic should return error")
	}
	_, err = mgr.findSubscribers("##")
	if err == nil {
		t.Errorf("find subscribers for invalid topic should return error")
	}

	mgr = newTopicManager(100000)
	mgr.subscribe([]string{"a/b/c/d/e"}, []byte{0}, "A")
	mgr.subscribe([]string{"a/b/c/f/g"}, []byte{0}, "A")
	mgr.subscribe([]string{"m/x/v/f/g"}, []byte{0}, "B")
	mgr.subscribe([]string{"m/x"}, []byte{0}, "C")

	mgr.unsubscribe([]string{"a/b/c/d/e"}, "A")
	mgr.unsubscribe([]string{"a/b/c/f/g"}, "A")
	mgr.unsubscribe([]string{"m/x/v/f/g"}, "B")
	mgr.unsubscribe([]string{"m/x"}, "C")
	if len(mgr.root.clients) != 0 || len(mgr.root.nodes) != 0 {
		t.Errorf("topic manager not clear memory when topics are unsubscribes")
	}

	mgr = newTopicManager(1000000)
	checkSubscriptions := func(subTopic string, recvTopic []string, notRecvTopic []string) {
		mgr.subscribe([]string{subTopic}, []byte{0}, "tmpClient")
		for _, topic := range recvTopic {
			clients, err := mgr.findSubscribers(topic)
			if err != nil {
				t.Errorf("findSubscribers for topic failed, %v", err)
			}
			if val, ok := clients["tmpClient"]; !ok || (val != 0) {
				t.Errorf("subscribe topic %v but not recv topic %v", subTopic, topic)
			}
		}
		for _, topic := range notRecvTopic {
			clients, err := mgr.findSubscribers(topic)
			if err != nil {
				t.Errorf("findSubscribers for topic failed, %v", err)
			}
			if _, ok := clients["tmpClient"]; ok {
				t.Errorf("subscribe topic %v but recv topic %v", subTopic, topic)
			}
		}
		mgr.unsubscribe([]string{subTopic}, "tmpClient")
	}
	checkSubscriptions("sport/tennis/player1/#", []string{"sport/tennis/player1", "sport/tennis/player1/ranking", "sport/tennis/player1/score/wimbledon"}, []string{})
	checkSubscriptions("sport/#", []string{"sport"}, []string{})

	checkSubscriptions("sport/tennis/+", []string{"sport/tennis/player1", "sport/tennis/player2"}, []string{"sport/tennis/player1/ranking"})
	checkSubscriptions("sport/+", []string{"sport/"}, []string{"sport"})
	checkSubscriptions("+/+", []string{"/finance"}, []string{})
	checkSubscriptions("/+", []string{"/finance"}, []string{})
	checkSubscriptions("+", []string{}, []string{"/finance"})
}

// this certPem and keyPem come from golang crypto/tls/testdata
// with original name: example-key.pem and example-key.pem
const certPem = `
-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----
`

const keyPem = `
-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----
`

func TestTLSConfig(t *testing.T) {
	spec := Spec{}
	_, err := spec.tlsConfig()
	if err == nil {
		t.Errorf("no certificate, should return error")
	}

	spec = Spec{
		Certificate: []Certificate{
			{"fake", "fakeCert", "fakeKey"},
		},
	}
	_, err = spec.tlsConfig()
	if err == nil {
		t.Errorf("no vallid certificate, should return error")
	}

	spec = Spec{
		Certificate: []Certificate{
			{"demo", string(certPem), string(keyPem)},
		},
	}
	_, err = spec.tlsConfig()
	if err != nil {
		t.Errorf("should return nil for correct cert and key pair err:<%v>", err)
	}
}

func TestSessMgr(t *testing.T) {
	broker := getDefaultBroker()
	defer broker.close()

	sessMgr := broker.sessMgr
	sess := &Session{
		storeCh: sessMgr.storeCh,
		info: &SessionInfo{
			EGName:    "testEg",
			Name:      "mqttProxy",
			Topics:    map[string]int{"a": 1, "b": 0},
			ClientID:  "testClient",
			CleanFlag: true,
		},
	}
	sessStr, err := sess.encode()
	if err != nil {
		t.Errorf("session encode failed, err:%v", err)
	}
	newSess := sessMgr.newSessionFromYaml(&sessStr)
	if !reflect.DeepEqual(sess.info, newSess.info) {
		t.Errorf("sessMgr produce wrong session")
	}
}

func TestClient(t *testing.T) {
	assert := assert.New(t)
	svcConn, clientConn := net.Pipe()
	go func() {
		io.ReadAll(svcConn)
	}()
	connect := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
	connect.ClientIdentifier = "cid"
	connect.Username = "username"
	connect.WillFlag = true
	connect.WillQos = 1
	connect.WillTopic = "will"
	connect.WillMessage = []byte("i am gone")

	client := newClient(connect, nil, clientConn, nil)
	will := client.info.will
	if (will.Qos != connect.WillQos) || (will.TopicName != connect.WillTopic) || string(will.Payload) != string(connect.WillMessage) {
		t.Error("produce wrong will msg")
	}

	err := client.processPacket(connect)
	if err == nil {
		t.Errorf("double connect should return error")
	}
	connack := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
	err = client.processPacket(connack)
	if err == nil {
		t.Errorf("client should not send connack")
	}

	pubrec := packets.NewControlPacket(packets.Pubrec).(*packets.PubrecPacket)
	err = client.processPacket(pubrec)
	if err == nil {
		t.Errorf("qos2 not support now")
	}

	suback := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
	err = client.processPacket(suback)
	if err == nil {
		t.Errorf("server not subscribe")
	}
	unsuback := packets.NewControlPacket(packets.Unsuback).(*packets.UnsubackPacket)
	err = client.processPacket(unsuback)
	if err == nil {
		t.Errorf("server not subscribe")
	}

	pingreq := packets.NewControlPacket(packets.Pingreq).(*packets.PingreqPacket)
	err = client.processPacket(pingreq)
	if err != nil {
		t.Errorf("ping should success")
	}
	pingresp := packets.NewControlPacket(packets.Pingresp).(*packets.PingrespPacket)
	err = client.processPacket(pingresp)
	if err == nil {
		t.Errorf("broker not ping")
	}

	assert.Equal("username", client.UserName())
	client.Store("key", "value")

	value, ok := client.Load("key")
	assert.Equal(true, ok)
	assert.Equal("value", value)

	client.Delete("key")
	_, ok = client.Load("key")
	assert.Equal(false, ok)
}

func TestBrokerListen(t *testing.T) {
	// invalid tls
	spec := &Spec{
		Name:   "test-1",
		EGName: "test-1",
		Port:   1883,
		UseTLS: true,
		Certificate: []Certificate{
			{"fake", "abc", "abc"},
		},
	}
	broker := getBrokerFromSpec(spec)
	if broker != nil {
		t.Errorf("invalid tls config should return nil broker")
	}

	// valid tls
	spec = &Spec{
		Name:   "test-1",
		EGName: "test-1",
		Port:   1883,
		UseTLS: true,
		Certificate: []Certificate{
			{"demo", certPem, keyPem},
		},
	}
	broker = getBrokerFromSpec(spec)
	if broker == nil {
		t.Errorf("valid tls config should not return nil broker")
	}

	broker1 := getBrokerFromSpec(spec)
	if broker1 != nil {
		t.Errorf("not valid port should return nil broker")
	}

	// not valid port should return nil
	spec = &Spec{
		Name:   "test-1",
		EGName: "test-1",
		Port:   1883,
	}
	broker2 := getBrokerFromSpec(spec)
	if broker2 != nil {
		t.Errorf("not valid port should return nil broker")
	}
	broker.mqttAPIPrefix(mqttAPITopicPublishPrefix)
	broker.registerAPIs()
	broker.close()
}

func TestBrokerHandleConn(t *testing.T) {
	broker := getDefaultBroker()

	// broker handleConn return if error happen
	svcConn, clientConn := net.Pipe()
	go clientConn.Write([]byte("fake data for paho.packets.ReadPacket to return error, to make that happens, this fake data should be long enough."))
	broker.handleConn(svcConn)

	// not use connect to connect
	svcConn, clientConn = net.Pipe()
	subscribe := packets.NewControlPacket(packets.Subscribe).(*packets.SubscribePacket)
	go subscribe.Write(clientConn)
	broker.handleConn(svcConn)

	// use in valid connect to connect
	svcConn, clientConn = net.Pipe()
	connect := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
	connect.ReservedBit = 1
	go connect.Write(clientConn)
	go func() {
		io.ReadAll(clientConn)
	}()
	broker.handleConn(svcConn)
	broker.close()
}

func TestMQTTProxy(t *testing.T) {
	mp := MQTTProxy{}
	mp.Status()

	broker := getDefaultBroker()

	mp.broker = broker
	broker.reconnectWatcher()
	mp.Close()

	ans, err := updatePort("http://example.com:1234", "demo.com:2345")
	assert.Nil(t, err)
	assert.Equal(t, "http://example.com:2345", ans)
}

func TestPipeline(t *testing.T) {
	// create test pipeline first
	yamlStr := `
name: mqtt-test-pipeline
kind: Pipeline
protocol: MQTT
flow:
- filter: mqtt-filter
filters:
- name: mqtt-filter
  kind: MockMQTTFilter
  userName: test
  port: 1234
  backendType: Kafka
  keysToStore:
  - filter
  publishBackendClientID: true
`

	super := supervisor.NewDefaultMock()
	superSpec, err := super.NewSpec(yamlStr)
	if err != nil {
		t.Errorf("supervisor unmarshal yaml failed, %s", err)
		t.Skip()
	}
	pipe := pipeline.Pipeline{}
	pipe.Init(superSpec)
	defer pipe.Close()

	publishPipe, backend := getPublishPipeline(t)
	defer publishPipe.Close()

	// get broker
	broker := getDefaultBroker()
	broker.pipelines[Subscribe] = "mqtt-test-pipeline"
	broker.pipelines[Unsubscribe] = "mqtt-test-pipeline"
	broker.pipelines[Disconnect] = "mqtt-test-pipeline"
	defer broker.close()

	// set some client
	clientNum := 20
	for i := 0; i < clientNum; i++ {
		client := getMQTTClient(t, strconv.Itoa(i), "test", "test", true)
		topic := fmt.Sprintf("client-%d", i)
		text := "text"
		token := client.Publish(topic, 1, false, text)
		token.Wait()

		token = client.Subscribe(strconv.Itoa(i), 1, nil)
		token.Wait()

		token = client.Unsubscribe(strconv.Itoa(i))
		token.Wait()

		p := backend.get()
		if p.TopicName != topic || string(p.Payload) != text {
			t.Errorf("get wrong publish")
		}
		c := broker.getClient(strconv.Itoa(i))
		if _, ok := c.Load("filter"); !ok {
			t.Errorf("filter write key value failed")
		}
		client.Disconnect(200)
	}
	// wait all close
	for i := 0; i < 10; i++ {
		num := len(broker.currentClients())
		if num == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	filterStatus := pipe.Status().ObjectStatus.(*pipeline.Status).Filters["mqtt-filter"].(pipeline.MockMQTTStatus)
	if len(filterStatus.ClientCount) != clientNum {
		t.Errorf("filter get wrong result %v for client num", len(filterStatus.ClientCount))
	}
	if len(filterStatus.ClientDisconnect) != clientNum {
		t.Errorf("filter get wrong result %v for client disconnect", filterStatus.ClientDisconnect)
	}
	if len(filterStatus.Subscribe) != clientNum {
		t.Errorf("filter get wrong result %v for client subscribe", filterStatus.Subscribe)
	}
	if len(filterStatus.Unsubscribe) != clientNum {
		t.Errorf("filter get wrong result %v for client unsubscribe", filterStatus.Unsubscribe)
	}
}

func TestAuthByPipeline(t *testing.T) {
	broker := getDefaultBroker()
	broker.pipelines[Connect] = "mqtt-test-pipeline"
	defer broker.close()

	// since there are no such pipeline
	// connect to broker will fail
	option := paho.NewClientOptions().AddBroker("tcp://0.0.0.0:1883").SetClientID("test").SetUsername("fakeuser").SetPassword("fakepasswd")
	client := paho.NewClient(option)
	if token := client.Connect(); token.Wait() && token.Error() == nil {
		t.Errorf("non auth user should fail%s", token.Error())
	}
	client.Disconnect(200)

	// filter with auth username and passwd
	yamlStr := `
name: mqtt-test-pipeline
kind: Pipeline
protocol: MQTT
flow:
- filter: mqtt-filter
filters:
- name: mqtt-filter
  kind: MockMQTTFilter
  userName: filter-auth-name
  password: filter-auth-passwd
  connectKey: connect`

	super := supervisor.NewDefaultMock()
	superSpec, err := super.NewSpec(yamlStr)
	if err != nil {
		t.Errorf("supervisor unmarshal yaml failed, %s", err)
		t.Skip()
	}
	pipe := pipeline.Pipeline{}
	pipe.Init(superSpec)
	defer pipe.Close()

	// same username and passwd with filter, success
	option = paho.NewClientOptions().AddBroker("tcp://0.0.0.0:1883").SetClientID("test").SetUsername("filter-auth-name").SetPassword("filter-auth-passwd")
	client = paho.NewClient(option)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		t.Errorf("auth user should success %s", token.Error())
	}
	c := broker.getClient("test")
	if _, ok := c.Load("connect"); !ok {
		t.Errorf("filter set connect key failed")
	}

	client.Disconnect(200)

	// different username and passwd with filer, fail
	option = paho.NewClientOptions().AddBroker("tcp://0.0.0.0:1883").SetClientID("test").SetUsername("fake").SetPassword("fakepasswd")
	client = paho.NewClient(option)
	if token := client.Connect(); token.Wait() && token.Error() == nil {
		t.Errorf("non auth user should fail %s", token.Error())
	}
	client.Disconnect(200)
}

func TestMaxAllowedConnection(t *testing.T) {
	spec := getDefaultSpec()
	spec.MaxAllowedConnection = 10
	broker := getBrokerFromSpec(spec)
	defer broker.close()

	clients := []paho.Client{}
	clientNum := 10
	for i := 0; i < clientNum; i++ {
		client := getMQTTClient(t, strconv.Itoa(i), "test", "test", true)
		clients = append(clients, client)
	}
	var num int
	for i := 0; i < 10; i++ {
		num = len(broker.currentClients())
		if num == clientNum {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if num != clientNum {
		t.Fatalf("wrong client connection number, got %v, expected %v", num, clientNum)
	}
	for i := clientNum; i < 2*clientNum; i++ {
		client := getUnConnectClient(strconv.Itoa(i), "test", "test", true)
		if token := client.Connect(); token.Wait() && token.Error() == nil {
			t.Errorf("client %v connect should fail but got nil error, %v", i, token.Error())
		}
	}
	for _, c := range clients {
		c.Disconnect(200)
	}
}

func TestConnectionLimit(t *testing.T) {
	spec := getDefaultSpec()
	spec.ConnectionLimit = &RateLimit{
		RequestRate: 10,
		TimePeriod:  1000,
	}
	broker := getBrokerFromSpec(spec)
	defer broker.close()

	// use all rate
	for i := 0; i < 10; i++ {
		broker.connectionLimiter.acquirePermission(1000)
	}
	client := getUnConnectClient("test", "test", "test", true)
	if token := client.Connect(); token.Wait() && token.Error() == nil {
		t.Errorf("client test connect should fail but got nil error, %v", token.Error())
	}
}

func TestClientPublishLimit(t *testing.T) {
	spec := getDefaultSpec()
	spec.ClientPublishLimit = &RateLimit{
		RequestRate: 10,
		TimePeriod:  1000,
	}
	broker := getBrokerFromSpec(spec)
	defer broker.close()

	client := getMQTTClient(t, "test", "test", "test", true)
	time.Sleep(50 * time.Millisecond)

	// acquire all permission
	c := broker.getClient("test")
	for i := 0; i < 10; i++ {
		c.publishLimit.acquirePermission(100)
	}

	token := client.Publish("123", 1, false, []byte("test"))
	if token.WaitTimeout(1 * time.Second) {
		t.Errorf("client publish should fail, since we set client publish limit")
	}
}

func TestHTTPGetAllSession(t *testing.T) {
	broker := getDefaultBroker()
	defer broker.close()

	// connect 10 clients
	clients := []paho.Client{}
	clientNum := 10
	for i := 0; i < clientNum; i++ {
		cid := strconv.Itoa(i)
		client := getMQTTClient(t, cid, "test", "test", true)
		if err := checkSessionStore(broker, cid, ""); err != nil {
			t.Fatal(err)
		}
		if token := client.Subscribe("topic", 1, nil); token.Wait() && token.Error() != nil {
			t.Errorf("subscribe qos0 error %s", token.Error())
		}
		if err := checkSessionStore(broker, cid, "topic"); err != nil {
			t.Fatal(err)
		}
		clients = append(clients, client)
	}

	// we use goroutine to store session, make sure all sessions have been stored before we go forward.
	for i := 0; i < 20; i++ {
		sessions, _ := broker.sessMgr.store.getPrefix(sessionStoreKey(""), true)
		if len(sessions) == clientNum {
			break
		}
		time.Sleep(50 * time.Millisecond)
		if i == 19 && len(sessions) != clientNum {
			t.Fatalf("not all sessions have been stored %v", sessions)
		}
	}

	// start server
	srv := newServer(":8888")
	srv.addHandlerFunc("/session/query", broker.httpGetAllSessionHandler)
	if err := srv.start(); err != nil {
		t.Errorf("couldn't start server: %s", err)
	}
	defer srv.shutdown()

	tests := []struct {
		url    string
		ok     bool
		ansLen int
	}{
		{"http://localhost:8888/session/query?page=1&page_size=20&q=", true, 10},
		{"http://localhost:8888/session/query?page=1&page_size=8&q=", true, 8},
		{"http://localhost:8888/session/query", true, 10},
		{"http://localhost:8888/session/query?page=1", false, 0},
		{"http://localhost:8888/session/query?page_size=2", false, 0},
		{"http://localhost:8888/session/query?q=", false, 0},
		{"http://localhost:8888/session/query?page=0&page_size=10&q=", false, 0},
		{"http://localhost:8888/session/query?page=1&page_size=0&q=", false, 0},
		{"http://localhost:8888/session/query?page=2&page_size=20&q=", true, 0},
		{"http://localhost:8888/session/query?page=1&page_size=10&q=233", true, 0},
	}
	for _, test := range tests {
		req, err := http.NewRequest(http.MethodGet, test.url, nil)
		if err != nil {
			t.Errorf("get request failed, %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Errorf("client request failed, %v", err)
		}
		ok := resp.StatusCode == http.StatusOK
		if ok != test.ok {
			t.Errorf("get wrong result")
		}
		if ok {
			sessions := &HTTPSessions{}
			json.NewDecoder(resp.Body).Decode(sessions)
			if len(sessions.Sessions) != test.ansLen {
				t.Errorf("get wrong session number wanted %v, got %v", test.ansLen, len(sessions.Sessions))
				sessions, _ := broker.sessMgr.store.getPrefix(sessionStoreKey(""), true)
				broker.Lock()
				t.Errorf("broker clients %v, sessions %v", broker.clients, sessions)
				broker.Unlock()
			}
		}
		resp.Body.Close()
	}
	for _, c := range clients {
		c.Disconnect(200)
	}
}

func TestHTTPDeleteSession(t *testing.T) {
	broker := getDefaultBroker()
	defer broker.close()

	// connect 10 clients
	clients := []paho.Client{}
	clientNum := 10
	for i := 0; i < clientNum; i++ {
		client := getMQTTClient(t, strconv.Itoa(i), "test", "test", true)
		clients = append(clients, client)
	}

	// start server
	srv := newServer(":8888")
	srv.addHandlerFunc("/sessions", broker.httpDeleteSessionHandler)
	if err := srv.start(); err != nil {
		t.Errorf("couldn't start server: %s", err)
	}
	defer srv.shutdown()

	data := HTTPSessions{
		Sessions: []*HTTPSession{
			{SessionID: "1"},
			{SessionID: "2"},
		},
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Errorf("marshal http session %v failed, %v", data, err)
	}
	req, _ := http.NewRequest(http.MethodDelete, "http://localhost:8888/sessions", bytes.NewBuffer(jsonData))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete session failed, %v", err)
	}
	defer resp.Body.Close()

	for i := 0; i < 10; i++ {
		if len(broker.currentClients()) == clientNum-2 {
			break
		}
		if i == 9 {
			t.Errorf("session delete failed %v", broker.currentClients())
		}
		time.Sleep(50 * time.Millisecond)
	}

	for _, c := range clients {
		c.Disconnect(200)
	}
}

func TestHTTPTransferHeaderCopy(t *testing.T) {
	done := make(chan bool, 2)

	broker0 := getDefaultBroker()
	srv0 := newServer(":8888")
	srv0.addHandlerFunc("/mqtt", func(w http.ResponseWriter, r *http.Request) {
		broker0.httpTopicsPublishHandler(w, r)
		done <- true
	})
	srv0.start()

	spec := getDefaultSpec()
	spec.Name = "test1"
	spec.EGName = "test1"
	spec.Port = 1884
	broker1 := getBrokerFromSpec(spec)

	srv1 := newServer(":8889")
	srv1.addHandlerFunc("/mqtt", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(b3.TraceID) != "123" {
			t.Errorf("wrong trace id received")
		}
		done <- true
	})
	srv1.start()

	// set data to broker1
	data := HTTPJsonData{
		Topic:   "client",
		QoS:     1,
		Payload: "data",
		Base64:  false,
	}
	jsonData, _ := json.Marshal(data)
	req, _ := http.NewRequest(http.MethodPost, "http://localhost:8888/mqtt", bytes.NewBuffer(jsonData))
	req.Header.Add(b3.TraceID, "123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("mqtt client do error")
	}
	defer resp.Body.Close()

	<-done
	<-done
	broker0.close()
	broker1.close()
	srv0.shutdown()
	srv1.shutdown()
}
