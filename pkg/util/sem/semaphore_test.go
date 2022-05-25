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

package sem

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSemaphore0(t *testing.T) {
	s := NewSem(0)

	c := make(chan struct{})
	go func(s *Semaphore, c chan struct{}) {
		s.Acquire()
		c <- struct{}{}
	}(s, c)

	select {
	case <-c:
		t.Errorf("trans: 1, maxSem: 0")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSemaphoreChangeAtRuntime(t *testing.T) {
	s := NewSem(10)

	testcases := []int64{40, 2}
	for _, c := range testcases {
		runCase(s, c, t)
	}
}

func runCase(s *Semaphore, maxCount int64, t *testing.T) {
	// step 0, change max count at runtime
	done := s.SetMaxCount(maxCount)
	<-done

	// step 1, try to acquire max count, should be OK
	wg := &sync.WaitGroup{}
	wg.Add(int(maxCount))
	for i := 0; i < int(maxCount); i++ {
		go func() {
			s.Acquire()
			wg.Done()
		}()
	}
	wg.Wait()

	// step 2: try to acquire one more, should timeout
	ctx, _ := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err := s.AcquireWithContext(ctx)
	if err == nil {
		t.Fatalf("sema count exceeds the maxCount: %d", maxCount)
	}

	for i := 0; i < int(maxCount); i++ {
		s.Release()
	}
}
