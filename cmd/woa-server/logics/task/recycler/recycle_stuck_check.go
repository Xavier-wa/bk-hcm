/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2025 THL A29 Limited,
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

package recycler

import (
	"runtime/debug"
	"time"

	"hcm/cmd/woa-server/dal/task/dao"
	"hcm/cmd/woa-server/dal/task/table"
	"hcm/pkg"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/metadata"
)

// StartStuckCheckLoop 开始检查回收任务长时间无动作
func (r *recycler) StartStuckCheckLoop(kt *kit.Kit) {
	cfg := cc.WoaServer().StuckCheck.Recycle
	if !cfg.Enable {
		logs.Infof("recycle order stuck check is disabled, rid: %s", kt.Rid)
		return
	}

	logs.Infof("start recycle order stuck check loop, interval: %v, range: [-%v,-%v] start up delay: %v, rid: %s",
		cfg.Interval, cfg.MaxTime, cfg.MinTime, cfg.StartUpDelay, kt.Rid)

	time.Sleep(cfg.StartUpDelay)

	err := r.checkStuckRecycleOrder(kt, cfg.MinTime, cfg.MaxTime, cfg.CvmDelayTime, cfg.PmDelayTime)
	if err != nil {
		logs.Errorf("check recycle order stuck failed, err: %v, rid: %s", err, kt.Rid)
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-kt.Ctx.Done():
			return
		case <-ticker.C:
			subkit := kt.NewSubKit()
			err := r.checkStuckRecycleOrder(subkit, cfg.MinTime, cfg.MaxTime, cfg.CvmDelayTime, cfg.PmDelayTime)
			if err != nil {
				logs.Errorf("[%s] check recycle order stuck failed, err: %v, rid: %s",
					constant.CvmRecycleStuck, err, subkit.Rid)
				// no return
			}
		}
	}
}

// checkStuckRecycleOrder 检查是否有回收任务长时间状态未更新且非终态
func (r *recycler) checkStuckRecycleOrder(kt *kit.Kit, minStayDuration, maxStayDuration, cvmDelayTime,
	pmDelayTime time.Duration) error {

	defer func() {
		if err := recover(); err != nil {
			logs.Errorf("[%s] check recycle order stuck panic: %v, stack: %s",
				constant.CvmRecycleStuck, err, debug.Stack())
		}
		logs.Infof("check recycle order stuck end, rid: %s", kt.Rid)
	}()
	logs.Infof("check recycle order stuck start, minStayDuration: %v, maxStayDuration: %v, rid: %s",
		minStayDuration, maxStayDuration, kt.Rid)
	endTime := time.Now().Add(-minStayDuration)
	startTime := time.Now().Add(-maxStayDuration)
	flt := map[string]any{
		"status": map[string]any{
			// 排除 默认、未提交、预检失败、审批中，审批拒绝、结束、终止的单
			pkg.BKDBNIN: []table.RecycleStatus{
				table.RecycleStatusDefault,
				table.RecycleStatusUncommit,
				table.RecycleStatusDetectFailed,
				table.RecycleStatusAudit,
				table.RecycleStatusRejected,
				table.RecycleStatusDone,
				table.RecycleStatusTerminate,
			},
		},
		"update_at": map[string]any{
			pkg.BKDBLTE: endTime,
			pkg.BKDBGTE: startTime,
		},
	}
	page := metadata.BasePage{
		Sort:        "create_at",
		Limit:       100,
		Start:       0,
		EnableCount: false,
	}
	for {
		orders, err := dao.Set().RecycleOrder().FindManyRecycleOrder(kt.Ctx, page, flt)
		if err != nil {
			logs.Errorf("failed to get recycle order, err: %v, rid: %s", err, kt.Rid)
			return err
		}
		r.handleRecycleStuckOrders(kt, orders, minStayDuration, cvmDelayTime, pmDelayTime)
		if len(orders) < page.Limit {
			break
		}
		page.Start = page.Start + page.Limit
	}

	return nil
}

func (r *recycler) handleRecycleStuckOrders(kt *kit.Kit, orders []*table.RecycleOrder, minStayDuration, cvmDelayTime,
	pmDelayTime time.Duration) {

	now := time.Now()
	for _, order := range orders {
		stayTime := now.Sub(order.UpdateAt)
		offsetDuration := time.Duration(0)
		if order.Status == table.RecycleStatusReturning {
			if order.ReturnPlan == table.RetPlanDelay {
				// 延迟退回： CVM会先隔离15天，物理机会先隔离1天
				switch order.ResourceType {
				case table.ResourceTypeCvm:
					offsetDuration += cvmDelayTime
				case table.ResourceTypePm:
					offsetDuration += pmDelayTime
				default:
					// 其他情况正常按配置时间算
				}
			} else {
				// 物理机立即销毁隔离2小时
				switch order.ResourceType {
				case table.ResourceTypePm:
					offsetDuration += time.Hour * 2
				default:
					// 其他情况正常按配置时间算
				}
			}
		}
		if stayTime < (minStayDuration + offsetDuration) {
			logs.Infof("skip recycle order stuck warning of %s, status: %s, return info: %s/%s, stay: %s, rid: %s",
				order.SuborderID, order.Status, order.ResourceType, order.ReturnPlan, stayTime, kt.Rid)
			continue
		}

		logs.Warnf("[%s] recycle order %s stuck at %s over %s, return info: %s/%s, rid: %s",
			constant.CvmRecycleStuck,
			order.SuborderID, order.Status, stayTime, order.ResourceType, order.ReturnPlan, kt.Rid)

	}
}
