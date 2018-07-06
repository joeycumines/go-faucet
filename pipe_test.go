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
	"time"
	"fmt"
	"testing"
	"io"
	"errors"
)

func ExamplePipe_roundRobinPingPong() {
	var pipe Pipe

	x := 0
	y := 0

	pipe.AddInput(
		func(context.Context) (interface{}, bool, error) {
			x++
			return x, true, nil
		},
	)

	pipe.AddInput(
		func(context context.Context) (interface{}, bool, error) {
			y--
			return y, true, nil
		},
	)

	ch := make(chan struct{})

	pipe.AddOutput(
		func(ctx context.Context, value interface{}) error {
			fmt.Printf("ping: %v\n", value)
			ch <- struct{}{}
			return nil
		},
	)

	pipe.AddOutput(
		func(ctx context.Context, value interface{}) error {
			<-ch
			fmt.Printf("pong: %v\n", value)
			return nil
		},
	)

	pipe.Start(context.Background(), time.Millisecond*1000)

	go func() {
		time.Sleep(time.Millisecond * 5500)
		pipe.Stop()
	}()

	<-pipe.Done()

	// Output:
	// ping: 1
	// pong: 1
	// ping: -1
	// pong: -1
	// ping: 2
	// pong: 2
	// ping: -2
	// pong: -2
	// ping: 3
	// pong: 3
}

func ExamplePipe_fallbackInputs() {
	var pipe Pipe

	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			return nil, false, nil
		},
	)

	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			return 1, true, nil
		},
	)

	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			return nil, false, nil
		},
	)

	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			return 2, true, nil
		},
	)

	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			return -111, false, nil
		},
	)

	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			return 3, true, nil
		},
	)

	pipe.AddOutput(
		func(ctx context.Context, value interface{}) error {
			fmt.Println(value)
			return nil
		},
	)

	pipe.Start(context.Background(), time.Millisecond*50)

	time.Sleep(time.Second + (time.Millisecond * 25))

	pipe.Stop()

	<-pipe.Done()

	if err := pipe.Err(); err != nil {
		panic(err)
	}

	// Output:
	// 1
	// 1
	// 2
	// 2
	// 3
	// 3
	// 1
	// 1
	// 2
	// 2
	// 3
	// 3
	// 1
	// 1
	// 2
	// 2
	// 3
	// 3
	// 1
	// 1
}

func TestPipe_AddInput_nil(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
	}()
	var pipe Pipe
	pipe.AddInput(nil)
}

func TestPipe_AddOutput_nil(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
	}()
	var pipe Pipe
	pipe.AddOutput(nil)
}

func TestPipe_Err(t *testing.T) {
	var pipe Pipe
	pipe.err = io.EOF
	if pipe.Err() != io.EOF {
		t.Fatal("unexpected err")
	}
}

func TestPipe_Start_nilCtx(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
	}()
	var pipe Pipe
	pipe.Start(nil, 1)
}

func TestPipe_Start_zeroTicker(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
	}()
	var pipe Pipe
	pipe.Start(context.Background(), 0)
}

func TestPipe_Start_negTicker(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
	}()
	var pipe Pipe
	pipe.Start(context.Background(), -1)
}

func TestPipe_Start_twice(t *testing.T) {
	var startedOnce bool
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
		if !startedOnce {
			t.Fatal("not started once")
		}
	}()
	var pipe Pipe
	pipe.Start(context.Background(), 1)
	startedOnce = true
	pipe.Start(context.Background(), 1)
}

func TestPipe_Stop_notStarted(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
	}()
	var pipe Pipe
	pipe.Stop()
}

func TestPipe_ensure_nilReceiver(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
	}()
	var pipe *Pipe
	pipe.ensure()
}

func TestPipe_inputPanic(t *testing.T) {
	var pipe Pipe
	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			return nil, false, nil
		},
	)
	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			panic("some_panic")
		},
	)
	pipe.Start(context.Background(), 1)
	<-pipe.Done()
	err := pipe.Err()
	if err == nil || err.Error() != "faucet.Pipe.input.1 recovered from panic (string): some_panic" {
		t.Fatal("unexpected error", err)
	}
}

func TestPipe_outputPanic(t *testing.T) {
	var pipe Pipe
	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			return nil, true, nil
		},
	)
	pipe.AddOutput(
		func(ctx context.Context, value interface{}) error {
			panic("some_panic")
		},
	)
	pipe.Start(context.Background(), 1)
	<-pipe.Done()
	err := pipe.Err()
	if err == nil || err.Error() != "faucet.Pipe.output.0 recovered from panic (string): some_panic" {
		t.Fatal("unexpected error", err)
	}
}

