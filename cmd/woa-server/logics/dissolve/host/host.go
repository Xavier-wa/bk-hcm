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

package host

import (
	dissolveconfig "hcm/cmd/woa-server/logics/dissolve/config"
	"hcm/pkg/api/core"
	dsproto "hcm/pkg/api/data-service/dissolve"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	daotypes "hcm/pkg/dal/dao/types"
	hostdaotypes "hcm/pkg/dal/dao/types/dissolve/host"
	define "hcm/pkg/dal/table/dissolve/host"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/thirdparty/caiche"
	"hcm/pkg/thirdparty/es"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"
)

// RecycledHost provides interface for operations of recycle host.
type RecycledHost interface {
	Create(kt *kit.Kit, hosts []define.RecycleHostTable) ([]string, error)
	Update(kt *kit.Kit, host *define.RecycleHostTable) error
	List(kt *kit.Kit, opt *daotypes.ListOption) (*hostdaotypes.ListRecycleHostDetails, error)
	Delete(kt *kit.Kit, ids []string) error
	IsDissolveHost(kt *kit.Kit, assetIDs []string) (map[string]bool, error)
	ListProjects(kt *kit.Kit) ([]caiche.Project, error)
	Sync(kt *kit.Kit) error
}

type logics struct {
	cliSet       *client.ClientSet
	cmdbCli      cmdb.Client
	esCli        *es.EsCli
	thirdCli     *thirdparty.Client
	config       dissolveconfig.Config
	ignoreBiz    []int64
	svrTypeNames []string
	originDate   string
}

// New create recycle host logics.
func New(cliSet *client.ClientSet, cmdbCli cmdb.Client, esCli *es.EsCli, thirdCli *thirdparty.Client,
	cfg dissolveconfig.Config, ignoreBiz []int64, svrTypeNames []string, originDate string) RecycledHost {

	return &logics{
		cliSet:       cliSet,
		cmdbCli:      cmdbCli,
		esCli:        esCli,
		thirdCli:     thirdCli,
		config:       cfg,
		ignoreBiz:    ignoreBiz,
		svrTypeNames: svrTypeNames,
		originDate:   originDate,
	}
}

// Create recycle host via data-service client.
func (l *logics) Create(kt *kit.Kit, hosts []define.RecycleHostTable) ([]string, error) {
	ids := make([]string, 0, len(hosts))
	for _, batch := range slice.Split(hosts, constant.BatchOperationMaxLimit) {
		req := &dsproto.BatchCreateRecycleHostReq{Hosts: toCreateReqs(batch)}
		resp, err := l.cliSet.DataService().TCloudZiyan.Dissolve.BatchCreateRecycleHost(kt, req)
		if err != nil {
			logs.Errorf("create recycle host failed, err: %v, count: %d, rid: %s", err, len(batch), kt.Rid)
			return nil, err
		}
		ids = append(ids, resp.IDs...)
	}

	return ids, nil
}

// Update recycle host via data-service client.
func (l *logics) Update(kt *kit.Kit, host *define.RecycleHostTable) error {
	req := &dsproto.BatchUpdateRecycleHostReq{
		Filter: tools.EqualExpression("id", host.ID),
		Data:   toUpdateData(host),
	}
	if err := l.cliSet.DataService().TCloudZiyan.Dissolve.BatchUpdateRecycleHost(kt, req); err != nil {
		logs.Errorf("update recycle host failed, err: %v, id: %s, rid: %s", err, host.ID, kt.Rid)
		return err
	}

	return nil
}

// Delete recycle host via data-service client.
func (l *logics) Delete(kt *kit.Kit, ids []string) error {
	req := &dsproto.BatchDeleteRecycleHostReq{Filter: tools.ContainersExpression("id", ids)}
	if err := l.cliSet.DataService().TCloudZiyan.Dissolve.BatchDeleteRecycleHost(kt, req); err != nil {
		logs.Errorf("delete recycle host failed, err: %v, ids: %+v, rid: %s", err, ids, kt.Rid)
		return err
	}

	return nil
}

