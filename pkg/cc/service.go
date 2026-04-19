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

package cc

import (
	"fmt"
	"net"
	"sync"
	"time"
)

var (
	initOnce sync.Once

	// serviceName is the runtime service's name.
	serviceName Name
)

// InitService set the initial service.
func InitService(sn Name) {
	initOnce.Do(func() {
		time.Local = time.FixedZone("UTC", 0)
		serviceName = sn
	})
}

// ServiceName return the current runtime service's name.
func ServiceName() Name {
	return serviceName
}

// Name is the name of the service
type Name string

const (
	// APIServerName is api server's name
	APIServerName Name = "api-server"
	// CloudServerName is cloud server's name
	CloudServerName Name = "cloud-server"
	// DataServiceName is data service's name
	DataServiceName Name = "data-service"
	// HCServiceName is hc service's name
	HCServiceName Name = "hc-service"
	// AuthServerName is the auth server's service name
	AuthServerName Name = "auth-server"
	// WebServerName is the web page server's name
	WebServerName Name = "web-server"
	// TaskServerName is task server's name
	TaskServerName Name = "task-server"
	// AccountServerName is account server's name
	AccountServerName Name = "account-server"
	// AgentServerName is agent server's name
	AgentServerName Name = "agent-server"
)

// Setting defines all service Setting interface.
type Setting interface {
	trySetFlagBindIP(ip net.IP) error
	trySetDefault()
	Validate() error
	TenantEnable() bool
}

// ApiServerSetting defines api server used setting options.
type ApiServerSetting struct {
	Network Network      `yaml:"network"`
	Service Service      `yaml:"service"`
	Log     LogOption    `yaml:"log"`
	Tenant  TenantConfig `yaml:"tenant"`
}

// trySetFlagBindIP try set flag bind ip.
func (s *ApiServerSetting) trySetFlagBindIP(ip net.IP) error {
	return s.Network.trySetFlagBindIP(ip)
}

// trySetDefault set the ApiServerSetting default value if user not configured.
func (s *ApiServerSetting) trySetDefault() {
	s.Network.trySetDefault()
	s.Service.trySetDefault()
	s.Log.trySetDefault()

	return
}

// Validate ApiServerSetting option.
func (s ApiServerSetting) Validate() error {

	if err := s.Network.validate(); err != nil {
		return err
	}

	if err := s.Service.validate(); err != nil {
		return err
	}

	return nil
}

// TenantEnable get tenant is enabled.
func (s *ApiServerSetting) TenantEnable() bool {
	return s.Tenant.Enabled
}

// TaskManagement ...
type TaskManagement struct {
	// 关闭任务管理轮询
	Disable bool `yaml:"disable"`
}

// CloudServerSetting defines cloud server used setting options.
type CloudServerSetting struct {
	// 内部版配置
	FinOps ApiGateway `yaml:"finops"`
	MOA    MOA        `yaml:"moa"`

	Network          Network          `yaml:"network"`
	Service          Service          `yaml:"service"`
	Log              LogOption        `yaml:"log"`
	Crypto           Crypto           `yaml:"crypto"`
	BkHcmUrl         string           `yaml:"bkHcmUrl"`
	BkApigwHCMURL    string           `yaml:"bkApigwHCMUrl"`
	CloudResource    CloudResource    `yaml:"cloudResource"`
	Recycle          Recycle          `yaml:"recycle"`
	BillConfig       BillConfig       `yaml:"billConfig"`
	Itsm             ApiGateway       `yaml:"itsm"`
	CloudSelection   CloudSelection   `yaml:"cloudSelection"`
	Cmsi             CMSI             `yaml:"cmsi"`
	UserMgr          ApiGateway       `yaml:"userMgr"`
	OrgTopoConfig    BillConfig       `yaml:"orgTopoConfig"`
	TaskManagement   TaskManagement   `yaml:"taskManagement"`
	Tenant           TenantConfig     `yaml:"tenant"`
	Cmdb             ApiGateway       `yaml:"cmdb"`
	CCHostPoolBiz    int64            `yaml:"ccHostPoolBiz"`
	ConcurrentConfig ConcurrentConfig `yaml:"concurrentConfig"`
	TmpFileDir       string           `yaml:"tmpFileDir"`
}

