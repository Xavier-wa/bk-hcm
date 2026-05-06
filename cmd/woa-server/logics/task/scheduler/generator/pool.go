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

// Package generator generate task
package generator

import (
	"fmt"
	"strconv"

	pooltable "hcm/cmd/woa-server/dal/pool/table"
	pooltypes "hcm/cmd/woa-server/types/pool"
	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/utils"
)

const (
	// PoolOrderLinkPrefix pool order link prefix
	PoolOrderLinkPrefix = "http://scr.ied.com/#/resource-manage/online/detail?id="
)

// launchRecallHost launch recall pool host
func (g *Generator) launchRecallHost(kt *kit.Kit, order *types.ApplyOrder, recall *types.MatchPoolSpec) (string,
	error) {
	// 1. init generate record
	generateId, err := g.initGenerateRecord(kt, order.ResourceType, order.SubOrderId, uint(recall.Replicas), false)
	if err != nil {
		logs.Errorf("failed to init generate record, order id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return "", fmt.Errorf("failed to init generate record, order id: %s, err: %v", order.SubOrderId, err)
	}

	// 2. create and check recall order
	taskID, err := g.createAndCheckRecallOrder(kt, order, recall, generateId)
	if err != nil {
		logs.Errorf("failed to create and check recall order, order id: %s, err: %v, rid: %s",
			order.SubOrderId, err, kt.Rid)
		return generateId, err
	}

	// 3. get pool recalled instances
	hosts, err := g.listRecalledInstance(kt, taskID)
	if err != nil {
		logs.Errorf("failed to list recalled hosts, order id: %s, recall order id: %s, err: %v, rid: %s",
			order.SubOrderId, taskID, err, kt.Rid)

		// update generate record status to Done
		if errRecord := g.UpdateGenerateRecord(kt, order, generateId,
			types.GenerateStatusFailed, err.Error(), "", nil); errRecord != nil {
			logs.Errorf("failed to update generate record, order id: %s, recall order id: %s, err: %v, rid: %s",
				order.SubOrderId, taskID, errRecord, kt.Rid)
			return generateId, fmt.Errorf("failed to update generate record, order id: %s, recall order id: %d,"+
				"err: %v", order.SubOrderId, taskID, errRecord)
		}

		return generateId, fmt.Errorf("failed to list recalled hosts, order id: %s, recall order id: %d, err: %v",
			order.SubOrderId, taskID, err)
	}

	deviceList := make([]*types.DeviceInfo, 0)
	successIps := make([]string, 0)
	recallTaskID := strconv.Itoa(int(taskID))
	for _, host := range hosts {
		deviceList = append(deviceList, &types.DeviceInfo{
			Ip:               host.Labels[pooltable.IPKey],
			AssetId:          host.Labels[pooltable.AssetIDKey],
			GenerateTaskId:   recallTaskID,
			GenerateTaskLink: PoolOrderLinkPrefix + recallTaskID,
			Deliverer:        "icr",
		})
		successIps = append(successIps, host.Labels[pooltable.IPKey])
	}

	// 4. save recalled instances info
	if err = g.createGeneratedDevices(kt, order, generateId, deviceList); err != nil {
		logs.Errorf("failed to update generated device, order id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return generateId, fmt.Errorf("failed to update generated device, order id: %s, err: %v", order.SubOrderId, err)
	}

	// 5. update generate record status to success
	if err = g.UpdateGenerateRecord(kt, order, generateId, types.GenerateStatusSuccess,
		"success", "", successIps); err != nil {
		logs.Errorf("failed to update generate record, order id: %s, recall order id: %d, err: %v, rid: %s",
			order.SubOrderId, taskID, err, kt.Rid)
		return generateId, fmt.Errorf("failed to update generate record, order id: %s, recall order id: %d, err: %v",
			order.SubOrderId, taskID, err)
	}

	return generateId, nil
}

// createAndCheckRecallOrder create and check pool recall order
func (g *Generator) createAndCheckRecallOrder(kt *kit.Kit, order *types.ApplyOrder, recall *types.MatchPoolSpec,
	generateID string) (uint64, error) {

	// 1. launch create recall order request
	req := &pooltypes.CreateRecallOrderReq{
		DeviceType:    recall.DeviceType,
		ImageID:       recall.ImageID,
		OsType:        recall.OsType,
		Replicas:      uint(recall.Replicas),
		BkCloudRegion: recall.BkCloudRegion,
		BkCloudZone:   recall.BkCloudZone,
	}

	recallOrderResp, err := g.poolLogics.Pool().CreateRecallOrder(kt, req)

	recallOrderID, err := recallOrderResp.Int64("id")
	if err != nil {
		logs.Errorf("failed to create recall order parse int, recallOrderResp: %+v, err: %v, rid: %s",
			recallOrderResp, err, kt.Rid)
		return 0, err
	}

	recallApplyOrder := &types.ApplyOrder{SubOrderId: order.SubOrderId, ResourceType: types.ResourceTypePool}
	taskID := uint64(recallOrderID)
	if err != nil {
		logs.Errorf("failed to create recall order, order id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)

		// update generate record status to failed
		if errRecord := g.UpdateGenerateRecord(kt, recallApplyOrder, generateID,
			types.GenerateStatusFailed, err.Error(), "", nil); errRecord != nil {
			logs.Errorf("failed to update generate record, order id: %s, err: %v, rid: %s",
				order.SubOrderId, errRecord, kt.Rid)
			return taskID, fmt.Errorf("failed to update generate record, order id: %s, err: %v", order.SubOrderId,
				errRecord)
		}

		return taskID, fmt.Errorf("failed to create recall order, order id: %s, err: %v", order.SubOrderId, err)
	}

	// 2. update generate record status to query
	if err = g.UpdateGenerateRecord(kt, recallApplyOrder, generateID, types.GenerateStatusHandling, "handling",
		strconv.Itoa(int(taskID)), nil); err != nil {
		logs.Errorf("failed to update generate record, order id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return taskID, fmt.Errorf("failed to update generate record, order id: %s, err: %v", order.SubOrderId, err)
	}

	// 3. check recall order result
	if err = g.checkRecallOrder(taskID); err != nil {
		logs.Errorf("failed to check recall order, order id: %s, recall order id: %s, err: %v, rid: %s",
			order.SubOrderId, taskID, err, kt.Rid)

		// update generate record status to Done
		if errRecord := g.UpdateGenerateRecord(kt, order, generateID,
			types.GenerateStatusFailed, err.Error(), "", nil); errRecord != nil {
			logs.Errorf("failed to check recall order, order id: %s, task id: %s, err: %v, rid: %s",
				order.SubOrderId, taskID, errRecord, kt.Rid)
			return taskID, fmt.Errorf("failed to check recall order, order id: %s, recall order id: %d, err: %v",
				order.SubOrderId, taskID, errRecord)
		}

		return taskID, fmt.Errorf("failed to check recall order, order id: %s, recall order id: %d, err: %v",
			order.SubOrderId, taskID, err)
	}

	return taskID, nil
}

// checkRecallOrder check pool recall order
func (g *Generator) checkRecallOrder(id uint64) error {
	checkFunc := func(obj interface{}, err error) (bool, error) {
		if err != nil {
			return false, fmt.Errorf("failed to query recall order by id %d, err: %v", id, err)
		}

		if obj == nil {
			return false, fmt.Errorf("recall order %d not found", id)
		}

		resp, ok := obj.(*pooltypes.GetRecallOrderRst)
		if !ok {
			return false, fmt.Errorf("object with order id %d is not a recall order response: %+v", id, obj)
		}

		num := len(resp.Info)
		if num != 1 {
			return false, fmt.Errorf("query recall order return %d orders with order id: %d", num, id)
		}

		order := resp.Info[0]

		if order.Status.Phase == pooltable.OpTaskPhaseInit || order.Status.Phase == pooltable.OpTaskPhaseRunning {
			return false, fmt.Errorf("recall order %d handling", id)
		}

		if order.Status.Phase != pooltable.OpTaskPhaseSuccess {
			return true, fmt.Errorf("order %d failed, status: %s", id, order.Status.Phase)
		}

		return true, nil
	}

	doFunc := func() (interface{}, error) {
		// construct order status request
		req := &pooltypes.GetRecallOrderReq{
			ID: id,
		}

		// call pool api to query recall order status
		return g.poolLogics.Pool().GetRecallOrder(kit.New(), req)
	}

	// TODO: get retry strategy from config
	_, err := utils.Retry(doFunc, checkFunc, 86400, 300)

	return err
}

// listRecalledInstance lists recalled instances recall order id
func (g *Generator) listRecalledInstance(kt *kit.Kit, id uint64) ([]*pooltable.RecallDetail, error) {
	req := &pooltypes.GetRecalledInstReq{
		ID: id,
	}

	resp, err := g.poolLogics.Pool().GetRecalledInstance(kt, req)
	if err != nil {
		return nil, err
	}

	return resp.Info, nil
}
