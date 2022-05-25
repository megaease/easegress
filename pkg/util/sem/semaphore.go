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

	"golang.org/x/sync/semaphore"
)

const maxCapacity int64 = 20_000_000

// Semaphore supports to change the max sema amount at runtime.
//  Semaphore employs golang.org/x/sync/semaphore.Weighted with a maxCapacity.
//  And tuning the realCapacity by an Acquire and Release in the background.
//  the realCapacity can not exceed the maxCapacity.
type Semaphore struct {
	sem          *semaphore.Weighted
	lock         sync.Mutex
	realCapacity int64
}

// NewSem new a Semaphore
func NewSem(n uint32) *Semaphore {
	s := &Semaphore{
		sem:          semaphore.NewWeighted(maxCapacity),
		realCapacity: int64(n),
	}

	s.sem.Acquire(context.Background(), maxCapacity-s.realCapacity)
	return s
}

// Acquire acquires the semaphore
func (s *Semaphore) Acquire() {
	s.AcquireWithContext(context.Background())
}

// AcquireWithContext acquires the semaphore with context
func (s *Semaphore) AcquireWithContext(ctx context.Context) error {
	return s.sem.Acquire(ctx, 1)
}

// Release releases one semaphore.
func (s *Semaphore) Release() {
	s.sem.Release(1)
}

// SetMaxCount set the size of 's' to 'n', this is an async operation and
// the caller can watch the returned 'done' channel like below if it wants
// to be notified at the completion:
//       done := s.SetMaxCount(100)
//       <-done
// Note after receiving the notification, the caller should NOT assume the
// size of 's' is 'n' unless it knows there are no concurrent calls to
// 'SetMaxCount'.
func (s *Semaphore) SetMaxCount(n int64) (done chan struct{}) {
	done = make(chan struct{})

	if n > maxCapacity {
		n = maxCapacity
	}

	s.lock.Lock()
	old := s.realCapacity
	s.realCapacity = n
	s.lock.Unlock()

	go func() {
		if n > old {
			s.sem.Release(n - old)
		} else if n < old {
			s.sem.Acquire(context.Background(), old-n)
		}
		close(done)
	}()

	return
}