// trySetFlagBindIP try set flag bind ip.
func (s *CloudServerSetting) trySetFlagBindIP(ip net.IP) error {
	return s.Network.trySetFlagBindIP(ip)
}

// trySetDefault set the CloudServerSetting default value if user not configured.
func (s *CloudServerSetting) trySetDefault() {
	s.Network.trySetDefault()
	s.Service.trySetDefault()
	s.Log.trySetDefault()
	s.ConcurrentConfig.trySetDefault()
	if s.TmpFileDir == "" {
		s.TmpFileDir = "/tmp"
	}

	return
}

// Validate CloudServerSetting option.
func (s CloudServerSetting) Validate() error {

	if err := s.Network.validate(); err != nil {
		return err
	}

	if err := s.Service.validate(); err != nil {
		return err
	}

	if err := s.Crypto.validate(); err != nil {
		return err
	}

	if err := s.Cmdb.validate(); err != nil {
		return err
	}

	if s.BkHcmUrl == "" {
		return fmt.Errorf("bkHcmUrl should not be empty")
	}

	if s.BkApigwHCMURL == "" {
		return fmt.Errorf("bkApigwHCMUrl should not be empty")
	}

	if err := s.CloudResource.validate(); err != nil {
		return err
	}

	if err := s.Recycle.validate(); err != nil {
		return err
	}

	if err := s.Itsm.validate(); err != nil {
		return err
	}

	if err := s.Cmsi.validate(); err != nil {
		return err
	}
	if err := s.MOA.validate(); err != nil {
		return err
	}

	if s.CCHostPoolBiz == 0 {
		return fmt.Errorf("ccHostPoolBiz should not be empty")
	}

	return nil
}

// TenantEnable get tenant is enabled.
func (s *CloudServerSetting) TenantEnable() bool {
	return s.Tenant.Enabled
}

// DataServiceSetting defines data service used setting options.
type DataServiceSetting struct {
	OBSDatabase *DataBase `yaml:"obsDatabase,omitempty"`

	Network     Network      `yaml:"network"`
	Service     Service      `yaml:"service"`
	Log         LogOption    `yaml:"log"`
	Database    DataBase     `yaml:"database"`
	Objectstore ObjectStore  `yaml:"objectstore"`
	Crypto      Crypto       `yaml:"crypto"`
	Cmdb        ApiGateway   `yaml:"cmdb"`
	Tenant      TenantConfig `yaml:"tenant"`
}

// trySetFlagBindIP try set flag bind ip.
func (s *DataServiceSetting) trySetFlagBindIP(ip net.IP) error {
	return s.Network.trySetFlagBindIP(ip)
}

// trySetDefault set the DataServiceSetting default value if user not configured.
func (s *DataServiceSetting) trySetDefault() {
	if s.OBSDatabase != nil {
		s.OBSDatabase.trySetDefault()
	}

	s.Network.trySetDefault()
	s.Service.trySetDefault()
	s.Log.trySetDefault()
	s.Database.trySetDefault()

	return
}

// Validate DataServiceSetting option.
func (s DataServiceSetting) Validate() error {
	if s.OBSDatabase != nil {
		if err := s.OBSDatabase.validate(); err != nil {
			return err
		}
	}

	if err := s.Network.validate(); err != nil {
		return err
	}

	if err := s.Service.validate(); err != nil {
		return err
	}

	if err := s.Database.validate(); err != nil {
		return err
	}

	if err := s.Crypto.validate(); err != nil {
		return err
	}

	if err := s.Cmdb.validate(); err != nil {
		return err
	}

	return nil
}

// TenantEnable get tenant is enabled.
func (s *DataServiceSetting) TenantEnable() bool {
	return s.Tenant.Enabled
}

