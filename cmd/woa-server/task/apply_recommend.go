/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

// Package task ...
package task

import (
	"time"

	"hcm/cmd/woa-server/logics/applyrecommend"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/enumor"
	croncore "hcm/pkg/cron/core"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/serviced"
)

// ApplyRecommendOfflineTask is the cron task for apply recommend offline stats.
type ApplyRecommendOfflineTask struct {
	clientSet *client.ClientSet
	sd        serviced.State
}

// NewApplyRecommendOfflineTask creates a new apply recommend offline task.
func NewApplyRecommendOfflineTask(clientSet *client.ClientSet, sd serviced.State) (croncore.Task, error) {
	return &ApplyRecommendOfflineTask{
		clientSet: clientSet,
		sd:        sd,
	}, nil
}

// Name returns the task name.
func (t *ApplyRecommendOfflineTask) Name() string {
	return string(enumor.CronTaskApplyRecommendOffline)
}

// Next returns the next execution time.
func (t *ApplyRecommendOfflineTask) Next() (time.Time, error) {
	interval := cc.WoaServer().ApplyRecommend.Interval
	return time.Now().Add(time.Duration(interval) * time.Minute), nil
}

// GetURL returns the URL used to trigger the task externally.
func (t *ApplyRecommendOfflineTask) GetURL() string {
	return "/apply_recommend/sync"
}

// Do executes the task.
func (t *ApplyRecommendOfflineTask) Do(kt *kit.Kit) error {
	if t.sd == nil || !t.sd.IsMaster() {
		logs.V(5).Infof("current node is not master, skip apply recommend offline task, rid: %s", kt.Rid)
		return nil
	}

	logs.Infof("master node executing apply recommend offline task, rid: %s", kt.Rid)

	logics := applyrecommend.NewLogics(t.clientSet)
	if err := logics.GenerateRecommend(kt); err != nil {
		logs.Errorf("generate apply recommend failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	logs.Infof("apply recommend offline task success, rid: %s", kt.Rid)
	return nil
}
