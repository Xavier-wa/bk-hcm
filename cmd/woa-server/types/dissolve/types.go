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

package dissolve

import (
	"errors"
	"fmt"
	"time"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	hostdefine "hcm/pkg/dal/table/dissolve/host"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
)

// -------------------------- Create --------------------------

// RecycleHostCreateReq define recycle host create request.
type RecycleHostCreateReq struct {
	Hosts []hostdefine.RecycleHostTable `json:"hosts" validate:"required"`
}

// Validate recycle host create request.
func (req *RecycleHostCreateReq) Validate() error {
	if len(req.Hosts) == 0 {
		return errors.New("hosts is required")
	}

	if len(req.Hosts) > constant.BatchOperationMaxLimit {
		return fmt.Errorf("recycle host count should <= %d, but got: %d", constant.BatchOperationMaxLimit,
			len(req.Hosts))
	}

	return nil
}

// RecycleHostCreateResp define recycle host create response.
type RecycleHostCreateResp struct {
	IDs []string `json:"ids"`
}

// -------------------------- Update --------------------------

// RecycleHostUpdateReq define recycle host update request.
type RecycleHostUpdateReq struct {
	hostdefine.RecycleHostTable `json:",inline"`
}

// Validate recycle host update request.
func (req *RecycleHostUpdateReq) Validate() error {
	if len(req.ID) == 0 {
		return errors.New("id is required")
	}

	return nil
}

// -------------------------- List --------------------------

// RecycleHostListReq recycle host list req.
type RecycleHostListReq struct {
	Field  []string           `json:"field" validate:"omitempty"`
	Filter *filter.Expression `json:"filter" validate:"omitempty"`
	Page   *core.BasePage     `json:"page" validate:"required"`
}

// Validate recycle host list request.
func (req *RecycleHostListReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	pageOpt := &core.PageOption{
		EnableUnlimitedLimit: false,
		MaxLimit:             core.DefaultMaxPageLimit,
		DisabledSort:         false,
	}
	if err := req.Page.Validate(pageOpt); err != nil {
		return err
	}

	return nil
}

// HostListReq host list request.
type HostListReq struct {
	ResDissolveReq `json:",inline"`
	Page           *core.BasePage `json:"page" validate:"required"`
}

// Validate host list request.
func (req *HostListReq) Validate() error {
	if err := req.ResDissolveReq.Validate(); err != nil {
		return err
	}

	pageOpt := &core.PageOption{
		EnableUnlimitedLimit: false,
		MaxLimit:             50000, // 由于前端需要导出数据，这里特殊调整限制值
		DisabledSort:         false,
	}
	if err := req.Page.Validate(pageOpt); err != nil {
		return err
	}

	return nil
}

// ResDissolveReq resource dissolve request.
type ResDissolveReq struct {
	// ProjectIDs 裁撤项目ID
	ProjectIDs []int `json:"project_ids"`
	// GroupIDs 组织ID
	GroupIDs []int64 `json:"group_ids"`
	// BizIDs 业务ID
	BizIDs []int64 `json:"bk_biz_ids"`
	// Operators 负责人
	Operators []string `json:"operators"`
	// Regions 地域ID
	Regions []string `json:"regions"`
	// ExpectAbolishTimes 裁撤截止时间
	ExpectAbolishTimes []string `json:"expect_abolish_times"`
}

// Validate table list request.
func (req *ResDissolveReq) Validate() error {
	return nil
}

// DissolveStatus 对外暴露的两态裁撤状态
type DissolveStatus string

const (
	// DissolveStatusComplete 已裁撤，对应 DB abolish_phase=complete
	DissolveStatusComplete DissolveStatus = "complete"
	// DissolveStatusIncomplete 未裁撤，对应 DB abolish_phase in (incomplete, bsiComplete, retain)
	DissolveStatusIncomplete DissolveStatus = "incomplete"
)

// Validate 校验裁撤状态
func (s DissolveStatus) Validate() error {
	if s != DissolveStatusComplete && s != DissolveStatusIncomplete {
		return errors.New("status is invalid")
	}

	return nil
}

