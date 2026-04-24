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

package framework

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tasktable "hcm/cmd/woa-server/dal/task/table"
	"hcm/cmd/woa-server/service/data-migration/config"
	tasktypes "hcm/cmd/woa-server/types/task"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/table"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	tabletypes "hcm/pkg/dal/table/types"
	cvt "hcm/pkg/tools/converter"
)

func (e *MigrationEngine) buildMongoDocFromMySQL(cfg *config.TableMigrationConfig, source interface{}) (interface{}, error) {
	switch cfg.TargetTable {
	case table.ZiyanCvmApplyOrderTable:
		return buildMongoApplyTicket(source)
	case table.ZiyanCvmApplySuborderTable:
		return buildMongoApplyOrder(source)
	case table.ZiyanCvmApplyStepTable:
		return buildMongoApplyStep(source)
	case table.ZiyanCvmGenerateRecordTable:
		return buildMongoGenerateRecord(source)
	case table.ZiyanCvmApplyInitTaskTable:
		return buildMongoInitRecord(source)
	case table.ZiyanCvmDeliverRecordTable:
		return buildMongoDeliverRecord(source)
	case table.ZiyanCvmDeviceInfoTable:
		return buildMongoDeviceInfo(source)
	case table.ZiyanCvmModifyRecordTable:
		return buildMongoModifyRecord(source)
	default:
		return nil, fmt.Errorf("reverse migration is not supported for table %s", cfg.TargetTable)
	}
}

func buildMongoApplyTicket(source interface{}) (interface{}, error) {
	row, ok := source.(*cvmapplytable.ZiyanCvmApplyOrder)
	if !ok {
		return nil, fmt.Errorf("invalid source type %T for %s", source, table.ZiyanCvmApplyOrderTable)
	}

	follower := make([]string, 0)
	if err := unmarshalJSONField(row.Follower, &follower); err != nil {
		return nil, fmt.Errorf("unmarshal follower failed: %w", err)
	}

	suborders := make([]*tasktypes.Suborder, 0)
	if err := unmarshalJSONField(row.Suborders, &suborders); err != nil {
		return nil, fmt.Errorf("unmarshal suborders failed: %w", err)
	}

	oldSuborders := make([]*tasktypes.Suborder, 0)
	if err := unmarshalJSONField(row.OldSuborders, &oldSuborders); err != nil {
		return nil, fmt.Errorf("unmarshal old_suborders failed: %w", err)
	}

	return &tasktypes.ApplyTicket{
		OrderId:      row.OrderID,
		ItsmTicketId: row.ItsmTicketID,
		Stage:        row.Stage,
		BkBizId:      row.BkBizID,
		User:         row.BkUsername,
		Follower:     follower,
		EnableNotice: row.EnableNotice,
		RequireType:  row.RequireType,
		ExpectTime:   row.ExpectTime,
		Remark:       row.Remark,
		Suborders:    suborders,
		OldSuborders: oldSuborders,
		CreateAt:     parseTableTime(row.CreatedAt),
		UpdateAt:     parseTableTime(row.UpdatedAt),
		ProductType:  row.ProductType,
	}, nil
}

