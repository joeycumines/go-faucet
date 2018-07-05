package faucet

import (
	"testing"
	"time"
	"fmt"
)

func TestRatePerMinute_success(t *testing.T) {
	type TestCase struct {
		Count    int
		Duration time.Duration
	}

	testCases := []TestCase{
		{
			Count:    1,
			Duration: time.Minute,
		},
		{
			Count:    2,
			Duration: time.Second * 30,
		},
		{
			Count:    3,
			Duration: time.Minute / 3,
		},
		{
			Count:    0,
			Duration: 0,
		},
	}

	for i, testCase := range testCases {
		name := fmt.Sprintf("TestRatePerMinute_success_#%d", i+1)

		duration := RatePerMinute(testCase.Count)

		if duration != testCase.Duration {
			t.Error(name, "duration", duration, "!= expected", testCase.Duration)
		}
	}
}