// ToAbolishPhases 将对外两态转换为 DB 四态
func (s DissolveStatus) ToAbolishPhases() []enumor.AbolishPhase {
	switch s {
	case DissolveStatusComplete:
		return []enumor.AbolishPhase{enumor.Complete}
	case DissolveStatusIncomplete:
		return []enumor.AbolishPhase{enumor.Incomplete, enumor.BsiComplete, enumor.Retain}
	default:
		return nil
	}
}

// FromAbolishPhase 将 DB 四态归并为对外两态
func FromAbolishPhase(phase enumor.AbolishPhase) DissolveStatus {
	if phase == enumor.Complete {
		return DissolveStatusComplete
	}

	return DissolveStatusIncomplete
}

// HostDetailListReq 裁撤主机明细列表请求
type HostDetailListReq struct {
	BizIDs             []int64        `json:"bk_biz_ids"`
	ProjectIDs         []int          `json:"project_ids"`
	GroupIDs           []int64        `json:"group_ids"`
	Operators          []string       `json:"operators"`
	Modules            []string       `json:"modules"`
	InnerIPs           []string       `json:"inner_ips"`
	AssetIDs           []string       `json:"asset_ids"`
	Status             DissolveStatus `json:"status"`
	ExpectAbolishTimes []string       `json:"expect_abolish_times"`
	Page               *core.BasePage `json:"page" validate:"required"`
}

// Validate 校验明细列表请求
func (req *HostDetailListReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	if req.Status != "" {
		if err := req.Status.Validate(); err != nil {
			return err
		}
	}

	pageOpt := &core.PageOption{
		EnableUnlimitedLimit: false,
		MaxLimit:             core.DefaultMaxPageLimit,
		DisabledSort:         false,
	}

	return req.Page.Validate(pageOpt)
}

// HostDetailExportListReq 查询导出的裁撤主机明细请求
type HostDetailExportListReq struct {
	BizIDs             []int64        `json:"bk_biz_ids"`
	ProjectIDs         []int          `json:"project_ids"`
	GroupIDs           []int64        `json:"group_ids"`
	Operators          []string       `json:"operators"`
	Modules            []string       `json:"modules"`
	InnerIPs           []string       `json:"inner_ips"`
	AssetIDs           []string       `json:"asset_ids"`
	Status             DissolveStatus `json:"status"`
	ExpectAbolishTimes []string       `json:"expect_abolish_times"`
	// SnapshotDate ES 快照日期(yyyyMMdd)，传入时按该日期快照补充扩展字段
	SnapshotDate string         `json:"snapshot_date"`
	Page         *core.BasePage `json:"page" validate:"required"`
}

// Validate 校验查询导出的裁撤主机明细请求
func (req *HostDetailExportListReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	if req.Status != "" {
		if err := req.Status.Validate(); err != nil {
			return err
		}
	}

	pageOpt := &core.PageOption{
		EnableUnlimitedLimit: false,
		MaxLimit:             constant.HostDetailExportListMaxLimit,
		DisabledSort:         false,
	}

	return req.Page.Validate(pageOpt)
}

// HostDetail 裁撤主机明细
type HostDetail struct {
	ID                string         `json:"id"`
	AssetID           string         `json:"asset_id"`
	InnerIP           string         `json:"inner_ip"`
	DeviceType        string         `json:"device_type"`
	Module            string         `json:"module"`
	Status            DissolveStatus `json:"status"`
	ProjectID         int            `json:"project_id"`
	ProjectName       string         `json:"project_name"`
	Region            string         `json:"region"`
	BkBizID           int64          `json:"bk_biz_id"`
	GroupID           int64          `json:"group_id"`
	Operators         []string       `json:"operators"`
	CPUCore           int            `json:"cpu_core"`
	ExpectAbolishTime string         `json:"expect_abolish_time"`
	Extension         *HostExtension `json:"extension,omitempty"`
}