// HCServiceSetting defines hc service used setting options.
type HCServiceSetting struct {
	// 自研云增加的配置写在这里
	Esb                   Esb      `yaml:"esb"`
	ZiyanSecrets          []Secret `yaml:"ziyanSecrets"`
	SecurityGroupSkipList []string `yaml:"securityGroupMgmtSkipList"`

	Network       Network      `yaml:"network"`
	Service       Service      `yaml:"service"`
	Log           LogOption    `yaml:"log"`
	SyncConfig    SyncConfig   `yaml:"sync"`
	Tenant        TenantConfig `yaml:"tenant"`
	Cmdb          ApiGateway   `yaml:"cmdb"`
	CCHostPoolBiz int64        `yaml:"ccHostPoolBiz"`
	Crp           Crp          `yaml:"crp"`
}

// trySetFlagBindIP try set flag bind ip.
func (s *HCServiceSetting) trySetFlagBindIP(ip net.IP) error {
	return s.Network.trySetFlagBindIP(ip)
}

// trySetDefault set the HCServiceSetting default value if user not configured.
func (s *HCServiceSetting) trySetDefault() {
	s.Network.trySetDefault()
	s.Service.trySetDefault()
	s.Log.trySetDefault()
	s.SyncConfig.trySetDefault()

	return
}

// Validate HCServiceSetting option.
func (s HCServiceSetting) Validate() error {

	if err := s.Network.validate(); err != nil {
		return err
	}

	if err := s.Service.validate(); err != nil {
		return err
	}
	if err := s.SyncConfig.Validate(); err != nil {
		return fmt.Errorf("syncConfig validate error: %w", err)
	}

	for _, secret := range s.ZiyanSecrets {
		if err := secret.Validate(); err != nil {
			return err
		}
	}

	if err := s.Cmdb.validate(); err != nil {
		return err
	}

	if s.CCHostPoolBiz == 0 {
		return fmt.Errorf("ccHostPoolBiz should not be empty")
	}

	if err := s.Crp.validate(); err != nil {
		return err
	}

	return nil
}

// TenantEnable get tenant is enabled.
func (s *HCServiceSetting) TenantEnable() bool {
	return s.Tenant.Enabled
}

// AuthServerSetting defines auth server used setting options.
type AuthServerSetting struct {
	Network Network      `yaml:"network"`
	Service Service      `yaml:"service"`
	Log     LogOption    `yaml:"log"`
	Esb     Esb          `yaml:"esb"`
	Cmdb    ApiGateway   `yaml:"cmdb"`
	Tenant  TenantConfig `yaml:"tenant"`

	IAM IAM `yaml:"iam"`
}

// trySetFlagBindIP try set flag bind ip.
func (s *AuthServerSetting) trySetFlagBindIP(ip net.IP) error {
	return s.Network.trySetFlagBindIP(ip)
}

// trySetDefault set the AuthServerSetting default value if user not configured.
func (s *AuthServerSetting) trySetDefault() {
	s.Network.trySetDefault()
	s.Service.trySetDefault()
	s.Log.trySetDefault()

	return
}

// Validate AuthServerSetting option.
func (s AuthServerSetting) Validate() error {
	if err := s.Network.validate(); err != nil {
		return err
	}

	if err := s.Service.validate(); err != nil {
		return err
	}

	if err := s.Esb.validate(); err != nil {
		return err
	}

	if err := s.Cmdb.validate(); err != nil {
		return err
	}

	if err := s.IAM.validate(); err != nil {
		return err
	}

	return nil
}

// TenantEnable get tenant is enabled.
func (s *AuthServerSetting) TenantEnable() bool {
	return s.Tenant.Enabled
}

// WebServerSetting defines api server used setting options.
type WebServerSetting struct {
	Network       Network       `yaml:"network"`
	Service       Service       `yaml:"service"`
	Log           LogOption     `yaml:"log"`
	Web           Web           `yaml:"web"`
	Esb           Esb           `yaml:"esb"`
	Itsm          ApiGateway    `yaml:"itsm"`
	ChangeLogPath ChangeLogPath `yaml:"changeLogPath"`
	Notice        Notice        `yaml:"notice"`
	TemplatePath  string        `yaml:"templatePath"`
	Tenant        TenantConfig  `yaml:"tenant"`
	Cmdb          ApiGateway    `yaml:"cmdb"`
}

// trySetFlagBindIP try set flag bind ip.
func (s *WebServerSetting) trySetFlagBindIP(ip net.IP) error {
	return s.Network.trySetFlagBindIP(ip)
}