func buildMongoApplyOrder(source interface{}) (interface{}, error) {
	row, ok := source.(*cvmapplytable.ZiyanCvmApplySuborder)
	if !ok {
		return nil, fmt.Errorf("invalid source type %T for %s", source, table.ZiyanCvmApplySuborderTable)
	}

	follower, spec, upgradeCVMList, planExpendGroup, err := parseApplyOrderJSONFields(row)
	if err != nil {
		return nil, err
	}

	return &tasktypes.ApplyOrder{
		OrderId:           row.OrderID,
		SubOrderId:        row.SuborderID,
		BkBizId:           row.BkBizID,
		User:              row.BkUsername,
		Follower:          follower,
		Auditor:           row.Auditor,
		RequireType:       row.RequireType,
		ExpectTime:        row.ExpectTime,
		ResourceType:      row.ResourceType,
		Source:            row.Source,
		ProductType:       row.ProductType,
		Spec:              spec,
		UpgradeCVMList:    upgradeCVMList,
		AntiAffinityLevel: row.AntiAffinityLevel,
		EnableDiskCheck:   cvt.PtrToVal(row.EnableDiskCheck),
		Description:       row.Description,
		Remark:            row.Remark,
		Stage:             row.Stage,
		Status:            row.Status,
		OriginNum:         cvt.PtrToVal(row.OriginNum),
		TotalNum:          cvt.PtrToVal(row.TotalNum),
		SuccessNum:        cvt.PtrToVal(row.SuccessNum),
		PendingNum:        cvt.PtrToVal(row.PendingNum),
		AppliedCore:       cvt.PtrToVal(row.AppliedCore),
		DeliveredCore:     cvt.PtrToVal(row.DeliveredCore),
		PlanExpendGroup:   planExpendGroup,
		ObsProject:        row.ObsProject,
		RetryTime:         cvt.PtrToVal(row.RetryTime),
		ModifyTime:        cvt.PtrToVal(row.ModifyTime),
		CreateAt:          parseTableTime(row.CreatedAt),
		UpdateAt:          parseTableTime(row.UpdatedAt),
	}, nil
}

func parseApplyOrderJSONFields(row *cvmapplytable.ZiyanCvmApplySuborder) ([]string, *tasktypes.ResourceSpec,
	[]*tasktypes.UpgradeCVMSpec, []tasktypes.PlanExpendGroup, error) {

	follower := make([]string, 0)
	if err := unmarshalJSONField(row.Follower, &follower); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("unmarshal follower failed: %w", err)
	}

	systemDisk := enumor.DiskSpec{}
	if err := unmarshalJSONField(row.SystemDisk, &systemDisk); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("unmarshal system_disk failed: %w", err)
	}

	dataDisk := make([]enumor.DiskSpec, 0)
	if err := unmarshalJSONField(row.DataDisk, &dataDisk); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("unmarshal data_disk failed: %w", err)
	}

	zones := make([]string, 0)
	if err := unmarshalJSONField(row.Zones, &zones); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("unmarshal zones failed: %w", err)
	}

	failedZoneIDs := make([]string, 0)
	if err := unmarshalJSONField(row.FailedZoneIds, &failedZoneIDs); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("unmarshal failed_zone_ids failed: %w", err)
	}

	upgradeCVMList := make([]*tasktypes.UpgradeCVMSpec, 0)
	if err := unmarshalJSONField(row.UpgradeCvmList, &upgradeCVMList); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("unmarshal upgrade_cvm_list failed: %w", err)
	}

	planExpendGroup := make([]tasktypes.PlanExpendGroup, 0)
	if err := unmarshalJSONField(row.PlanExpendGroup, &planExpendGroup); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("unmarshal plan_expend_group failed: %w", err)
	}

	spec := buildApplyOrderResourceSpec(row, failedZoneIDs, systemDisk, dataDisk, zones)
	return follower, spec, upgradeCVMList, planExpendGroup, nil
}

func buildApplyOrderResourceSpec(row *cvmapplytable.ZiyanCvmApplySuborder, failedZoneIDs []string,
	systemDisk enumor.DiskSpec, dataDisk []enumor.DiskSpec, zones []string) *tasktypes.ResourceSpec {

	return &tasktypes.ResourceSpec{
		Region:            row.Region,
		Zone:              row.Zone,
		DeviceGroup:       row.DeviceGroup,
		DeviceSize:        row.DeviceSize,
		DeviceType:        row.DeviceType,
		ImageId:           row.ImageID,
		Image:             row.Image,
		DiskSize:          row.DiskSize,
		DiskType:          row.DiskType,
		NetworkType:       row.NetworkType,
		Vpc:               row.Vpc,
		Subnet:            row.Subnet,
		OsType:            row.OsType,
		RaidType:          row.RaidType,
		Isp:               row.Isp,
		ChargeType:        row.ChargeType,
		ChargeMonths:      row.ChargeMonths,
		InheritInstanceId: row.InheritInstanceID,
		BkAssetID:         row.BkAssetID,
		FailedZoneIDs:     failedZoneIDs,
		SystemDisk:        systemDisk,
		DataDisk:          dataDisk,
		Zones:             zones,
		ResAssign:         row.ResAssign,
		CPUThreadSwitch:   row.CPUThreadSwitch,
	}
}

