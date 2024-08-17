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

package faucet

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// AddInput adds fn to the pipe as input, note it will panic if fn or the pipe are nil.
func (p *Pipe) AddInput(fn func(ctx context.Context) (interface{}, bool, error)) {
	p.ensure()

	if fn == nil {
		panic(errors.New("faucet.AddInput nil fn"))
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.inputs = append(p.inputs, fn)
}

// AddOutput adds fn to the pipe as output, note it will panic if fn or the pipe are nil.
func (p *Pipe) AddOutput(fn func(ctx context.Context, value interface{}) error) {
	p.ensure()

	if fn == nil {
		panic(errors.New("faucet.AddOutput nil fn"))
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.outputs = append(p.outputs, fn)
}

// Err returns any internal error, which will be set on pipe worker failure, note it panics if the pipe is nil.
func (p *Pipe) Err() error {
	p.ensure()

	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.err
}

// Done returns the internal "done" channel, that will be closed after the worker exits (never if the pipe is never
// started), it will panic if the pipe is nil.
func (p *Pipe) Done() <-chan struct{} {
	p.ensure()

	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.done
}

// Start initialises the pipe worker with the provided context and duration, each pipe may be started exactly once,
// and it will panic if ctx or the pipe are nil, or rate is note greater than zero.
// Note that it will tick immediately (async), unlike time.Ticker.
func (p *Pipe) Start(ctx context.Context, rate time.Duration) {
	p.ensure()

	if ctx == nil {
		panic(errors.New("faucet.Pipe.Start nil context"))
	}

	if rate <= 0 {
		panic(errors.New("faucet.Pipe.Start rate <= 0"))
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.ticker != nil {
		panic(errors.New("faucet.Pipe.Start already started"))
	}

	p.ticker = time.NewTicker(rate)
	p.ctx, p.cancel = context.WithCancel(ctx)

	go p.worker()
	go p.cleanup()
}

// Stop will prevent further ticks from succeeding (that are not already in progress), note it will panic if the
// pipe is nil, or hasn't already been started.
func (p *Pipe) Stop() {
	p.ensure()

	if p.ticker == nil {
		panic(errors.New("faucet.Pipe.Stop not started"))
	}

	p.close.Do(func() {
		close(p.stop)
	})
}

func (p *Pipe) ensure() {
	if p == nil {
		panic(errors.New("faucet.Pipe nil receiver"))
	}

	p.open.Do(func() {
		p.stop = make(chan struct{})
		p.done = make(chan struct{})
	})
}

func (p *Pipe) cleanup() {
	defer p.Stop()
	defer func() {
		<-p.done
	}()
	defer p.ticker.Stop()
	<-p.ctx.Done()
}

func (p *Pipe) worker() {
	defer close(p.done)
	defer p.cancel()

	var (
		err       error
		lastInput int
		first     = true
	)

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("faucet.Pipe.input.%d recovered from panic (%T): %+v", lastInput, r, r)
		}

		p.mutex.Lock()
		defer p.mutex.Unlock()

		p.err = err
	}()

	for i := 0; ; i++ {
		err = p.ctx.Err()

		if err != nil {
			err = fmt.Errorf("faucet.Pipe context error: %v", err)
			return
		}

		// tick checks the input(s)
		tick := func() bool {
			p.mutex.Lock()
			defer p.mutex.Unlock()

			// double check if we are stopped (the channel might still have a tick after close)
			select {
			case <-p.stop:
				return false

			default:
			}

			inputLength, outputLength := len(p.inputs), len(p.outputs)

			for j := 0; j < inputLength; j++ {
				err = p.ctx.Err()

				if err != nil {
					err = fmt.Errorf("faucet.Pipe context error: %v", err)
					return false
				}

				var (
					value interface{}
					ok    bool
				)

				lastInput = (i + j) % inputLength

				value, ok, err = p.inputs[lastInput](p.ctx)

				if err != nil {
					// input error, will exit with error
					err = fmt.Errorf("faucet.Pipe.input.%d error: %v", lastInput, err)
					return false
				}

				if !ok {
					// try the next input
					continue
				}

				// fetched an input, fan out the output

				if outputLength == 0 {
					// nothing to fan out to, done for this tick
					return true
				}

				errs := make(chan error, outputLength)

				for x, output := range p.outputs {
					go func(x int, output func(context.Context, interface{}) error) {
						var err error

						defer func() {
							if r := recover(); r != nil {
								err = fmt.Errorf("faucet.Pipe.output.%d recovered from panic (%T): %+v", x, r, r)
							}

							errs <- err
						}()

						err = output(p.ctx, value)

						if err != nil {
							err = fmt.Errorf("faucet.Pipe.output.%d error: %v", x, err)
						}
					}(x, output)
				}

				for x := 0; x < outputLength; x++ {
					outputErr := <-errs

					if outputErr == nil {
						continue
					}

					if err == nil {
						err = outputErr
						continue
					}

					err = fmt.Errorf("%v | %v", err, outputErr)
				}

				// fanned out all output, possibly with errors, done for this tick
				return err == nil
			}

			// did not retrieve any input (from any input), but no error, done for this tick
			return true
		}

		// the first iteration will immediately tick (so as to start immediately)
		if first {
			first = false
			if !tick() {
				return
			}
			// next iteration (so i increments)
			continue
		}

		select {
		case <-p.stop:
			// stop has been called
			return

		case <-p.ctx.Done():
			// context has been canceled
			err = fmt.Errorf("faucet.Pipe context error: %v", p.ctx.Err())
			return

		case <-p.ticker.C:
			// ticker has been triggered, poll inputs
			if !tick() {
				return
			}
		}
	}
}