func TestPipe_contextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var pipe Pipe
	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			return nil, true, nil
		},
	)
	pipe.Start(ctx, time.Millisecond)
	time.Sleep(time.Millisecond * 100)
	select {
	case <-pipe.Done():
		t.Fatal("expected not done")
	default:
	}
	cancel()
	<-pipe.Done()
	err := pipe.Err()
	if err == nil || err.Error() != "faucet.Pipe context error: context canceled" {
		t.Fatal("unexpected error", err)
	}
}

func TestPipe_inputError(t *testing.T) {
	var pipe Pipe

	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			return nil, false, errors.New("some_error")
		},
	)

	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			t.Error("shouldn't be called")
			panic("unexpected call")
		},
	)

	pipe.Start(context.Background(), 1)

	<-pipe.Done()

	err := pipe.Err()

	if err == nil || err.Error() != "faucet.Pipe.input.0 error: some_error" {
		t.Fatal("unexpected error", err)
	}
}

func TestPipe_outputError(t *testing.T) {
	var pipe Pipe

	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			if ctx.Value(1) != 2 {
				t.Error("unexpected ctx", ctx)
			}
			return "some_value", true, nil
		},
	)

	pipe.AddOutput(
		func(ctx context.Context, value interface{}) error {
			if ctx.Value(1) != 2 {
				t.Error("unexpected ctx", ctx)
			}
			if value != "some_value" {
				t.Error("unexpected value", value)
			}
			return errors.New("error_1")
		},
	)

	pipe.AddOutput(
		func(ctx context.Context, value interface{}) error {
			if ctx.Value(1) != 2 {
				t.Error("unexpected ctx", ctx)
			}
			if value != "some_value" {
				t.Error("unexpected value", value)
			}
			time.Sleep(time.Millisecond * 100)
			return errors.New("error_2")
		},
	)

	pipe.AddOutput(
		func(ctx context.Context, value interface{}) error {
			if ctx.Value(1) != 2 {
				t.Error("unexpected ctx", ctx)
			}
			if value != "some_value" {
				t.Error("unexpected value", value)
			}
			time.Sleep(time.Millisecond * 200)
			return errors.New("error_3")
		},
	)

	pipe.Start(context.WithValue(context.Background(), 1, 2), 1)

	<-pipe.Done()

	err := pipe.Err()

	if err == nil || err.Error() != "faucet.Pipe.output.0 error: error_1 | faucet.Pipe.output.1 error: error_2 | faucet.Pipe.output.2 error: error_3" {
		t.Fatal("unexpected err", err)
	}
}

func TestPipe_noInput_1(t *testing.T) {
	var pipe Pipe

	pipe.AddOutput(
		func(ctx context.Context, value interface{}) error {
			t.Error("should never be called")
			panic("should never be called")
		},
	)

	pipe.Start(context.Background(), time.Millisecond)

	time.Sleep(time.Millisecond * 100)

	pipe.Stop()

	<-pipe.Done()

	if err := pipe.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestPipe_noInput_2(t *testing.T) {
	var pipe Pipe

	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			return 12141, false, nil
		},
	)
	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			return 12141, false, nil
		},
	)
	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			return nil, false, nil
		},
	)

	pipe.AddOutput(
		func(ctx context.Context, value interface{}) error {
			t.Error("should never be called")
			panic("should never be called")
		},
	)

	pipe.Start(context.Background(), time.Millisecond)

	time.Sleep(time.Millisecond * 100)

	pipe.Stop()

	<-pipe.Done()

	if err := pipe.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestPipe_noOutput(t *testing.T) {
	var (
		pipe  Pipe
		count int
	)

	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			count++
			return 3, true, nil
		},
	)

	pipe.Start(context.Background(), time.Millisecond*150)

	time.Sleep(time.Millisecond * 200)

	pipe.Stop()

	<-pipe.Done()

	if err := pipe.Err(); err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatal("unexpected count", count)
	}
}

func TestPipe_addAfterStart(t *testing.T) {
	var (
		pipe   Pipe
		input  = make(chan struct{})
		output = make(chan struct{})
	)

	pipe.Start(context.Background(), time.Millisecond)

	time.Sleep(time.Millisecond * 50)

	pipe.AddInput(
		func(ctx context.Context) (interface{}, bool, error) {
			input <- struct{}{}
			return true, true, nil
		},
	)

	pipe.AddOutput(
		func(ctx context.Context, value interface{}) error {
			output <- struct{}{}
			return nil
		},
	)

	<-input

	time.Sleep(time.Millisecond * 50)

	pipe.Stop()

	<-output

	<-pipe.Done()

	if err := pipe.Err(); err != nil {
		t.Fatal(err)
	}
}
