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

package kafka

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

type mockAsyncProducer struct {
	ch chan *sarama.ProducerMessage
}

func (m *mockAsyncProducer) AsyncClose()                               {}
func (m *mockAsyncProducer) Successes() <-chan *sarama.ProducerMessage { return nil }
func (m *mockAsyncProducer) Errors() <-chan *sarama.ProducerError      { return nil }

func (m *mockAsyncProducer) Input() chan<- *sarama.ProducerMessage {
	return m.ch
}

func (m *mockAsyncProducer) Close() error {
	return fmt.Errorf("mock producer close failed")
}

var _ sarama.AsyncProducer = (*mockAsyncProducer)(nil)

func newMockAsyncProducer() sarama.AsyncProducer {
	return &mockAsyncProducer{
		ch: make(chan *sarama.ProducerMessage, 100),
	}
}

func defaultFilterSpec(spec *Spec) *httppipeline.FilterSpec {
	meta := &httppipeline.FilterMetaSpec{
		Name:     "kafka",
		Kind:     Kind,
		Pipeline: "pipeline-demo",
	}
	filterSpec := httppipeline.MockFilterSpec(nil, nil, "", meta, spec)
	return filterSpec
}

func TestKafka(t *testing.T) {
	assert := assert.New(t)
	k := &Kafka{}
	filterSpec := defaultFilterSpec(&Spec{})

	assert.NotEmpty(k.Description())
	assert.Nil(k.Status())
	assert.Panics(func() { k.Init(filterSpec) }, "no valid backend should panic")

	newK := &Kafka{}
	assert.Panics(func() { newK.Inherit(filterSpec, k) })
}

func TestHandleHTTP(t *testing.T) {
	assert := assert.New(t)
	kafka := Kafka{
		spec: &Spec{
			Topic: &Topic{
				Default: "default-topic",
				Dynamic: &Dynamic{
					Header: "x-kafka-topic",
				},
			},
		},
		producer: newMockAsyncProducer(),
		done:     make(chan struct{}),
	}
	kafka.setHeader(kafka.spec)
	go kafka.checkProduceError()
	defer kafka.Close()

	// test header
	req, err := http.NewRequest(http.MethodPost, "127.0.0.1", strings.NewReader("text"))
	assert.Nil(err)
	req.Header.Add("x-kafka-topic", "kafka")
	w := httptest.NewRecorder()
	ctx := context.New(w, req, tracing.NoopTracing, "no trace")
	ctx.SetHandlerCaller(func(lastResult string) string {
		return lastResult
	})

	ans := kafka.Handle(ctx)
	assert.Equal("", ans)

	msg := <-kafka.producer.(*mockAsyncProducer).ch
	assert.Equal("kafka", msg.Topic)
	assert.Equal(0, len(msg.Headers))
	value, err := msg.Value.Encode()
	assert.Nil(err)
	assert.Equal("text", string(value))

	// test default
	req, err = http.NewRequest(http.MethodPost, "127.0.0.1", strings.NewReader("text"))
	assert.Nil(err)
	w = httptest.NewRecorder()
	ctx = context.New(w, req, tracing.NoopTracing, "no trace")
	ctx.SetHandlerCaller(func(lastResult string) string {
		return lastResult
	})

	ans = kafka.Handle(ctx)
	assert.Equal("", ans)

	msg = <-kafka.producer.(*mockAsyncProducer).ch
	assert.Equal("default-topic", msg.Topic)
	assert.Equal(0, len(msg.Headers))
	value, err = msg.Value.Encode()
	assert.Nil(err)
	assert.Equal("text", string(value))
}
