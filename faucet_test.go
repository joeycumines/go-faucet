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
