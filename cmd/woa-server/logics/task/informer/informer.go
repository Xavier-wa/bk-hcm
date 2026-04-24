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

// Package informer define informer interface with leader election support
package informer

import (
	"context"
	"sync"
	"time"

	"hcm/cmd/woa-server/logics/task/informer/apply"
	"hcm/cmd/woa-server/logics/task/informer/generate"
	"hcm/cmd/woa-server/storage/dal"
	"hcm/cmd/woa-server/storage/stream"
	ziyan "hcm/pkg/client/data-service/tcloud-ziyan"
	"hcm/pkg/logs"
	"hcm/pkg/serviced"
)

// Interface informer interface
type Interface interface {
	// Apply apply informer interface
	Apply() apply.Interface
	// Generate generate informer interface
	Generate() generate.Interface
	// Close stops leader monitor and all running informers
	Close()
}

// leaderAwareInformer wraps informer with leader election support
type leaderAwareInformer struct {
	sd      serviced.State
	loopW   stream.LoopInterface
	watchDB dal.DB

	// actual informers
	applyInformer    apply.Interface
	generateInformer generate.Interface
	ziyanClient      *ziyan.Client

	// control flags
	started   bool
	startedMu sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// New create a leader-aware informer that only runs on master node
func New(ziyanClient *ziyan.Client, sd serviced.State) (Interface, error) {
	ctx, cancel := context.WithCancel(context.Background())

	lai := &leaderAwareInformer{
		sd:          sd,
		ctx:         ctx,
		cancel:      cancel,
		ziyanClient: ziyanClient,
	}

	if sd.IsMaster() {
		if err := lai.startInformers(); err != nil {
			logs.Errorf("failed to start informers on init, err: %v", err)
		}
	}

	// Start monitoring leader state changes
	go lai.monitorLeaderState()

	return lai, nil
}

// monitorLeaderState monitors leader election state and starts/stops informers accordingly
func (lai *leaderAwareInformer) monitorLeaderState() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastIsMaster bool

	for {
		select {
		case <-lai.ctx.Done():
			logs.Infof("leader monitor stopped")
			return
		case <-ticker.C:
			isMaster := lai.sd.IsMaster()

			if isMaster == lastIsMaster {
				continue
			}

			// State changed
			lastIsMaster = isMaster
			if isMaster {
				logs.Infof("current node become master, starting MongoDB informers...")
				if err := lai.startInformers(); err != nil {
					logs.Errorf("failed to start informers, err: %v", err)
					lastIsMaster = false
				}
				continue
			}
			logs.Infof("current node become follower, stopping MongoDB informers...")
			lai.stopInformers()
		}
	}
}

// startInformers starts all MongoDB change stream informers
func (lai *leaderAwareInformer) startInformers() error {
	lai.startedMu.Lock()
	defer lai.startedMu.Unlock()

	if lai.started {
		logs.Warnf("informers already started, skip")
		return nil
	}

	var err error

	// Start apply informer
	lai.applyInformer, err = apply.New(lai.ziyanClient)
	if err != nil {
		logs.Errorf("failed to start apply informer, err: %v", err)
		return err
	}

	// Start generate informer
	lai.generateInformer, err = generate.New(lai.ziyanClient)
	if err != nil {
		lai.applyInformer.Stop()
		lai.applyInformer = nil
		logs.Errorf("failed to start generate informer, err: %v", err)
		return err
	}

	lai.started = true
	logs.Infof("all MongoDB informers started successfully")
	return nil
}

// stopInformers stops all MongoDB change stream informers
func (lai *leaderAwareInformer) stopInformers() {
	lai.startedMu.Lock()
	defer lai.startedMu.Unlock()

	if !lai.started {
		logs.Warnf("informers not started, skip stop")
		return
	}

	if lai.applyInformer != nil {
		lai.applyInformer.Stop()
	}
	if lai.generateInformer != nil {
		lai.generateInformer.Stop()
	}

	lai.applyInformer = nil
	lai.generateInformer = nil

	lai.started = false
	logs.Infof("all MongoDB informers stopped")
}

// Close stops leader monitor and all MongoDB informers
func (lai *leaderAwareInformer) Close() {
	lai.cancel()
	lai.stopInformers()
}

// Apply returns apply informer (returns nil if informers are not running)
func (lai *leaderAwareInformer) Apply() apply.Interface {
	lai.startedMu.RLock()
	defer lai.startedMu.RUnlock()
	return lai.applyInformer
}

// Generate returns generate informer (returns nil if informers are not running)
func (lai *leaderAwareInformer) Generate() generate.Interface {
	lai.startedMu.RLock()
	defer lai.startedMu.RUnlock()
	return lai.generateInformer
}