// List recycle host via data-service client.
func (l *logics) List(kt *kit.Kit, opt *daotypes.ListOption) (*hostdaotypes.ListRecycleHostDetails, error) {
	req := &dsproto.RecycleHostListReq{Filter: opt.Filter, Page: opt.Page, Fields: opt.Fields}
	resp, err := l.cliSet.DataService().TCloudZiyan.Dissolve.ListRecycleHost(kt, req)
	if err != nil {
		logs.Errorf("list recycle host failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return &hostdaotypes.ListRecycleHostDetails{Count: resp.Count, Details: resp.Details}, nil
}

// IsDissolveHost check if host is dissolve host.
func (l *logics) IsDissolveHost(kt *kit.Kit, assetIDs []string) (map[string]bool, error) {
	result := make(map[string]bool, len(assetIDs))
	for _, id := range assetIDs {
		result[id] = false
	}

	for _, ids := range slice.Split(assetIDs, int(core.DefaultMaxPageLimit)) {
		req := &daotypes.ListOption{
			Filter: tools.ExpressionAnd(
				tools.RuleIn("asset_id", ids),
				tools.RuleNotEqual("abolish_phase", enumor.Complete),
			),
			Fields: []string{"asset_id"},
			Page:   core.NewDefaultBasePage(),
		}

		list, err := l.List(kt, req)
		if err != nil {
			logs.Errorf("list recycle host failed, err: %v, ids: %+v, rid: %s", err, ids, kt.Rid)
			return nil, err
		}

		for _, one := range list.Details {
			result[cvt.PtrToVal(one.AssetID)] = true
		}
	}

	return result, nil
}

// ListProjects 透传裁撤系统项目列表，供前端配置裁撤项目选择。
func (l *logics) ListProjects(kt *kit.Kit) ([]caiche.Project, error) {
	projects, err := l.thirdCli.CaiChe.ListProjects(kt)
	if err != nil {
		logs.Errorf("list caiche projects failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return projects, nil
}

func toCreateReqs(hosts []define.RecycleHostTable) []dsproto.RecycleHostCreateReq {
	reqs := make([]dsproto.RecycleHostCreateReq, 0, len(hosts))
	for _, h := range hosts {
		reqs = append(reqs, dsproto.RecycleHostCreateReq{
			AssetID:           cvt.PtrToVal(h.AssetID),
			InnerIP:           cvt.PtrToVal(h.InnerIP),
			DeviceType:        cvt.PtrToVal(h.DeviceType),
			Module:            cvt.PtrToVal(h.Module),
			AbolishPhase:      cvt.PtrToVal(h.AbolishPhase),
			ProjectName:       cvt.PtrToVal(h.ProjectName),
			ProjectID:         cvt.PtrToVal(h.ProjectID),
			Region:            cvt.PtrToVal(h.Region),
			BkBizID:           cvt.PtrToVal(h.BkBizID),
			GroupID:           cvt.PtrToVal(h.GroupID),
			Operators:         h.Operators,
			CPUCore:           cvt.PtrToVal(h.CPUCore),
			IsIgnore:          cvt.PtrToVal(h.IsIgnore),
			ExpectAbolishTime: cvt.PtrToVal(h.ExpectAbolishTime),
		})
	}

	return reqs
}

func toUpdateData(h *define.RecycleHostTable) *dsproto.RecycleHostUpdateData {
	return &dsproto.RecycleHostUpdateData{
		AbolishPhase:      h.AbolishPhase,
		ProjectName:       h.ProjectName,
		Module:            h.Module,
		InnerIP:           h.InnerIP,
		DeviceType:        h.DeviceType,
		Region:            h.Region,
		BkBizID:           h.BkBizID,
		GroupID:           h.GroupID,
		Operators:         h.Operators,
		CPUCore:           h.CPUCore,
		IsIgnore:          h.IsIgnore,
		ExpectAbolishTime: h.ExpectAbolishTime,
	}
}
