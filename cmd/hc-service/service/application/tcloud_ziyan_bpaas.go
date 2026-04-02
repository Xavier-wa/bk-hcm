package application

import (
	cloudadaptor "hcm/cmd/hc-service/logics/cloud-adaptor"
	ressync "hcm/cmd/hc-service/logics/res-sync"
	"hcm/cmd/hc-service/service/capability"
	coreziyan "hcm/pkg/api/core/ziyan"
	hcservice "hcm/pkg/api/hc-service"
	"hcm/pkg/client"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/json"
)

// InitApplicationService initial the application service
func InitApplicationService(cap *capability.Capability) {
	a := &application{
		ad:      cap.CloudAdaptor,
		cs:      cap.ClientSet,
		syncCli: cap.ResSyncCli,
	}

	h := rest.NewHandler()

	h.Add("QueryBPaasApplicationDetail", "POST",
		"/vendors/tcloud-ziyan/application/bpaas/query", a.QueryBPaasApplicationDetail)
	h.Add("DeliverBPaasApplication", "POST",
		"/vendors/tcloud-ziyan/application/bpaas/deliver", a.DeliverBPaasApplication)

	h.Load(cap.WebService)
}

type application struct {
	ad      *cloudadaptor.CloudAdaptorClient
	cs      *client.ClientSet
	syncCli ressync.Interface
}

// QueryBPaasApplicationDetail ...
func (a *application) QueryBPaasApplicationDetail(cts *rest.Contexts) (any, error) {
	req := new(hcservice.GetBPaasApplicationReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	ziyan, err := a.ad.TCloudZiyan(cts.Kit, req.AccountID)
	if err != nil {
		return nil, err
	}
	bpaasDetail, err := ziyan.GetBPaasApplicationDetail(cts.Kit, req.BPaasSN)
	if err != nil {
		logs.Errorf("fail to get bpaas application detail, err: %v, application id: %v, rid: %s",
			err, req.BPaasSN, cts.Kit.Rid)
		return nil, err
	}
	return bpaasDetail, nil
}

// DeliverBPaasApplication BPaaS审批通过后，根据 action 类型分发到对应 handler 执行交付补偿
func (a *application) DeliverBPaasApplication(cts *rest.Contexts) (any, error) {
	req := new(hcservice.DeliverBPaasApplicationReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	content := new(coreziyan.BPaasApplicationContent)
	if err := json.UnmarshalFromString(req.Content, content); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	handler := GetBPaasDeliverHandler(content.Action, content, a.syncCli)
	if handler == nil {
		logs.Warnf("no bpaas deliver handler for action: %s, skip, rid: %s", content.Action, cts.Kit.Rid)
		return nil, nil
	}

	if err := handler.Deliver(cts.Kit); err != nil {
		logs.Errorf("bpaas deliver failed, action: %s, account: %s, err: %v, rid: %s",
			content.Action, content.AccountID, err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}
