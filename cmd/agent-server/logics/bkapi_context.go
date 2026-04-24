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

package logics

import (
	"context"
	"fmt"

	"hcm/pkg/criteria/constant"
)

// WithBKUsername returns a new context carrying the given BK username.
func WithBKUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, constant.UserKey, username)
}

// WithBKTicket returns a new context carrying the given BK ticket.
func WithBKTicket(ctx context.Context, ticket string) context.Context {
	return context.WithValue(ctx, constant.BKTicket, ticket)
}

// BKUsernameFromContext extracts the BK username stored in ctx.
// Returns an empty string if not set.
func BKUsernameFromContext(ctx context.Context) string {
	v, _ := ctx.Value(constant.UserKey).(string)
	return v
}

// BKTicketFromContext extracts the BK ticket stored in ctx.
// Returns an empty string if not set.
func BKTicketFromContext(ctx context.Context) string {
	v, _ := ctx.Value(constant.BKTicket).(string)
	return v
}

// bkapiAuthHeaderValue builds the JSON value for the X-Bkapi-Authorization header.
func bkapiAuthHeaderValue(appCode, appSecret, username, ticket string) string {
	return fmt.Sprintf(
		`{"bk_app_code":"%s","bk_app_secret":"%s","bk_username":"%s","bk_ticket":"%s"}`,
		appCode, appSecret, username, ticket,
	)
}

// bkapiMCPAuthHeaderValue builds the X-Bkapi-Authorization header value for MCP requests.
func bkapiMCPAuthHeaderValue(appCode, appSecret, username, ticket string) string {
	return fmt.Sprintf(
		`{"bk_app_code":"%s","bk_app_secret":"%s","bk_username":"%s","bk_ticket":"%s"}`,
		appCode, appSecret, username, ticket,
	)
}