func buildMongoApplyStep(source interface{}) (interface{}, error) {
	row, ok := source.(*cvmapplytable.ZiyanCvmApplyStep)
	if !ok {
		return nil, fmt.Errorf("invalid source type %T for %s", source, table.ZiyanCvmApplyStepTable)
	}

	return &tasktypes.ApplyStep{
		SubOrderId: row.SuborderID,
		StepId:     row.StepID,
		StepName:   row.StepName,
		Status:     cvt.PtrToVal(row.Status),
		Message:    row.Message,
		TotalNum:   cvt.PtrToVal(row.TotalNum),
		SuccessNum: cvt.PtrToVal(row.SuccessNum),
		FailedNum:  cvt.PtrToVal(row.FailedNum),
		RunningNum: cvt.PtrToVal(row.RunningNum),
		CreateAt:   parseTableTime(row.CreatedAt),
		UpdateAt:   parseTableTime(row.UpdatedAt),
		StartAt:    parseStringTime(row.StartAt),
		EndAt:      parseStringTime(row.EndAt),
	}, nil
}

func buildMongoGenerateRecord(source interface{}) (interface{}, error) {
	row, ok := source.(*cvmapplytable.ZiyanCvmGenerateRecord)
	if !ok {
		return nil, fmt.Errorf("invalid source type %T for %s", source, table.ZiyanCvmGenerateRecordTable)
	}

	successList := make([]string, 0)
	if err := unmarshalJSONField(row.SuccessList, &successList); err != nil {
		return nil, fmt.Errorf("unmarshal success_list failed: %w", err)
	}

	return &tasktypes.GenerateRecord{
		SubOrderId:      row.SuborderID,
		GenerateId:      row.GenerateID,
		GenerateType:    row.GenerateType,
		TaskId:          row.TaskID,
		TaskLink:        row.TaskLink,
		RequestInfo:     row.RequestInfo,
		Status:          cvt.PtrToVal(row.Status),
		IsMatched:       cvt.PtrToVal(row.IsMatched),
		Message:         row.Message,
		TotalNum:        cvt.PtrToVal(row.TotalNum),
		SuccessNum:      cvt.PtrToVal(row.SuccessNum),
		SuccessList:     successList,
		CreateAt:        parseTableTime(row.CreatedAt),
		UpdateAt:        parseTableTime(row.UpdatedAt),
		StartAt:         parseStringTime(row.StartAt),
		EndAt:           parseStringTime(row.EndAt),
		IsManualMatched: row.IsManualMatched,
	}, nil
}

func buildMongoInitRecord(source interface{}) (interface{}, error) {
	row, ok := source.(*cvmapplytable.ZiyanCvmApplyInitTask)
	if !ok {
		return nil, fmt.Errorf("invalid source type %T for %s", source, table.ZiyanCvmApplyInitTaskTable)
	}

	return &tasktypes.InitRecord{
		SubOrderId: row.SuborderID,
		Ip:         row.IP,
		TaskId:     row.TaskID,
		TaskLink:   row.TaskLink,
		Status:     cvt.PtrToVal(row.Status),
		Message:    row.Message,
		CreateAt:   parseTableTime(row.CreatedAt),
		UpdateAt:   parseTableTime(row.UpdatedAt),
		StartAt:    parseStringTime(row.StartAt),
		EndAt:      parseStringTime(row.EndAt),
	}, nil
}