// trySetDefault set the ApiServerSetting default value if user not configured.
func (s *WebServerSetting) trySetDefault() {
	s.Network.trySetDefault()
	s.Service.trySetDefault()
	s.Log.trySetDefault()
	s.ChangeLogPath.trySetDefault()
	if len(s.TemplatePath) == 0 {
		s.TemplatePath = "template"
	}

	return
}

// Validate ApiServerSetting option.
func (s WebServerSetting) Validate() error {

	if err := s.Network.validate(); err != nil {
		return err
	}

	if err := s.Service.validate(); err != nil {
		return err
	}

	if err := s.Web.validate(); err != nil {
		return err
	}

	if err := s.Esb.validate(); err != nil {
		return err
	}

	if err := s.Cmdb.validate(); err != nil {
		return err
	}

	if err := s.Itsm.validate(); err != nil {
		return err
	}

	if err := s.Notice.validate(); err != nil {
		return err
	}

	return nil
}

// TenantEnable get tenant is enabled.
func (s *WebServerSetting) TenantEnable() bool {
	return s.Tenant.Enabled
}

// LabelSwitch switch for labels
type LabelSwitch struct {
	AwsCN bool `json:"awsCN" yaml:"awsCN"`
}

// TaskServerSetting defines task server used setting options.
type TaskServerSetting struct {
	// 自研云增加的配置写在这里
	OBSDatabase *DataBase   `yaml:"obsDatabase,omitempty"`
	Cmdb        ApiGateway  `yaml:"cmdb"`
	AlarmCli    *AlarmCli   `yaml:"alarm,omitempty"`
	SamPwdCli   *ApiGateway `yaml:"sampwd,omitempty"`

	Network  Network      `yaml:"network"`
	Service  Service      `yaml:"service"`
	Database DataBase     `yaml:"database"`
	Log      LogOption    `yaml:"log"`
	Async    Async        `yaml:"async"`
	Tenant   TenantConfig `yaml:"tenant"`

	UseLabel LabelSwitch `yaml:"useLabel"`
}

// trySetFlagBindIP try set flag bind ip.
func (s *TaskServerSetting) trySetFlagBindIP(ip net.IP) error {
	return s.Network.trySetFlagBindIP(ip)
}

// trySetDefault set the TaskServerSetting default value if user not configured.
func (s *TaskServerSetting) trySetDefault() {
	s.Network.trySetDefault()
	s.Service.trySetDefault()
	s.Database.trySetDefault()
	s.Log.trySetDefault()
	s.Async.trySetDefault()

	if s.OBSDatabase != nil {
		s.OBSDatabase.trySetDefault()
	}

	return
}

// Validate TaskServerSetting option.
func (s TaskServerSetting) Validate() error {

	if err := s.Network.validate(); err != nil {
		return err
	}

	if err := s.Service.validate(); err != nil {
		return err
	}

	if err := s.Database.validate(); err != nil {
		return err
	}

	if s.OBSDatabase != nil {
		if err := s.OBSDatabase.validate(); err != nil {
			return err
		}
	}

	if s.AlarmCli != nil {
		if err := s.AlarmCli.validate(); err != nil {
			return err
		}
	}

	if s.SamPwdCli != nil {
		if err := s.SamPwdCli.validate(); err != nil {
			return err
		}
	}

	if err := s.Cmdb.validate(); err != nil {
		return fmt.Errorf("cmdb validate error: %w", err)
	}

	return nil
}

// TenantEnable get tenant is enabled.
func (s *TaskServerSetting) TenantEnable() bool {
	return s.Tenant.Enabled
}

