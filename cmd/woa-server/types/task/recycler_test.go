/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2022 THL A29 Limited, a Tencent company. All rights reserved.
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
	"testing"
	"time"

	"hcm/pkg"
	"hcm/pkg/criteria/mapstr"
	"hcm/pkg/tools/metadata"

	"github.com/stretchr/testify/assert"
)

func TestGetRecycleHostReq_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     GetRecycleHostReq
		wantErr bool
	}{
		{
			name: "valid return time range",
			req: GetRecycleHostReq{
				BizID:       []int64{2},
				ReturnStart: "2024-01-01",
				ReturnEnd:   "2024-07-28",
				Page:        metadata.BasePage{Limit: 10},
			},
			wantErr: false,
		},
		{
			name: "valid create time range",
			req: GetRecycleHostReq{
				BizID:       []int64{2},
				CreateStart: "2024-06-01",
				CreateEnd:   "2024-07-28",
				Page:        metadata.BasePage{Limit: 10},
			},
			wantErr: false,
		},
		{
			name: "valid both time ranges",
			req: GetRecycleHostReq{
				BizID:       []int64{2},
				ReturnStart: "2024-01-01",
				ReturnEnd:   "2024-07-28",
				CreateStart: "2024-06-01",
				CreateEnd:   "2024-07-28",
				Page:        metadata.BasePage{Limit: 10},
			},
			wantErr: false,
		},
		{
			name: "no time range",
			req: GetRecycleHostReq{
				BizID: []int64{2},
				Page:  metadata.BasePage{Limit: 10},
			},
			wantErr: false,
		},
		{
			name: "return start only",
			req: GetRecycleHostReq{
				BizID:       []int64{2},
				ReturnStart: "2024-01-01",
				Page:        metadata.BasePage{Limit: 10},
			},
			wantErr: true,
		},
		{
			name: "create end only",
			req: GetRecycleHostReq{
				BizID:     []int64{2},
				CreateEnd: "2024-07-28",
				Page:      metadata.BasePage{Limit: 10},
			},
			wantErr: true,
		},
		{
			name: "return start later than end",
			req: GetRecycleHostReq{
				BizID:       []int64{2},
				ReturnStart: "2024-07-28",
				ReturnEnd:   "2024-01-01",
				Page:        metadata.BasePage{Limit: 10},
			},
			wantErr: true,
		},
		{
			name: "invalid return date format",
			req: GetRecycleHostReq{
				BizID:       []int64{2},
				ReturnStart: "2024/01/01",
				ReturnEnd:   "2024-07-28",
				Page:        metadata.BasePage{Limit: 10},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestGetRecycleHostReq_GetFilter(t *testing.T) {
	tests := []struct {
		name    string
		req     GetRecycleHostReq
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "return time filter only",
			req: GetRecycleHostReq{
				BizID:       []int64{2},
				ReturnStart: "2024-01-01",
				ReturnEnd:   "2024-07-28",
			},
			want: map[string]interface{}{
				"bk_biz_id": mapstrIn(2),
				"return_time": map[string]interface{}{
					pkg.BKDBGTE: "2024-01-01",
					pkg.BKDBLT:  "2024-07-29",
				},
			},
		},
		{
			name: "create time filter only",
			req: GetRecycleHostReq{
				BizID:       []int64{2},
				CreateStart: "2024-06-01",
				CreateEnd:   "2024-07-28",
			},
			want: map[string]interface{}{
				"bk_biz_id": mapstrIn(2),
				"create_at": map[string]interface{}{
					pkg.BKDBGTE: mustParseTime("2024-06-01"),
					pkg.BKDBLT:  mustParseTime("2024-07-29"),
				},
			},
		},
		{
			name: "both time filters",
			req: GetRecycleHostReq{
				BizID:       []int64{2},
				ReturnStart: "2024-01-01",
				ReturnEnd:   "2024-07-28",
				CreateStart: "2024-06-01",
				CreateEnd:   "2024-07-28",
			},
			want: map[string]interface{}{
				"bk_biz_id": mapstrIn(2),
				"return_time": map[string]interface{}{
					pkg.BKDBGTE: "2024-01-01",
					pkg.BKDBLT:  "2024-07-29",
				},
				"create_at": map[string]interface{}{
					pkg.BKDBGTE: mustParseTime("2024-06-01"),
					pkg.BKDBLT:  mustParseTime("2024-07-29"),
				},
			},
		},
		{
			name: "no time filter",
			req: GetRecycleHostReq{
				BizID: []int64{2},
			},
			want: map[string]interface{}{
				"bk_biz_id": mapstrIn(2),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter, err := tc.req.GetFilter()
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, filter)
		})
	}
}

func mapstrIn(bizID int64) mapstr.MapStr {
	return mapstr.MapStr{
		pkg.BKDBIN: []int64{bizID},
	}
}

func mustParseTime(date string) time.Time {
	t, err := time.Parse(dateLayout, date)
	if err != nil {
		panic(err)
	}
	return t
}
