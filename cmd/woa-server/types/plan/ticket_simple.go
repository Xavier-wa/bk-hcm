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

package plan

import (
	"errors"
	"fmt"
	"unicode/utf8"

	devicetype "hcm/pkg/api/core/cloud/device-type"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	cvt "hcm/pkg/tools/converter"

	"github.com/shopspring/decimal"
)

// ResPlanDefaultDemandSource is demand_source filled by the simple flow.
// Product wording「统一指标变化」aligns with enum value DemandSourceIndChg ("指标变化").
var ResPlanDefaultDemandSource = enumor.DemandSourceIndChg

// ResPlanDefaultResMode is res_mode filled by the simple flow (按机型).
var ResPlanDefaultResMode = enumor.ResModeByDeviceType

// CreateResPlanTicketSimpleReq is a simplified create resource plan ticket request.
// Convert to CreateResPlanTicketReq via ToCreateResPlanTicketReq before calling existing handlers.
type CreateResPlanTicketSimpleReq struct {
	DemandClass enumor.DemandClass             `json:"demand_class" validate:"required"`
	Demands     []CreateResPlanDemandSimpleReq `json:"demands" validate:"required"`
	Remark      string                         `json:"remark" validate:"required"`
}

// Validate validates CreateResPlanTicketSimpleReq.
func (r *CreateResPlanTicketSimpleReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	if err := r.DemandClass.Validate(); err != nil {
		return err
	}

	for i := range r.Demands {
		if err := r.Demands[i].Validate(); err != nil {
			return fmt.Errorf("demands[%d]: %w", i, err)
		}
	}

	lenRemark := utf8.RuneCountInString(r.Remark)
	if lenRemark < 20 || lenRemark > 1024 {
		return errors.New("len remark should be >= 20 and <= 1024")
	}

	return nil
}

// CreateResPlanDemandSimpleReq is a simplified single demand (omits res_mode, demand_source, demand_res_types).
type CreateResPlanDemandSimpleReq struct {
	ObsProject     enumor.ObsProject `json:"obs_project" validate:"required"`
	ExpectTime     string            `json:"expect_time" validate:"required"`
	ReturnPlanTime string            `json:"return_plan_time" validate:"omitempty"`
	RegionID       string            `json:"region_id" validate:"required"`
	ZoneID         string            `json:"zone_id" validate:"omitempty"`
	Remark         string            `json:"remark" validate:"omitempty"`
	Cvm            *CvmSimpleInput   `json:"cvm" validate:"omitempty"`
	Cbs            *CbsSimpleInput   `json:"cbs" validate:"omitempty"`
}

// Validate validates CreateResPlanDemandSimpleReq.
func (r *CreateResPlanDemandSimpleReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	if err := r.ObsProject.ValidateResPlan(); err != nil {
		return err
	}

	lenRemark := utf8.RuneCountInString(r.Remark)
	if lenRemark > 255 {
		return errors.New("len remark should <= 255")
	}

	if r.Cvm == nil && r.Cbs == nil {
		return errors.New("cvm and cbs cannot both be empty")
	}

	if r.Cvm != nil {
		if err := r.Cvm.Validate(); err != nil {
			return err
		}
	}

	if r.Cbs != nil {
		if err := r.Cbs.Validate(); err != nil {
			return err
		}
	}

	if r.ObsProject == enumor.ObsProjectShortLease && r.ReturnPlanTime == "" {
		return errors.New("obs project is short lease, return plan time should not be empty")
	}

	return nil
}

// CvmSimpleInput is simplified CVM input: default res_mode is 按机型;
// at least one of Os (数量) or CpuCore must be set.
// Optional CpuCore/Memory override computed totals when Os is set;
// optional Memory overrides mem when only CpuCore is set.
type CvmSimpleInput struct {
	DeviceType string           `json:"device_type"`
	Os         *decimal.Decimal `json:"os" validate:"omitempty"`
	CpuCore    *int64           `json:"cpu_core" validate:"omitempty"`
	Memory     *int64           `json:"memory" validate:"omitempty"`
}

// Validate validates CvmSimpleInput (at least one of Os or CpuCore when Cvm block is used).
func (c *CvmSimpleInput) Validate() error {
	if len(c.DeviceType) == 0 {
		return errors.New("cvm device type should not be empty")
	}

	hasOs := c.Os != nil
	hasCPU := c.CpuCore != nil
	if !hasOs && !hasCPU {
		return errors.New("cvm requires at least one of os or cpu_core")
	}

	if hasOs {
		if c.Os.IsNegative() {
			return errors.New("os should be >= 0")
		}
	}

	if hasCPU && *c.CpuCore < 0 {
		return errors.New("cpu core should be >= 0")
	}

	if c.Memory != nil && *c.Memory < 0 {
		return errors.New("memory should be >= 0")
	}

	return nil
}

// CbsSimpleInput mirrors CBS fields needed when simple passes a cbs block.
type CbsSimpleInput struct {
	DiskType enumor.DiskType `json:"disk_type"`
	DiskIo   *int64          `json:"disk_io"`
	DiskSize *int64          `json:"disk_size"`
}