func buildMongoDeliverRecord(source interface{}) (interface{}, error) {
	row, ok := source.(*cvmapplytable.ZiyanCvmDeliverRecord)
	if !ok {
		return nil, fmt.Errorf("invalid source type %T for %s", source, table.ZiyanCvmDeliverRecordTable)
	}

	return &tasktypes.DeliverRecord{
		SubOrderId:       row.SuborderID,
		Ip:               row.IP,
		AssetId:          row.AssetID,
		Status:           cvt.PtrToVal(row.Status),
		Message:          row.Message,
		Deliverer:        row.Deliverer,
		GenerateTaskId:   row.GenerateTaskID,
		GenerateTaskLink: row.GenerateTaskLink,
		InitTaskId:       row.InitTaskID,
		InitTaskLink:     row.InitTaskLink,
		IsManualMatched:  cvt.PtrToVal(row.IsManualMatched),
		CreateAt:         parseTableTime(row.CreatedAt),
		UpdateAt:         parseTableTime(row.UpdatedAt),
		StartAt:          parseStringTime(row.StartAt),
		EndAt:            parseStringTime(row.EndAt),
	}, nil
}

func buildMongoDeviceInfo(source interface{}) (interface{}, error) {
	row, ok := source.(*cvmapplytable.ZiyanCvmDeviceInfo)
	if !ok {
		return nil, fmt.Errorf("invalid source type %T for %s", source, table.ZiyanCvmDeviceInfoTable)
	}

	return &tasktypes.DeviceInfo{
		OrderId:          uint64(row.OrderID),
		SubOrderId:       row.SuborderID,
		GenerateId:       row.GenerateID,
		BkBizId:          int(row.BkBizID),
		User:             row.BkUsername,
		BkHostId:         row.BkHostID,
		Ip:               row.IP,
		AssetId:          row.AssetID,
		InstanceID:       row.InstanceID,
		RequireType:      row.RequireType,
		ResourceType:     row.ResourceType,
		DeviceType:       row.DeviceType,
		Description:      row.Description,
		Remark:           row.Remark,
		ZoneName:         row.ZoneName,
		ZoneID:           int(row.ZoneID),
		CloudZone:        row.CloudZone,
		CloudRegion:      row.CloudRegion,
		ModuleName:       row.ModuleName,
		Equipment:        row.RackID,
		IsMatched:        cvt.PtrToVal(row.IsMatched),
		IsChecked:        cvt.PtrToVal(row.IsChecked),
		IsInited:         cvt.PtrToVal(row.IsInited),
		IsDelivered:      cvt.PtrToVal(row.IsDelivered),
		Deliverer:        row.Deliverer,
		GenerateTaskId:   row.GenerateTaskID,
		GenerateTaskLink: row.GenerateTaskLink,
		InitTaskId:       row.InitTaskID,
		InitTaskLink:     row.InitTaskLink,
		IsManualMatched:  row.IsManualMatched,
		OwnerIP:          row.OwnerIP,
		CreateAt:         parseTableTime(row.CreatedAt),
		UpdateAt:         parseTableTime(row.UpdatedAt),
	}, nil
}

func buildMongoModifyRecord(source interface{}) (interface{}, error) {
	row, ok := source.(*cvmapplytable.ZiyanCvmModifyRecord)
	if !ok {
		return nil, fmt.Errorf("invalid source type %T for %s", source, table.ZiyanCvmModifyRecordTable)
	}

	preData, err := buildModifyDataFromRow(
		row.PreTotalNum, row.PreReplicas, row.PreRegion, row.PreZone, row.PreDeviceType, row.PreImageID,
		row.PreDiskSize, row.PreDiskType, row.PreNetworkType, row.PreVpc, row.PreSubnet,
		row.PreSystemDiskType, row.PreSystemDiskSize, row.PreSystemDiskNum, row.PreDataDisk, row.PreZones,
		row.PreResAssign, row.PreBkAssetID, row.PreInheritInstanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("build pre_data failed: %w", err)
	}

	curData, err := buildModifyDataFromRow(
		row.CurTotalNum, row.CurReplicas, row.CurRegion, row.CurZone, row.CurDeviceType, row.CurImageID,
		row.CurDiskSize, row.CurDiskType, row.CurNetworkType, row.CurVpc, row.CurSubnet,
		row.CurSystemDiskType, row.CurSystemDiskSize, row.CurSystemDiskNum, row.CurDataDisk, row.CurZones,
		row.CurResAssign, row.CurBkAssetID, row.CurInheritInstanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("build cur_data failed: %w", err)
	}

	return &tasktable.ModifyRecord{
		ID:         row.ID,
		SuborderID: row.SuborderID,
		User:       row.BkUsername,
		Details: &tasktable.ModifyDetail{
			PreData: preData,
			CurData: curData,
		},
		CreatedAt: parseTableTime(row.CreatedAt),
		UpdatedAt: parseTableTime(row.UpdatedAt),
		Status:    row.Status,
		Approver:  row.Approver,
	}, nil
}

