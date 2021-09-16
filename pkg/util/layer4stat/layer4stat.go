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

package layer4stat

import (
	"sync"

	"github.com/megaease/easegress/pkg/util/sampler"
	"github.com/rcrowley/go-metrics"
)

type (
	// Layer4Stat is the statistics tool for TCP traffic.
	Layer4Stat struct {
		mutex sync.Mutex

		count  uint64 // for tcp connection
		rate1  metrics.EWMA
		rate5  metrics.EWMA
		rate15 metrics.EWMA

		errCount  uint64
		errRate1  metrics.EWMA
		errRate5  metrics.EWMA
		errRate15 metrics.EWMA

		m1ErrPercent  float64
		m5ErrPercent  float64
		m15ErrPercent float64

		total uint64
		min   uint64
		mean  uint64
		max   uint64

		durationSampler *sampler.DurationSampler

		reqSize  uint64
		respSize uint64
	}

	// Status contains all status generated by HTTPStat.
	Status struct {
		Count uint64  `yaml:"count"`
		M1    float64 `yaml:"m1"`
		M5    float64 `yaml:"m5"`
		M15   float64 `yaml:"m15"`

		ErrCount uint64  `yaml:"errCount"`
		M1Err    float64 `yaml:"m1Err"`
		M5Err    float64 `yaml:"m5Err"`
		M15Err   float64 `yaml:"m15Err"`

		M1ErrPercent  float64 `yaml:"m1ErrPercent"`
		M5ErrPercent  float64 `yaml:"m5ErrPercent"`
		M15ErrPercent float64 `yaml:"m15ErrPercent"`

		Min  uint64 `yaml:"min"`
		Max  uint64 `yaml:"max"`
		Mean uint64 `yaml:"mean"`

		P25  float64 `yaml:"p25"`
		P50  float64 `yaml:"p50"`
		P75  float64 `yaml:"p75"`
		P95  float64 `yaml:"p95"`
		P98  float64 `yaml:"p98"`
		P99  float64 `yaml:"p99"`
		P999 float64 `yaml:"p999"`

		ReqSize  uint64 `yaml:"reqSize"`
		RespSize uint64 `yaml:"respSize"`

		Codes map[int]uint64 `yaml:"codes"`
	}
)

// Status get layer4 proxy status
func (s *Layer4Stat) Status() *Status {
	panic("implement me")
}

// New get new layer4 stat
func New() *Layer4Stat {
	panic("implement me")
}
