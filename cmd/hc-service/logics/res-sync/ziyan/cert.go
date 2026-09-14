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
	"strconv"
	"time"

	"hcm/cmd/hc-service/logics/res-sync/common"
	typecert "hcm/pkg/adaptor/types/cert"
	adcore "hcm/pkg/adaptor/types/core"
	"hcm/pkg/api/core"
	corecert "hcm/pkg/api/core/cloud/cert"
	protocloud "hcm/pkg/api/data-service/cloud"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/assert"
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"
	"hcm/pkg/ziyan"
)

// getBkBizIdByBs2Fn resolves bs2_name_id to bk_biz_id. Overridden in unit tests.
var getBkBizIdByBs2Fn = ziyan.GetBkBizIdByBs2

// SyncCertOption ...
type SyncCertOption struct {
	// BkBizID is used only when creating new certs before cloud tags are available.
	// Sync callers must pass constant.UnassignedBiz; do not use a positive biz id during sync update.
	BkBizID int64 `json:"bk_biz_id" validate:"omitempty"`
	// should match params' cloud id
	PreCachedCertList []typecert.TCloudCert
}

// Validate ...
func (opt SyncCertOption) Validate() error {
	return validator.Validate.Struct(opt)
}

// Cert ...
func (cli *client) Cert(kt *kit.Kit, params *SyncBaseParams, opt *SyncCertOption) (*SyncResult, error) {
	if err := validator.ValidateTool(params, opt); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	certFromCloud := opt.PreCachedCertList
	if certFromCloud == nil {
		var err error
		certFromCloud, err = cli.listCertFromCloud(kt, params)
		if err != nil {
			return nil, err
		}
	}

	certFromDB, err := cli.listCertFromDB(kt, params)
	if err != nil {
		return nil, err
	}

	if len(certFromCloud) == 0 && len(certFromDB) == 0 {
		return new(SyncResult), nil
	}

	certFromCloud, err = cli.fillCertBkBizID(kt, certFromCloud)
	if err != nil {
		return nil, err
	}

	addSlice, updateMap, delCloudIDs := common.Diff[typecert.TCloudCert, *corecert.Cert[corecert.TCloudCertExtension]](
		certFromCloud, certFromDB, isCertChange)

	if err = cli.deleteCert(kt, params.AccountID, params.Region, delCloudIDs); err != nil {
		return nil, err
	}

	if err = cli.createCert(kt, params.AccountID, opt, addSlice); err != nil {
		return nil, err
	}

	if err = cli.updateCert(kt, params.AccountID, updateMap); err != nil {
		return nil, err
	}

	return new(SyncResult), nil
}

func (cli *client) deleteCert(kt *kit.Kit, accountID, region string, delCloudIDs []string) error {
	if len(delCloudIDs) <= 0 {
		return nil
	}

	deleteReq := &protocloud.CertBatchDeleteReq{
		Filter: tools.ContainersExpression("cloud_id", delCloudIDs),
	}
	if err := cli.dbCli.Global.BatchDeleteCert(kt.Ctx, kt.Header(), deleteReq); err != nil {
		logs.Errorf("[%s] request dataservice to batch delete cert failed, err: %v, rid: %s",
			enumor.TCloudZiyan, err, kt.Rid)
		return err
	}

	return nil
}

func (cli *client) updateCert(kt *kit.Kit, accountID string, updateMap map[string]typecert.TCloudCert) error {
	if len(updateMap) <= 0 {
		return nil
	}

	certs := make([]*protocloud.CertExtUpdateReq[corecert.TCloudCertExtension], 0)

	for id, one := range updateMap {
		cert, err := convCertCloudToDBUpdate(id, accountID, one)
		if err != nil {
			return err
		}
		certs = append(certs, cert)
	}

	var updateReq protocloud.CertExtBatchUpdateReq[corecert.TCloudCertExtension]
	for _, item := range certs {
		updateReq = append(updateReq, item)
	}
	if _, err := cli.dbCli.TCloudZiyan.BatchUpdateCert(kt.Ctx, kt.Header(), &updateReq); err != nil {
		logs.Errorf("[%s] request dataservice BatchUpdateCert failed, err: %v, rid: %s", enumor.TCloudZiyan,
			err, kt.Rid)
		return err
	}

	logs.Infof("[%s] sync cert to update cert success, accountID: %s, count: %d, rid: %s", enumor.TCloudZiyan,
		accountID, len(updateMap), kt.Rid)

	return nil
}

