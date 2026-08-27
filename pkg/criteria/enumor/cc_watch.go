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

// CCWatchOp is the `operation` label value of the cc watch host dispatch metrics.
type CCWatchOp string

const (
	// CCWatchOpUpsert 主机 upsert 下发批。
	CCWatchOpUpsert CCWatchOp = "upsert"
	// CCWatchOpDelete 主机删除下发批。
	CCWatchOpDelete CCWatchOp = "delete"
)

// Validate ...
func (op CCWatchOp) Validate() error {
	switch op {
	case CCWatchOpUpsert, CCWatchOpDelete:
		return nil
	default:
		return fmt.Errorf("invalid cc watch operation: %s", op)
	}
}

// CCWatchResult is the `result` label value of the cc watch sub-batch sync metrics.
type CCWatchResult string

const (
	// CCWatchResultSuccess 子批同步成功。
	CCWatchResultSuccess CCWatchResult = "success"
	// CCWatchResultFailed 子批同步失败。
	CCWatchResultFailed CCWatchResult = "failed"
)

// Validate ...
func (r CCWatchResult) Validate() error {
	switch r {
	case CCWatchResultSuccess, CCWatchResultFailed:
		return nil
	default:
		return fmt.Errorf("invalid cc watch result: %s", r)
	}
}

// CCWatchStep is the step name of one cc watch batch in the timing collector.
type CCWatchStep string

const (
	// CCWatchStepCCWatch 从 cc 长轮询拉取事件。
	CCWatchStepCCWatch CCWatchStep = "cc_watch"
	// CCWatchStepUpsert 主机 upsert 下发段。
	CCWatchStepUpsert CCWatchStep = "upsert"
	// CCWatchStepDelete 主机删除下发段。
	CCWatchStepDelete CCWatchStep = "delete"
	// CCWatchStepConsume 消费阶段（含分发）。
	CCWatchStepConsume CCWatchStep = "consume"
)

// Validate ...
func (s CCWatchStep) Validate() error {
	switch s {
	case CCWatchStepCCWatch, CCWatchStepUpsert, CCWatchStepDelete, CCWatchStepConsume:
		return nil
	default:
		return fmt.Errorf("invalid cc watch step: %s", s)
	}
}

// CCWatchPatchStatus is the dispatch status of one (vendor, space) patch.
type CCWatchPatchStatus string

const (
	// CCWatchPatchStatusRunning 批下发中。
	CCWatchPatchStatusRunning CCWatchPatchStatus = "running"
	// CCWatchPatchStatusDone 批下发完成。
	CCWatchPatchStatusDone CCWatchPatchStatus = "done"
)

// Validate ...
func (s CCWatchPatchStatus) Validate() error {
	switch s {
	case CCWatchPatchStatusRunning, CCWatchPatchStatusDone:
		return nil
	default:
		return fmt.Errorf("invalid cc watch patch status: %s", s)
	}
}
