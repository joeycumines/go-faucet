package faucet

import (
	"context"
	"time"
	"fmt"
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
