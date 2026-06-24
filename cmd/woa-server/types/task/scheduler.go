/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package task ...
package task

import (
	"errors"
	"fmt"
	"time"

	"hcm/cmd/woa-server/dal/task/table"
	"hcm/pkg"
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/thirdparty/api-gateway/bkbotapproval"
	"hcm/pkg/thirdparty/api-gateway/itsm"
	"hcm/pkg/thirdparty/cvmapi"
	"hcm/pkg/tools/util"
)

// ApplyOrder resource apply order
type ApplyOrder struct {
	OrderId      uint64                   `json:"order_id" bson:"order_id"`
	SubOrderId   string                   `json:"suborder_id" bson:"suborder_id"`
	BkBizId      int64                    `json:"bk_biz_id" bson:"bk_biz_id"`
	User         string                   `json:"bk_username" bson:"bk_username"`
	Follower     []string                 `json:"follower" bson:"follower"`
	Auditor      string                   `json:"auditor" bson:"auditor"`
	RequireType  enumor.RequireType       `json:"require_type" bson:"require_type"`
	ExpectTime   string                   `json:"expect_time" bson:"expect_time"`
	ResourceType ResourceType             `json:"resource_type" bson:"resource_type"`
	Source       enumor.ApplyTicketSource `json:"source" bson:"source"`
	ProductType  enumor.ProductType       `json:"product_type" bson:"product_type"`
	Spec         *ResourceSpec            `json:"spec" bson:"spec"`
	// UpgradeCVMList cvm升降配列表
	UpgradeCVMList    []*UpgradeCVMSpec `json:"upgrade_cvm_list" bson:"upgrade_cvm_list"`
	AntiAffinityLevel string            `json:"anti_affinity_level" bson:"anti_affinity_level"`
	EnableDiskCheck   bool              `json:"enable_disk_check" bson:"enable_disk_check"`
	Description       string            `json:"description" bson:"description"`
	Remark            string            `json:"remark" bson:"remark"`
	Stage             TicketStage       `json:"stage" bson:"stage"`
	Status            ApplyStatus       `json:"status" bson:"status"`
	OriginNum         uint              `json:"origin_num" bson:"origin_num"` // 原始需求总数量，不会修改
	TotalNum          uint              `json:"total_num" bson:"total_num"`   // 需要交付的总数量，业务会修改
	SuccessNum        uint              `json:"success_num" bson:"success_num"`
	PendingNum        uint              `json:"pending_num" bson:"pending_num"`
	// AppliedCore 注意：该字段目前只会记录虚拟机申请的核心数量
	AppliedCore uint `json:"applied_core" bson:"applied_core,omitempty"`
	// DeliveredCore 注意：该字段目前只会记录虚拟机交付的核心数量
	DeliveredCore uint `json:"delivered_core" bson:"delivered_core,omitempty"`
	// PlanExpendGroup 本单据对应的预测消耗记录，按机型、地域分组（同一单可能出现多种机型、地域）
	PlanExpendGroup []PlanExpendGroup `json:"plan_expend_group" bson:"plan_expend_group"`
	ObsProject      enumor.ObsProject `json:"obs_project" bson:"obs_project"`
	RetryTime       uint              `json:"retry_time" bson:"retry_time"`
	ModifyTime      uint              `json:"modify_time" bson:"modify_time"`
	CreateAt        time.Time         `json:"create_at" bson:"create_at"`
	UpdateAt        time.Time         `json:"update_at" bson:"update_at"`
}

// IsSuborderTerminated 判断子单是否已终止：终止后剩余的主机不再继续生产
func (subOrder *ApplyOrder) IsSuborderTerminated() bool {
	return subOrder.Stage == enumor.TicketStageTerminate
}

// UpgradeCVMSpec cvm升降配规格
type UpgradeCVMSpec struct {
	InstanceID           string   `json:"instance_id" bson:"instance_id"`
	PrivateIPv4Addresses []string `json:"private_ipv4_addresses" bson:"private_ipv4_addresses"`
	PrivateIPv6Addresses []string `json:"private_ipv6_addresses" bson:"private_ipv6_addresses"`
	BkAssetID            string   `json:"bk_asset_id" bson:"bk_asset_id"`
	Operator             string   `json:"operator" bson:"operator"`               // 主负责人
	BkBakOperator        string   `json:"bk_bak_operator" bson:"bk_bak_operator"` // 备份负责人
	DeviceType           string   `json:"device_type" bson:"device_type"`
	RegionID             string   `json:"region_id" bson:"region_id"`
	ZoneID               string   `json:"zone_id" bson:"zone_id"`
	TargetInstanceType   string   `json:"target_instance_type" bson:"target_instance_type"`
}

// ResourceType resource type（类型定义已下沉至 pkg/criteria/enumor/cvm_apply.go）
type ResourceType = enumor.ResourceType

// ResourceType resource type
const (
	ResourceTypePm          = enumor.ResourceTypePm
	ResourceTypeCvm         = enumor.ResourceTypeCvm
	ResourceTypeIdcDvm      = enumor.ResourceTypeIdcDvm
	ResourceTypeQcloudDvm   = enumor.ResourceTypeQcloudDvm
	ResourceTypePool        = enumor.ResourceTypePool
	ResourceTypeOthers      = enumor.ResourceTypeOthers
	ResourceTypeUnsupported = enumor.ResourceTypeUnsupported
	ResourceTypeUpgradeCvm  = enumor.ResourceTypeUpgradeCvm

	ApplyLimit = 1000
)

// AllResourceType all resource type
var AllResourceType = []ResourceType{
	ResourceTypePm,
	ResourceTypeCvm,
	ResourceTypeIdcDvm,
	ResourceTypeQcloudDvm,
	ResourceTypeUpgradeCvm,
}

// ApplyStatus apply status（类型定义已下沉至 pkg/criteria/enumor/cvm_apply.go）
type ApplyStatus = enumor.ApplyStatus

/*
	apply status:

WaitForMatch	待匹配。初始状态
Matching		匹配执行中
MatchedSome		已完成部分资源匹配
Paused			已暂停
Done			终止
*/
const (
	ApplyStatusWaitForMatch      = enumor.ApplyStatusWaitForMatch
	ApplyStatusMatching          = enumor.ApplyStatusMatching
	ApplyStatusMatchedSome       = enumor.ApplyStatusMatchedSome
	ApplyStatusPaused            = enumor.ApplyStatusPaused
	ApplyStatusDone              = enumor.ApplyStatusDone
	ApplyStatusTerminate         = enumor.ApplyStatusTerminate
	ApplyStatusGracefulTerminate = enumor.ApplyStatusGracefulTerminate
	ApplyStatusConfirming        = enumor.ApplyStatusConfirming
)

// GenerateRecord apply order vm generate record
type GenerateRecord struct {
	SubOrderId   string `json:"suborder_id" bson:"suborder_id"`
	GenerateId   string `json:"generate_id" bson:"generate_id"`
	GenerateType string `json:"generate_type" bson:"generate_type"`
	TaskId       string `json:"task_id" bson:"task_id"`
	TaskLink     string `json:"task_link" bson:"task_link"`
	RequestInfo  string `json:"request_info" bson:"request_info"`
	// 0: success, 1: handling, 2: failed
	Status          GenerateStepStatus `json:"status" bson:"status"`
	IsMatched       bool               `json:"is_matched" bson:"is_matched"`
	Message         string             `json:"message" bson:"message"`
	TotalNum        uint               `json:"total_num" bson:"total_num"`
	SuccessNum      uint               `json:"success_num" bson:"success_num"`
	SuccessList     []string           `json:"success_list" bson:"success_list"`
	CreateAt        time.Time          `json:"create_at" bson:"create_at"`
	UpdateAt        time.Time          `json:"update_at" bson:"update_at"`
	StartAt         time.Time          `json:"start_at" bson:"start_at"`
	EndAt           time.Time          `json:"end_at" bson:"end_at"`
	IsManualMatched bool               `json:"is_manual_matched" bson:"is_manual_matched"` // 是否手工匹配
}

// GenerateStepStatus generate step status（类型定义已下沉至 pkg/criteria/enumor/cvm_apply.go）
type GenerateStepStatus = enumor.GenerateStepStatus

// GenerateStepStatus generate step status
const (
	GenerateStatusInit     = enumor.GenerateStatusInit
	GenerateStatusSuccess  = enumor.GenerateStatusSuccess
	GenerateStatusHandling = enumor.GenerateStatusHandling
	GenerateStatusFailed   = enumor.GenerateStatusFailed
	GenerateStatusSuspend  = enumor.GenerateStatusSuspend
)

// GetApplyDeviceReq get resource apply delivered devices request
type GetApplyDeviceReq struct {
	BkBizIDs []int64            `json:"bk_biz_ids"`
	Filter   *filter.Expression `json:"filter"`
	Page     *core.BasePage     `json:"page"`
}

