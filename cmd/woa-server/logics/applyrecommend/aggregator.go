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

// Package applyrecommend provides logic for offline apply recommend stats.
package applyrecommend

import (
	"sort"

	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/enumor"
)

// userCountKey is the six-tuple key for user dimension aggregation.
type userCountKey struct {
	BkBizID     int64
	BkUsername  string
	RequireType enumor.RequireType
	Region      string
	DeviceType  string
	ImageID     string
}

// bizCountKey is the five-tuple key for biz dimension aggregation.
type bizCountKey struct {
	BkBizID     int64
	RequireType enumor.RequireType
	Region      string
	DeviceType  string
	ImageID     string
}

type bizUserKey struct {
	BkBizID    int64
	BkUsername string
}

// AggregateUser takes a user count map and returns per-biz Top-K user recommendations.
// Returns map[bkBizID][]createReq.
func AggregateUser(userCounts map[userCountKey]int,
	maxRows int) map[bizUserKey][]cvmapplyproto.ZiyanCvmApplyUserRecommendCreateReq {

	type item struct {
		key   userCountKey
		count int
	}

	// group by (bk_biz_id, bk_username)
	groups := make(map[bizUserKey][]item)
	for k, cnt := range userCounts {
		gk := bizUserKey{BkBizID: k.BkBizID, BkUsername: k.BkUsername}
		groups[gk] = append(groups[gk], item{key: k, count: cnt})
	}

	result := make(map[bizUserKey][]cvmapplyproto.ZiyanCvmApplyUserRecommendCreateReq)
	for gk, items := range groups {
		// sort by count DESC
		sort.Slice(items, func(i, j int) bool {
			return items[i].count > items[j].count
		})
		limit := maxRows
		if len(items) < limit {
			limit = len(items)
		}
		for _, it := range items[:limit] {
			result[gk] = append(result[gk], cvmapplyproto.ZiyanCvmApplyUserRecommendCreateReq{
				BkBizID:     it.key.BkBizID,
				BkUsername:  it.key.BkUsername,
				RequireType: it.key.RequireType,
				Region:      it.key.Region,
				DeviceType:  it.key.DeviceType,
				ImageID:     it.key.ImageID,
				Count:       it.count,
			})
		}
	}
	return result
}

// AggregateBiz takes a biz count map and returns per-biz Top-K biz recommendations.
// Returns map[bkBizID][]createReq.
func AggregateBiz(bizCounts map[bizCountKey]int,
	maxRows int) map[int64][]cvmapplyproto.ZiyanCvmApplyBizRecommendCreateReq {

	type item struct {
		key   bizCountKey
		count int
	}

	// group by bk_biz_id
	groups := make(map[int64][]item)
	for k, cnt := range bizCounts {
		groups[k.BkBizID] = append(groups[k.BkBizID], item{key: k, count: cnt})
	}

	result := make(map[int64][]cvmapplyproto.ZiyanCvmApplyBizRecommendCreateReq)
	for bizID, items := range groups {
		// sort by count DESC
		sort.Slice(items, func(i, j int) bool {
			return items[i].count > items[j].count
		})
		limit := maxRows
		if len(items) < limit {
			limit = len(items)
		}
		for _, it := range items[:limit] {
			result[bizID] = append(result[bizID], cvmapplyproto.ZiyanCvmApplyBizRecommendCreateReq{
				BkBizID:     it.key.BkBizID,
				RequireType: it.key.RequireType,
				Region:      it.key.Region,
				DeviceType:  it.key.DeviceType,
				ImageID:     it.key.ImageID,
				Count:       it.count,
			})
		}
	}
	return result
}