func buildModifyDataFromRow(totalNum, replicas *uint, region, zone, deviceType, imageID string,
	diskSize *int, diskType enumor.DiskType, networkType, vpc, subnet string,
	systemDiskType enumor.DiskType, systemDiskSize, systemDiskNum *int,
	dataDiskRaw, zonesRaw tabletypes.JsonField, resAssign enumor.ResAssign,
	bkAssetID, inheritInstanceID string) (*tasktable.ModifyData, error) {
	dataDisk := make([]enumor.DiskSpec, 0)
	if err := unmarshalJSONField(dataDiskRaw, &dataDisk); err != nil {
		return nil, fmt.Errorf("unmarshal data_disk failed: %w", err)
	}

	zones := make([]string, 0)
	if err := unmarshalJSONField(zonesRaw, &zones); err != nil {
		return nil, fmt.Errorf("unmarshal zones failed: %w", err)
	}

	return &tasktable.ModifyData{
		TotalNum:    cvt.PtrToVal(totalNum),
		Replicas:    cvt.PtrToVal(replicas),
		Region:      region,
		Zone:        zone,
		DeviceType:  deviceType,
		ImageId:     imageID,
		DiskSize:    int64(cvt.PtrToVal(diskSize)),
		DiskType:    diskType,
		NetworkType: networkType,
		Vpc:         vpc,
		Subnet:      subnet,
		SystemDisk: enumor.DiskSpec{
			DiskType: systemDiskType,
			DiskSize: uint(cvt.PtrToVal(systemDiskSize)),
			DiskNum:  uint(cvt.PtrToVal(systemDiskNum)),
		},
		DataDisk:          dataDisk,
		Zones:             zones,
		ResAssign:         resAssign,
		BkAssetID:         bkAssetID,
		InheritInstanceID: inheritInstanceID,
	}, nil
}

func unmarshalJSONField(raw interface{}, target interface{}) error {
	if raw == nil {
		return nil
	}

	var rawStr string
	switch val := raw.(type) {
	case tabletypes.JsonField:
		rawStr = string(val)
	case string:
		rawStr = val
	default:
		return fmt.Errorf("unsupported json field type %T", raw)
	}

	if len(rawStr) == 0 {
		return nil
	}

	trimmed := strings.TrimSpace(rawStr)
	if len(trimmed) == 0 || trimmed == "{}" || trimmed == "null" {
		return nil
	}

	return json.Unmarshal([]byte(trimmed), target)
}

func parseTableTime(raw tabletypes.Time) time.Time {
	if len(raw) == 0 {
		return time.Time{}
	}
	value := string(raw)

	t, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return t
	}

	t, err = time.Parse(constant.TimeStdFormat, value)
	if err == nil {
		return t
	}

	t, err = time.Parse("2006-01-02 15:04:05", value)
	if err == nil {
		return t
	}

	return time.Time{}
}

func parseStringTime(raw string) time.Time {
	if len(raw) == 0 {
		return time.Time{}
	}
	t, err := time.Parse(constant.TimeStdFormat, raw)
	if err == nil {
		return t
	}
	t, err = time.Parse(time.RFC3339, raw)
	if err == nil {
		return t
	}
	return time.Time{}
}