// Validate whether GetApplyDeviceReq is valid
func (req *GetApplyDeviceReq) Validate() error {
	if req.Filter == nil {
		return errf.New(errf.InvalidParameter, "filter is required")
	}

	if req.Page == nil {
		return errf.New(errf.InvalidParameter, "page is required")
	}

	// 兼容原来直接调用req.Page.Validate(true)逻辑，只是放开了最大的限制
	page := req.Page
	if page.Count {
		if page.Start > 0 || page.Limit > 0 || page.Sort != "" {
			return fmt.Errorf("params page can not be set")
		}
		return nil
	}
	if page.Limit > constant.CvmApplyDeviceExportLimit && page.Limit != pkg.BKNoLimit {
		return fmt.Errorf("limit exceed max page size: %d", constant.CvmApplyDeviceExportLimit)
	}

	return nil
}

// GetApplyDeviceRst get resource apply delivered devices result
type GetApplyDeviceRst struct {
	Count int64         `json:"count"`
	Info  []*DeviceInfo `json:"info"`
}

// GetDeliverDeviceReq get resource apply delivered devices request
type GetDeliverDeviceReq struct {
	OrderId    uint64 `json:"order_id" bson:"order_id"`
	SuborderId string `json:"suborder_id" bson:"suborder_id"`
}

// Validate whether GetDeliverDeviceReq is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (req *GetDeliverDeviceReq) Validate() (errKey string, err error) {
	if req.OrderId <= 0 {
		return "order_id", errors.New("invalid order_id <= 0")
	}
	return "", nil
}

// ExportDeliverDeviceReq export resource apply delivered devices request
type ExportDeliverDeviceReq struct {
	BkBizId int64              `json:"bk_biz_id"`
	Filter  *filter.Expression `json:"filter"`
}

// Validate whether ExportDeliverDeviceReq is valid
func (req *ExportDeliverDeviceReq) Validate() error {
	if req.BkBizId <= 0 {
		return errors.New("invalid bk_biz_id <= 0")
	}

	if req.Filter == nil {
		return errf.New(errf.InvalidParameter, "filter is required")
	}

	return nil
}

// GetMatchDeviceReq get resource apply match devices request
type GetMatchDeviceReq struct {
	ResourceType      ResourceType `json:"resource_type"`
	Ips               []string     `json:"ips"`
	Spec              *MatchSpec   `json:"spec"`
	AntiAffinityLevel string       `json:"anti_affinity_level"`
	TotalNum          int64        `json:"total_num"`
	PendingNum        int64        `json:"pending_num"`
}

// MatchSpec resource apply match specification
type MatchSpec struct {
	Region             []string `json:"region"`
	Zone               []string `json:"zone"`
	DeviceType         []string `json:"device_type"`
	Image              []string `json:"image"`
	OsType             string   `json:"os_type"`
	RaidType           []string `json:"raid_type"`
	DiskType           []string `json:"disk_type"`
	NetworkType        []string `json:"network_type"`
	Isp                []string `json:"isp"`
	InstanceChargeType string   `json:"instance_charge_type"`
}

// Validate whether GetMatchDeviceReq is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (req *GetMatchDeviceReq) Validate() (errKey string, err error) {
	// TODO
	return "", nil
}

// GetMatchDeviceRst get resource apply match devices result
type GetMatchDeviceRst struct {
	Count int64          `json:"count"`
	Info  []*MatchDevice `json:"info"`
}

// MatchDevice resource apply match device info
type MatchDevice struct {
	BkHostId           int64     `json:"bk_host_id"`
	AssetId            string    `json:"asset_id"`
	Ip                 string    `json:"ip"`
	OuterIp            string    `json:"outer_ip"`
	Isp                string    `json:"isp"`
	DeviceType         string    `json:"device_type"`
	OsType             string    `json:"os_type"`
	Region             string    `json:"region"`
	Zone               string    `json:"zone"`
	Module             string    `json:"module"`
	Equipment          int64     `json:"equipment"`
	IdcUnit            string    `json:"idc_unit"`
	IdcLogicArea       string    `json:"idc_logic_area"`
	RaidType           string    `json:"raid_type"`
	InputTime          string    `json:"input_time"`
	MatchScore         float64   `json:"match_score"`
	MatchTag           bool      `json:"match_tag"`
	InstanceChargeType string    `json:"instance_charge_type"`
	BillingStartTime   time.Time `json:"billing_start_time"`
	BillingExpireTime  time.Time `json:"billing_expire_time"`
}

// MatchDeviceReq resource apply manual match devices request
type MatchDeviceReq struct {
	SuborderId string              `json:"suborder_id"`
	Operator   string              `json:"operator"`
	Device     []*MatchDeviceBrief `json:"device"`
}

// MatchDeviceBrief match device brief info
type MatchDeviceBrief struct {
	BkHostId int64  `json:"bk_host_id"`
	AssetId  string `json:"asset_id"`
	Ip       string `json:"ip"`
}

// Validate whether MatchDeviceReq is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (req *MatchDeviceReq) Validate() (errKey string, err error) {
	// TODO
	return "", nil
}

// MatchPoolDeviceReq match pool device request
type MatchPoolDeviceReq struct {
	SuborderId string           `json:"suborder_id"`
	Spec       []*MatchPoolSpec `json:"spec"`
}

// MatchPoolSpec resource apply pool device match specification
type MatchPoolSpec struct {
	DeviceType    string `json:"device_type"`
	ImageID       string `json:"image_id"`
	OsType        string `json:"os_type"`
	Replicas      int64  `json:"replicas"`
	BkCloudRegion string `json:"bk_cloud_region"`
	BkCloudZone   string `json:"bk_cloud_zone"`
}

// Validate whether MatchPoolDeviceReq is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (param *MatchPoolDeviceReq) Validate() (errKey string, err error) {
	if param.SuborderId == "" {
		return "suborder_id", errors.New("suborder_id cannot be empty")
	}

	if len(param.Spec) == 0 {
		return "spec", errors.New("spec cannot be empty")
	}

	for index, spec := range param.Spec {
		if spec == nil {
			return fmt.Sprintf("spec[%d]", index), errors.New("spec cannot be empty")
		}

		if spec.BkCloudRegion == "" {
			return fmt.Sprintf("spec[%d].bk_cloud_region", index), errors.New("spec.bk_cloud_region cannot be empty")
		}

		if spec.BkCloudZone == "" {
			return fmt.Sprintf("spec[%d].bk_cloud_zone", index), errors.New("spec.bk_cloud_zone cannot be empty")
		}

		if spec.DeviceType == "" {
			return fmt.Sprintf("spec[%d].device_type", index), errors.New("spec.device_type cannot be empty")
		}

		if spec.ImageID == "" && spec.OsType == "" {
			return fmt.Sprintf("spec[%d].image_id/os_type", index), errors.New("spec.image_id/os_type cannot be empty")
		}

		if spec.Replicas <= 0 {
			return fmt.Sprintf("spec[%d].replicas", index), errors.New("spec.replicas should be positive")
		}

		if spec.Replicas > pkg.BKMaxInstanceLimit {
			return fmt.Sprintf("spec[%d].replicas", index), fmt.Errorf("spec.replicas exceed limit %d",
				pkg.BKMaxInstanceLimit)
		}
	}

	return "", nil
}

// DeviceInfo device info
type DeviceInfo struct {
	OrderId      uint64             `json:"order_id" bson:"order_id"`
	SubOrderId   string             `json:"suborder_id" bson:"suborder_id"`
	GenerateId   string             `json:"generate_id" bson:"generate_id"`
	BkBizId      int                `json:"bk_biz_id" bson:"bk_biz_id"`
	User         string             `json:"bk_username" bson:"bk_username"`
	BkHostId     int64              `json:"bk_host_id" bson:"bk_host_id"`
	Ip           string             `json:"ip" bson:"ip"`
	AssetId      string             `json:"asset_id" bson:"asset_id"`
	InstanceID   string             `json:"instance_id" bson:"instance_id"`
	RequireType  enumor.RequireType `json:"require_type" bson:"require_type"`
	ResourceType ResourceType       `json:"resource_type" bson:"resource_type"`
	DeviceType   string             `json:"device_type" bson:"device_type"`
	ImageID      string             `json:"image_id" bson:"image_id"`
	Description  string             `json:"description" bson:"description"`
	Remark       string             `json:"remark" bson:"remark"`
	ZoneName     string             `json:"zone_name" bson:"zone_name"`
	ZoneID       int                `json:"zone_id" bson:"zone_id"`
	CloudZone    string             `json:"cloud_zone" bson:"cloud_zone"`
	// CloudRegion TODO 仅升降配有该字段，CVM生产返回的数据中没有该字段，待排查原因；影响交付后预测用量的判断
	CloudRegion       string    `json:"cloud_region" bson:"cloud_region"`
	ModuleName        string    `json:"module_name" bson:"module_name"`
	Equipment         string    `json:"rack_id" bson:"rack_id"`
	IsMatched         bool      `json:"is_matched" bson:"is_matched"`
	IsChecked         bool      `json:"is_checked" bson:"is_checked"`
	IsInited          bool      `json:"is_inited" bson:"is_inited"`
	IsDiskChecked     bool      `json:"is_disk_checked" bson:"is_disk_checked"`
	IsDelivered       bool      `json:"is_delivered" bson:"is_delivered"`
	Deliverer         string    `json:"deliverer" bson:"deliverer"`
	GenerateTaskId    string    `json:"generate_task_id" bson:"generate_task_id"`
	GenerateTaskLink  string    `json:"generate_task_link" bson:"generate_task_link"`
	InitTaskId        string    `json:"init_task_id" bson:"init_task_id"`
	InitTaskLink      string    `json:"init_task_link" bson:"init_task_link"`
	DiskCheckTaskId   string    `json:"disk_check_task_id" bson:"disk_check_task_id"`
	DiskCheckTaskLink string    `json:"disk_check_task_link" bson:"disk_check_task_link"`
	IsManualMatched   bool      `json:"is_manual_matched" bson:"is_manual_matched"` // 是否手工匹配
	OwnerIP           string    `json:"owner_ip" bson:"owner_ip"`                   // 所属的母机IP
	CreateAt          time.Time `json:"create_at" bson:"create_at"`
	UpdateAt          time.Time `json:"update_at" bson:"update_at"`
}