// WoaServerSetting defines woa server used setting options.
type WoaServerSetting struct {
	Network           Network    `yaml:"network"`
	Service           Service    `yaml:"service"`
	Database          DataBase   `yaml:"database"`
	Log               LogOption  `yaml:"log"`
	Cmdb              ApiGateway `yaml:"cmdb"`
	FinOps            ApiGateway `yaml:"finops"`
	BkHcmURL          string     `yaml:"bkHcmUrl"`
	BkApigwHCMURL     string     `yaml:"bkApigwHCMUrl"`
	MongoDB           MongoDB    `yaml:"mongodb"`
	Watch             MongoDB    `yaml:"watch"`
	Redis             Redis      `yaml:"redis"`
	ClientConfig      `yaml:",inline"`
	ItsmFlows         []ItsmFlow        `yaml:"itsmFlows"`
	CancelItsmFlows   []ItsmFlow        `yaml:"cancelItsmFlows"`
	ResDissolve       ResourceDissolve  `yaml:"resourceDissolve"`
	Es                Es                `yaml:"elasticsearch"`
	Blacklist         string            `yaml:"blacklist"`
	UseMongo          bool              `yaml:"useMongo"`
	Recover           Recover           `yaml:"recover"`
	LocalTimezone     string            `yaml:"localTimezone"`
	RollingServer     RollingServer     `yaml:"rollingServer"`
	ResPlan           ResPlan           `yaml:"resPlan"`
	ResourceSync      ResourceSync      `yaml:"resourceSync"`
	Cmsi              CMSI              `yaml:"cmsi"`
	StuckCheck        StuckCheck        `yaml:"stuckCheck"`
	ApplyTicketConfig ApplyTicketConfig `yaml:"applyTicketConfig"`
	RecycleNotice     RecycleNotice     `yaml:"recycleNotice"`

	Tenant TenantConfig `yaml:"tenant"`
}

// trySetFlagBindIP try set flag bind ip.
func (s *WoaServerSetting) trySetFlagBindIP(ip net.IP) error {
	return s.Network.trySetFlagBindIP(ip)
}

// trySetDefault set the WoaServerSetting default value if user not configured.
func (s *WoaServerSetting) trySetDefault() {
	s.Network.trySetDefault()
	s.Service.trySetDefault()
	s.Log.trySetDefault()
	s.StuckCheck.trySetDefault()

	return
}

// Validate TaskServerSetting option.
func (s WoaServerSetting) Validate() error {
	if err := s.Network.validate(); err != nil {
		return err
	}

	if err := s.Service.validate(); err != nil {
		return err
	}

	if err := s.Cmdb.validate(); err != nil {
		return err
	}

	if err := s.FinOps.validate(); err != nil {
		return err
	}

	if s.BkHcmURL == "" {
		return fmt.Errorf("bkHcmUrl should not be empty")
	}

	if s.BkApigwHCMURL == "" {
		return fmt.Errorf("bkApigwHCMUrl should not be empty")
	}

	// 开启Mongo之后才校验参数
	if s.UseMongo {
		if err := s.MongoDB.validate(); err != nil {
			return err
		}

		if err := s.Watch.validate(); err != nil {
			return err
		}
	}

	if err := s.Redis.validate(); err != nil {
		return err
	}

	if err := s.Database.validate(); err != nil {
		return err
	}

	if err := s.ClientConfig.validate(); err != nil {
		return err
	}

	if err := s.ResDissolve.validate(); err != nil {
		return err
	}

	if err := s.Es.validate(); err != nil {
		return err
	}

	if err := s.ResourceSync.Validate(); err != nil {
		return err
	}

	if err := s.ApplyTicketConfig.validate(); err != nil {
		return err
	}
	if err := s.RecycleNotice.validate(); err != nil {
		return err
	}

	return nil
}

// TenantEnable get tenant is enabled.
func (s *WoaServerSetting) TenantEnable() bool {
	return s.Tenant.Enabled
}

// AccountServerSetting defines task server used setting options.
type AccountServerSetting struct {
	// 自研云增加的配置写在这里
	FinOps         ApiGateway           `yaml:"finops"`
	Jarvis         Jarvis               `yaml:"jarvis"`
	ExchangeRate   ExchangeRate         `yaml:"exchangeRate"`
	IEGObsOption   IEGObsOption         `yaml:"obs"`
	Esb            Esb                  `yaml:"esb"`
	Network        Network              `yaml:"network"`
	Service        Service              `yaml:"service"`
	Controller     BillControllerOption `yaml:"controller"`
	Log            LogOption            `yaml:"log"`
	BillAllocation BillAllocationOption `yaml:"billAllocation"`
	TmpFileDir     string               `yaml:"tmpFileDir"`
	Tenant         TenantConfig         `yaml:"tenant"`
	Cmdb           ApiGateway           `yaml:"cmdb"`
}

