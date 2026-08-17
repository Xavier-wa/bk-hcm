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

package applyrecommend

import (
	"testing"

	"hcm/pkg/criteria/enumor"
	cvmapply "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/kit"

	"github.com/stretchr/testify/assert"
)

func TestSkipDeviceForCount_RequireType(t *testing.T) {
	kt := &kit.Kit{Rid: "test-rid"}
	validImages := map[string]struct{}{"img-1": {}}

	tests := []struct {
		name        string
		requireType enumor.RequireType
		wantSkip    bool
	}{
		{name: "regular", requireType: enumor.RequireTypeRegular, wantSkip: false},
		{name: "spring", requireType: enumor.RequireTypeSpring, wantSkip: false},
		{name: "dissolve", requireType: enumor.RequireTypeDissolve, wantSkip: false},
		{name: "roll server", requireType: enumor.RequireTypeRollServer, wantSkip: false},
		{name: "green channel", requireType: enumor.RequireTypeGreenChannel, wantSkip: false},
		{name: "spring res pool", requireType: enumor.RequireTypeSpringResPool, wantSkip: false},
		{name: "short lease", requireType: enumor.RequireTypeShortLease, wantSkip: false},
		{name: "zero", requireType: 0, wantSkip: true},
		{name: "type 4", requireType: 4, wantSkip: true},
		{name: "type 5", requireType: 5, wantSkip: true},
		{name: "unknown 99", requireType: 99, wantSkip: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := newCountDevice(tc.requireType)
			got := skipDeviceForCount(kt, item, validImages)
			assert.Equal(t, tc.wantSkip, got)
		})
	}
}

func newCountDevice(requireType enumor.RequireType) *cvmapply.ZiyanCvmDeviceInfo {
	return &cvmapply.ZiyanCvmDeviceInfo{
		ID:          "dev-1",
		CloudRegion: "ap-guangzhou",
		ImageID:     "img-1",
		RequireType: requireType,
	}
}