// ApplyTicket resource apply ticket
type ApplyTicket struct {
	OrderId      uint64             `json:"order_id" bson:"order_id"`
	ItsmTicketId string             `json:"itsm_ticket_id" bson:"itsm_ticket_id"`
	Stage        TicketStage        `json:"stage" bson:"stage"`
	BkBizId      int64              `json:"bk_biz_id" bson:"bk_biz_id"`
	User         string             `json:"bk_username" bson:"bk_username"`
	Follower     []string           `json:"follower" bson:"follower"`
	EnableNotice bool               `json:"enable_notice" bson:"enable_notice"`
	RequireType  enumor.RequireType `json:"require_type" bson:"require_type"`
	ExpectTime   string             `json:"expect_time" bson:"expect_time"`
	Remark       string             `json:"remark" bson:"remark"`
	Suborders    []*Suborder        `json:"suborders" bson:"suborders"`
	OldSuborders []*Suborder        `json:"old_suborders" bson:"old_suborders"`
	CreateAt     time.Time          `json:"create_at" bson:"create_at"`
	UpdateAt     time.Time          `json:"update_at" bson:"update_at"`
	// 生产类型(business:业务生产 admin:管理员生产)
	ProductType enumor.ProductType `json:"product_type"`
}

// TicketStage resource apply ticket stage（类型定义已下沉至 pkg/criteria/enumor/cvm_apply.go）
type TicketStage = enumor.TicketStage

// TicketStage resource apply ticket stage
const (
	TicketStageUncommit   = enumor.TicketStageUncommit
	TicketStageAudit      = enumor.TicketStageAudit
	TicketStageTerminate  = enumor.TicketStageTerminate
	TicketStageRunning    = enumor.TicketStageRunning
	TicketStageSuspend    = enumor.TicketStageSuspend
	TicketStageDone       = enumor.TicketStageDone
	TicketStageConfirming = enumor.TicketStageConfirming
)

// TicketStageEnums ticket stage enum
var TicketStageEnums = map[TicketStage]string{
	TicketStageUncommit:   "未提交",
	TicketStageAudit:      "待审核",
	TicketStageTerminate:  "终止",
	TicketStageRunning:    "备货中",
	TicketStageSuspend:    "备货异常",
	TicketStageDone:       "完成",
	TicketStageConfirming: "待确认(需求调整)",
}

// TicketStageOrder defines the order of ticket stages
var TicketStageOrder = []TicketStage{
	TicketStageUncommit,
	TicketStageAudit,
	TicketStageTerminate,
	TicketStageRunning,
	TicketStageSuspend,
	TicketStageDone,
	TicketStageConfirming,
}

// GetApplyTicketReq get apply ticket request parameter
type GetApplyTicketReq struct {
	OrderId uint64 `json:"order_id"`
	BkBizID int64  `json:"bk_biz_id"`
}

// Validate whether GetApplyTicketReq is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (req *GetApplyTicketReq) Validate() (errKey string, err error) {
	// TODO
	return "", nil
}

// GetApplyTicketRst get apply order result
type GetApplyTicketRst struct {
	*ApplyTicket `json:",inline"`
}

// ApplyAuditItsm resource apply ticket audit info
type ApplyAuditItsm struct {
	OrderId        uint64                `json:"order_id"`
	ItsmTicketId   string                `json:"itsm_ticket_id"`
	ItsmTicketLink string                `json:"itsm_ticket_link"`
	Status         string                `json:"status"`
	CurrentSteps   []*ApplyAuditItsmStep `json:"current_steps"`
	Logs           []*ApplyAuditItsmLog  `json:"logs"`
}

// ApplyAuditItsmStep resource apply ticket current audit step
type ApplyAuditItsmStep struct {
	Name           string          `json:"name"`
	Processors     []string        `json:"processors"`
	StateId        int64           `json:"state_id"`
	ProcessorsAuth map[string]bool `json:"processors_auth"`
}

// ApplyAuditItsmLog resource apply ticket audit log
type ApplyAuditItsmLog struct {
	Operator  string `json:"operator"`
	OperateAt string `json:"operate_at"`
	Message   string `json:"message"`
	Source    string `json:"source"`
}

// ApplyAuditCrp resource apply ticket audit info
type ApplyAuditCrp struct {
	CrpTicketId   string             `json:"crp_ticket_id"`
	CrpTicketLink string             `json:"crp_ticket_link"`
	Logs          []ApplyAuditCrpLog `json:"logs"`
	CurrentStep   ApplyAuditCrpStep  `json:"current_step"`
}

// ApplyAuditCrpLog resource apply ticket current audit step
type ApplyAuditCrpLog struct {
	TaskNo        int64  `json:"task_no"`
	TaskName      string `json:"task_name"`
	OperateResult string `json:"operate_result"`
	Operator      string `json:"operator"`
	OperateInfo   string `json:"operate_info"`
	OperateTime   string `json:"operate_time"`
}

// ApplyAuditCrpStep resource apply ticket current audit step
type ApplyAuditCrpStep struct {
	CurrentTaskNo    int                `json:"current_task_no"`
	CurrentTaskName  string             `json:"current_task_name"`
	Status           int                `json:"status"`
	StatusDesc       string             `json:"status_desc"`
	FailInstanceInfo []FailInstanceInfo `json:"fail_instance_info"`
}

// FailInstanceInfo resource apply ticket current audit step
type FailInstanceInfo struct {
	ErrorMsgTypeEn string `json:"error_msg_type_en"`
	ErrorType      string `json:"error_type"`
	ErrorMsgTypeCn string `json:"error_msg_type_cn"`
	RequestId      string `json:"request_id"`
	ErrorMsg       string `json:"error_msg"`
	Operator       string `json:"operator"`
	ErrorCount     int    `json:"error_count"`
}

// GetApplyAuditItsmReq get apply ticket audit info request parameter
type GetApplyAuditItsmReq struct {
	OrderId uint64 `json:"order_id" validate:"required"`
	BkBizID int64  `json:"bk_biz_id" validate:"required"`
}

// Validate GetApplyAuditItsmReq
func (req *GetApplyAuditItsmReq) Validate() (err error) {
	return validator.Validate.Struct(req)
}

// GetApplyAuditItsmRst get apply ticket audit info result
type GetApplyAuditItsmRst struct {
	*ApplyAuditItsm `json:",inline"`
}

// GetApplyAuditCrpReq get apply ticket audit info request parameter
type GetApplyAuditCrpReq struct {
	CrpTicketId string `json:"crp_ticket_id" validate:"required"`
	SuborderId  string `json:"suborder_id" validate:"required"`
}

// Validate GetApplyAuditCrpReq
func (req *GetApplyAuditCrpReq) Validate() (err error) {
	return validator.Validate.Struct(req)
}

// GetApplyAuditCrpRst get apply ticket audit info result
type GetApplyAuditCrpRst struct {
	*ApplyAuditCrp `json:",inline"`
}

// BizApplyAuditReq biz audit apply ticket request parameter
type BizApplyAuditReq struct {
	OrderId  uint64 `json:"order_id" validate:"required"`
	StateId  int64  `json:"state_id" validate:"required"`
	Approval bool   `json:"approval"`
	Remark   string `json:"remark"`
}

// Validate whether BizApplyAuditReq is valid
func (req *BizApplyAuditReq) Validate() (err error) {
	return validator.Validate.Struct(req)
}

// ResApplyAuditReq 资源下单审核请求参数
type ResApplyAuditReq = BizApplyAuditReq

// ApplyAuditReq audit apply ticket request parameter
type ApplyAuditReq struct {
	OrderId      uint64 `json:"order_id"`
	ItsmTicketId string `json:"itsm_ticket_id"`
	StateId      int64  `json:"state_id"`
	Operator     string `json:"operator"`
	Approval     bool   `json:"approval"`
	Remark       string `json:"remark"`
}

// Validate whether ApplyAuditReq is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (req *ApplyAuditReq) Validate() (errKey string, err error) {
	// TODO
	return "", nil
}

// ApproveApplyReq audit apply ticket request parameter
type ApproveApplyReq struct {
	OrderId  uint64 `json:"order_id"`
	Operator string `json:"operator"`
	Approval bool   `json:"approval"`
	Remark   string `json:"remark"`
}

// Validate whether ApproveApplyReq is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (req *ApproveApplyReq) Validate() (errKey string, err error) {
	// TODO
	return "", nil
}