func buildCertBatchCreateReq(accountID string, addSlice []typecert.TCloudCert) (
	*protocloud.CertBatchCreateReq[corecert.TCloudCertExtension], error) {

	createReq := new(protocloud.CertBatchCreateReq[corecert.TCloudCertExtension])
	for _, one := range addSlice {
		domainJson, err := types.NewJsonField(one.SubjectAltName)
		if err != nil {
			return nil, fmt.Errorf("json marshal extension failed, err: %w", err)
		}

		createReq.Certs = append(createReq.Certs, protocloud.CertBatchCreate[corecert.TCloudCertExtension]{
			CloudID:          one.GetCloudID(),
			Name:             converter.PtrToVal(one.Alias),
			Vendor:           string(enumor.TCloudZiyan),
			AccountID:        accountID,
			BkBizID:          one.BkBizID,
			Domain:           domainJson,
			CertType:         enumor.CertType(converter.PtrToVal(one.CertificateType)),
			EncryptAlgorithm: converter.PtrToVal(one.EncryptAlgorithm),
			CertStatus:       strconv.FormatUint(converter.PtrToVal(one.Status), 10),
			CloudCreatedTime: convTCloudTimeStd(converter.PtrToVal(one.InsertTime)),
			CloudExpiredTime: convTCloudTimeStd(converter.PtrToVal(one.CertEndTime)),
			Tags:             one.GetTagMap(),
		})
	}
	return createReq, nil
}

func applyCreateCertBkBizIDFallback(addSlice []typecert.TCloudCert, opt *SyncCertOption) []typecert.TCloudCert {
	result := append([]typecert.TCloudCert(nil), addSlice...)
	if opt == nil || opt.BkBizID <= 0 {
		return result
	}
	for i := range result {
		if result[i].BkBizID == constant.UnassignedBiz {
			result[i].BkBizID = opt.BkBizID
		}
	}
	return result
}

func (cli *client) createCert(kt *kit.Kit, accountID string, opt *SyncCertOption,
	addSlice []typecert.TCloudCert) error {

	if len(addSlice) <= 0 {
		return nil
	}

	addSlice = applyCreateCertBkBizIDFallback(addSlice, opt)

	createReq, err := buildCertBatchCreateReq(accountID, addSlice)
	if err != nil {
		return err
	}

	if _, err := cli.dbCli.TCloudZiyan.BatchCreateCert(kt.Ctx, kt.Header(), createReq); err != nil {
		logs.Errorf("[%s] request dataservice to create tcloud-ziyan cert failed, createReq: %+v, err: %v, rid: %s",
			enumor.TCloudZiyan, createReq, err, kt.Rid)
		return err
	}

	return nil
}

// 不支持按id批量获取，直接获取全部数据可以降低调用腾讯云api次数
func (cli *client) listAllCertFromCloud(kt *kit.Kit) ([]typecert.TCloudCert, error) {

	list := make([]typecert.TCloudCert, 0, 100)
	opt := &typecert.TCloudListOption{
		Page: &adcore.TCloudPage{Offset: 0, Limit: adcore.TCloudQueryLimit},
	}
	for {
		result, err := cli.cloudCli.ListCert(kt, opt)
		if err != nil {
			logs.Errorf("[%s] list all cert from cloud failed, account: %s, opt: %v, err: %v, rid: %s",
				enumor.TCloudZiyan, cli.accountID, opt, err, kt.Rid)
			return nil, err
		}

		list = append(list, result...)

		if uint64(len(result)) < opt.Page.Limit {
			break
		}
		opt.Page.Offset += opt.Page.Limit

	}

	return list, nil
}

