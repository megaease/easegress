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

package resilience

import (
	"context"
	"math/rand"
	"time"
)

// RetryKind is the kind of Retry.
var RetryKind = &Kind{
	Name: "Retry",

	DefaultPolicy: func() Policy {
		return &RetryPolicy{
			RetryRule: RetryRule{
				MaxAttempts:   3,
				WaitDuration:  "500ms",
				BackOffPolicy: "random",
			},
		}
	},
}

var _ Policy = (*RetryPolicy)(nil)

type (
	// RetryPolicy defines the retry policy.
	RetryPolicy struct {
		BaseSpec  `yaml:",inline"`
		RetryRule `yaml:",inline"`
	}

	// RetryRule is the detailed config of retry
	RetryRule struct {
		MaxAttempts         int    `yaml:"maxAttempts" jsonschema:"omitempty,minimum=1"`
		WaitDuration        string `yaml:"waitDuration" jsonschema:"omitempty,format=duration"`
		waitDuration        time.Duration
		BackOffPolicy       string  `yaml:"backOffPolicy" jsonschema:"omitempty,enum=random,enum=exponential"`
		RandomizationFactor float64 `yaml:"randomizationFactor" jsonschema:"omitempty,minimum=0,maximum=1"`
	}
)

// Validate validates the retry policy.
func (p *RetryPolicy) Validate() error {
	// TODO
	return nil
}

// CreateWrapper creates a Wrapper. For the RetryPolicy, just reuse itself.
func (p *RetryPolicy) CreateWrapper() Wrapper {
	if d := p.WaitDuration; d != "" {
		p.waitDuration, _ = time.ParseDuration(d)
	}
	if p.waitDuration <= 0 {
		p.waitDuration = time.Millisecond * 500
	}
	return p
}

// Wrap wraps the handler function.
func (p *RetryPolicy) Wrap(handler HandlerFunc) HandlerFunc {
	return func(ctx context.Context) error {
		var err error
		base := float64(p.waitDuration)

		for attempt := 0; attempt < p.MaxAttempts; attempt++ {
			err = handler(ctx)
			if err == nil {
				return nil
			}

			delta := base * p.RandomizationFactor
			d := base - delta + float64(rand.Intn(int(delta*2+1)))

			select {
			case <-ctx.Done():
				return err
			case <-time.After(time.Duration(d)):
			}
			if p.BackOffPolicy == "exponential" {
				base *= 1.5
			}
		}

		return err
	}
}