// ApplyAutoAuditReq automatic audit apply ticket request parameter
type ApplyAutoAuditReq struct {
	OrderId uint64 `json:"order_id"`
}

// Validate whether ApplyAutoAuditReq is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (req *ApplyAutoAuditReq) Validate() (errKey string, err error) {
	// TODO
	return "", nil
}

// ApplyAutoAuditRst automatic audit apply ticket result
type ApplyAutoAuditRst struct {
	Operator string `json:"operator"`
	Approval int    `json:"approval"`
	Remark   string `json:"remark"`
}

// ApplyReq resource apply request
type ApplyReq struct {
	OrderId      uint64             `json:"order_id" bson:"order_id"`
	BkBizId      int64              `json:"bk_biz_id" bson:"bk_biz_id"`
	User         string             `json:"bk_username" bson:"bk_username"`
	Follower     []string           `json:"follower" bson:"follower"`
	EnableNotice bool               `json:"enable_notice" bson:"enable_notice"`
	RequireType  enumor.RequireType `json:"require_type" bson:"require_type"`
	ExpectTime   string             `json:"expect_time" bson:"expect_time"`
	Remark       string             `json:"remark" bson:"remark"`
	Suborders    []*Suborder        `json:"suborders" bson:"suborders"`
	OldSuborders *[]*Suborder       `json:"old_suborders" bson:"old_suborders"`
	ProductType  enumor.ProductType `json:"product_type" bson:"product_type"`
}

// Validate whether ApplyRequest is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (req *ApplyReq) Validate() error {
	if req.BkBizId <= 0 {
		return fmt.Errorf("invalid bk_biz_id <= 0")
	}

	if len(req.User) == 0 {
		return fmt.Errorf("bk_username cannot be empty")
	}

	if err := req.RequireType.Validate(); err != nil {
		return err
	}

	if _, err := time.Parse(datetimeLayout, req.ExpectTime); err != nil {
		return fmt.Errorf("expect_time should be in format like \"%s\"", datetimeLayout)
	}

	remarkLimit := 256
	if len(req.Remark) > remarkLimit {
		return fmt.Errorf("remark exceed size limit %d", remarkLimit)
	}

	if len(req.Suborders) <= 0 {
		return fmt.Errorf("suborders cannot be empty")
	}

	suborderLimit := 100
	if len(req.Suborders) > suborderLimit {
		return fmt.Errorf("suborders exceed max suborders %d", suborderLimit)
	}

	for _, suborder := range req.Suborders {
		if err := suborder.Validate(); err != nil {
			return err
		}
	}

	if req.RequireType == enumor.RequireTypeRollServer {
		if err := req.validateAsRollingServer(); err != nil {
			return err
		}
	}

	return nil
}

// validateAsRollingServer validate whether rolling server suborders are valid
func (req *ApplyReq) validateAsRollingServer() error {
	// 如果需求类型为滚服类型，那么必须传入继承的云主机实例ID
	for _, suborder := range req.Suborders {
		if suborder.Spec == nil {
			return fmt.Errorf("spec cannot be empty")
		}

		if len(suborder.Spec.InheritInstanceId) == 0 {
			return fmt.Errorf("inherit_instance_id cannot be empty")
		}
	}

	return nil
}

// Suborder resource apply suborder info
type Suborder struct {
	SuborderID        string                   `json:"suborder_id" bson:"suborder_id"`
	ResourceType      ResourceType             `json:"resource_type" bson:"resource_type"`
	Replicas          uint                     `json:"replicas" bson:"replicas"`
	AntiAffinityLevel string                   `json:"anti_affinity_level" bson:"anti_affinity_level"`
	EnableDiskCheck   bool                     `json:"enable_disk_check" bson:"enable_disk_check"`
	Remark            string                   `json:"remark" bson:"remark"`
	Source            enumor.ApplyTicketSource `json:"source" bson:"source"`
	Spec              *ResourceSpec            `json:"spec" bson:"spec"`
	AppliedCore       uint                     `json:"applied_core" bson:"applied_core,omitempty"`
	// UpgradeCVMList cvm升降配列表
	UpgradeCVMList []*UpgradeCVMSpec `json:"upgrade_cvm_list" bson:"upgrade_cvm_list"`
}

// Validate whether Suborder is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (s *Suborder) Validate() error {
	if s == nil {
		return fmt.Errorf("suborder cannot be empty")
	}

	if util.InArray(s.ResourceType, AllResourceType) != true {
		return fmt.Errorf("unkown resource_type")
	}

	if s.Replicas <= 0 {
		return fmt.Errorf("invalid replicas <= 0")
	}
	// replicas limit 1000
	if s.Replicas > ApplyLimit {
		return fmt.Errorf("replicas exceed apply limit: %d", ApplyLimit)
	}

	remarkLimit := 256
	if len(s.Remark) > remarkLimit {
		return fmt.Errorf("remark exceed size limit %d", remarkLimit)
	}

	// 除了升降配之外，其他资源类型必须传入规格
	if s.ResourceType != ResourceTypeUpgradeCvm && s.Spec == nil {
		return fmt.Errorf("spec cannot be empty")
	}

	if s.Spec != nil {
		if err := s.Spec.Validate(s.ResourceType); err != nil {
			return err
		}
	}

	return nil
}

// ResourceSpec resource specifications
type ResourceSpec struct {
	Region      string          `json:"region" bson:"region"`
	Zone        string          `json:"zone" bson:"zone"`
	DeviceGroup string          `json:"device_group" bson:"device_group"` // 机型族
	DeviceSize  enumor.CoreType `json:"device_size" bson:"device_size"`   // 机型核心类型(小核心、中核心、大核心)
	CPUCore     int64           `json:"cpu_core"`                         // CPU核心数
	DeviceType  string          `json:"device_type" bson:"device_type"`
	ImageId     string          `json:"image_id" bson:"image_id"`
	Image       string          `json:"image" bson:"image"`
	DiskSize    int64           `json:"disk_size" bson:"disk_size"`
	DiskType    enumor.DiskType `json:"disk_type" bson:"disk_type"`
	NetworkType string          `json:"network_type" bson:"network_type"`
	Vpc         string          `json:"vpc" bson:"vpc"`
	Subnet      string          `json:"subnet" bson:"subnet"`
	OsType      string          `json:"os_type" bson:"os_type"`
	RaidType    string          `json:"raid_type" bson:"raid_type"`
	// 外网运营商: "电信","联通","移动","CAP"
	Isp string `json:"isp" bson:"isp"`
	// 数据盘挂载点
	MountPath   string `json:"mount_path" bson:"mount_path"`
	CpuProvider string `json:"cpu_provider" bson:"cpu_provider"`
	Kernel      string `json:"kernel" bson:"kernel"`
	// 计费模式(计费模式：PREPAID包年包月，POSTPAID_BY_HOUR按量计费，默认为：PREPAID)
	ChargeType cvmapi.ChargeType `json:"charge_type" bson:"charge_type"`
	// 计费时长，单位：月
	ChargeMonths uint `json:"charge_months" bson:"charge_months"`
	// 被继承云主机实例ID
	InheritInstanceId string `json:"inherit_instance_id" bson:"inherit_instance_id"`
	// 继承的固资号
	BkAssetID string `json:"bk_asset_id" bson:"bk_asset_id"`
	// 分区生产时报错的可用区ID列表
	FailedZoneIDs []string          `json:"failed_zone_ids" bson:"failed_zone_ids"`
	SystemDisk    enumor.DiskSpec   `json:"system_disk" bson:"system_disk"`
	DataDisk      []enumor.DiskSpec `json:"data_disk" bson:"data_disk"`
	Zones         []string          `json:"zones" bson:"zones"` //  多可用区
	// ResAssign 资源分配方式（1表示“有资源区域优先”、2表示“分Campus生产”）
	ResAssign enumor.ResAssign `json:"res_assign" bson:"res_assign"`
	// CPU超线程(0:默认 1:关闭 2:开启)
	CPUThreadSwitch enumor.CPUThreadSwitch `json:"cpu_thread_switch" bson:"cpu_thread_switch"`
}

// Validate whether ResourceSpec is valid
func (s *ResourceSpec) Validate(resType ResourceType) error {
	if s == nil {
		return fmt.Errorf("spec cannot be empty")
	}

	// 地域、可用区、VPC、子网校验
	if err := s.validateRegionZoneAndNetwork(); err != nil {
		return err
	}

	if len(s.DeviceType) == 0 {
		return fmt.Errorf("spec.device_type cannot be empty")
	}

	// 磁盘校验
	if err := s.ValidateDisk(); err != nil {
		return err
	}

	// 镜像ID校验
	if err := s.validateImageId(resType); err != nil {
		return err
	}

	// 计费模式校验
	if err := s.validateChargeType(); err != nil {
		return err
	}

	// CPU超线程参数校验
	if err := s.validateCPUThreadSwitch(); err != nil {
		return err
	}

	return nil
}

// validateImageId 校验镜像ID
func (s *ResourceSpec) validateImageId(resType ResourceType) error {
	switch resType {
	case ResourceTypeCvm:
		if len(s.ImageId) == 0 {
			return fmt.Errorf("spec.image_id cannot be empty")
		}
	}

	return nil
}

