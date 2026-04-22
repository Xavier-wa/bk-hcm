package hcziyancli

import (
	"encoding/json"

	hcservice "hcm/pkg/api/hc-service"
	"hcm/pkg/client/common"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
)

// ApplicationClient is hc service bpaas api client.
type ApplicationClient struct {
	client rest.ClientInterface
}

// NewApplicationClient create a new account api client.
func NewApplicationClient(client rest.ClientInterface) *ApplicationClient {
	return &ApplicationClient{
		client: client,
	}
}

// QueryBPaasApplicationDetail 查询bpaas申请单详情
func (a *ApplicationClient) QueryBPaasApplicationDetail(kt *kit.Kit, req *hcservice.GetBPaasApplicationReq) (
	*json.RawMessage, error) {

	return common.Request[hcservice.GetBPaasApplicationReq, json.RawMessage](
		a.client, rest.POST, kt, req, "/application/bpaas/query")
}

// DeliverBPaasApplication BPaaS审批通过后触发安全组规则补偿同步
func (a *ApplicationClient) DeliverBPaasApplication(kt *kit.Kit,
	req *hcservice.DeliverBPaasApplicationReq) error {

	return common.RequestNoResp(a.client, rest.POST, kt, req, "/application/bpaas/deliver")
}
