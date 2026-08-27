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

package ziyan

import (
	"fmt"
	"time"

	cvmrelmgr "hcm/cmd/hc-service/logics/res-sync/cvm-rel-manager"
	typecvm "hcm/pkg/adaptor/types/cvm"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/cmdb"

	"golang.org/x/sync/errgroup"
)

// HostWithRelRes ...
func (cli *client) HostWithRelRes(kt *kit.Kit, params *SyncHostParams) (result *SyncResult, err error) {
	if params == nil {
		logs.Errorf("params is nil, rid: %s", kt.Rid)
		return nil, fmt.Errorf("params is nil")
	}

	if err := params.Validate(); err != nil {
		logs.Errorf("param is invalid, err: %v, rid: %s", err, kt.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	kt, tr := newSyncHostTrace(kt, params)
	defer func() {
		tr.logSummary(kt, err)
		tr.FlushMetrics(err)
	}()

	ccHosts, err := cli.getBizHostFromCCByHostIDs(kt, params.BizID, params.HostIDs, cmdb.HostFields)
	if err != nil {
		logs.Errorf("get host from cc by host id failed, err: %v, ids: %v, rid: %s", err, params.HostIDs, kt.Rid)
		return nil, err
	}
	tr.SetCCResult(len(ccHosts))

	// 如果cvm全部不存在，仅同步主机即可，有可能主机被从云上删除
	if len(ccHosts) == 0 {
		return cli.Host(kt, params)
	}

	regionCVMMap, err := cli.getCVM(kt, ccHosts)
	if err != nil {
		logs.Errorf("get cvm failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 如果这一批主机中没有cvm，那么直接同步主机信息即可
	if len(regionCVMMap) == 0 {
		return cli.Host(kt, params)
	}

	if err = cli.syncCvmRelRes(kt, params, regionCVMMap); err != nil {
		logs.Errorf("sync cvm rel res failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if _, err = cli.Host(kt, params); err != nil {
		return nil, err
	}

	if err = cli.syncCvmRel(kt, regionCVMMap); err != nil {
		logs.Errorf("sync cvm relation failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	return new(SyncResult), nil
}

// syncCvmPreRes 同步cvm关联的资源
func (cli *client) syncCvmRelRes(kt *kit.Kit, params *SyncHostParams,
	regionCVMMap map[string][]typecvm.TCloudCvm) error {

	if len(regionCVMMap) == 0 {
		return nil
	}

	tr := syncHostTraceFromCtx(kt.Ctx)
	defer tr.Track(string(enumor.ResSyncStepRelRes))()

	var eg, _ = errgroup.WithContext(kt.Ctx)
	pipeline := make(chan struct{}, 20)
	doFunc := func(region string, relMgr *cvmrelmgr.CvmRelManger, resType enumor.CloudResourceType,
		syncFunc func(kt *kit.Kit, cloudIDs []string) error) error {

		defer func() {
			<-pipeline
		}()

		start := time.Now()
		err := relMgr.Sync(kt, resType, syncFunc)
		if err != nil {
			logs.Errorf("[%s] sync cvm associate %s failed, err: %v, rid: %s", enumor.TCloudZiyan, resType, err, kt.Rid)
			return err
		}

		logs.Infof("sync ziyan rel res done, region: %s, res: %s, cost: %s, rid: %s", region, resType,
			time.Since(start), kt.Rid)

		return nil
	}

	for region, cvms := range regionCVMMap {
		regionVal := region
		// 获取cvm和关联资源的关联关系
		mgr, err := cli.buildCvmRelManger(kt, regionVal, cvms)
		if err != nil {
			logs.Errorf("[%s] build cvm rel manager failed, err: %v, rid: %s", enumor.TCloudZiyan, err, kt.Rid)
			return err
		}

		syncFuncMap := map[enumor.CloudResourceType]func(kt *kit.Kit, cloudIDs []string) error{
			enumor.VpcCloudResType: func(kt *kit.Kit, cloudIDs []string) error {
				assResParams := &SyncBaseParams{
					AccountID: params.AccountID,
					Region:    regionVal,
					CloudIDs:  cloudIDs,
				}
				if _, err = cli.Vpc(kt, assResParams, new(SyncVpcOption)); err != nil {
					return err
				}

				return nil
			},
			enumor.SubnetCloudResType: func(kt *kit.Kit, cloudIDs []string) error {
				assResParams := &SyncBaseParams{
					AccountID: params.AccountID,
					Region:    regionVal,
					CloudIDs:  cloudIDs,
				}
				if _, err = cli.Subnet(kt, assResParams, new(SyncSubnetOption)); err != nil {
					return err
				}

				return nil
			},
			enumor.SecurityGroupCloudResType: func(kt *kit.Kit, cloudIDs []string) error {
				assResParams := &SyncBaseParams{
					AccountID: params.AccountID,
					Region:    regionVal,
					CloudIDs:  cloudIDs,
				}
				if _, err = cli.SecurityGroup(kt, assResParams, new(SyncSGOption)); err != nil {
					return err
				}

				return nil
			},
		}

		for resType, syncFunc := range syncFuncMap {
			pipeline <- struct{}{}
			curMgr := mgr
			curResType := resType
			curSyncFunc := syncFunc

			eg.Go(func() error {
				return doFunc(regionVal, curMgr, curResType, curSyncFunc)
			})
		}
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

// syncCvmRel 同步cvm与关联资源的关系
func (cli *client) syncCvmRel(kt *kit.Kit, regionCVMMap map[string][]typecvm.TCloudCvm) error {
	tr := syncHostTraceFromCtx(kt.Ctx)
	defer tr.Track(string(enumor.ResSyncStepHostRel))()

	for region, cvms := range regionCVMMap {
		// 获取cvm和关联资源的关联关系
		mgr, err := cli.buildCvmRelManger(kt, region, cvms)
		if err != nil {
			logs.Errorf("[%s] build cvm rel manager failed, err: %v, rid: %s", enumor.TCloudZiyan, err, kt.Rid)
			return err
		}

		// sync cvm_sg_rel
		syncRelOpt := &cvmrelmgr.SyncRelOption{
			Vendor: enumor.TCloudZiyan,
		}

		syncRelOpt.ResType = enumor.SecurityGroupCloudResType
		if err = mgr.SyncRel(kt, syncRelOpt); err != nil {
			logs.Errorf("[%s] sync host_securityGroup_rel failed, err: %v, rid: %s", enumor.TCloudZiyan, err, kt.Rid)
			return err
		}

		logs.Infof("sync ziyan cvm rel done, region: %s, cvms: %d, rid: %s", region, len(cvms), kt.Rid)
	}

	return nil
}

func (cli *client) buildCvmRelManger(kt *kit.Kit, region string, cvmFromCloud []typecvm.TCloudCvm) (
	*cvmrelmgr.CvmRelManger, error) {

	if len(cvmFromCloud) == 0 {
		return nil, fmt.Errorf("cvms that from cloud is required")
	}

	mgr := cvmrelmgr.NewCvmRelManager(cli.dbCli)
	for _, cvm := range cvmFromCloud {
		// SecurityGroup
		for _, SecurityGroupId := range cvm.SecurityGroupIds {
			if SecurityGroupId == nil {
				continue
			}

			mgr.CvmAppendAssResCloudID(cvm.GetCloudID(), enumor.SecurityGroupCloudResType,
				*SecurityGroupId)
		}

		// Vpc&Subnet
		if cvm.VirtualPrivateCloud != nil {
			if cvm.VirtualPrivateCloud.VpcId != nil {
				mgr.CvmAppendAssResCloudID(cvm.GetCloudID(), enumor.VpcCloudResType,
					*cvm.VirtualPrivateCloud.VpcId)
			}

			if cvm.VirtualPrivateCloud.SubnetId != nil {
				mgr.CvmAppendAssResCloudID(cvm.GetCloudID(), enumor.SubnetCloudResType,
					*cvm.VirtualPrivateCloud.SubnetId)
			}
		}
	}

	return mgr, nil
}
