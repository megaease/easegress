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

package topicmapper

import (
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"
)

const (
	// Kind is the kind of TopicMapper
	Kind = "TopicMapper"

	resultMQTTTopicMapFailed = "MQTTTopicMapFailed"
)

func init() {
	pipeline.Register(&TopicMapper{})
}

type (
	// TopicMapper map MQTT multi-level topic into topic and key-value headers
	TopicMapper struct {
		filterSpec *pipeline.FilterSpec
		spec       *Spec
		mapFn      topicMapFunc
	}
)

var (
	_ pipeline.Filter     = (*TopicMapper)(nil)
	_ pipeline.MQTTFilter = (*TopicMapper)(nil)
)

// Kind return kind of TopicMapper
func (k *TopicMapper) Kind() string {
	return Kind
}

// DefaultSpec return default spec of TopicMapper
func (k *TopicMapper) DefaultSpec() interface{} {
	return &Spec{}
}

// Description return description of TopicMapper
func (k *TopicMapper) Description() string {
	return "Kafka is a backend of MQTTProxy"
}

// Results return possible results of TopicMapper
func (k *TopicMapper) Results() []string {
	return []string{resultMQTTTopicMapFailed}
}

// Init init TopicMapper
func (k *TopicMapper) Init(filterSpec *pipeline.FilterSpec) {
	if filterSpec.Protocol() != context.MQTT {
		panic("filter TopicMapper only support MQTT protocol for now")
	}
	k.filterSpec, k.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	k.mapFn = getTopicMapFunc(k.spec)
	if k.mapFn == nil {
		panic("invalid spec for TopicMapper")
	}
}

// Inherit init TopicMapper based on previous generation
func (k *TopicMapper) Inherit(filterSpec *pipeline.FilterSpec, previousGeneration pipeline.Filter) {
	previousGeneration.Close()
	k.Init(filterSpec)
}

// Close close TopicMapper
func (k *TopicMapper) Close() {
}

// Status return status of TopicMapper
func (k *TopicMapper) Status() interface{} {
	return nil
}

// HandleMQTT handle MQTT context
func (k *TopicMapper) HandleMQTT(ctx context.MQTTContext) *context.MQTTResult {
	if ctx.PacketType() != context.MQTTPublish {
		return &context.MQTTResult{}
	}

	publish := ctx.PublishPacket()
	topic, headers, err := k.mapFn(publish.TopicName)
	if err != nil {
		logger.Errorf("map topic %v failed, %v", publish.TopicName, err)
		ctx.SetEarlyStop()
		return &context.MQTTResult{ErrString: resultMQTTTopicMapFailed}
	}
	ctx.SetKV(k.spec.SetKV.Topic, topic)
	ctx.SetKV(k.spec.SetKV.Headers, headers)
	return &context.MQTTResult{ErrString: ""}
}
