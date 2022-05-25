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

package spec

import (
	"fmt"
	"testing"
)

func TestInValidSpec(t *testing.T) {
	spec := Spec{
		Name:           "demo",
		Image:          "helloworld:1.0",
		Port:           10000,
		AutoScaleType:  AutoScaleMetricCPU,
		AutoScaleValue: "0.1",
		MinReplica:     1,
		MaxReplica:     2,
		LimitCPU:       "xx",
		LimitMemory:    "80h",
		RequestCPU:     "yy",
		RequestMemory:  "xx",
	}

	var err error
	if err = spec.Validate(); err == nil {
		t.Errorf("test failed spec should not be valid")
	}
	fmt.Printf("spec %#v is invalid, due to %v\n", spec, err)
}

func TestInvalidKnative(t *testing.T) {
	spec := Spec{
		Name:           "demo",
		Image:          "helloworld:1.0",
		Port:           10000,
		AutoScaleType:  AutoScaleMetricCPU,
		AutoScaleValue: "0.1",
		MinReplica:     1,
		MaxReplica:     2,
		LimitCPU:       "aaa",
		LimitMemory:    "80Mi",
		RequestCPU:     "100m",
		RequestMemory:  "60Mi",
	}

	var err error
	if err = spec.Validate(); err == nil {
		t.Errorf("test failed spec should not be valid, err: %v", err)
	}
}

func TestValidSpec(t *testing.T) {
	spec := Spec{
		Name:           "demo",
		Image:          "helloworld:1.0",
		Port:           10000,
		AutoScaleType:  AutoScaleMetricCPU,
		AutoScaleValue: "0.1",
		MinReplica:     1,
		MaxReplica:     2,
		LimitCPU:       "180m",
		LimitMemory:    "80Mi",
		RequestCPU:     "100m",
		RequestMemory:  "60Mi",
	}

	var err error
	if err = spec.Validate(); err != nil {
		t.Errorf("test failed spec should not be valid, err: %v", err)
	}
}

func TestInValidSpecReplica(t *testing.T) {
	spec := Spec{
		Name:           "demo",
		Image:          "helloworld:1.0",
		Port:           10000,
		AutoScaleType:  AutoScaleMetricCPU,
		AutoScaleValue: "0.1",
		MinReplica:     2,
		MaxReplica:     1,
		LimitCPU:       "180m",
		LimitMemory:    "80Mi",
		RequestCPU:     "100m",
		RequestMemory:  "60Mi",
	}

	var err error
	if err = spec.Validate(); err == nil {
		t.Errorf("test failed spec should not be valid, err: %v", err)
	}
}

func TestInValidSpecAutoScaleType(t *testing.T) {
	spec := Spec{
		Name:           "demo",
		Image:          "helloworld:1.0",
		Port:           10000,
		AutoScaleType:  "nothing",
		AutoScaleValue: "0.1",
		MinReplica:     2,
		MaxReplica:     1,
		LimitCPU:       "180m",
		LimitMemory:    "80Mi",
		RequestCPU:     "100m",
		RequestMemory:  "60Mi",
	}

	var err error
	if err = spec.Validate(); err == nil {
		t.Errorf("test failed spec should not be invalid, err: %v", err)
	}
	fmt.Println("err is ", err)
}

func TestNext(t *testing.T) {
	fsm, _ := InitFSM(InitState())

	f := &Function{
		Spec: &Spec{
			Name:           "demo",
			Image:          "helloworld:1.0",
			Port:           10000,
			AutoScaleValue: "0.1",
			MinReplica:     1,
			MaxReplica:     2,
			LimitCPU:       "180m",
			LimitMemory:    "80Mi",
			RequestCPU:     "100m",
			RequestMemory:  "60Mi",
		},
		Fsm:    fsm,
		Status: &Status{},
	}

	updated, err := f.Next(ReadyEvent)
	if err != nil {
		t.Errorf("test failed Next should succ, err: %v", err)
	}

	t.Logf("updated: %v, new status: %s\n", updated, f.Fsm.Current())
}

func TestInvalidNext(t *testing.T) {
	fsm, _ := InitFSM(InitState())

	f := &Function{
		Spec: &Spec{
			Name:           "demo",
			Image:          "helloworld:1.0",
			Port:           10000,
			AutoScaleValue: "0.1",
			MinReplica:     1,
			MaxReplica:     2,
			LimitCPU:       "180m",
			LimitMemory:    "80Mi",
			RequestCPU:     "100m",
			RequestMemory:  "60Mi",
		},
		Fsm:    fsm,
		Status: &Status{},
	}

	_, err := f.Next(StartEvent)

	if err == nil {
		t.Errorf("test failed Next should failed, start event will be rejected in initial state")
	}
}