// trySetFlagBindIP try set flag bind ip.
func (s *AccountServerSetting) trySetFlagBindIP(ip net.IP) error {
	return s.Network.trySetFlagBindIP(ip)
}

// trySetDefault set the TaskServerSetting default value if user not configured.
func (s *AccountServerSetting) trySetDefault() {
	s.Network.trySetDefault()
	s.Service.trySetDefault()
	s.Controller.trySetDefault()
	s.Log.trySetDefault()
	if s.TmpFileDir == "" {
		s.TmpFileDir = "/tmp"
	}

	//  内部版配置
	s.ExchangeRate.trySetDefault()
}

// Validate TaskServerSetting option.
func (s AccountServerSetting) Validate() error {

	if err := s.Network.validate(); err != nil {
		return err
	}

	if err := s.Service.validate(); err != nil {
		return err
	}

	if err := s.Jarvis.validate(); err != nil {
		return err
	}

	if err := s.IEGObsOption.validate(); err != nil {
		return err
	}

	if err := s.BillAllocation.validate(); err != nil {
		return err
	}

	if err := s.Cmdb.validate(); err != nil {
		return err
	}

	if err := s.Esb.validate(); err != nil {
		return err
	}

	return nil
}

// TenantEnable get tenant is enabled.
func (s *AccountServerSetting) TenantEnable() bool {
	return s.Tenant.Enabled
}

// ChangeLogPath ...
type ChangeLogPath struct {
	Chinese string `yaml:"ch"`
	English string `yaml:"en"`
}

func (c *ChangeLogPath) trySetDefault() {
	if c.Chinese == "" {
		c.Chinese = "changelog/ch"
	}
	if c.English == "" {
		c.English = "changelog/en"
	}
}

// ApplyTicketConfig defines apply ticket config related settings.
type ApplyTicketConfig struct {
	PurchaseToResourcePool PurchaseToResourcePool `yaml:"purchaseToResourcePool"`
}

func (s *ApplyTicketConfig) validate() error {
	if err := s.PurchaseToResourcePool.validate(); err != nil {
		return err
	}

	return nil
}

// PurchaseToResourcePool defines purchase to resource pool related settings.
type PurchaseToResourcePool struct {
	User  string `yaml:"user"`
	BizID int64  `yaml:"bizID"`
}

func (s *PurchaseToResourcePool) validate() error {
	if s.User == "" {
		return fmt.Errorf("user should not be empty")
	}

	if s.BizID <= 0 {
		return fmt.Errorf("bizID should not be empty")
	}

	return nil
}

// AgentStorage defines persistent storage settings for the AGUI runner.
// When session DSN is empty, chat history uses in-memory storage (lost on restart).
// When memory DSN is empty, long-term memory is disabled.
type AgentStorage struct {
	Session AgentSessionStorage `yaml:"session"`
	Memory  AgentMemoryStorage  `yaml:"memory"`
}

// AgentSessionStorage defines MySQL settings for AGUI chat history (session) persistence.
type AgentSessionStorage struct {
	// DSN is the MySQL connection string. Leave empty to use in-memory storage.
	// Example: "user:password@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=true"
	DSN string `yaml:"dsn"`
	// TablePrefix is an optional prefix for all session table names (e.g. "hcm_").
	TablePrefix string `yaml:"tablePrefix"`
	// SkipDBInit skips automatic table creation. Set true if tables are managed externally.
	SkipDBInit bool `yaml:"skipDBInit"`
	// Summary configures automatic LLM-based session summarization.
	// Requires the AGUI LLM model to be configured (aidev section).
	Summary AgentSessionSummary `yaml:"summary"`
}