// validateCPUThreadSwitch 校验CPU超线程参数
func (s *ResourceSpec) validateCPUThreadSwitch() error {
	if s.CPUThreadSwitch == 0 {
		return nil
	}

	if err := s.CPUThreadSwitch.Validate(s.DeviceType); err != nil {
		return err
	}

	return nil
}

// validateChargeType 校验计费模式
func (s *ResourceSpec) validateChargeType() error {
	if len(s.ChargeType) == 0 {
		return nil
	}

	if err := s.ChargeType.Validate(); err != nil {
		return err
	}

	// 包年包月时，计费时长必传
	if s.ChargeType == cvmapi.ChargeTypePrePaid && s.ChargeMonths < 1 {
		return fmt.Errorf("spec.charge_months invalid value < 1")
	}

	return nil
}

// validateRegionZoneAndNetwork 校验地域、可用区、VPC、子网
func (s *ResourceSpec) validateRegionZoneAndNetwork() error {
	if len(s.Region) == 0 {
		return fmt.Errorf("spec.region cannot be empty")
	}

	if len(s.Vpc) > 0 && len(s.Subnet) == 0 {
		return fmt.Errorf("spec.subnet cannot be empty while vpc is set")
	}

	// 可用区校验
	if len(s.Zone) == 0 && len(s.Zones) == 0 {
		return fmt.Errorf("spec.zone or spec.zones cannot be empty")
	}

	// 如果是多可用区或"全部"可用区，那么Vpc和Subnet不能指定，必须为空
	if (len(s.Zones) == 1 && s.Zones[0] == cvmapi.CvmZoneAll) || len(s.Zones) > 1 {
		// 资源分配方式校验
		if err := s.ResAssign.Validate(); err != nil {
			return err
		}
		// VPC、子网校验
		if len(s.Vpc) > 0 || len(s.Subnet) > 0 {
			return fmt.Errorf("spec.vpc and spec.subnet cannot be set at multiple spec.zones num > 1")
		}
	}

	return nil
}

// ValidateDisk validate disk spec
func (s *ResourceSpec) ValidateDisk() error {
	// 兼容旧的数据盘校验
	if len(s.DataDisk) == 0 {
		if s.DiskSize < 0 {
			return fmt.Errorf("spec.disk_size invalid value < 0")
		}

		diskLimit := int64(constant.DataDiskMaxSize)
		if s.DiskSize > diskLimit {
			return fmt.Errorf("spec.disk_size exceed limit %d", diskLimit)
		}

		// 规格为 10 的倍数
		diskUnit := int64(constant.DataDiskMultiple)
		modDisk := s.DiskSize % diskUnit
		if modDisk != 0 {
			return fmt.Errorf("spec.disk_size must be in multiples of %d", diskUnit)
		}
	}

	// 系统盘类型校验
	if len(s.SystemDisk.DiskType) > 0 {
		if err := s.SystemDisk.Validate(); err != nil {
			return err
		}
		if s.SystemDisk.DiskSize < constant.SystemDiskMinSize || s.SystemDisk.DiskSize > constant.SystemDiskMaxSize {
			return fmt.Errorf("spec.system_disk_size invalid value, must be in range [%d, %d]",
				constant.SystemDiskMinSize, constant.SystemDiskMaxSize)
		}
		// 系统盘大小必须是50的倍数
		if s.SystemDisk.DiskSize%constant.SystemDiskMultiple != 0 {
			return fmt.Errorf("spec.system_disk_size must be a multiple of %d", constant.SystemDiskMultiple)
		}
	}

	// 数据盘类型校验
	dataDiskTotalNum := uint(0)
	for _, dd := range s.DataDisk {
		if err := dd.Validate(); err != nil {
			return err
		}
		if dd.DiskSize < constant.DataDiskMinSize || dd.DiskSize > constant.DataDiskMaxSize {
			return fmt.Errorf("spec.data_disk_size invalid value, must be in range [%d, %d]",
				constant.DataDiskMinSize, constant.DataDiskMaxSize)
		}
		// 数据盘大小必须是10的倍数
		if dd.DiskSize%constant.DataDiskMultiple != 0 {
			return fmt.Errorf("spec.data_disk_size must be a multiple of %d", constant.DataDiskMultiple)
		}
		dataDiskTotalNum += dd.DiskNum
	}
	// 数据盘总数量不能超过20块
	if dataDiskTotalNum < 0 || dataDiskTotalNum > constant.DataDiskTotalNum {
		return fmt.Errorf("spec.data_disk_total_num invalid value, must be in range [0, %d]", constant.DataDiskTotalNum)
	}

	return nil
}

// IsCVMSeparateCampus 是否分Campus申请单
func (s *ResourceSpec) IsCVMSeparateCampus() bool {
	if s.ResAssign == enumor.CampusResAssign || s.Zone == cvmapi.CvmSeparateCampus {
		return true
	}
	return false
}

// CreateApplyOrderResult result of create apply order
type CreateApplyOrderResult struct {
	OrderId uint64 `json:"order_id"`
}

// CheckApplyOrderResp 提单前只读校验结果。
// 用于区分"业务不通过"与"系统异常"：业务校验未通过时 Pass=false 并携带可读 Reason（HTTP 200），
// 系统异常（DB、CRP、预测服务等调用失败）则以 error 形式返回，不会落到该结构体。
type CheckApplyOrderResp struct {
	// Pass 是否通过提单前置校验。
	Pass bool `json:"pass"`
	// Reason 业务未通过的详细原因，供调用方（含大模型）提示用户调整规格或数量；通过时为空。
	Reason string `json:"reason"`
}

// UnifyOrderList list of unify order
type UnifyOrderList []*UnifyOrder

// Len returns list length
func (m UnifyOrderList) Len() int {
	return len(m)
}

// Swap swaps two items in the list
func (m UnifyOrderList) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

// Less compares two items
func (m UnifyOrderList) Less(i, j int) bool {
	return m[i].CreateAt.Before(m[j].CreateAt)
}

// UnifyOrder get apply order result object, including apply ticket and order
type UnifyOrder struct {
	OrderId           uint64                   `json:"order_id" bson:"order_id"`
	SubOrderId        string                   `json:"suborder_id" bson:"suborder_id"`
	BkBizId           int64                    `json:"bk_biz_id" bson:"bk_biz_id"`
	User              string                   `json:"bk_username" bson:"bk_username"`
	RequireType       enumor.RequireType       `json:"require_type" bson:"require_type"`
	ResourceType      ResourceType             `json:"resource_type" bson:"resource_type"`
	ExpectTime        string                   `json:"expect_time" bson:"expect_time"`
	Description       string                   `json:"description" bson:"description"`
	Remark            string                   `json:"remark" bson:"remark"`
	Spec              *ResourceSpec            `json:"spec" bson:"spec"`
	AntiAffinityLevel string                   `json:"anti_affinity_level" bson:"anti_affinity_level"`
	EnableDiskCheck   bool                     `json:"enable_disk_check" bson:"enable_disk_check"`
	Stage             TicketStage              `json:"stage" bson:"stage"`
	Status            ApplyStatus              `json:"status" bson:"status"`
	OriginNum         uint                     `json:"origin_num" bson:"origin_num"` // 原始需求总数量，不会修改
	TotalNum          uint                     `json:"total_num" bson:"total_num"`   // 需要交付的总数量，业务会修改
	SuccessNum        uint                     `json:"success_num" bson:"success_num"`
	PendingNum        uint                     `json:"pending_num" bson:"pending_num"`
	ProductNum        uint                     `json:"product_num" bson:"product_num"` // 实际生产成功的总数量
	ModifyTime        uint                     `json:"modify_time" bson:"modify_time"`
	Source            enumor.ApplyTicketSource `json:"source" bson:"source"`
	CreateAt          time.Time                `json:"create_at" bson:"create_at"`
	UpdateAt          time.Time                `json:"update_at" bson:"update_at"`
}

// GetApplyParam get apply order request parameter
type GetApplyParam struct {
	BkBizID     []int64                    `json:"bk_biz_id" bson:"bk_biz_id"`
	OrderID     []uint64                   `json:"order_id" bson:"order_id"`
	SuborderID  []string                   `json:"suborder_id" bson:"suborder_id"`
	User        []string                   `json:"bk_username" bson:"bk_username"`
	RequireType []int64                    `json:"require_type" bson:"require_type"`
	Stage       []TicketStage              `json:"stage" bson:"stage"`
	Start       string                     `json:"start" bson:"start"`
	End         string                     `json:"end" bson:"end"`
	Page        *core.BasePage             `json:"page" bson:"page"`
	GetProduct  bool                       `json:"get_product" bson:"get_product"` // 是否获取CVM生产数据
	Source      []enumor.ApplyTicketSource `json:"source" bson:"source"`
	// 生产类型：business业务生产，admin管理员生产
	ProductType []enumor.ProductType `json:"product_type" bson:"product_type"`
}

