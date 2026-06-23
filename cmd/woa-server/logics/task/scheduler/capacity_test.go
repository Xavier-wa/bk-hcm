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

package scheduler

import (
	"strings"
	"testing"

	types "hcm/cmd/woa-server/types/task"
)

func TestBuildCapacityReason(t *testing.T) {
	spec := &types.ResourceSpec{Region: "ap-guangzhou", DeviceType: "S5.LARGE8"}
	reason := buildCapacityReason(0, spec, []string{"ap-guangzhou-3", "ap-guangzhou-4"}, 10, 3)

	for _, want := range []string{"子单1", "S5.LARGE8", "ap-guangzhou", "ap-guangzhou-3/ap-guangzhou-4",
		"需要 10 台", "可用 3 台"} {
		if !strings.Contains(reason, want) {
			t.Errorf("capacity reason %q should contain %q", reason, want)
		}
	}
}