// AgentSessionSummary configures LLM-based session summarization for the AGUI runner.
// When enabled, the configured LLM (aidev section) compresses old conversation history
// into a summary once the configured thresholds are met, preventing context-window overflow.
type AgentSessionSummary struct {
	// Enabled turns on automatic session summarization. Default: false.
	Enabled bool `yaml:"enabled"`
	// Policy controls how multiple thresholds combine: "any" (OR) or "all" (AND). Default: "any".
	Policy string `yaml:"policy"`
	// EventThreshold triggers summarization when un-summarised event count exceeds this value.
	// 0 means this condition is not used.
	EventThreshold int `yaml:"eventThreshold"`
	// TokenThreshold triggers summarization when estimated token count exceeds this value.
	// 0 means this condition is not used.
	TokenThreshold int `yaml:"tokenThreshold"`
	// IdleThreshold triggers summarization when the session has been idle for this long.
	// Use Go duration format, e.g. "30m", "1h". Empty means this condition is not used.
	IdleThreshold string `yaml:"idleThreshold"`
	// MaxWords caps the word count of the generated summary. 0 means no cap.
	MaxWords int `yaml:"maxWords"`
}

// AgentMemoryStorage defines MySQL settings for AGUI long-term memory persistence.
type AgentMemoryStorage struct {
	// DSN is the MySQL connection string. Leave empty to disable memory persistence.
	DSN string `yaml:"dsn"`
	// TableName is the table name for storing memories. Default: "memories".
	TableName string `yaml:"tableName"`
	// SkipDBInit skips automatic table creation. Set true if tables are managed externally.
	SkipDBInit bool `yaml:"skipDBInit"`
	// Limit is the maximum number of memory entries per user. Default: 100.
	Limit int `yaml:"limit"`
}

// AgentMCPFilter configures MCP tool name filtering for the AGUI agent.
type AgentMCPFilter struct {
	// Mode is the filter mode: "include" (default) keeps only listed tools,
	// "exclude" removes listed tools.
	Mode string `yaml:"mode"`
	// Names lists the tool names to include or exclude.
	Names []string `yaml:"names"`
}

// AgentMCPReconnect configures automatic MCP session reconnection.
type AgentMCPReconnect struct {
	// Enabled turns on auto-reconnect when the session expires or drops.
	Enabled bool `yaml:"enabled"`
	// MaxAttempts is the maximum reconnect attempts per operation (1-10, default: 3).
	MaxAttempts int `yaml:"maxAttempts"`
}

// AgentMCPToolSet defines one MCP server toolset for the AGUI agent.
type AgentMCPToolSet struct {
	// Name is a unique label for this toolset (used for conflict resolution).
	Name string `yaml:"name"`
	// Type identifies the toolset category. When set to "bkaidev", the server
	// automatically injects an X-Bkapi-Authorization header on every MCP request
	// using tools.bkAIDev credentials (appCode, appSecret) and the bk_ticket
	// extracted from the incoming HTTP request Cookie.
	Type string `yaml:"type"`
	// Transport is the connection method: "stdio", "sse", or "streamable_http".
	Transport string `yaml:"transport"`
	// ServerURL is the MCP server base URL (required for sse / streamable_http).
	ServerURL string `yaml:"serverUrl"`
	// Headers are extra HTTP headers sent on every request (e.g. auth tokens).
	Headers map[string]string `yaml:"headers"`
	// Command is the executable to launch (required for stdio).
	Command string `yaml:"command"`
	// Args are the arguments passed to the stdio command.
	Args []string `yaml:"args"`
	// Timeout is the per-request deadline, e.g. "10s", "30s". Empty means no timeout.
	Timeout string `yaml:"timeout"`
	// Filter optionally restricts which tools from this MCP server are exposed.
	Filter *AgentMCPFilter `yaml:"filter"`
	// Reconnect configures automatic session reconnection on failure.
	Reconnect *AgentMCPReconnect `yaml:"reconnect"`
}

// AgentSkillsConfig configures the AGUI agent's skill repository.
type AgentSkillsConfig struct {
	// Root is the primary skills directory (each sub-directory with a SKILL.md is a skill).
	Root string `yaml:"root"`
	// ExtraDirs lists additional skill directories scanned at lower precedence.
	ExtraDirs []string `yaml:"extraDirs"`
}

