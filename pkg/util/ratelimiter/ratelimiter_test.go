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

package ratelimiter

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

var now time.Time

func setup() {
	now = time.Now()
	nowFunc = func() time.Time {
		return now
	}
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	os.Exit(code)
}

func TestNewDefaultPolicy(t *testing.T) {
	policy := NewDefaultPolicy()

	if policy.TimeoutDuration != 100*time.Millisecond ||
		policy.LimitRefreshPeriod != 10*time.Millisecond ||
		policy.LimitForPeriod != 50 {
		t.Errorf("Incorrect default configuration %v\n", policy)
	}
}

func TestConcurrent(t *testing.T) {
	policy := NewPolicy(50*time.Millisecond, 10*time.Millisecond, 5)

	var wg sync.WaitGroup
	limiter := New(policy)
	fn := func() {
		permitted := limiter.WaitPermission()
		if !permitted {
			t.Errorf("WaitPermission should succeed")
		}
		wg.Done()
	}

	wg.Add(30)
	for i := 0; i < 30; i++ {
		go fn()
	}
	wg.Wait()

	if limiter.WaitPermission() {
		t.Errorf("WaitPermission should fail")
	}

	now = now.Add(time.Millisecond * 5)
	if limiter.WaitPermission() {
		t.Errorf("WaitPermission should fail")
	}

	now = now.Add(time.Millisecond * 5)
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go fn()
	}
	wg.Wait()

	if limiter.WaitPermission() {
		t.Errorf("WaitPermission should fail")
	}

	now = now.Add(time.Millisecond * 100)
	wg.Add(30)
	for i := 0; i < 30; i++ {
		go fn()
	}
	wg.Wait()

	if limiter.WaitPermission() {
		t.Errorf("WaitPermission should fail")
	}
}

func TestRateLimiter(t *testing.T) {
	policy := NewPolicy(50*time.Millisecond, 10*time.Millisecond, 5)

	limiter := New(policy)
	limiter.SetStateListener(func(event *Event) {
		fmt.Printf("%v\n", event)
	})
	limiter.SetState(StateNormal)
	for i := 0; i < 30; i++ {
		permitted, d := limiter.AcquirePermission()
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
		if d != time.Duration(i/policy.LimitForPeriod)*policy.LimitRefreshPeriod {
			t.Errorf("wait duration of %d should not be: %s", i, d.String())
		}
	}

	limiter.SetState(StateDisabled)
	if permitted, d := limiter.AcquirePermission(); !permitted {
		t.Errorf("AcquirePermission should succeeded")
	} else if d != 0 {
		t.Errorf("wait duration should not be: %s", d.String())
	}

	limiter.SetState(StateNormal)
	for i := 0; i < 30; i++ {
		permitted, d := limiter.AcquirePermission()
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
		if d != time.Duration(i/policy.LimitForPeriod)*policy.LimitRefreshPeriod {
			t.Errorf("wait duration of %d should not be: %s", i, d.String())
		}
	}

	limiter.SetState(StateLimiting)
	if permitted, d := limiter.AcquirePermission(); permitted {
		t.Errorf("AcquirePermission should fail")
	} else if d != policy.TimeoutDuration {
		t.Errorf("wait duration should not be: %s", d.String())
	}

	limiter.SetState(StateLimiting)
	now = now.Add(time.Millisecond * 5)
	if permitted, d := limiter.AcquirePermission(); permitted {
		t.Errorf("AcquirePermission should fail")
	} else if d != policy.TimeoutDuration {
		t.Errorf("wait duration should not be: %s", d.String())
	}

	now = now.Add(time.Millisecond * 6)
	for i := 0; i < 5; i++ {
		if permitted, d := limiter.AcquirePermission(); !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		} else if d != policy.TimeoutDuration-time.Millisecond {
			t.Errorf("wait duration of %d should not be: %s", i, d.String())
		}
	}

	if permitted, d := limiter.AcquirePermission(); permitted {
		t.Errorf("AcquirePermission should fail")
	} else if d != policy.TimeoutDuration {
		t.Errorf("wait duration should not be: %s", d.String())
	}

	now = now.Add(time.Millisecond * 89)
	for i := 0; i < 30; i++ {
		if permitted, d := limiter.AcquirePermission(); !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		} else if d != time.Duration(i/policy.LimitForPeriod)*policy.LimitRefreshPeriod {
			t.Errorf("wait duration of %d should not be: %s", i, d.String())
		}
	}

	if permitted, d := limiter.AcquirePermission(); permitted {
		t.Errorf("AcquirePermission should fail")
	} else if d != policy.TimeoutDuration {
		t.Errorf("wait duration should not be: %s", d.String())
	}
	limiter.SetState(StateDisabled)
}

func TestRateLimiterN(t *testing.T) {
	policy := NewPolicy(50*time.Millisecond, 10*time.Millisecond, 5)

	limiter := New(policy)
	limiter.SetStateListener(func(event *Event) {
		fmt.Printf("%v\n", event)
	})
	limiter.SetState(StateNormal)
	for i := 0; i < 15; i++ {
		permitted, _ := limiter.AcquireNPermission(2)
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}

	limiter.SetState(StateDisabled)
	if permitted, _ := limiter.AcquirePermission(); !permitted {
		t.Errorf("AcquirePermission should succeeded")
	}

	limiter.SetState(StateNormal)
	for i := 0; i < 10; i++ {
		permitted, _ := limiter.AcquireNPermission(3)
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}

	limiter.SetState(StateLimiting)
	if permitted, _ := limiter.AcquirePermission(); permitted {
		t.Errorf("AcquirePermission should fail")
	}

	limiter.SetState(StateLimiting)
	now = now.Add(time.Millisecond * 5)
	if permitted, _ := limiter.AcquirePermission(); permitted {
		t.Errorf("AcquirePermission should fail")
	}

	now = now.Add(time.Millisecond * 6)
	for i := 0; i < 5; i++ {
		if permitted, _ := limiter.AcquirePermission(); !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}

	if permitted, _ := limiter.AcquirePermission(); permitted {
		t.Errorf("AcquirePermission should fail")
	}

	now = now.Add(time.Millisecond * 89)
	for i := 0; i < 5; i++ {
		if permitted, _ := limiter.AcquireNPermission(6); !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}

	if permitted, _ := limiter.AcquirePermission(); permitted {
		t.Errorf("AcquirePermission should fail")
	}
	limiter.SetState(StateDisabled)
}