// Validate whether GetApplyParam is valid
func (param *GetApplyParam) Validate() error {
	arrayLimit := 20
	if len(param.BkBizID) == 0 {
		return fmt.Errorf("bk_biz_id is required")
	}
	if len(param.OrderID) > arrayLimit {
		return fmt.Errorf("order_id exceed limit %d", arrayLimit)
	}

	if len(param.SuborderID) > arrayLimit {
		return fmt.Errorf("suborder_id exceed limit %d", arrayLimit)
	}

	if len(param.User) > arrayLimit {
		return fmt.Errorf("bk_username exceed limit %d", arrayLimit)
	}

	if len(param.RequireType) > arrayLimit {
		return fmt.Errorf("require_type exceed limit %d", arrayLimit)
	}

	if len(param.Stage) > arrayLimit {
		return fmt.Errorf("stage exceed limit %d", arrayLimit)
	}

	if len(param.ProductType) > arrayLimit {
		return fmt.Errorf("product_type exceed limit %d", arrayLimit)
	}

	for _, pt := range param.ProductType {
		if err := pt.Validate(); err != nil {
			return err
		}
	}

	if param.Page == nil {
		return fmt.Errorf("page is required")
	}

	if err := param.Page.Validate(); err != nil {
		return err
	}

	return nil
}

const (
	dateLayout     = "2006-01-02"
	datetimeLayout = "2006-01-02 15:04:05"
	// OneDayDuration is one day duration
	OneDayDuration = time.Hour * 24
)

// GetFilter get mgo filter
func (param *GetApplyParam) GetFilter(isTicket bool) *filter.Expression {
	rules := make([]*filter.AtomRule, 0)
	rules = param.appendBaseRules(rules)
	if isTicket {
		rules = param.appendTicketStageRules(rules)
	} else {
		rules = param.appendOrderRules(rules)
	}
	rules = param.appendTimeRangeRules(rules)
	return tools.ExpressionAnd(rules...)
}

// appendBaseRules appends common base filter rules
func (param *GetApplyParam) appendBaseRules(rules []*filter.AtomRule) []*filter.AtomRule {
	if len(param.BkBizID) > 0 {
		rules = append(rules, tools.RuleIn("bk_biz_id", param.BkBizID))
	}
	if len(param.OrderID) > 0 {
		rules = append(rules, tools.RuleIn("order_id", param.OrderID))
	}
	if len(param.User) > 0 {
		rules = append(rules, tools.RuleIn("bk_username", param.User))
	}
	if len(param.RequireType) > 0 {
		rules = append(rules, tools.RuleIn("require_type", param.RequireType))
	}
	if len(param.ProductType) == 0 {
		rules = append(rules, tools.RuleNotEqual("product_type", enumor.ProductTypeAdmin))
	} else {
		rules = append(rules, tools.RuleIn("product_type", param.ProductType))
	}
	return rules
}

// appendTicketStageRules appends ticket stage filter rules (for ticket query)
func (param *GetApplyParam) appendTicketStageRules(rules []*filter.AtomRule) []*filter.AtomRule {
	// ApplyTicket is the main order table, query only valid ticket stages.
	ticketStageList := make([]TicketStage, 0)
	if util.InArray(TicketStageUncommit, param.Stage) || len(param.Stage) == 0 {
		ticketStageList = append(ticketStageList, TicketStageUncommit)
	}
	if util.InArray(TicketStageAudit, param.Stage) || len(param.Stage) == 0 {
		ticketStageList = append(ticketStageList, TicketStageAudit)
	}
	if util.InArray(TicketStageTerminate, param.Stage) || len(param.Stage) == 0 {
		ticketStageList = append(ticketStageList, TicketStageTerminate)
	}
	if util.InArray(TicketStageRunning, param.Stage) {
		ticketStageList = append(ticketStageList, TicketStageRunning)
	}
	if len(ticketStageList) == 0 {
		return rules
	}
	return append(rules, tools.RuleIn("stage", ticketStageList))
}

// ShouldQueryTicketList indicates whether ticket table should be queried in order list API.
func (param *GetApplyParam) ShouldQueryTicketList() bool {
	if param.OnlyQuerySubOrderList() {
		return false
	}

	if len(param.Stage) == 0 {
		return true
	}

	for _, stage := range param.Stage {
		if stage.ShouldQueryTicketList() {
			return true
		}
	}
	return false
}

// appendOrderRules appends order filter rules (for order query)
func (param *GetApplyParam) appendOrderRules(rules []*filter.AtomRule) []*filter.AtomRule {
	if len(param.SuborderID) > 0 {
		rules = append(rules, tools.RuleIn("suborder_id", param.SuborderID))
	}
	if len(param.Stage) > 0 {
		rules = append(rules, tools.RuleIn("stage", param.Stage))
	}
	if len(param.Source) == 0 {
		rules = append(rules, tools.RuleNotEqual("source", enumor.ApplyTicketSrcPurchaseToResPool))
	} else {
		rules = append(rules, tools.RuleIn("source", param.Source))
	}
	return rules
}

// appendTimeRangeRules appends time range filter rules
func (param *GetApplyParam) appendTimeRangeRules(rules []*filter.AtomRule) []*filter.AtomRule {
	if len(param.Start) != 0 {
		startTime, err := time.Parse(dateLayout, param.Start)
		if err == nil {
			rules = append(rules, tools.RuleGreaterThanEqual("created_at", startTime.Format(constant.TimeStdFormat)))
		}
	}
	if len(param.End) != 0 {
		endTime, err := time.Parse(dateLayout, param.End)
		if err == nil {
			// '%lte: 2006-01-02' means '%lt: 2006-01-03 00:00:00'
			rules = append(rules, tools.RuleLessThan("created_at",
				endTime.AddDate(0, 0, 1).Format(constant.TimeStdFormat)))
		}
	}
	return rules
}

// OnlyQuerySubOrderList only query suborder list
// CVM申请-单据列表接口跟单据详情是共用的，查询子单详情时，需要限定只查询子单表，不查询主单信息（因为主单表不存在子单ID字段）
func (param *GetApplyParam) OnlyQuerySubOrderList() bool {
	return len(param.SuborderID) > 0
}

// GetApplyOrderRst get apply order result
type GetApplyOrderRst struct {
	Count int64         `json:"count"`
	Info  []*UnifyOrder `json:"info"`
}

// GetBizApplyParam get business apply order request parameter
type GetBizApplyParam struct {
	BkBizID int64          `json:"bk_biz_id" bson:"bk_biz_id"`
	Start   string         `json:"start" bson:"start"`
	End     string         `json:"end" bson:"end"`
	Page    *core.BasePage `json:"page" bson:"page"`
}

// Validate whether GetApplyParam is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (param *GetBizApplyParam) Validate() (errKey string, err error) {
	if param.BkBizID <= 0 {
		return "bk_biz_id", errors.New("invalid bk_biz_id <= 0")
	}

	if param.Start != "" {
		if _, err := time.Parse(dateLayout, param.Start); err != nil {
			return "start", fmt.Errorf("start should be in format like \"%s\"", dateLayout)
		}
	}

	if param.End != "" {
		if _, err := time.Parse(dateLayout, param.End); err != nil {
			return "end", fmt.Errorf("end should be in format like \"%s\"", dateLayout)
		}
	}

	if param.Page == nil {
		return "page", errors.New("page is required")
	}

	if err = param.Page.Validate(); err != nil {
		return "page", err
	}

	if param.Page.Start < 0 {
		return "page.start", fmt.Errorf("invalid start < 0")
	}

	if param.Page.Limit <= 0 {
		return "page.limit", fmt.Errorf("invalid limit <= 0")
	}

	if param.Page.Limit > 100 {
		return "page.limit", fmt.Errorf("exceed limit 100")
	}

	return "", nil
}

// GetApplyDetailReq get apply order detail request
type GetApplyDetailReq struct {
	SuborderId string `json:"suborder_id"`
}

// GetApplyDetailRst get apply order detail result
type GetApplyDetailRst struct {
	Count int64        `json:"count"`
	Info  []*ApplyStep `json:"info"`
}

// GetApplyGenerateReq get apply order generate record request
type GetApplyGenerateReq struct {
	SuborderId string             `json:"suborder_id" validate:"required"`
	Filter     *filter.Expression `json:"filter"`
	Page       *core.BasePage     `json:"page"`
}

// Validate whether GetApplyGenerateReq is valid
func (req *GetApplyGenerateReq) Validate() error {
	if req.Filter == nil {
		return errf.New(errf.InvalidParameter, "filter is required")
	}

	if req.Page == nil {
		return errf.New(errf.InvalidParameter, "page is required")
	}

	if err := req.Page.Validate(); err != nil {
		return err
	}

	return validator.Validate.Struct(req)
}

// GetApplyGenerateRst get apply order generate record result
type GetApplyGenerateRst struct {
	Count int64             `json:"count"`
	Info  []*GenerateRecord `json:"info"`
}

// GetApplyInitReq get apply order init record request
type GetApplyInitReq struct {
	SuborderId string             `json:"suborder_id" validate:"required"`
	Filter     *filter.Expression `json:"filter"`
	Page       *core.BasePage     `json:"page"`
}

// Validate whether GetApplyInitReq is valid
func (req *GetApplyInitReq) Validate() error {
	if req.Filter == nil {
		return errf.New(errf.InvalidParameter, "filter is required")
	}

	if req.Page == nil {
		return errf.New(errf.InvalidParameter, "page is required")
	}

	if err := req.Page.Validate(); err != nil {
		return err
	}

	return validator.Validate.Struct(req)
}

