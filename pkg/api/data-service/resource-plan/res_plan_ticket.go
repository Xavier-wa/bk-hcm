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

package resourceplan

import (
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table/types"
)

// ResPlanTicketUpdateReq is resource plan ticket update request.
type ResPlanTicketUpdateReq struct {
	Remark           string             `json:"remark" validate:"omitempty"`
	DemandClass      enumor.DemandClass `json:"demand_class" validate:"omitempty"`
	Demands          *types.JsonField   `json:"demands" validate:"omitempty"`
	SubmittedAt      string             `json:"submitted_at" validate:"omitempty"`
	OriginalOS       float64            `json:"original_os" validate:"omitempty"`
	OriginalCPUCore  int64              `json:"original_cpu_core" validate:"omitempty"`
	OriginalMemory   int64              `json:"original_memory" validate:"omitempty"`
	OriginalDiskSize int64              `json:"original_disk_size" validate:"omitempty"`
	UpdatedOS        float64            `json:"updated_os" validate:"omitempty"`
	UpdatedCPUCore   int64              `json:"updated_cpu_core" validate:"omitempty"`
	UpdatedMemory    int64              `json:"updated_memory" validate:"omitempty"`
	UpdatedDiskSize  int64              `json:"updated_disk_size" validate:"omitempty"`
}

// Validate validates ResPlanTicketUpdateReq.
func (r *ResPlanTicketUpdateReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	if r.DemandClass != "" {
		if err := r.DemandClass.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// OverwriteResPlanTicketReq is overwrite resource plan ticket request.
type OverwriteResPlanTicketReq struct {
	TicketID string                 `json:"ticket_id" validate:"required"`
	Ticket   ResPlanTicketUpdateReq `json:"ticket" validate:"required"`
}

// Validate validates OverwriteResPlanTicketReq.
func (r *OverwriteResPlanTicketReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	return r.Ticket.Validate()
}
