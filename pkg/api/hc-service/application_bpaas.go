package hcservice

import "hcm/pkg/criteria/validator"

// GetBPaasApplicationReq 查询bpaas申请单详情
type GetBPaasApplicationReq struct {
	BPaasSN   uint64 `json:"bpaas_sn"  validate:"required,gt=0"`
	AccountID string `json:"account_id"  validate:"required"`
}

// Validate ...
func (r *GetBPaasApplicationReq) Validate() error {
	return validator.Validate.Struct(r)
}

// DeliverBPaasApplicationReq BPaaS审批通过后触发资源补偿交付的请求，透传申请单 content 由 hc-service 按 action 分发处理
type DeliverBPaasApplicationReq struct {
	Content string `json:"content" validate:"required"`
}

// Validate ...
func (r *DeliverBPaasApplicationReq) Validate() error {
	return validator.Validate.Struct(r)
}