// GetApplyInitRst get apply order init record result
type GetApplyInitRst struct {
	Count int64         `json:"count"`
	Info  []*InitRecord `json:"info"`
}

// GetApplyDeliverReq get apply order deliver record request
type GetApplyDeliverReq struct {
	SuborderId string             `json:"suborder_id" validate:"required"`
	Filter     *filter.Expression `json:"filter"`
	Page       *core.BasePage     `json:"page"`
}

// Validate whether GetApplyDeliverReq is valid
func (req *GetApplyDeliverReq) Validate() error {
	if req.Filter == nil {
		return errf.New(errf.InvalidParameter, "filter is required")
	}

	if req.Page == nil {
		return errf.New(errf.InvalidParameter, "page is required")
	}

	if err := req.Page.Validate(); err != nil {
		return err
	}

	return validator.Validate.Struct(req)
}

// GetApplyDeliverRst get apply order deliver record result
type GetApplyDeliverRst struct {
	Count int64            `json:"count"`
	Info  []*DeliverRecord `json:"info"`
}

// ApplyStep apply order detail step info
type ApplyStep struct {
	SubOrderId string         `json:"suborder_id" bson:"suborder_id"`
	StepId     int            `json:"step_id" bson:"step_id"`
	StepName   string         `json:"step_name" bson:"step_name"`
	Status     StepStatusType `json:"status" bson:"status"`
	Message    string         `json:"message" bson:"message"`
	TotalNum   uint           `json:"total_num" bson:"total_num"`
	SuccessNum uint           `json:"success_num" bson:"success_num"`
	FailedNum  uint           `json:"failed_num" bson:"failed_num"`
	RunningNum uint           `json:"running_num" bson:"running_num"`
	CreateAt   time.Time      `json:"create_at" bson:"create_at"`
	UpdateAt   time.Time      `json:"update_at" bson:"update_at"`
	StartAt    time.Time      `json:"start_at" bson:"start_at"`
	EndAt      time.Time      `json:"end_at" bson:"end_at"`
}

// StepIdType step id
type StepIdType int

// StepStatusType step status（类型定义已下沉至 pkg/criteria/enumor/cvm_apply.go）
type StepStatusType = enumor.StepStatusType

// StepIdType step id type
const (
	StepIdCommit      StepIdType = 1
	StepIdGenerate    StepIdType = 2
	StepIdInit        StepIdType = 3
	StepIdDiskCheck   StepIdType = 4
	StepIdDeliver     StepIdType = 5
	StepNameCommit    string     = "下单"
	StepNameGenerate  string     = "生产"
	StepNameInit      string     = "初始化"
	StepNameDiskCheck string     = "本地盘性能压测"
	StepNameDeliver   string     = "交付"

	StepStatusInit     = enumor.StepStatusInit
	StepStatusSuccess  = enumor.StepStatusSuccess
	StepStatusHandling = enumor.StepStatusHandling
	StepStatusFailed   = enumor.StepStatusFailed

	StepMsgInit     string = "init"
	StepMsgSuccess  string = "success"
	StepMsgHandling string = "handling"
)

// InitRecord apply order init record
type InitRecord struct {
	SubOrderId string         `json:"suborder_id" bson:"suborder_id"`
	Ip         string         `json:"ip" bson:"ip"`
	TaskId     string         `json:"task_id" bson:"task_id"`
	TaskLink   string         `json:"task_link" bson:"task_link"`
	Status     InitStepStatus `json:"status" bson:"status"`
	Message    string         `json:"message" bson:"message"`
	CreateAt   time.Time      `json:"create_at" bson:"create_at"`
	UpdateAt   time.Time      `json:"update_at" bson:"update_at"`
	StartAt    time.Time      `json:"start_at" bson:"start_at"`
	EndAt      time.Time      `json:"end_at" bson:"end_at"`
}

// InitStepStatus init step status（类型定义已下沉至 pkg/criteria/enumor/cvm_apply.go）
type InitStepStatus = enumor.InitStepStatus

// InitStepStatus init step status
const (
	InitStatusInit     = enumor.InitStatusInit
	InitStatusSuccess  = enumor.InitStatusSuccess
	InitStatusHandling = enumor.InitStatusHandling
	InitStatusFailed   = enumor.InitStatusFailed
)

// DeliverRecord apply order deliver record
type DeliverRecord struct {
	SubOrderId       string            `json:"suborder_id" bson:"suborder_id"`
	Ip               string            `json:"ip" bson:"ip"`
	AssetId          string            `json:"asset_id" bson:"asset_id"`
	Status           DeliverStepStatus `json:"status" bson:"status"`
	Message          string            `json:"message" bson:"message"`
	Deliverer        string            `json:"deliverer" bson:"deliverer"`
	GenerateTaskId   string            `json:"generate_task_id" bson:"generate_task_id"`
	GenerateTaskLink string            `json:"generate_task_link" bson:"generate_task_link"`
	InitTaskId       string            `json:"init_task_id" bson:"init_task_id"`
	InitTaskLink     string            `json:"init_task_link" bson:"init_task_link"`
	IsManualMatched  bool              `json:"is_manual_matched" bson:"is_manual_matched"` // 是否手工匹配
	CreateAt         time.Time         `json:"create_at" bson:"create_at"`
	UpdateAt         time.Time         `json:"update_at" bson:"update_at"`
	StartAt          time.Time         `json:"start_at" bson:"start_at"`
	EndAt            time.Time         `json:"end_at" bson:"end_at"`
}

// DeliverStepStatus deliver step status（类型定义已下沉至 pkg/criteria/enumor/cvm_apply.go）
type DeliverStepStatus = enumor.DeliverStepStatus

// DeliverStepStatus deliver step status
const (
	DeliverStatusInit     = enumor.DeliverStatusInit
	DeliverStatusSuccess  = enumor.DeliverStatusSuccess
	DeliverStatusHandling = enumor.DeliverStatusHandling
	DeliverStatusFailed   = enumor.DeliverStatusFailed
)

// StartApplyOrderReq start apply order request
type StartApplyOrderReq struct {
	SuborderID []string `json:"suborder_id"`
}

// Validate whether StartApplyOrderReq is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (param *StartApplyOrderReq) Validate() error {
	if len(param.SuborderID) == 0 {
		return fmt.Errorf("suborder_id should be set")
	}

	for _, subOrderID := range param.SuborderID {
		if len(subOrderID) == 0 {
			return fmt.Errorf("suborder_id should not be empty")
		}
	}

	arrayLimit := 20

	if len(param.SuborderID) > arrayLimit {
		return fmt.Errorf("suborder_id exceed limit %d", arrayLimit)
	}

	return nil
}

// TerminateApplyOrderReq terminate apply order request
type TerminateApplyOrderReq struct {
	SuborderID []string `json:"suborder_id"`
}

// Validate whether TerminateApplyOrderReq is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (param *TerminateApplyOrderReq) Validate() error {
	if len(param.SuborderID) == 0 {
		return fmt.Errorf("suborder_id should be set")
	}

	for _, subOrderID := range param.SuborderID {
		if len(subOrderID) == 0 {
			return fmt.Errorf("suborder_id is not empty")
		}
	}

	arrayLimit := 20

	if len(param.SuborderID) > arrayLimit {
		return fmt.Errorf("suborder_id exceed limit %d", arrayLimit)
	}

	return nil
}

// ModifyApplyReq modify apply order request
type ModifyApplyReq struct {
	SuborderID string        `json:"suborder_id" bson:"suborder_id"`
	User       string        `json:"bk_username" bson:"bk_username"`
	Replicas   uint          `json:"replicas" bson:"replicas"` // 剩余生产数量，不是总数量
	TotalNum   uint          `json:"-"`                        // 需要交付的总数量
	ProductNum uint          `json:"-"`                        // 已生产成功的总数量
	Spec       *ResourceSpec `json:"spec" bson:"spec"`
}

// Validate whether ModifyApplyReq is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (param *ModifyApplyReq) Validate() error {
	if len(param.SuborderID) == 0 {
		return fmt.Errorf("suborder_id should be set")
	}

	if param.Spec == nil {
		return fmt.Errorf("spec cannot be empty")
	}

	if err := param.Spec.Validate(ResourceTypeCvm); err != nil {
		return err
	}

	return nil
}

// RecommendApplyReq get apply order modification recommendation request
type RecommendApplyReq struct {
	SuborderID string `json:"suborder_id" bson:"suborder_id"`
}

// Validate whether RecommendApplyReq is valid
// errKey: invalid key
// err: detail reason why errKey is invalid
func (param *RecommendApplyReq) Validate() (errKey string, err error) {
	if len(param.SuborderID) == 0 {
		return "suborder_id", fmt.Errorf("suborder_id should be set")
	}

	return "", nil
}

// RecommendApplyRst get apply order modification recommendation result
type RecommendApplyRst struct {
	SuborderID string        `json:"suborder_id" bson:"suborder_id"`
	Replicas   uint          `json:"replicas" bson:"replicas"`
	Spec       *ResourceSpec `json:"spec" bson:"spec"`
}

