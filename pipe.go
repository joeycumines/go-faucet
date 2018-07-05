package faucet

import (
	"time"
	"errors"
	"context"
	"fmt"
)

func (p *Pipe) AddInput(fn func(ctx context.Context) (interface{}, bool, error)) {
	p.ensure()

	if fn == nil {
		panic(errors.New("faucet.AddInput nil fn"))
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.inputs = append(p.inputs, fn)
}

func (p *Pipe) AddOutput(fn func(ctx context.Context, value interface{}) error) {
	p.ensure()

	if fn == nil {
		panic(errors.New("faucet.AddOutput nil fn"))
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.outputs = append(p.outputs, fn)
}

func (p *Pipe) Err() error {
	p.ensure()

	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.err
}

func (p *Pipe) Done() <-chan struct{} {
	p.ensure()

	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.done
}

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

func (p *Pipe) Stop() {
	p.ensure()

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.ticker == nil {
		panic(errors.New("faucet.Pipe.Stop not started"))
	}

	p.stopped = true

	p.stop.Do(func() {
		p.ticker.Stop()
		close(p.done)
	})
}

func (p *Pipe) ensure() {
	if p == nil {
		panic(errors.New("faucet.Pipe nil receiver"))
	}

	if p.done != nil {
		return
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.done != nil {
		return
	}

	p.done = make(chan struct{})
}

func (p *Pipe) cleanup() {
	defer p.Stop()
	<-p.ctx.Done()
}

func (p *Pipe) worker() {
	defer p.cancel()

	var err error

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("faucet.Pipe recovered from panic (%T): %+v", r, r)
		}

		p.mutex.Lock()
		defer p.mutex.Unlock()

		p.err = err
	}()

	for i := 0; err == nil && !p.stopped; i++ {
		err = p.ctx.Err()

		if err != nil {
			err = fmt.Errorf("faucet.Pipe context error: %v", err)
			return
		}

		select {
		case <-p.done:
			// stop has been called
			return

		case <-p.ctx.Done():
			// context has been canceled
			err = fmt.Errorf("faucet.Pipe context error: %v", p.ctx.Err())
			return

		case <-p.ticker.C:
			// ticker has been triggered, poll inputs
			func() {
				p.mutex.Lock()
				defer p.mutex.Unlock()

				if p.stopped {
					// avoid a race on stop, will exit without error
					return
				}

				inputLength, outputLength := len(p.inputs), len(p.outputs)

				for j := 0; j < inputLength; j++ {
					var (
						value interface{}
						ok    bool
					)

					x := (i + j) % inputLength

					value, ok, err = p.inputs[x](p.ctx)

					if err != nil {
						// input error, will exit with error
						err = fmt.Errorf("faucet.Pipe.input.%d error: %v", x, err)
						return
					}

					if !ok {
						// try the next input
						continue
					}

					// fetched an input, fan out the output

					if outputLength == 0 {
						// nothing to fan out to, done for this tick
						return
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
					return
				}
			}()
		}
	}
}
