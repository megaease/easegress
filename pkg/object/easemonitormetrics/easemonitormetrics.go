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

package easemonitormetrics

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Shopify/sarama"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/statussynccontroller"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/easemonitor"
)

const (
	// Kind is EaseMonitorMetrics kind.
	Kind = "EaseMonitorMetrics"
)

var hostIPv4 string

func init() {
	supervisor.Register(&EaseMonitorMetrics{})

	hostIPv4 = getHostIPv4()
	if hostIPv4 == "" {
		panic(fmt.Errorf("get host ipv4 failed"))
	}
}

type (
	// EaseMonitorMetrics is Object EaseMonitorMetrics.
	EaseMonitorMetrics struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		ssc *statussynccontroller.StatusSyncController

		// sarama.AsyncProducer
		client      atomic.Value
		clientMutex sync.Mutex

		done chan struct{}
	}

	// Spec describes the EaseMonitorMetrics.
	Spec struct {
		Kafka *KafkaSpec `yaml:"kafka" jsonschema:"required"`
	}

	// KafkaSpec is the spec for kafka producer.
	KafkaSpec struct {
		Brokers []string `yaml:"brokers" jsonschema:"required,uniqueItems=true"`
		Topic   string   `yaml:"topic" jsonschema:"required"`
	}

	// Status is the status of EaseMonitorMetrics.
	Status struct {
		Health string `json:"health"`
	}
)

// Category returns the category of EaseMonitorMetrics.
func (emm *EaseMonitorMetrics) Category() supervisor.ObjectCategory {
	return supervisor.CategoryBusinessController
}

// Kind returns the kind of EaseMonitorMetrics.
func (emm *EaseMonitorMetrics) Kind() string {
	return "EaseMonitorMetrics"
}

// DefaultSpec returns the default spec of EaseMonitorMetrics.
func (emm *EaseMonitorMetrics) DefaultSpec() interface{} {
	return &Spec{
		Kafka: &KafkaSpec{
			Brokers: []string{"localhost:9092"},
		},
	}
}

// Init initializes EaseMonitorMetrics.
func (emm *EaseMonitorMetrics) Init(superSpec *supervisor.Spec) {
	emm.superSpec, emm.spec, emm.super = superSpec, superSpec.ObjectSpec().(*Spec), superSpec.Super()
	emm.reload()
}

// Inherit inherits previous generation of EaseMonitorMetrics.
func (emm *EaseMonitorMetrics) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	emm.Init(superSpec)
}

func (emm *EaseMonitorMetrics) reload() {
	ssc, exists := emm.super.GetSystemController(statussynccontroller.Kind)
	if !exists {
		logger.Errorf("BUG: status sync controller not found")
	}

	emm.ssc = ssc.Instance().(*statussynccontroller.StatusSyncController)
	emm.done = make(chan struct{})

	_, err := emm.getClient()
	if err != nil {
		logger.Errorf("%s get kafka producer client failed: %v", emm.superSpec.Name(), err)
	}

	go emm.run()
}

func (emm *EaseMonitorMetrics) getClient() (sarama.AsyncProducer, error) {
	client := emm.client.Load()
	if client != nil {
		return client.(sarama.AsyncProducer), nil
	}

	emm.clientMutex.Lock()
	defer emm.clientMutex.Unlock()

	// NOTE: Default config is good enough for now.
	config := sarama.NewConfig()
	config.ClientID = emm.superSpec.Name()
	config.Version = sarama.V0_10_2_0

	producer, err := sarama.NewAsyncProducer(emm.spec.Kafka.Brokers, config)
	if err != nil {
		msgFmt := "start sarama producer failed(brokers: %v): %v"
		return nil, fmt.Errorf(msgFmt, emm.spec.Kafka.Brokers, err)
	}

	go func() {
		for {
			select {
			case <-emm.done:
				return
			case err, ok := <-producer.Errors():
				if !ok {
					return
				}
				logger.Errorf("produce failed: %v", err)
			}
		}
	}()

	emm.client.Store(producer)

	logger.Infof("%s build kafka producer successfully", emm.superSpec.Name())

	return producer, nil
}

func (emm *EaseMonitorMetrics) closeClient() {
	emm.clientMutex.Lock()
	defer emm.clientMutex.Unlock()

	value := emm.client.Load()
	if value == nil {
		return
	}
	client := value.(sarama.AsyncProducer)

	err := client.Close()
	if err != nil {
		logger.Errorf("%s close kafka producer failed: %v", emm.superSpec.Name(), err)
	}
}

func (emm *EaseMonitorMetrics) run() {
	var latestTimestamp int64
	interval := statussynccontroller.SyncStatusPaceInUnixSeconds * time.Second

	for {
		select {
		case <-emm.done:
			emm.closeClient()
			return
		case <-time.After(interval):
			latestTimestamp = emm.sendMetrics(latestTimestamp)
		}
	}
}

func (emm *EaseMonitorMetrics) sendMetrics(latestTimestamp int64) int64 {
	client, err := emm.getClient()
	if err != nil {
		msgFmt := "%s get kafka producer failed: %v"
		logger.Errorf(msgFmt, emm.superSpec.Name(), err)
		return latestTimestamp
	}

	for _, record := range emm.ssc.GetStatusSnapshots() {
		if record.UnixTimestamp <= latestTimestamp {
			continue
		}
		latestTimestamp = record.UnixTimestamp

		for service, status := range record.Statuses {
			metricer, ok := status.ObjectStatus.(easemonitor.Metricer)
			if !ok {
				continue
			}

			for _, m := range metricer.ToMetrics(service) {
				m.Category = "application"
				m.HostName = emm.super.Options().Name
				m.HostIpv4 = hostIPv4
				m.System = emm.super.Options().ClusterName
				m.Timestamp = latestTimestamp * 1000

				data, err := json.Marshal(m)
				if err != nil {
					logger.Errorf("marshal %#v to json failed: %v", m, err)
				}

				client.Input() <- &sarama.ProducerMessage{
					Topic: emm.spec.Kafka.Topic,
					Value: sarama.ByteEncoder(data),
				}
			}
		}
	}

	return latestTimestamp
}

// Status returns status of EtcdServiceRegister.
func (emm *EaseMonitorMetrics) Status() *supervisor.Status {
	s := &Status{}

	_, err := emm.getClient()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	return &supervisor.Status{ObjectStatus: s}
}

// Close closes EaseMonitorMetrics.
func (emm *EaseMonitorMetrics) Close() {
	close(emm.done)
}

func getHostIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			panic(err)
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			ipv4 := ip.To4()
			if !ip.IsLoopback() && ipv4 != nil {
				return ipv4.String()
			}
		}
	}

	return ""
}
