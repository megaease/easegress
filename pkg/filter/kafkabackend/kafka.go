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
	"io/ioutil"

	"github.com/Shopify/sarama"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
)

const (
	// Kind is the kind of Kafka
	Kind = "Kafka"

	resultParseErr = "parseErr"
)

func init() {
	httppipeline.Register(&Kafka{})
}

type (
	// Kafka is kafka backend for MQTT proxy
	Kafka struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec
		producer   sarama.AsyncProducer
		done       chan struct{}
	}
)

var _ httppipeline.Filter = (*Kafka)(nil)

// Kind return kind of Kafka
func (k *Kafka) Kind() string {
	return Kind
}

// DefaultSpec return default spec of Kafka
func (k *Kafka) DefaultSpec() interface{} {
	return &Spec{}
}

// Description return description of Kafka
func (k *Kafka) Description() string {
	return "Kafka is a backend of MQTTProxy"
}

// Results return possible results of Kafka
func (k *Kafka) Results() []string {
	return []string{resultParseErr}
}

// Init init Kafka
func (k *Kafka) Init(filterSpec *httppipeline.FilterSpec) {
	k.filterSpec, k.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	if k.spec.TopicHeaderKey == "" {
		panic("filter kafka not set topic header key")
	}

	k.done = make(chan struct{})

	config := sarama.NewConfig()
	config.ClientID = filterSpec.Name()
	config.Version = sarama.V1_0_0_0
	producer, err := sarama.NewAsyncProducer(k.spec.Backend, config)
	if err != nil {
		panic(fmt.Errorf("start sarama producer with address %v failed: %v", k.spec.Backend, err))
	}

	go func() {
		for {
			select {
			case <-k.done:
				return
			case err, ok := <-producer.Errors():
				if !ok {
					return
				}
				logger.SpanErrorf(nil, "sarama producer failed: %v", err)
			}
		}
	}()

	k.producer = producer
}

// Inherit init Kafka based on previous generation
func (k *Kafka) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	k.Init(filterSpec)
}

// Close close Kafka
func (k *Kafka) Close() {
	close(k.done)
	err := k.producer.Close()
	if err != nil {
		logger.Errorf("close kafka producer failed: %v", err)
	}
}

// Status return status of Kafka
func (k *Kafka) Status() interface{} {
	return nil
}

// HandleMQTT handle MQTT context
func (k *Kafka) Handle(ctx context.HTTPContext) (result string) {
	topic := ctx.Request().Header().Get(k.spec.TopicHeaderKey)
	if topic == "" {
		return resultParseErr
	}
	body, err := ioutil.ReadAll(ctx.Request().Body())
	if err != nil {
		return resultParseErr
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(body),
	}
	k.producer.Input() <- msg
	return ""
}