// HostExtension ES 快照补充的主机性能/属性扩展字段（仅传入 snapshot_date 且快照命中时返回）
type HostExtension struct {
	// OuterIP 公网IP
	OuterIP string `json:"outer_ip"`
	// DeviceType SCM设备类型
	DeviceType string `json:"device_type"`
	// ModuleName 裁撤模块名称
	ModuleName string `json:"module_name"`
	// IdcUnitName 存放机房管理单元
	IdcUnitName string `json:"idc_unit_name"`
	// SfwNameVersion 操作系统
	SfwNameVersion string `json:"sfw_name_version"`
	// GoUpDate 上架时间
	GoUpDate string `json:"go_up_date"`
	// RaidName RAID结构
	RaidName string `json:"raid_name"`
	// LogicArea 逻辑区域
	LogicArea string `json:"logic_area"`
	// DeviceLayer 设备技术分类
	DeviceLayer string `json:"device_layer"`
	// CPUScore CPU得分
	CPUScore float64 `json:"cpu_score"`
	// MemScore 内存得分
	MemScore float64 `json:"mem_score"`
	// InnerNetTrafficScore 内网流量得分
	InnerNetTrafficScore float64 `json:"inner_net_traffic_score"`
	// DiskIoScore 磁盘IO得分
	DiskIoScore float64 `json:"disk_io_score"`
	// DiskUtilScore 磁盘IO使用率得分
	DiskUtilScore float64 `json:"disk_util_score"`
	// IsPass 是否达标
	IsPass bool `json:"is_pass"`
	// Mem4linux 内存使用量(G)
	Mem4linux float64 `json:"mem4linux"`
	// InnerNetTraffic 内网流量(Mb/s)
	InnerNetTraffic float64 `json:"inner_net_traffic"`
	// OuterNetTraffic 外网流量(Mb/s)
	OuterNetTraffic float64 `json:"outer_net_traffic"`
	// DiskIo 磁盘IO(Blocks/s)
	DiskIo float64 `json:"disk_io"`
	// DiskUtil 磁盘IO使用率
	DiskUtil float64 `json:"disk_util"`
	// DiskTotal 磁盘总量(G)
	DiskTotal float64 `json:"disk_total"`
	// GroupName 运维小组
	GroupName string `json:"group_name"`
	// Center 业务中心
	Center string `json:"center"`
}

// HostDetailListResult 裁撤主机明细列表响应
type HostDetailListResult struct {
	Count   int64        `json:"count"`
	Details []HostDetail `json:"details"`
}

// ExpectAbolishTimeListReq 查询裁撤截止时间列表请求
type ExpectAbolishTimeListReq struct {
	Filter *filter.Expression `json:"filter" validate:"omitempty"`
}

// Validate 校验查询裁撤截止时间列表请求
func (req *ExpectAbolishTimeListReq) Validate() error {
	return nil
}

// ExpectAbolishTimeListResult 查询裁撤截止时间列表响应
type ExpectAbolishTimeListResult struct {
	// ExpectAbolishTimes 去重升序的裁撤截止时间列表
	ExpectAbolishTimes []string `json:"expect_abolish_times"`
}

// ResDissolveTable resource dissolve table
type ResDissolveTable struct {
	Items []BizDetail `json:"items"`
}

// BizDetail 业务裁撤进度统计行，合计行的 bk_biz_id 为 0
type BizDetail struct {
	// BkBizID 业务ID，合计行为 0
	BkBizID int64 `json:"bk_biz_id"`
	// OriginHostCount 原始裁撤设备数
	OriginHostCount int64 `json:"origin_host_count"`
	// OriginCpuCore 原始裁撤CPU总核数
	OriginCpuCore int64 `json:"origin_cpu_core"`
	// CurrentHostCount 当前裁撤设备数
	CurrentHostCount int64 `json:"current_host_count"`
	// CurrentCpuCore 当前裁撤CPU总核数
	CurrentCpuCore int64 `json:"current_cpu_core"`
	// DeliveredCpuCore 已申领CPU核数
	DeliveredCpuCore int64 `json:"delivered_cpu_core"`
	// Progress 裁撤进度
	Progress string `json:"progress"`
}

// -------------------------- Delete --------------------------

// RecycleHostDeleteReq recycle host delete request.
type RecycleHostDeleteReq struct {
	IDs []string `json:"ids" validate:"required,min=1"`
}

// Validate recycle host delete request.
func (req *RecycleHostDeleteReq) Validate() error {
	if len(req.IDs) > constant.BatchOperationMaxLimit {
		return fmt.Errorf("batch delete limit is %d", constant.BatchOperationMaxLimit)
	}

	return validator.Validate.Struct(req)
}