// AgentBKAIDevConfig holds BK application credentials used by MCP toolsets
// of type "bkaidev" to construct the X-Bkapi-Authorization header.
type AgentBKAIDevConfig struct {
	// AppCode is the BK application code (bk_app_code).
	AppCode string `yaml:"appCode"`
	// AppSecret is the BK application secret (bk_app_secret).
	AppSecret string `yaml:"appSecret"`
}

// AgentToolsConfig holds all tool configurations injected into the AGUI agent.
type AgentToolsConfig struct {
	// MCPToolSets lists MCP server toolsets to expose to the AGUI agent.
	MCPToolSets []AgentMCPToolSet `yaml:"mcp"`
	// Skills configures the filesystem-backed skill repository.
	Skills *AgentSkillsConfig `yaml:"skills"`
	// BKAIDev provides BK application credentials for MCP toolsets with type "bkaidev".
	// When an MCP toolset has type: "bkaidev", the server injects X-Bkapi-Authorization
	// on every request using these credentials combined with the per-request bk_ticket
	// extracted from the incoming HTTP request Cookie.
	BKAIDev *AgentBKAIDevConfig `yaml:"bkAIDev"`
}

// AgentAGUI configures the AG-UI protocol endpoint and its optional history feature.
type AgentAGUI struct {
	// Enable enables the AG-UI protocol endpoint.
	// When true, the AG-UI HTTP handler is mounted on the specified Path.
	Enable bool `yaml:"enable"`
	// AppName namespaces all session data in the backend storage.
	// Recommended to set in all deployments.
	// Example: "hcm-agent"
	AppName string `yaml:"appName"`
	// AllowedModels is the list of AI model identifiers that the platform permits.
	// When empty, the platform default list (pkg/criteria/enumor.DefaultAllowedAIModels) is used.
	// Example: ["gpt-4o", "deepseek-v3", "hunyuan-turbo"]
	AllowedModels []string `yaml:"allowedModels"`
	// DefaultModel is the default LLM model identifier used by the agent.
	// When empty, the first model in AllowedModels is used as default.
	DefaultModel string `yaml:"defaultModel"`
	// Stream enables token-level streaming when calling the upstream LLM.
	// When true, each token is forwarded to the client as a separate TEXT_MESSAGE_CONTENT
	// SSE event, producing a real-time typewriter effect.
	// When false (default), the LLM response is returned as a single event after completion.
	Stream bool `yaml:"stream"`
}

func (a *AgentAGUI) trySetDefault() {
	// TODO
}

// AgentServerSetting defines agent server used setting options.
type AgentServerSetting struct {
	Network Network          `yaml:"network"`
	Service Service          `yaml:"service"`
	Log     LogOption        `yaml:"log"`
	AIDev   AIDevConfig      `yaml:"aidev"`
	Storage AgentStorage     `yaml:"storage"`
	Tools   AgentToolsConfig `yaml:"tools"`
	AGUI    AgentAGUI        `yaml:"agui"`
}

// trySetFlagBindIP try set flag bind ip.
func (s *AgentServerSetting) trySetFlagBindIP(ip net.IP) error {
	return s.Network.trySetFlagBindIP(ip)
}

// trySetDefault set the AgentServerSetting default value if user not configured.
func (s *AgentServerSetting) trySetDefault() {
	s.Network.trySetDefault()
	s.Service.trySetDefault()
	s.Log.trySetDefault()
	s.AGUI.trySetDefault()
}

// Validate AgentServerSetting option.
func (s AgentServerSetting) Validate() error {
	if err := s.Network.validate(); err != nil {
		return err
	}

	if err := s.Service.validate(); err != nil {
		return err
	}

	return nil
}

// TenantEnable returns false as agent-server does not support multi-tenancy.
func (s *AgentServerSetting) TenantEnable() bool {
	return false
}

// AIDevConfig holds the configuration for the AIDev API gateway.
type AIDevConfig struct {
	ApiGateway `yaml:",inline"`
	// APIKey is the optional API key (Bearer token) for the gateway.
	// Leave empty if the gateway does not require one.
	APIKey string `yaml:"apiKey"`
}

// Validate AIDevConfig option.
func (s *AIDevConfig) Validate() error {
	if err := s.ApiGateway.validate(); err != nil {
		return err
	}
	return nil
}
