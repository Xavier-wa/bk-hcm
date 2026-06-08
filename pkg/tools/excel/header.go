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

package excel

import (
	"reflect"

	"hcm/pkg/criteria/errf"
)

// TicketExportRowHeaders extracts ordered column headers from struct fields with an excel tag.
// The object parameter must be a struct or a non-nil pointer to struct.
func TicketExportRowHeaders(object any) ([]interface{}, error) {
	if object == nil {
		return nil, errf.New(errf.InvalidParameter, "object is nil")
	}

	t := reflect.TypeOf(object)
	if t.Kind() == reflect.Ptr {
		if reflect.ValueOf(object).IsNil() {
			return nil, errf.New(errf.InvalidParameter, "object is nil pointer")
		}
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, errf.Newf(errf.InvalidParameter, "object must be a struct, got %s", t.Kind())
	}

	headers := make([]interface{}, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		if tag := t.Field(i).Tag.Get("excel"); tag != "" {
			headers = append(headers, tag)
		}
	}
	return headers, nil
}
