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

package connectcontrol

import (
	"regexp"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"
)

const (
	// Kind is the kind of ConnectControl
	Kind = "ConnectControl"

	resultBannedClientOrTopic = "bannedClientOrTopicError"
)

// ErrBannedClientOrTopic is error for banned client or topic

func init() {
	pipeline.Register(&ConnectControl{})
}

type (
	// ConnectControl is used to control MQTT clients connect status,
	// if MQTTContext ClientID in bannedClients, the connection will be closed,
	// if MQTTContext publish topic in bannedTopics, the connection will be closed.
	ConnectControl struct {
		filterSpec     *pipeline.FilterSpec
		spec           *Spec
		bannedClients  map[string]struct{}
		bannedTopics   map[string]struct{}
		bannedClientRe *regexp.Regexp
		bannedTopicRe  *regexp.Regexp
		status         *Status
	}

	// Spec describes the ConnectControl
	Spec struct {
		BannedClientRe string   `yaml:"bannedClientRe" jsonschema:"omitempty"`
		BannedClients  []string `yaml:"bannedClients" jsonschema:"omitempty"`
		BannedTopicRe  string   `yaml:"bannedTopicRe" jsonschema:"omitempty"`
		BannedTopics   []string `yaml:"bannedTopics" jsonschema:"omitempty"`
		EarlyStop      bool     `yaml:"earlyStop" jsonschema:"omitempty"`
	}

	// Status is ConnectControl filter status
	Status struct {
		BannedClientRe  string `yaml:"bannedClientRe" jsonschema:"omitempty"`
		BannedClientNum int    `yaml:"bannedClientNum" jsonschema:"omitempty"`
		BannedTopicRe   string `yaml:"bannedTopicRe" jsonschema:"omitempty"`
		BannedTopicNum  int    `yaml:"bannedTopicNum" jsonschema:"omitempty"`
	}
)

var (
	_ pipeline.Filter     = (*ConnectControl)(nil)
	_ pipeline.MQTTFilter = (*ConnectControl)(nil)
)

// Kind return kind of ConnectControl
func (cc *ConnectControl) Kind() string {
	return Kind
}

// DefaultSpec return default spec of ConnectControl
func (cc *ConnectControl) DefaultSpec() interface{} {
	return &Spec{}
}

// Description return description of ConnectControl
func (cc *ConnectControl) Description() string {
	return "ConnectControl control connections of MQTT clients"
}

// Results return results of ConnectControl
func (cc *ConnectControl) Results() []string {
	return []string{resultBannedClientOrTopic}
}

// Init init ConnectControl with pipeline filter spec
func (cc *ConnectControl) Init(filterSpec *pipeline.FilterSpec) {
	if filterSpec.Protocol() != context.MQTT {
		panic("filter ConnectControl only support MQTT protocol for now")
	}
	cc.filterSpec, cc.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	cc.bannedClients = make(map[string]struct{})
	cc.bannedTopics = make(map[string]struct{})
	cc.reload()
}

func (cc *ConnectControl) reload() {
	if len(cc.spec.BannedClientRe) > 0 {
		r, err := regexp.Compile(cc.spec.BannedClientRe)
		if err != nil {
			logger.Errorf("filter ConnectControl compile BannedClientRe %s failed, %s", cc.spec.BannedClientRe, err)
		} else {
			cc.bannedClientRe = r
		}
	}
	if len(cc.spec.BannedTopicRe) > 0 {
		r, err := regexp.Compile(cc.spec.BannedTopicRe)
		if err != nil {
			logger.Errorf("filter ConnectControl compile BannedTopicRe %s failed, %s", cc.spec.BannedTopicRe, err)
		} else {
			cc.bannedTopicRe = r
		}
	}

	for _, c := range cc.spec.BannedClients {
		cc.bannedClients[c] = struct{}{}
	}
	for _, t := range cc.spec.BannedTopics {
		cc.bannedTopics[t] = struct{}{}
	}

	cc.status = &Status{
		BannedClientRe:  cc.spec.BannedClientRe,
		BannedTopicRe:   cc.spec.BannedTopicRe,
		BannedClientNum: len(cc.spec.BannedClients),
		BannedTopicNum:  len(cc.spec.BannedTopics),
	}
}

// Inherit init ConnectControl with previous generation
func (cc *ConnectControl) Inherit(filterSpec *pipeline.FilterSpec, previousGeneration pipeline.Filter) {
	previousGeneration.Close()
	cc.Init(filterSpec)
}

// Status return status of ConnectControl
func (cc *ConnectControl) Status() interface{} {
	return cc.status
}

// Close close ConnectControl gracefully
func (cc *ConnectControl) Close() {
}

func (cc *ConnectControl) checkBan(ctx context.MQTTContext) bool {
	cid := ctx.Client().ClientID()
	if cc.bannedClientRe != nil && cc.bannedClientRe.MatchString(cid) {
		return true
	}
	if _, ok := cc.bannedClients[cid]; ok {
		return true
	}
	topic := ctx.PublishPacket().TopicName
	if cc.bannedTopicRe != nil && cc.bannedTopicRe.MatchString(topic) {
		return true
	}
	if _, ok := cc.bannedTopics[topic]; ok {
		return true
	}
	return false
}

// HandleMQTT handle MQTT request
func (cc *ConnectControl) HandleMQTT(ctx context.MQTTContext) *context.MQTTResult {
	if ctx.PacketType() != context.MQTTPublish {
		return &context.MQTTResult{}
	}

	if cc.checkBan(ctx) {
		ctx.SetDisconnect()
		if cc.spec.EarlyStop {
			ctx.SetEarlyStop()
		}
		return &context.MQTTResult{ErrString: resultBannedClientOrTopic}
	}
	return &context.MQTTResult{}
}
