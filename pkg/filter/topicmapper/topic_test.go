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
	"fmt"
	"reflect"
	"testing"

	"github.com/megaease/easegress/pkg/logger"
)

func init() {
	logger.InitNop()
}

func getDefaultSpec() *Spec {
	// /d2s/{tenant}/{device_type}/{things_id}/log/eventName
	// /d2s/{tenant}/{device_type}/{things_id}/status
	// /d2s/{tenant}/{device_type}/{things_id}/event
	// /d2s/{tenant}/{device_type}/{things_id}/raw
	// /g2s/{gwTenantId}/{gwInfoModelId}/{gwThingsId}/d2s/{tenantId}/{infoModelId}/{thingsId}/data
	spec := Spec{
		SetKV: &SetKV{
			Topic:   "topic",
			Headers: "headers",
		},
		MatchIndex: 0,
		Route: []*PolicyRe{
			{"g2s", "g2s"},
			{"d2s", "d2s"},
		},
		Policies: []*Policy{
			{
				Name:       "d2s",
				TopicIndex: 4,
				Route: []TopicRe{
					{"to_cloud", []string{"log", "status", "event"}},
					{"to_raw", []string{"raw"}},
				},
				Headers: map[int]string{
					0: "d2s",
					1: "tenant",
					2: "device_type",
					3: "things_id",
					4: "event",
					5: "eventName",
				},
			},
			{
				Name:       "g2s",
				TopicIndex: 8,
				Route: []TopicRe{
					{"to_cloud", []string{"log", "status", "event", "data"}},
					{"to_raw", []string{"raw"}},
				},
				Headers: map[int]string{
					0: "g2s",
					1: "gwTenantId",
					2: "gwInfoModelId",
					3: "gwThingsId",
					4: "d2s",
					5: "tenantId",
					6: "infoModelId",
					7: "thingsId",
					8: "event",
					9: "eventName",
				},
			},
		},
	}
	return &spec
}

func TestTopicMap(t *testing.T) {
	// /d2s/{tenant}/{device_type}/{things_id}/log/eventName
	// /d2s/{tenant}/{device_type}/{things_id}/status
	// /d2s/{tenant}/{device_type}/{things_id}/event
	// /d2s/{tenant}/{device_type}/{things_id}/raw
	// /g2s/{gwTenantId}/{gwInfoModelId}/{gwThingsId}/d2s/{tenantId}/{infoModelId}/{thingsId}/data
	spec := getDefaultSpec()
	mapFunc := getTopicMapFunc(spec)
	tests := []struct {
		mqttTopic string
		topic     string
		headers   map[string]string
	}{
		{
			mqttTopic: "/d2s/abc/phone/123/log/error",
			topic:     "to_cloud",
			headers:   map[string]string{"d2s": "d2s", "tenant": "abc", "device_type": "phone", "things_id": "123", "event": "log", "eventName": "error"},
		},
		{
			mqttTopic: "/d2s/xyz/tv/234/status/shutdown",
			topic:     "to_cloud",
			headers:   map[string]string{"d2s": "d2s", "tenant": "xyz", "device_type": "tv", "things_id": "234", "event": "status", "eventName": "shutdown"},
		},
		{
			mqttTopic: "/d2s/opq/car/345/raw",
			topic:     "to_raw",
			headers:   map[string]string{"d2s": "d2s", "tenant": "opq", "device_type": "car", "things_id": "345", "event": "raw"},
		},
		{
			mqttTopic: "/g2s/gwTenantId/gwInfoModelId/gwThingsId/d2s/tenantId/infoModelId/thingsId/data",
			topic:     "to_cloud",
			headers: map[string]string{
				"g2s": "g2s", "gwTenantId": "gwTenantId", "gwInfoModelId": "gwInfoModelId", "gwThingsId": "gwThingsId",
				"d2s": "d2s", "tenantId": "tenantId", "infoModelId": "infoModelId", "thingsId": "thingsId", "event": "data",
			},
		},
		{
			mqttTopic: "/g2s/gw123/gwInfo234/gwID345/d2s/456/654/123/raw",
			topic:     "to_raw",
			headers: map[string]string{
				"g2s": "g2s", "gwTenantId": "gw123", "gwInfoModelId": "gwInfo234", "gwThingsId": "gwID345",
				"d2s": "d2s", "tenantId": "456", "infoModelId": "654", "thingsId": "123", "event": "raw",
			},
		},
	}
	for _, tt := range tests {
		topic, headers, err := mapFunc(tt.mqttTopic)
		if err != nil || topic != tt.topic || !reflect.DeepEqual(tt.headers, headers) {
			if !reflect.DeepEqual(tt.headers, headers) {
				fmt.Printf("headers not equal, <%v>, <%v>\n", tt.headers, headers)
			}
			t.Errorf("unexpected result from topic map function <%+v>, <%+v>, want:<%+v>, got:<%+v>\n\n\n\n", tt.mqttTopic, topic, tt.headers, headers)
		}
	}

	_, _, err := mapFunc("/not-work")
	if err == nil {
		t.Errorf("map func should return err for not match topic")
	}
	mapFunc = getTopicMapFunc(nil)
	if mapFunc != nil {
		t.Errorf("nil mapFunc if TopicMapper is nil")
	}

	fakeMapper := Spec{
		MatchIndex: 0,
		Route: []*PolicyRe{
			{"fake", "]["},
		},
	}
	getTopicMapFunc(&fakeMapper)
	fakeMapper = Spec{
		MatchIndex: 0,
		Route: []*PolicyRe{
			{"demo", "demo"},
		},
		Policies: []*Policy{
			{
				Name:       "demo",
				TopicIndex: 1,
				Route: []TopicRe{
					{"topic", []string{"]["}},
				},
				Headers: map[int]string{1: "topic"},
			},
		},
	}
	getTopicMapFunc(&fakeMapper)
}

