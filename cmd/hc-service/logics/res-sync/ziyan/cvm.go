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
	"sync"

	"hcm/cmd/hc-service/logics/res-sync/common"
	adcore "hcm/pkg/adaptor/types/core"
	typescvm "hcm/pkg/adaptor/types/cvm"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/tools/slice"

	"golang.org/x/sync/errgroup"
)

func (cli *client) getVpcMap(kt *kit.Kit, accountID string, region string,
	cloudVpcIDs []string) (map[string]*common.VpcDB, error) {

	vpcMap := make(map[string]*common.VpcDB)

	elems := slice.Split(cloudVpcIDs, constant.CloudResourceSyncMaxLimit)
	for _, parts := range elems {
		vpcParams := &SyncBaseParams{
			AccountID: accountID,
			Region:    region,
			CloudIDs:  parts,
		}
		vpcFromDB, err := cli.listVpcFromDB(kt, vpcParams)
		if err != nil {
			return vpcMap, err
		}

		for _, vpc := range vpcFromDB {
			vpcMap[vpc.CloudID] = &common.VpcDB{
				VpcID: vpc.ID,
			}
		}
	}

	return vpcMap, nil
}

func (cli *client) getSubnetMap(kt *kit.Kit, accountID string, region string,
	cloudSubnetsIDs []string) (map[string]string, error) {

	subnetMap := make(map[string]string)

	elems := slice.Split(cloudSubnetsIDs, constant.CloudResourceSyncMaxLimit)
	for _, parts := range elems {
		subnetParams := &SyncBaseParams{
			AccountID: accountID,
			Region:    region,
			CloudIDs:  parts,
		}
		subnetFromDB, err := cli.listSubnetFromDB(kt, subnetParams)
		if err != nil {
			return subnetMap, err
		}

		for _, subnet := range subnetFromDB {
			subnetMap[subnet.CloudID] = subnet.ID
		}
	}

	return subnetMap, nil
}

func (cli *client) getCVM(kt *kit.Kit, ccHosts []cmdb.Host) (map[string][]typescvm.TCloudCvm, error) {
	if len(ccHosts) == 0 {
		return map[string][]typescvm.TCloudCvm{}, nil
	}

	tr := syncHostTraceFromCtx(kt.Ctx)
	defer tr.Track(stepListCloudCvm)()

	regionCloudIDMap := make(map[string][]string)
	for _, ccHost := range ccHosts {
		if ccHost.SvrSourceTypeID != cmdb.SvrSourceTypeIDCVM {
			continue
		}
		if ccHost.BkCloudRegion == "" {
			logs.Warnf("host id(%d) region data is nil, rid: %s", ccHost.BkHostID, kt.Rid)
			continue
		}
		if _, ok := regionCloudIDMap[ccHost.BkCloudRegion]; !ok {
			regionCloudIDMap[ccHost.BkCloudRegion] = make([]string, 0)
		}
		regionCloudIDMap[ccHost.BkCloudRegion] = append(regionCloudIDMap[ccHost.BkCloudRegion], ccHost.BkCloudInstID)
	}

	var eg, _ = errgroup.WithContext(kt.Ctx)
	var lock sync.Mutex
	pipeline := make(chan struct{}, 10)
	regionCVMap := make(map[string][]typescvm.TCloudCvm)
	doFunc := func(region string, ids []string) error {
		defer func() {
			<-pipeline
		}()

		opt := &typescvm.TCloudListOption{
			Region:   region,
			CloudIDs: ids,
			Page:     &adcore.TCloudPage{Offset: 0, Limit: adcore.TCloudQueryLimit},
		}
		cvms, err := cli.cloudCli.ListCvm(kt, opt)
		if err != nil {
			logs.Errorf("[%s] list cvm from cloud failed, err: %v, opt: %v, rid: %s", enumor.TCloudZiyan, err,
				*opt, kt.Rid)
			return err
		}
		if len(cvms) == 0 {
			logs.Warnf("can not find cvm, opt: %+v, rid: %s", *opt, kt.Rid)
			return nil
		}

		lock.Lock()
		if _, ok := regionCVMap[region]; !ok {
			regionCVMap[region] = make([]typescvm.TCloudCvm, 0)
		}
		regionCVMap[region] = append(regionCVMap[region], cvms...)
		lock.Unlock()

		return nil
	}

	for region, cloudIDs := range regionCloudIDMap {
		for _, batch := range slice.Split(cloudIDs, adcore.TCloudQueryLimit) {
			pipeline <- struct{}{}
			curRegion := region
			ids := batch
			eg.Go(func() error {
				return doFunc(curRegion, ids)
			})
		}
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	cvmCount := 0
	for _, cvms := range regionCVMap {
		cvmCount += len(cvms)
	}
	tr.SetCloudResult(cvmCount, len(regionCVMap))

	return regionCVMap, nil
}
