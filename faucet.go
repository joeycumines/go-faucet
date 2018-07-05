package faucet

import (
	"sync"
	"time"
	"context"
)

type (
	Pipe struct {
		mutex   sync.Mutex
		ticker  *time.Ticker
		err     error
		done    chan struct{}
		stopped bool
		stop    sync.Once
		inputs []func(context.Context) (interface{}, bool, error)
		outputs []func(context.Context, interface{}) error
		ctx     context.Context
		cancel  context.CancelFunc
	}
)

func RatePerMinute(count int) time.Duration {
	if count == 0 {
		return 0
	}
	return time.Minute / time.Duration(count)
}