// HostDissolveStatusCheckReq check host dissolve status request.
type HostDissolveStatusCheckReq struct {
	HostIDs []int64 `json:"bk_host_ids" validate:"required,min=1"`
}

// Validate ...
func (req *HostDissolveStatusCheckReq) Validate() error {
	if len(req.HostIDs) > constant.BatchOperationMaxLimit {
		return fmt.Errorf("check dissolve host status limit is %d", constant.BatchOperationMaxLimit)
	}

	return validator.Validate.Struct(req)
}

// HostDissolveStatusCheckResp check host dissolve status response.
type HostDissolveStatusCheckResp struct {
	Info []HostDissolveStatusCheckInfo `json:"info"`
}

// HostDissolveStatusCheckInfo check host dissolve status info.
type HostDissolveStatusCheckInfo struct {
	HostID int64 `json:"bk_host_id"`
	Status bool  `json:"status"`
}

// ListDissolveCpuCoreSummaryReq list dissolve cpu core summary request
type ListDissolveCpuCoreSummaryReq struct {
	BizID int64 `json:"bk_biz_id" validate:"required"`
}

// Validate ...
func (l *ListDissolveCpuCoreSummaryReq) Validate() error {
	return validator.Validate.Struct(l)
}

// CpuCoreSummary dissolve cpu core summary
type CpuCoreSummary struct {
	TotalCore        int64     `json:"total_core"`
	DeliveredCore    int64     `json:"delivered_core"`
	HostApplyTime    time.Time `json:"host_apply_time"`
	QuotaCoefficient float64   `json:"quota_coefficient"` // 配额系数
	QuotaOffset      int64     `json:"quota_offset"`      // 业务偏移额度（正数为调增，负数为调减）
	AvailableQuota   int64     `json:"available_quota"`   // 可申请额度
}

// QuotaOffsetItem 单个业务偏移配置
type QuotaOffsetItem struct {
	BkBizID int64                          `json:"bk_biz_id"` // 业务ID
	Offset  int64                          `json:"offset"`    // 偏移值
	Type    enumor.DissolveQuotaOffsetType `json:"type"`      // 调整类型：increase=调增，decrease=调减
	Memo    string                         `json:"memo"`      // 调整原因
}

// Validate 校验偏移配置
func (q *QuotaOffsetItem) Validate() error {
	if q.BkBizID <= 0 {
		return fmt.Errorf("invalid bk_biz_id: %d", q.BkBizID)
	}
	if err := q.Type.Validate(); err != nil {
		return err
	}
	if q.Offset < 0 {
		return errors.New("offset must be non-negative")
	}
	if len(q.Memo) > 512 {
		return errors.New("memo exceeds 512 characters")
	}
	return nil
}

// SignedOffset 返回带符号的偏移值（调增为正，调减为负）
func (q *QuotaOffsetItem) SignedOffset() int64 {
	if q.Type == enumor.DissolveQuotaOffsetTypeIncrease {
		return q.Offset
	}
	return -q.Offset
}

// Config dissolve config
type Config struct {
	HostApplyTime    *time.Time             `json:"host_apply_time"`
	ApprovalLimit    *float64               `json:"approval_limit"`
	QuotaCoefficient *float64               `json:"quota_coefficient,omitempty"`
	QuotaOffsets     []QuotaOffsetItem      `json:"quota_offsets,omitempty"`
	DissolveProjects []DissolveProjectCycle `json:"dissolve_projects,omitempty"`
}

// DissolveProjectCycle 裁撤周期
type DissolveProjectCycle struct {
	// Start 裁撤周期开始时间，格式 yyyy-MM-dd
	Start string `json:"start"`
	// End 裁撤周期结束时间，格式 yyyy-MM-dd
	End string `json:"end"`
	// Default 是否为当前裁撤周期，可有多个或零个，不要求唯一
	Default bool `json:"default"`
	// Projects 裁撤项目列表
	Projects []DissolveProjectItem `json:"projects"`
}

// DissolveProjectItem 裁撤项目项
type DissolveProjectItem struct {
	// ID 裁撤项目ID
	ID int `json:"id"`
	// Memo 项目备注
	Memo string `json:"memo"`
}

