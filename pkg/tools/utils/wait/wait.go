/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package wait provides loop and wait utility functions
package wait

import (
	"context"
	"math/rand"
	"time"
)

// NeverStop may be passed to Until to make it never stop.
var NeverStop <-chan struct{} = make(chan struct{})

// Until loops until stop channel is closed, running f every period.
func Until(f func() error, period time.Duration, ctx context.Context) {
	JitterUntil(f, period, 0.0, true, ctx)
}

// JitterUntil loops until stop channel is closed, running f every period.
//
// If jitterFactor is positive, the period is jittered before every run of f.
// If jitterFactor is not positive, the period is unchanged and not jittered.
//
// If sliding is true, the period is computed after f runs. If it is false then
// period includes the runtime for f.
//
// An initial delay of up to 1 minute (capped at period) is applied before the
// first invocation of f. The delay is jittered when jitterFactor is positive.
//
// Close stopCh to stop. f may not be invoked if stop channel is already
// closed. Pass NeverStop to if you don't want it stop.
func JitterUntil(f func() error, period time.Duration, jitterFactor float64, sliding bool, ctx context.Context) {
	// Random startup delay, max 1 minute.
	jitteredDelay := time.Minute
	if jitteredDelay > period {
		jitteredDelay = period
	}
	if jitterFactor > 0.0 {
		jitteredDelay = Jitter(jitteredDelay, jitterFactor)
	}
	select {
	case <-ctx.Done():
		return
	case <-time.After(jitteredDelay):
	}

	var t *time.Timer
	var sawTimeout bool

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		jitteredPeriod := period
		if jitterFactor > 0.0 {
			jitteredPeriod = Jitter(period, jitterFactor)
		}

		if !sliding {
			t = resetOrReuseTimer(t, jitteredPeriod, sawTimeout)
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					panic(r)
				}
			}()
			f()
		}()

		if sliding {
			t = resetOrReuseTimer(t, jitteredPeriod, sawTimeout)
		}

		// NOTE: b/c there is no priority selection in golang
		// it is possible for this to race, meaning we could
		// trigger t.C and stopCh, and t.C select falls through.
		// In order to mitigate we re-check stopCh at the beginning
		// of every loop to prevent extra executions of f().
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sawTimeout = true
		}
	}
}

// Jitter wait duration jitter
func Jitter(duration time.Duration, maxFactor float64) time.Duration {
	if maxFactor <= 0.0 {
		maxFactor = 1.0
	}
	wait := duration + time.Duration(rand.Float64()*maxFactor*float64(duration))
	return wait
}

// resetOrReuseTimer avoids allocating a new timer if one is already in use.
// Not safe for multiple threads.
func resetOrReuseTimer(t *time.Timer, d time.Duration, sawTimeout bool) *time.Timer {
	if t == nil {
		return time.NewTimer(d)
	}
	if !t.Stop() && !sawTimeout {
		<-t.C
	}
	t.Reset(d)
	return t
}