// GetApplyModifyReq get apply order modify record request
type GetApplyModifyReq struct {
	ID         []string                       `json:"id"`
	SuborderID []string                       `json:"suborder_id"`
	Status     []enumor.CvmModifyRecordStatus `json:"status"`
	Page       *core.BasePage                 `json:"page"`
}

// Validate whether GetApplyModifyReq is valid
func (param *GetApplyModifyReq) Validate() error {
	if param.Page == nil {
		return fmt.Errorf("page is required")
	}

	if err := param.Page.Validate(); err != nil {
		return err
	}

	if param.Page.Start < 0 {
		return fmt.Errorf("page.start invalid start < 0")
	}

	if param.Page.Limit < 0 {
		return fmt.Errorf("page.limit invalid limit < 0")
	}

	if param.Page.Limit > 500 {
		return fmt.Errorf("page.limit exceed limit 500")
	}

	return nil
}

// GetFilter get filter for MySQL/DataService
func (param *GetApplyModifyReq) GetFilter() *filter.Expression {
	rules := make([]*filter.AtomRule, 0)
	if len(param.ID) > 0 {
		rules = append(rules, tools.RuleIn("id", param.ID))
	}
	if len(param.SuborderID) > 0 {
		rules = append(rules, tools.RuleIn("suborder_id", param.SuborderID))
	}
	if len(param.Status) > 0 {
		rules = append(rules, tools.RuleIn("status", param.Status))
	}
	if len(rules) == 0 {
		return tools.AllExpression()
	}
	return tools.ExpressionAnd(rules...)
}

// GetApplyModifyRst get apply order modify record result
type GetApplyModifyRst struct {
	Count int64                 `json:"count"`
	Info  []*table.ModifyRecord `json:"info"`
}

// CheckInheritedHostReq check inherited host request
type CheckInheritedHostReq struct {
	AssetID     string             `json:"bk_asset_id" validate:"required"`
	BizID       int64              `json:"bk_biz_id"`
	Region      string             `json:"region" validate:"required"`
	RequireType enumor.RequireType `json:"require_type" validate:"required"`
}

// Validate CheckInheritedHostReq
func (c *CheckInheritedHostReq) Validate() error {
	return validator.Validate.Struct(c)
}

// CheckInheritedHostResp check inherited host response
type CheckInheritedHostResp struct {
	DeviceType           string    `json:"device_type"`
	DeviceGroup          string    `json:"device_group"`
	GenerationType       string    `json:"generation_type"`
	InstanceChargeType   string    `json:"instance_charge_type"`
	ChargeMonths         int       `json:"charge_months"`
	BillingStartTime     time.Time `json:"billing_start_time"`
	OldBillingExpireTime time.Time `json:"old_billing_expire_time"`
	NewBillingExpireTime time.Time `json:"new_billing_expire_time"`
	CloudInstID          string    `json:"bk_cloud_inst_id"`
}

// CancelApplyTicketItsmReq cancel apply ticket crp request
type CancelApplyTicketItsmReq struct {
	OrderID int64 `json:"order_id" validate:"required"`
}

// Validate CancelApplyTicketItsmReq
func (c *CancelApplyTicketItsmReq) Validate() error {
	return validator.Validate.Struct(c)
}

// CancelApplyTicketCrpReq cancel apply ticket crp request
type CancelApplyTicketCrpReq struct {
	SubOrderID string `json:"suborder_id" validate:"required"`
}

// Validate CancelApplyTicketCrpReq
func (c *CancelApplyTicketCrpReq) Validate() error {
	return validator.Validate.Struct(c)
}

// DeviceInitMsg device init msg
type DeviceInitMsg struct {
	Device *DeviceInfo
	JobUrl string
	JobID  string
	BizID  int64
}

// ConfirmApplyModifyCompare confirm apply modify compare
type ConfirmApplyModifyCompare struct {
	PreDeviceType string `json:"pre_device_type"`
	PreZone       string `json:"pre_zone"`
	PreNum        string `json:"pre_num"`
	PreVpc        string `json:"pre_vpc"`
	PreSubnet     string `json:"pre_subnet"`
	CurDeviceType string `json:"cur_device_type"`
	CurZone       string `json:"cur_zone"`
	CurNum        string `json:"cur_num"`
	CurVpc        string `json:"cur_vpc"`
	CurSubnet     string `json:"cur_subnet"`
}

// ConfirmApplyModifyReq confirm apply modify request
type ConfirmApplyModifyReq struct {
	BkUsername                                      string `json:"bk_username" validate:"required"`
	bkbotapproval.CvmApplyModifyConfirmCallbackData `json:",inline"`
}

// Validate validate
func (c *ConfirmApplyModifyReq) Validate() error {
	return validator.Validate.Struct(c)
}

// ConfirmApplyModifyResp confirm modify response
type ConfirmApplyModifyResp struct {
	ResponseMsg   string                    `json:"response_msg"`   // 点击按钮后回显的信息
	ResponseColor bkbotapproval.ButtonColor `json:"response_color"` // 点击按钮后回显的颜色
	RequestID     string                    `json:"request_id"`
}

// ListApplyAuditInfoReq list apply audit info request
type ListApplyAuditInfoReq struct {
	TicketIDs []uint64 `json:"ticket_ids" validate:"required,min=1,max=100"`
}

// Validate ...
func (l *ListApplyAuditInfoReq) Validate() error {
	return validator.Validate.Struct(l)
}

// ListApplyAuditInfoResp list apply audit info response
type ListApplyAuditInfoResp struct {
	Details []ListApplyAuditInfo `json:"details"`
}

// ListApplyAuditInfo list apply audit info
type ListApplyAuditInfo struct {
	TicketID     uint64                `json:"ticket_id"`
	Status       itsm.Status           `json:"status"`
	CurrentSteps []*ApplyAuditItsmStep `json:"current_steps"`
	TicketInfo   *GetApplyTicketRst    `json:"ticket_info"`
	EndAt        *time.Time            `json:"end_at,omitempty"`
}

// ApproveApplyTicketNodeReq audit apply ticket node request parameter
type ApproveApplyTicketNodeReq struct {
	TicketID uint64 `json:"ticket_id" validate:"required"`
	StateID  int64  `json:"state_id" validate:"required"`
	Operator string `json:"operator" validate:"required"`
	Approval *bool  `json:"approval" validate:"required"`
	Remark   string `json:"remark"`
}

// Validate ...
func (a *ApproveApplyTicketNodeReq) Validate() error {
	return validator.Validate.Struct(a)
}

// FindApproveNodeResultReq find approve node result request
type FindApproveNodeResultReq struct {
	TicketID uint64 `json:"ticket_id" validate:"required"`
	StateID  int64  `json:"state_id" validate:"required"`
}

// Validate ...
func (g *FindApproveNodeResultReq) Validate() error {
	return validator.Validate.Struct(g)
}

// ListHostApplyItsmTicketReq defines the hcm list host apply itsm ticket request
type ListHostApplyItsmTicketReq struct {
	CreateTime *time.Time `json:"create_time" validate:"required"`
}

// Validate ...
func (l *ListHostApplyItsmTicketReq) Validate() error {
	return validator.Validate.Struct(l)
}

// ListHostApplyItsmTicketData defines the hcm list host apply itsm ticket data
type ListHostApplyItsmTicketData struct {
	Tickets []HostApplyItsmTicket `json:"tickets"`
}

// HostApplyItsmTicket defines the hcm host apply itsm ticket
type HostApplyItsmTicket struct {
	ID            string               `json:"id"`
	Url           string               `json:"url"`
	User          string               `json:"user"`
	ApprovalState enumor.ApprovalState `json:"approval_state"`
	CreateTime    time.Time            `json:"create_time"`
}

// ListHostApplyCrpTicketReq defines the hcm list host apply crp ticket request
type ListHostApplyCrpTicketReq struct {
	CreateTime *time.Time `json:"create_time" validate:"required"`
}

// Validate ...
func (l *ListHostApplyCrpTicketReq) Validate() error {
	return validator.Validate.Struct(l)
}

// ListHostApplyCrpTicketData defines the hcm list host apply crp ticket data
type ListHostApplyCrpTicketData struct {
	Tickets []HostApplyCrpTicket `json:"tickets"`
}

// HostApplyCrpTicket defines the hcm host apply crp ticket
type HostApplyCrpTicket struct {
	ID            string               `json:"id"`
	Url           string               `json:"url"`
	User          string               `json:"user"`
	ApprovalState enumor.ApprovalState `json:"approval_state"`
	CreateTime    time.Time            `json:"create_time"`
}

// UpdateApplyTicketDemandReq update apply ticket demand request
type UpdateApplyTicketDemandReq struct {
	TicketID  uint64      `json:"ticket_id" validate:"required"`
	Suborders []*Suborder `json:"suborders" validate:"required"`
}

// Validate validate update apply ticket demand request
func (u *UpdateApplyTicketDemandReq) Validate() error {
	if err := validator.Validate.Struct(u); err != nil {
		return err
	}

	for _, suborder := range u.Suborders {
		if err := suborder.Validate(); err != nil {
			return err
		}
	}

	return nil
}
