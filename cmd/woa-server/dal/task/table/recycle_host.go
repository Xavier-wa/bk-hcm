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

// Package table defines the table structure of recycle host
package table

import (
	"time"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
)

// RecycleHost defines a recycle host's detail information
type RecycleHost struct {
	OrderID         uint64               `json:"order_id" bson:"order_id"`
	SuborderID      string               `json:"suborder_id" bson:"suborder_id"`
	BizID           int64                `json:"bk_biz_id" bson:"bk_biz_id"`
	BizName         string               `json:"bk_biz_name" bson:"bk_biz_name"`
	User            string               `json:"bk_username" bson:"bk_username"`
	HostID          int64                `json:"bk_host_id" bson:"bk_host_id"`
	AssetID         string               `json:"asset_id" bson:"asset_id"`
	IP              string               `json:"ip" bson:"ip"`
	BkHostOuterIP   string               `json:"bk_host_outerip" bson:"bk_host_outerip"`
	InstID          string               `json:"instance_id" bson:"instance_id"`
	DeviceType      string               `json:"device_type" bson:"device_type"`
	Zone            string               `json:"bk_zone_name" bson:"bk_zone_name"`
	SubZone         string               `json:"sub_zone" bson:"sub_zone"`
	CloudRegionID   string               `json:"cloud_region_id" bson:"cloud_region_id"`
	ModuleName      string               `json:"module_name" bson:"module_name"`
	Operator        string               `json:"operator" bson:"operator"`
	BakOperator     string               `json:"bak_operator" bson:"bak_operator"`
	InputTime       string               `json:"input_time" bson:"input_time"`
	Stage           RecycleStage         `json:"stage" bson:"stage"`
	Status          RecycleStatus        `json:"status" bson:"status"`
	ReturnID        string               `json:"return_id" bson:"return_id"`
	ReturnLink      string               `json:"return_link" bson:"return_link"`
	ReturnTag       string               `json:"return_tag" bson:"return_tag"`
	ReturnCostRate  float64              `json:"return_cost_rate" bson:"return_cost_rate"`
	ReturnPlanMsg   string               `json:"return_plan_msg" bson:"return_plan_msg"`
	ReturnTime      string               `json:"return_time" bson:"return_time"`
	CreateAt        time.Time            `json:"create_at" bson:"create_at"`
	UpdateAt        time.Time            `json:"update_at" bson:"update_at"`
	ResourceType    ResourceType         `json:"-" bson:"resource_type"`
	RecycleType     RecycleType          `json:"recycle_type" bson:"recycle_type"`
	ReturnPlan      RetPlanType          `json:"-" bson:"return_type"`
	Pool            PoolType             `json:"-" bson:"pool_type"`
	ObsProject      string               `json:"-" bson:"obs_project"`
	ReturnedWay     enumor.ReturnedWay   `json:"returned_way" bson:"returned_way"`
	DeviceGroup     string               `json:"device_group" bson:"device_group"`
	CpuCore         int64                `json:"cpu_core" bson:"cpu_core"`
	CoreType        enumor.CoreType      `json:"core_type" bson:"core_type"`
	SvrSourceTypeID cmdb.SvrSourceTypeID `json:"bk_svr_source_type_id"`
	Recyclable      bool                 `json:"-"`
	RecycleMessage  string               `json:"-"`
	// RollServerMatchedCore 记录该回收主机匹配到的滚服申领单及匹配的核数
	RollServerMatchedCore map[string]int64 `json:"-"`
	IsMatchRollServer     bool             `json:"-"`
	IsMatchShortRental    bool             `json:"-"`
}