// Validate validates CbsSimpleInput.
func (c *CbsSimpleInput) Validate() error {
	if len(c.DiskType) > 0 {
		if err := c.DiskType.Validate(); err != nil {
			return err
		}
	}

	if c.DiskIo == nil || *c.DiskIo < 0 {
		return errors.New("disk io should be >= 0")
	}

	if c.DiskSize == nil || *c.DiskSize < 0 {
		return errors.New("disk size should be >= 0")
	}

	return nil
}

// ToCreateResPlanTicketReq expands simple request into CreateResPlanTicketReq using device type metadata.
func (r *CreateResPlanTicketSimpleReq) ToCreateResPlanTicketReq(
	deviceTypeMap map[string]devicetype.DistinctDeviceType) (*CreateResPlanTicketReq, error) {

	if deviceTypeMap == nil {
		return nil, errors.New("deviceTypeMap is nil")
	}

	out := &CreateResPlanTicketReq{
		DemandClass: r.DemandClass,
		Remark:      r.Remark,
		Demands:     make([]CreateResPlanDemandReq, 0, len(r.Demands)),
	}

	for i := range r.Demands {
		d, err := r.Demands[i].toCreateResPlanDemandReq(deviceTypeMap)
		if err != nil {
			return nil, fmt.Errorf("demands[%d]: %w", i, err)
		}
		out.Demands = append(out.Demands, *d)
	}

	return out, nil
}

func (r *CreateResPlanDemandSimpleReq) toCreateResPlanDemandReq(
	deviceTypeMap map[string]devicetype.DistinctDeviceType) (*CreateResPlanDemandReq, error) {

	res := &CreateResPlanDemandReq{
		ObsProject:     r.ObsProject,
		ExpectTime:     r.ExpectTime,
		ReturnPlanTime: r.ReturnPlanTime,
		RegionID:       r.RegionID,
		ZoneID:         r.ZoneID,
		DemandSource:   ResPlanDefaultDemandSource,
		Remark:         r.Remark,
	}

	if r.Cvm != nil {
		meta, ok := deviceTypeMap[r.Cvm.DeviceType]
		if !ok {
			return nil, fmt.Errorf("device_type %q not found in metadata", r.Cvm.DeviceType)
		}

		osVal, cpuVal, memVal, err := deriveCvmScaleFromSimple(r.Cvm, meta)
		if err != nil {
			return nil, err
		}

		res.DemandResTypes = append(res.DemandResTypes, enumor.DemandResTypeCVM)
		res.Cvm = &struct {
			ResMode    enumor.ResMode   `json:"res_mode"`
			DeviceType string           `json:"device_type"`
			Os         *decimal.Decimal `json:"os"`
			CpuCore    *int64           `json:"cpu_core"`
			Memory     *int64           `json:"memory"`
		}{
			ResMode:    ResPlanDefaultResMode,
			DeviceType: r.Cvm.DeviceType,
			Os:         osVal,
			CpuCore:    cpuVal,
			Memory:     memVal,
		}
	}

	if r.Cbs != nil {
		res.DemandResTypes = append(res.DemandResTypes, enumor.DemandResTypeCBS)
		res.Cbs = &struct {
			DiskType enumor.DiskType `json:"disk_type"`
			DiskIo   *int64          `json:"disk_io"`
			DiskSize *int64          `json:"disk_size"`
		}{
			DiskType: r.Cbs.DiskType,
			DiskIo:   r.Cbs.DiskIo,
			DiskSize: r.Cbs.DiskSize,
		}
	}

	return res, nil
}

func deriveCvmScaleFromSimple(in *CvmSimpleInput, meta devicetype.DistinctDeviceType) (
	osOut *decimal.Decimal, cpuOut *int64, memOut *int64, err error) {

	hasOs := in.Os != nil
	hasCPU := in.CpuCore != nil

	if hasOs {
		cpuD := in.Os.Mul(decimal.NewFromInt(meta.CpuCore))
		memD := in.Os.Mul(decimal.NewFromInt(meta.Memory))
		cpu := cpuD.Round(0).IntPart()
		mem := memD.Round(0).IntPart()
		if hasCPU {
			cpu = *in.CpuCore
		}
		if in.Memory != nil {
			mem = *in.Memory
		}
		osCopy := *in.Os
		return &osCopy, cvt.ValToPtr(cpu), cvt.ValToPtr(mem), nil
	}

	// !hasOs: CpuCore is required (cannot both be empty; Validate also enforces this).
	if !hasCPU {
		return nil, nil, nil, errors.New("cvm scale input is empty")
	}
	if meta.CpuCore <= 0 {
		return nil, nil, nil, fmt.Errorf("device_type %q has invalid cpu_core in metadata",
			in.DeviceType)
	}

	cpu := *in.CpuCore
	osDec := decimal.NewFromInt(cpu).Div(decimal.NewFromInt(meta.CpuCore))
	mem := osDec.Mul(decimal.NewFromInt(meta.Memory))
	memI := mem.Round(0).IntPart()
	if in.Memory != nil {
		memI = *in.Memory
	}

	return cvt.ValToPtr(osDec), cvt.ValToPtr(cpu), cvt.ValToPtr(memI), nil
}
