/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package enumor

import "fmt"

// ResSyncSource is the `source` label value of the res sync metrics.
type ResSyncSource string

const (
	// ResSyncSourceFull 全量同步（cloud-server 定时任务触发）。
	ResSyncSourceFull ResSyncSource = "full"
	// ResSyncSourceIncremental 增量同步（cc watch 消费触发）。
	ResSyncSourceIncremental ResSyncSource = "incremental"
)

// Validate ...
func (s ResSyncSource) Validate() error {
	switch s {
	case ResSyncSourceFull, ResSyncSourceIncremental:
		return nil
	default:
		return fmt.Errorf("invalid res sync source: %s", s)
	}
}

// ResSyncStep is the `step` label value of the res sync cost metric.
// 主机同步的环节名各 vendor 通用；其他资源（disk/eip 等）环节不同，自行定义。
type ResSyncStep string

const (
	// ResSyncStepTotal 一次同步请求的整体耗时，各资源通用。
	ResSyncStepTotal ResSyncStep = "total"
	// ResSyncStepHost 主机数据同步段。
	ResSyncStepHost ResSyncStep = "host"
	// ResSyncStepRelRes 关联资源本体同步段（多段聚合同名）。
	ResSyncStepRelRes ResSyncStep = "rel_res"
	// ResSyncStepHostRel 主机与关联资源的关系同步段（多段聚合同名）。
	ResSyncStepHostRel ResSyncStep = "host_rel"
)

// Validate ...
func (s ResSyncStep) Validate() error {
	switch s {
	case ResSyncStepTotal, ResSyncStepHost, ResSyncStepRelRes, ResSyncStepHostRel:
		return nil
	default:
		return fmt.Errorf("invalid res sync step: %s", s)
	}
}

// ResSyncResult is the `result` label value of the res sync metrics.
type ResSyncResult string

const (
	// ResSyncResultSuccess 同步成功。
	ResSyncResultSuccess ResSyncResult = "success"
	// ResSyncResultFailed 同步失败。
	ResSyncResultFailed ResSyncResult = "failed"
)

// Validate ...
func (r ResSyncResult) Validate() error {
	switch r {
	case ResSyncResultSuccess, ResSyncResultFailed:
		return nil
	default:
		return fmt.Errorf("invalid res sync result: %s", r)
	}
}