func (cli *client) listCertFromCloud(kt *kit.Kit, params *SyncBaseParams) ([]typecert.TCloudCert, error) {
	if err := params.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	list := make([]typecert.TCloudCert, 0)
	for _, tmpCloudID := range params.CloudIDs {
		opt := &typecert.TCloudListOption{
			SearchKey: tmpCloudID,
			Page:      &adcore.TCloudPage{Offset: 0, Limit: 1},
		}
		result, err := cli.cloudCli.ListCert(kt, opt)
		if err != nil {
			logs.Errorf("[%s] list cert from cloud failed, account: %s, opt: %v, err: %v, rid: %s",
				enumor.TCloudZiyan, params.AccountID, opt, err, kt.Rid)
			return nil, err
		}

		list = append(list, result...)
	}

	return list, nil
}

func (cli *client) listCertFromDB(kt *kit.Kit, params *SyncBaseParams) (
	[]*corecert.Cert[corecert.TCloudCertExtension], error) {

	if err := params.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	req := &core.ListReq{
		Filter: &filter.Expression{
			Op: filter.And,
			Rules: []filter.RuleFactory{
				&filter.AtomRule{
					Field: "account_id",
					Op:    filter.Equal.Factory(),
					Value: params.AccountID,
				},
				&filter.AtomRule{
					Field: "cloud_id",
					Op:    filter.In.Factory(),
					Value: params.CloudIDs,
				},
			},
		},
		Page: core.NewDefaultBasePage(),
	}
	result, err := cli.dbCli.TCloudZiyan.ListCert(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("[%s] list cert from db failed, account: %s, req: %v, err: %v, rid: %s",
			enumor.TCloudZiyan, params.AccountID, req, err, kt.Rid)
		return nil, err
	}

	return result.Details, nil
}

