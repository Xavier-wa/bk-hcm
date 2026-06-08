/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
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

package enumor

import "testing"

func TestIntentType_Validate(t *testing.T) {
	tests := []struct {
		name    string
		intent  IntentType
		wantErr bool
	}{
		{
			name:    "host_apply is valid",
			intent:  IntentTypeHostApply,
			wantErr: false,
		},
		{
			name:    "resource_query is valid",
			intent:  IntentTypeResourceQuery,
			wantErr: false,
		},
		{
			name:    "chat is valid",
			intent:  IntentTypeChat,
			wantErr: false,
		},
		{
			name:    "unknown value fails validation",
			intent:  IntentType("unknown"),
			wantErr: true,
		},
		{
			name:    "empty string fails validation",
			intent:  IntentType(""),
			wantErr: true,
		},
		{
			name:    "partial match fails validation",
			intent:  IntentType("host"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.intent.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
