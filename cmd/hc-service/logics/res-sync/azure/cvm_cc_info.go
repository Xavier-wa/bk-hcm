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

// Package azure ...
package azure

import (
	ccinfo "hcm/cmd/hc-service/logics/res-sync/cc-info"
	"hcm/cmd/hc-service/logics/res-sync/common"
	"hcm/pkg/api/core/cloud/cvm"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// SyncCvmCCInfoParams ...
type SyncCvmCCInfoParams struct {
	Cvms []cvm.BaseCvm
}

// CvmCCInfo ...
func (cli *client) CvmCCInfo(kt *kit.Kit, params *SyncCvmCCInfoParams) (err error) {
	// report total only; resource cvm_cc_info is used to distinguish from cloud image sync (cvm).
	tr := common.NewResSyncTrace(kt, enumor.Azure, enumor.CvmCCInfoResType, nil)
	defer func() { tr.FlushMetrics(err) }()

	mgr := ccinfo.NewCvmCCInfoRelManager(cli.dbCli)

	if err = mgr.SyncCvmCCInfo(kt, params.Cvms); err != nil {
		logs.Errorf("sync azure cvm cc info failed, err: %v, cvms: %+v, rid: %s", err, params.Cvms, kt.Rid)
		return err
	}

	return nil
}
