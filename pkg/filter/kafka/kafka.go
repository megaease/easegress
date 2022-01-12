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

	"github.com/Shopify/sarama"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"
)

const (
	// Kind is the kind of Kafka
	Kind = "Kafka"

	resultMQTTTopicMapFailed = "MQTTTopicMapFailed"
)

func init() {
	pipeline.Register(&Kafka{})
}

type (
	// Kafka is kafka backend for MQTT proxy
	Kafka struct {
		filterSpec *pipeline.FilterSpec
		spec       *Spec
		producer   sarama.AsyncProducer
		done       chan struct{}
	}
)

var _ pipeline.Filter = (*Kafka)(nil)
var _ pipeline.MQTTFilter = (*Kafka)(nil)

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
	return []string{resultMQTTTopicMapFailed}
}

// Init init Kafka
func (k *Kafka) Init(filterSpec *pipeline.FilterSpec) {
	if filterSpec.Protocol() != context.MQTT {
		panic("filter Kafka only support MQTT protocol for now")
	}
	k.filterSpec, k.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
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
func (k *Kafka) Inherit(filterSpec *pipeline.FilterSpec, previousGeneration pipeline.Filter) {
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
func (k *Kafka) HandleMQTT(ctx context.MQTTContext) *context.MQTTResult {
	if ctx.PacketType() != context.MQTTPublish {
		return &context.MQTTResult{}
	}

	p := ctx.PublishPacket()
	logger.Debugf("produce msg with topic %s", p.Topic())

	kafkaHeaders := []sarama.RecordHeader{}
	p.VisitAllHeader(func(k, v string) {
		kafkaHeaders = append(kafkaHeaders, sarama.RecordHeader{Key: []byte(k), Value: []byte(v)})
	})

	msg := &sarama.ProducerMessage{
		Topic:   p.Topic(),
		Headers: kafkaHeaders,
		Value:   sarama.ByteEncoder(p.Payload()),
	}
	k.producer.Input() <- msg
	return &context.MQTTResult{}
}