// Validate 校验裁撤周期
func (c *DissolveProjectCycle) Validate() error {
	start, err := time.Parse(constant.DateLayout, c.Start)
	if err != nil {
		return fmt.Errorf("invalid start date: %s", c.Start)
	}

	end, err := time.Parse(constant.DateLayout, c.End)
	if err != nil {
		return fmt.Errorf("invalid end date: %s", c.End)
	}

	if start.After(end) {
		return fmt.Errorf("start date %s must not be after end date %s", c.Start, c.End)
	}

	if len(c.Projects) == 0 {
		return errors.New("projects can not be empty")
	}

	for _, p := range c.Projects {
		if p.ID <= 0 {
			return fmt.Errorf("invalid project id: %d", p.ID)
		}
	}

	return nil
}

// UpsertConfigReq upsert config request
type UpsertConfigReq struct {
	HostApplyTime    *time.Time             `json:"host_apply_time" validate:"omitempty"`
	ApprovalLimit    *float64               `json:"approval_limit" validate:"omitempty"`
	QuotaCoefficient *float64               `json:"quota_coefficient" validate:"omitempty"`
	QuotaOffsets     []QuotaOffsetItem      `json:"quota_offsets" validate:"omitempty"`
	DissolveProjects []DissolveProjectCycle `json:"dissolve_projects" validate:"omitempty"`
}

// Validate ...
func (u *UpsertConfigReq) Validate() error {
	if err := validator.Validate.Struct(u); err != nil {
		return err
	}

	if u.ApprovalLimit != nil {
		if cvt.PtrToVal(u.ApprovalLimit) < 0 || cvt.PtrToVal(u.ApprovalLimit) > 100 {
			return errors.New("approval_limit must between 0 and 100")
		}
	}

	if u.QuotaCoefficient != nil {
		coef := cvt.PtrToVal(u.QuotaCoefficient)
		if coef < 1 || coef > 100 {
			return errors.New("quota_coefficient must between 1 and 100")
		}
	}

	// 校验偏移配置
	seenBizIDs := make(map[int64]bool)
	for _, item := range u.QuotaOffsets {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("quota offset for biz %d: %w", item.BkBizID, err)
		}
		if seenBizIDs[item.BkBizID] {
			return fmt.Errorf("duplicate bk_biz_id: %d", item.BkBizID)
		}
		seenBizIDs[item.BkBizID] = true
	}

	// 校验裁撤项目配置
	for i := range u.DissolveProjects {
		if err := u.DissolveProjects[i].Validate(); err != nil {
			return err
		}
	}

	return nil
}

// UpdateDissolveQuotaOffsetReq 单业务偏移修改请求
type UpdateDissolveQuotaOffsetReq struct {
	Offset *int64                         `json:"offset" validate:"required,min=0"`
	Type   enumor.DissolveQuotaOffsetType `json:"type" validate:"required"`
	Memo   string                         `json:"memo" validate:"max=512"`
}

// Validate ...
func (u *UpdateDissolveQuotaOffsetReq) Validate() error {
	if err := validator.Validate.Struct(u); err != nil {
		return err
	}
	if err := u.Type.Validate(); err != nil {
		return err
	}
	// 显式校验 offset >= 0，因为 validator 的 min=0 对指针类型可能不生效
	if u.Offset != nil && *u.Offset < 0 {
		return errors.New("offset must be greater than or equal to 0")
	}
	if len(u.Memo) > 512 {
		return errors.New("memo exceeds 512 characters")
	}
	return nil
}

// SignedOffset 返回带符号的偏移值（调增为正，调减为负）
func (u *UpdateDissolveQuotaOffsetReq) SignedOffset() int64 {
	if u.Offset == nil {
		return 0
	}
	if u.Type == enumor.DissolveQuotaOffsetTypeIncrease {
		return *u.Offset
	}
	return -*u.Offset
}

// UpdateDissolveQuotaOffsetResp 单业务偏移修改响应
type UpdateDissolveQuotaOffsetResp struct {
	BkBizID      int64 `json:"bk_biz_id"`
	BeforeOffset int64 `json:"before_offset"`
	AfterOffset  int64 `json:"after_offset"`
}
