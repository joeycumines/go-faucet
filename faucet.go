/*
   Copyright 2018 Joseph Cumines

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
 */

// Package faucet implements a simple pattern for polling based rate limiting, using Golang's time.Ticker.
// Note that any nil arguments to any method or function in this package will trigger a panic.
package faucet

import (
	"sync"
	"time"
	"context"
)

type (
	// Pipe controls the rate of transfer between 1..n inputs, and 0..n outputs, where input is, for each tick,
	// polled from _any_ of the inputs (the order checked is adjusted each tick in a round-robin fashion), and all
	// outputs will receive the input, and must all return before the next tick is started.
	// Basically, this implementation is geared towards fan-in to fan-out, allowing it to be used in a wide range
	// of situations.
	Pipe struct {
		mutex sync.Mutex
		open  sync.Once
		close sync.Once

		err  error
		done chan struct{}
		stop chan struct{}

		ticker *time.Ticker
		inputs []func(context.Context) (interface{}, bool, error)
		outputs []func(context.Context, interface{}) error
		ctx     context.Context
		cancel  context.CancelFunc
	}
)

// RatePerMinute converts a rate per minute to a duration to achieve it, note it will return 0 for count 0, and
// negative duration for negative count.
func RatePerMinute(count int) time.Duration {
	if count == 0 {
		return 0
	}
	return time.Minute / time.Duration(count)
}