// fillCertBkBizID resolves bk_biz_id from cloud Bs2 tags. Cloud is the source of truth for sync update.
func (cli *client) fillCertBkBizID(kt *kit.Kit, certFromCloud []typecert.TCloudCert) (
	[]typecert.TCloudCert, error) {

	result := append([]typecert.TCloudCert(nil), certFromCloud...)
	bs2NameIds := make([]int64, 0, len(result))
	for i := range result {
		meta := ziyan.ParseResourceMetaIgnoreErr(result[i].GetTagMap())
		var bs2 int64 = ziyan.NotFoundID
		if meta != nil && meta.Bs2NameID > 0 {
			bs2 = meta.Bs2NameID
		}
		bs2NameIds = append(bs2NameIds, bs2)
	}

	bizIds, err := getBkBizIdByBs2Fn(kt, cli.dbCli, cli.cmdbCli, bs2NameIds)
	if err != nil {
		logs.Errorf("fail to get bkBizId by bs2NameIds for cert, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	if len(bizIds) != len(result) {
		return nil, fmt.Errorf("cert bizIds length(%d) not equal to cert length(%d)", len(bizIds), len(result))
	}

	for i := range result {
		result[i].BkBizID = bizIds[i]
	}
	return result, nil
}

func convCertCloudToDBUpdate(id, accountID string, one typecert.TCloudCert) (
	*protocloud.CertExtUpdateReq[corecert.TCloudCertExtension], error) {

	domainJson, err := types.NewJsonField(one.SubjectAltName)
	if err != nil {
		return nil, fmt.Errorf("json marshal extension failed, err: %w", err)
	}
	return &protocloud.CertExtUpdateReq[corecert.TCloudCertExtension]{
		ID:               id,
		Name:             converter.PtrToVal(one.Alias),
		Vendor:           string(enumor.TCloudZiyan),
		AccountID:        accountID,
		BkBizID:          one.BkBizID,
		Domain:           domainJson,
		CertType:         enumor.CertType(converter.PtrToVal(one.CertificateType)),
		EncryptAlgorithm: converter.PtrToVal(one.EncryptAlgorithm),
		CertStatus:       strconv.FormatUint(converter.PtrToVal(one.Status), 10),
		CloudCreatedTime: convTCloudTimeStd(converter.PtrToVal(one.InsertTime)),
		CloudExpiredTime: convTCloudTimeStd(converter.PtrToVal(one.CertEndTime)),
		Tags:             one.GetTagMap(),
	}, nil
}

func isCertChange(cloud typecert.TCloudCert, db *corecert.Cert[corecert.TCloudCertExtension]) bool {
	if cloud.BkBizID != db.BkBizID {
		return true
	}

	if converter.PtrToVal(cloud.Alias) != db.Name {
		return true
	}

	if !assert.IsPtrStringSliceEqual(cloud.SubjectAltName, db.Domain) {
		return true
	}

	if enumor.CertType(converter.PtrToVal(cloud.CertificateType)) != db.CertType {
		return true
	}

	statusCloud := strconv.FormatUint(converter.PtrToVal(cloud.Status), 10)
	if statusCloud != db.CertStatus {
		return true
	}

	if db.CloudExpiredTime != convTCloudTimeStd(converter.PtrToVal(cloud.CertEndTime)) {
		return true
	}

	if converter.PtrToVal(cloud.EncryptAlgorithm) != db.EncryptAlgorithm {
		return true
	}

	if len(cloud.Tags) != len(db.Tags) {
		return true
	}
	for _, tag := range cloud.Tags {
		value, ok := db.Tags.Get(converter.PtrToVal(tag.TagKey))
		if !ok {
			return true
		}
		if value != converter.PtrToVal(tag.TagValue) {
			return true
		}
	}

	return false
}

// RemoveCertDeleteFromCloud ...
func (cli *client) RemoveCertDeleteFromCloud(kt *kit.Kit, accountID, region string) error {
	req := &core.ListReq{
		Fields: []string{"id", "cloud_id"},
		Filter: &filter.Expression{
			Op: filter.And,
			Rules: []filter.RuleFactory{
				&filter.AtomRule{Field: "account_id", Op: filter.Equal.Factory(), Value: accountID},
			},
		},
		Page: &core.BasePage{
			Start: 0,
			Limit: constant.BatchOperationMaxLimit,
		},
	}
	// 全量获取一次云端证书数据
	allResultFromCloud, err := cli.listAllCertFromCloud(kt)
	certCloudIDMap := make(map[string]struct{}, len(allResultFromCloud))
	for _, cert := range allResultFromCloud {
		certCloudIDMap[cert.GetCloudID()] = struct{}{}
	}
	if err != nil {
		return err
	}
	delCloudIDs := make([]string, 0)
	for {
		resultFromDB, err := cli.dbCli.Global.ListCert(kt, req)
		if err != nil {
			logs.Errorf("[%s] request dataservice to list cert failed, req: %v, err: %v, rid: %s",
				enumor.TCloudZiyan, req, err, kt.Rid)
			return err
		}
		for _, detail := range resultFromDB.Details {
			if _, ok := certCloudIDMap[detail.CloudID]; !ok {
				delCloudIDs = append(delCloudIDs, detail.CloudID)
			}
		}

		if len(resultFromDB.Details) < constant.BatchOperationMaxLimit {
			break
		}

		req.Page.Start += constant.BatchOperationMaxLimit
	}
	if len(delCloudIDs) == 0 {
		return nil
	}

	for _, delCloudBatch := range slice.Split(delCloudIDs, constant.BatchOperationMaxLimit) {
		if err = cli.deleteCert(kt, accountID, region, delCloudBatch); err != nil {
			return err
		}

	}

	return nil
}

func convTCloudTimeStd(t string) string {
	parse, err := time.Parse(constant.DateTimeLayout, t)
	if err != nil {
		logs.Errorf("[%s] parse time failed, time: %s, err: %v", enumor.TCloudZiyan, t, err)
		return ""
	}
	return parse.Format(constant.TimeStdFormat)
}
