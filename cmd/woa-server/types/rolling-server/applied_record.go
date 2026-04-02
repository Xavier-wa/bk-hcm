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

package rollingserver

import "hcm/pkg/criteria/enumor"

// AppliedRecordDate applied record date
type AppliedRecordDate struct {
	Year  int
	Month int
	Day   int
}

// UnReturnedSubOrderMsg unreturned sub order msg
type UnReturnedSubOrderMsg struct {
	SubOrderID           string
	AppliedCore          int64
	ReturnedCore         int64
	ExemptedReturnedCore int64
	AppliedUser          string
	AppliedYear          int
	AppliedMonth         int
	AppliedDay           int
	NeedReturnedYear     int
	NeedReturnedMonth    int
	NeedReturnedDay      int
	FineState            enumor.RsUnReturnedSubOrderFineState
}