func TestTopicMap2(t *testing.T) {
	// nothing/d2s/{tenant}/{device_type}/{things_id}/log/eventName
	// nothing/d2s/{tenant}/{device_type}/{things_id}/status
	// nothing/d2s/{tenant}/{device_type}/{things_id}/event
	// nothing/d2s/{tenant}/{device_type}/{things_id}/raw
	// nothing/g2s/{gwTenantId}/{gwInfoModelId}/{gwThingsId}/d2s/{tenantId}/{infoModelId}/{thingsId}/data
	topicMapper := Spec{
		MatchIndex: 1,
		Route: []*PolicyRe{
			{"g2s", "g2s"},
			{"d2s", "d2s"},
		},
		Policies: []*Policy{
			{
				Name:       "d2s",
				TopicIndex: 5,
				Route: []TopicRe{
					{"to_cloud", []string{"log", "status", "event"}},
					{"to_raw", []string{"raw"}},
				},
				Headers: map[int]string{
					1: "d2s",
					2: "tenant",
					3: "device_type",
					4: "things_id",
					5: "event",
				},
			},
			{
				Name:       "g2s",
				TopicIndex: 9,
				Route: []TopicRe{
					{"to_cloud", []string{"log", "status", "event", "data"}},
					{"to_raw", []string{"raw"}},
				},
				Headers: map[int]string{
					1: "g2s",
					2: "gwTenantId",
					3: "gwInfoModelId",
					4: "gwThingsId",
					5: "d2s",
					6: "tenantId",
					7: "infoModelId",
					8: "thingsId",
					9: "event",
				},
			},
		},
	}
	mapFunc := getTopicMapFunc(&topicMapper)

	_, _, err := mapFunc("/level0")
	if err == nil {
		t.Errorf("map func should return err for not match topic")
	}
	_, _, err = mapFunc("nothing/g2s")
	if err == nil {
		t.Errorf("map func should return err for not match topic")
	}
	_, _, err = mapFunc("nothing/d2s/tenant/device_type/things_id/notexist")
	if err == nil {
		t.Errorf("map func should return err for not match topic")
	}
}
