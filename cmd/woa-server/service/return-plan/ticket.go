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

package returnplan

import (
	"errors"

	rptypes "hcm/cmd/woa-server/types/return-plan"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// ListBizReturnPlanTicket list biz return plan tickets.
func (s *service) ListBizReturnPlanTicket(cts *rest.Contexts) (interface{}, error) {
	bkBizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	req := new(rptypes.ListReturnPlanTicketReq)
	if err = cts.DecodeInto(req); err != nil {
		logs.Errorf("decode list biz return plan ticket request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	req.BkBizIDs = []int64{bkBizID}

	if err = req.Validate(); err != nil {
		logs.Errorf("validate list biz return plan ticket request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.Biz, Action: meta.Access}, BizID: bkBizID}
	if err = s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		return nil, err
	}

	opt, err := req.GenListOption()
	if err != nil {
		logs.Errorf("gen list return plan ticket option failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return s.returnPlanController.Fetch().ListReturnPlanTicket(cts.Kit, opt)
}

// GetBizReturnPlanTicket get biz return plan ticket detail.
func (s *service) GetBizReturnPlanTicket(cts *rest.Contexts) (interface{}, error) {
	bkBizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	ticketID := cts.PathParameter("id").String()
	if len(ticketID) == 0 {
		return nil, errf.NewFromErr(errf.InvalidParameter, errors.New("ticket id can not be empty"))
	}

	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.Biz, Action: meta.Access}, BizID: bkBizID}
	if err = s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		return nil, err
	}

	return s.returnPlanController.Fetch().GetReturnPlanTicket(cts.Kit, bkBizID, ticketID)
}

// ListBizReturnPlanSubTicket list biz return plan sub tickets.
func (s *service) ListBizReturnPlanSubTicket(cts *rest.Contexts) (interface{}, error) {
	bkBizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	req := new(rptypes.ListReturnPlanSubTicketReq)
	if err = cts.DecodeInto(req); err != nil {
		logs.Errorf("decode list biz return plan sub ticket request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err = req.Validate(); err != nil {
		logs.Errorf("validate list biz return plan sub ticket request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	req.BkBizID = bkBizID

	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.Biz, Action: meta.Access}, BizID: bkBizID}
	if err = s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		return nil, err
	}

	return s.returnPlanController.Fetch().ListReturnPlanSubTicket(cts.Kit, req)
}

// GetBizReturnPlanSubTicket get biz return plan sub ticket detail.
func (s *service) GetBizReturnPlanSubTicket(cts *rest.Contexts) (interface{}, error) {
	bkBizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	subTicketID := cts.PathParameter("id").String()
	if len(subTicketID) == 0 {
		return nil, errf.NewFromErr(errf.InvalidParameter, errors.New("sub ticket id can not be empty"))
	}

	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.Biz, Action: meta.Access}, BizID: bkBizID}
	if err = s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		return nil, err
	}

	return s.returnPlanController.Fetch().GetReturnPlanSubTicketDetail(cts.Kit, bkBizID, subTicketID)
}

// RetryBizReturnPlanTicket retry failed return plan sub tickets.
func (s *service) RetryBizReturnPlanTicket(cts *rest.Contexts) (any, error) {
	bkBizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	ticketID := cts.PathParameter("ticket_id").String()
	if len(ticketID) == 0 {
		return nil, errf.NewFromErr(errf.InvalidParameter, errors.New("ticket id can not be empty"))
	}

	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.ReturnPlan, Action: meta.Update}, BizID: bkBizID}
	if err = s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		return nil, err
	}

	if _, err = s.returnPlanController.Fetch().GetReturnPlanTicket(cts.Kit, bkBizID, ticketID); err != nil {
		logs.Errorf("verify return plan ticket before retry failed, err: %v, ticket_id: %s, rid: %s",
			err, ticketID, cts.Kit.Rid)
		return nil, err
	}

	if err = s.returnPlanController.RetryReturnPlanFailedSubTickets(cts.Kit, ticketID); err != nil {
		logs.Errorf("retry return plan ticket failed, err: %v, ticket_id: %s, rid: %s", err, ticketID, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

// TerminateBizReturnPlanTicket terminate a failed return plan ticket locally.
func (s *service) TerminateBizReturnPlanTicket(cts *rest.Contexts) (any, error) {
	bkBizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	ticketID := cts.PathParameter("ticket_id").String()
	if len(ticketID) == 0 {
		return nil, errf.NewFromErr(errf.InvalidParameter, errors.New("ticket id can not be empty"))
	}

	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.ReturnPlan, Action: meta.Update}, BizID: bkBizID}
	if err = s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		return nil, err
	}

	if _, err = s.returnPlanController.Fetch().GetReturnPlanTicket(cts.Kit, bkBizID, ticketID); err != nil {
		logs.Errorf("verify return plan ticket before terminate failed, err: %v, ticket_id: %s, rid: %s",
			err, ticketID, cts.Kit.Rid)
		return nil, err
	}

	if err = s.returnPlanController.TerminateReturnPlanFailedTicket(cts.Kit, ticketID); err != nil {
		logs.Errorf("terminate return plan ticket failed, err: %v, ticket_id: %s, rid: %s", err, ticketID, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

// OverwriteAppendBizReturnPlanTicket overwrite-append biz return plan ticket.
func (s *service) OverwriteAppendBizReturnPlanTicket(cts *rest.Contexts) (interface{}, error) {
	bkBizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	req := new(rptypes.OverwriteAppendReturnPlanTicketReq)
	if err = cts.DecodeInto(req); err != nil {
		logs.Errorf("decode overwrite-append biz return plan ticket request failed, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err = req.Validate(); err != nil {
		logs.Errorf("validate overwrite-append biz return plan ticket request failed, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.ReturnPlan, Action: meta.Create}, BizID: bkBizID}
	if err = s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		return nil, err
	}

	ticketID, err := s.returnPlanController.OverwriteAppendReturnPlanTicket(cts.Kit, bkBizID, req)
	if err != nil {
		logs.Errorf("overwrite-append biz return plan ticket failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &rptypes.OverwriteAppendReturnPlanTicketResp{ID: ticketID}, nil
}

// ListBizReturnReasonClass list return reason classes by obs project.
func (s *service) ListBizReturnReasonClass(cts *rest.Contexts) (interface{}, error) {
	bkBizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	req := new(rptypes.ListReturnReasonClassReq)
	if err = cts.DecodeInto(req); err != nil {
		logs.Errorf("decode list return reason class request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err = req.Validate(); err != nil {
		logs.Errorf("validate list return reason class request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.Biz, Action: meta.Access}, BizID: bkBizID}
	if err = s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		return nil, err
	}

	return s.returnPlanController.ListReturnReasonClass(cts.Kit, req)
}
