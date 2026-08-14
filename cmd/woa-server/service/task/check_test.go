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

package task

import (
	"errors"
	"testing"

	"hcm/pkg/criteria/errf"
)

func TestBusinessRejectReason(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantOk  bool
		wantMsg string
	}{
		{name: "nil error", err: nil, wantOk: false},
		{name: "invalid parameter is business", err: errf.New(errf.InvalidParameter, "参数非法"),
			wantOk: true, wantMsg: "参数非法"},
		{name: "resplan verify failed is business", err: errf.New(errf.ResPlanVerifyFailed, "预测余量不足"),
			wantOk: true, wantMsg: "预测余量不足"},
		{name: "cvm apply verify failed is business", err: errf.New(errf.CvmApplyVerifyFailed, "GPU 计费时长不满足"),
			wantOk: true, wantMsg: "GPU 计费时长不满足"},
		{name: "plain error is system anomaly", err: errors.New("db down"), wantOk: false},
		{name: "aborted is system anomaly", err: errf.New(errf.Aborted, "downstream failed"), wantOk: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg, ok := businessRejectReason(tc.err)
			if ok != tc.wantOk {
				t.Fatalf("businessRejectReason() ok = %v, want %v", ok, tc.wantOk)
			}
			if ok && msg != tc.wantMsg {
				t.Errorf("businessRejectReason() msg = %q, want %q", msg, tc.wantMsg)
			}
		})
	}
}

