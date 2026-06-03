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
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/logs"
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
	Session    AgentSessionStorage    `yaml:"session"`
	Memory     AgentMemoryStorage     `yaml:"memory"`
	Checkpoint AgentCheckpointStorage `yaml:"checkpoint"`
}

func (s *AgentStorage) trySetDefault() {
	s.Session.trySetDefault()
	s.Memory.trySetDefault()
	s.Checkpoint.trySetDefault()
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

func (s *AgentSessionStorage) trySetDefault() {
	s.Summary.trySetDefault()
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

func (s *AgentSessionSummary) trySetDefault() {}

// AgentMemoryStorage defines settings for AGUI long-term memory persistence.
// Backend controls which storage engine to use:
//   - "mysql"     – MySQL-backed storage, requires DSN.
//   - "sqlitevec" – SQLite + sqlite-vec (vector search), requires DBPath and Embedding config.
//   - ""          – (default) falls back to "mysql" when DSN is set, otherwise disabled.
type AgentMemoryStorage struct {
	// Backend selects the memory storage engine: "mysql" or "sqlitevec".
	// When empty, auto-detected from other fields (DSN → mysql, DBPath → sqlitevec).
	Backend string `yaml:"backend"`
	// DSN is the MySQL connection string (backend=mysql). Leave empty to disable MySQL memory.
	DSN string `yaml:"dsn"`
	// DBPath is the SQLite database file path (backend=sqlitevec).
	// Example: "/data/agent-server/memories.db"
	DBPath string `yaml:"dbPath"`
	// TableName is the table name for storing memories. Default: "memories".
	TableName string `yaml:"tableName"`
	// SkipDBInit skips automatic table creation. Set true if tables are managed externally.
	SkipDBInit bool `yaml:"skipDBInit"`
	// Limit is the maximum number of memory entries per user. Default: 100.
	Limit int `yaml:"limit"`
	// MaxSearchResults limits the number of results returned by vector search (Top-K).
	// Default: 10 (framework default). 0 means use framework default.
	MaxSearchResults int `yaml:"maxSearchResults"`
	// PreloadLimit sets the number of most-recent memories to inject into the system
	// prompt at the start of each conversation turn. Default: 20. 0 disables preloading.
	PreloadLimit int `yaml:"preloadLimit"`
	// Embedding configures the embedding model for vector-based memory (backend=sqlitevec).
	Embedding AgentEmbeddingConfig `yaml:"embedding"`
	// AutoExtract enables automatic LLM-based memory extraction after each Run.
	// When true, the agent calls an LLM after every conversation turn to identify
	// memorable facts and persist them to the memories table.
	AutoExtract bool `yaml:"autoExtract"`
	// AutoExtractMessages triggers extraction only when the number of new messages
	// exceeds this value. 0 means no message-count gate (always consider extracting).
	AutoExtractMessages int `yaml:"autoExtractMessages"`
	// AutoExtractInterval triggers extraction only when the given duration has
	// elapsed since the last extraction. 0 / empty means no interval gate.
	// Accepts Go duration strings, e.g. "30m", "1h".
	AutoExtractInterval string `yaml:"autoExtractInterval"`
	// AutoExtractPolicy combines the above checkers: "any" (OR, default) or "all" (AND).
	// "any"  – extract when at least one enabled checker passes.
	// "all"  – extract only when every enabled checker passes.
	AutoExtractPolicy string `yaml:"autoExtractPolicy"`
	// ExtractPromptFile is the path to a custom extraction prompt file.
	// When set, the file content replaces the framework's default extraction prompt,
	// allowing fine-grained control over what the LLM considers memorable.
	// Supports absolute or relative paths (relative to the process working directory).
	ExtractPromptFile string `yaml:"extractPromptFile"`
	// ExtractPrompt is the extract prompt content.
	ExtractPrompt string
}

func (s *AgentMemoryStorage) trySetDefault() {
	s.ExtractPrompt = loadPromptFile(s.ExtractPromptFile)
}

// AgentCheckpointStorage defines checkpoint storage settings for the graph agent.
// Checkpoint is used for interrupt/resume support in graph-based workflows.
// When backend is empty or "inmemory", checkpoints are stored in memory (lost on restart).
// When backend is "sqlite", checkpoints are persisted to a SQLite database.
type AgentCheckpointStorage struct {
	// Backend selects the checkpoint storage engine: "inmemory" or "sqlite".
	// Default: "inmemory".
	Backend enumor.GraphCheckpointBackend `yaml:"backend"`
	// DBPath is the SQLite database file path (backend=sqlite).
	// Example: "/data/agent-server/checkpoint.db"
	DBPath string `yaml:"dbPath"`
}

func (s *AgentCheckpointStorage) trySetDefault() {
	if s.Backend == "" {
		s.Backend = enumor.GraphCheckpointBackendInMemory
	}
}

// Validate validates the checkpoint storage configuration.
func (s *AgentCheckpointStorage) Validate() error {
	if err := s.Backend.Validate(); err != nil {
		return err
	}

	if s.Backend == enumor.GraphCheckpointBackendSQLite {
		if s.DBPath == "" {
			return fmt.Errorf("dbPath is required when backend is sqlite")
		}
	}
	return nil
}

// ResolveMemoryBackend determines which backend to use based on explicit config or auto-detection.
func (s *AgentMemoryStorage) ResolveMemoryBackend() string {
	if b := strings.TrimSpace(strings.ToLower(s.Backend)); b != "" {
		return b
	}
	if strings.TrimSpace(s.DBPath) != "" {
		return "sqlitevec"
	}
	if strings.TrimSpace(s.DSN) != "" {
		return "mysql"
	}
	return ""
}

// AgentEmbeddingConfig configures the embedding model used by vector-based memory backends.
// The API endpoint and authentication are inherited from the aidev gateway config,
// so only model-specific settings are needed here.
type AgentEmbeddingConfig struct {
	// Model is the embedding model name. Default: "text-embedding-3-small".
	Model string `yaml:"model"`
	// Dimensions is the embedding vector dimension. Default: 1536.
	Dimensions int `yaml:"dimensions"`
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
	// RequireConfirm when true requires the user to explicitly send "确认"
	// before any tool in this MCP toolset is actually executed.
	RequireConfirm bool `yaml:"requireConfirm"`
}

// AgentBKAIDevSyncSkillsConfig holds all skill configuration: filesystem paths
// for the skill repository and BKAIDev-specific sync parameters.
type AgentBKAIDevSyncSkillsConfig struct {
	// Root is the primary skills directory (each sub-directory with a SKILL.md is a skill).
	// Default Root is "./skills".
	Root string `yaml:"root"`
	// ExtraDirs lists additional skill directories scanned at lower precedence.
	ExtraDirs []string `yaml:"extraDirs"`
	// ArchiveDir stores downloaded skill zip files during sync (separate from Root).
	// Default ArchiveDir is "./{ROOT}/skill-archive".
	ArchiveDir string `yaml:"archiveDir"`
	// StorePath overrides the default localstore file path ({root}/store.json).
	// Default StorePath is "./{ROOT}/store.json".
	StorePath string `yaml:"storePath"`
	// Enabled turns on skill sync from BKAIDev. Default: false.
	Enabled bool `yaml:"enabled"`
	// SpaceID is the BKAIDev space identifier (list_app_v1_skills space_id).
	SpaceID string `yaml:"spaceID"`
	// SyncInterval is the cron sync interval, e.g. "5m". Default: "5m".
	SyncInterval string `yaml:"syncInterval"`
	// TagName is a map of tag filters for filtering skills after listing.
	// Key is the first-level tag name, value is the second-level tag name (can be empty).
	// Example: {"status": "enabled", "hcm_agent": "agent1"}
	// Since ListSkills API does not support array tag_name filtering yet,
	// we filter skills locally after fetching the full list.
	TagName map[string]string `yaml:"tagName"`
	// MaxParallel limits concurrent skill installs. Limit range: [1, 10]. Default: 5.
	MaxParallel int `yaml:"maxParallel"`
}

// StoreFile returns the localstore file path, defaulting to {root}/store.json.
func (c *AgentBKAIDevSyncSkillsConfig) StoreFile() string {
	return c.StorePath
}

// trySetDefault fills in zero-value fields with sensible defaults.
func (c *AgentBKAIDevSyncSkillsConfig) trySetDefault() {
	if c.MaxParallel <= 0 {
		c.MaxParallel = 5
	}
	if c.SyncInterval == "" {
		c.SyncInterval = "5m"
	}

	// c.Root为空默认为当前目录下的skills
	if strings.TrimSpace(c.Root) == "" {
		c.Root = "./skills"
	}

	if c.ArchiveDir == "" {
		// ArchiveDir 为空则默认为ROOT下面/skill-archive
		c.ArchiveDir = filepath.Join(strings.TrimSpace(c.Root), "skill-archive")
	}

	if c.StorePath == "" {
		// StorePath为空 ROOT为空 则默认为.下面/skill-version.json
		c.StorePath = filepath.Join(strings.TrimSpace(c.Root), "skill-version.json")
	}
}

// Validate checks skill sync settings.
func (c *AgentBKAIDevSyncSkillsConfig) Validate() error {
	if c.SpaceID == "" {
		return errors.New("spaceID is not set")
	}

	if c.MaxParallel <= 0 || c.MaxParallel > 10 {
		return errors.New("maxParallel must be between 1 and 10")
	}

	return nil
}

// AgentBKAIDevConfig holds BK application credentials used by MCP toolsets
// of type "bkaidev" to construct the X-Bkapi-Authorization header.
type AgentBKAIDevConfig struct {
	// AppCode is the BK application code (bk_app_code).
	AppCode string `yaml:"appCode"`
	// AppSecret is the BK application secret (bk_app_secret).
	AppSecret string `yaml:"appSecret"`
}

// AgentDynamicToolLoadingConfig configures BM25/keyword-based dynamic tool
// filtering so the LLM only sees tools relevant to each user message.
type AgentDynamicToolLoadingConfig struct {
	// Enabled turns on dynamic tool filtering. Default: false.
	Enabled bool `yaml:"enabled"`
	// Strategy is the search strategy: "keyword" or "bm25". Default: "bm25".
	Strategy string `yaml:"strategy"`
	// TopN is the maximum number of tools returned per search. Must be > 0 when enabled.
	TopN int `yaml:"topN"`
	// ScoreThreshold is a relative score cutoff (0.0–1.0). Results scoring below
	// maxScore*ScoreThreshold are discarded before the TopN cap is applied. Default: 0.
	ScoreThreshold float64 `yaml:"scoreThreshold"`
	// ToolTags maps raw MCP tool names to extra search keywords (e.g. Chinese synonyms).
	ToolTags map[string][]string `yaml:"toolTags"`
	// QueryContextWindow controls how many recent user messages are included in the
	// search query. A sliding window of the last N user messages from the session is
	// concatenated to form the query, so short follow-ups like "继续" still carry
	// enough context to match relevant tools. Default: 3.
	QueryContextWindow int `yaml:"queryContextWindow"`
	// Embedding holds model/dimension config when Strategy is "embedding".
	// Endpoint and auth inherit from the aidev gateway (same as memory sqlitevec).
	Embedding AgentEmbeddingConfig `yaml:"embedding"`
}

func (s *AgentDynamicToolLoadingConfig) trySetDefault() {
	if s.TopN <= 0 {
		s.TopN = 10
	}

	if s.QueryContextWindow <= 0 {
		s.QueryContextWindow = 3
	}
}

// AgentToolsConfig holds all tool configurations injected into the AGUI agent.
type AgentToolsConfig struct {
	// MCPToolSets lists MCP server toolsets to expose to the AGUI agent.
	MCPToolSets []AgentMCPToolSet `yaml:"mcp"`
	// BKAIDev provides BK application credentials for MCP toolsets with type "bkaidev".
	// When an MCP toolset has type: "bkaidev", the server injects X-Bkapi-Authorization
	// on every request using these credentials combined with the per-request bk_ticket
	// extracted from the incoming HTTP request Cookie.
	BKAIDev *AgentBKAIDevConfig `yaml:"bkAIDev"`
	// DynamicToolLoading configures index-based dynamic tool filtering.
	DynamicToolLoading *AgentDynamicToolLoadingConfig `yaml:"dynamicToolLoading"`
}

func (s *AgentToolsConfig) trySetDefault() {
	s.DynamicToolLoading.trySetDefault()
}

// NeedToRefreshToolSetsOnRun bkaidev 类型 MCP 需要用户的 token 进行鉴权，因此无法在启动时加载工具集，需要在每次运行时刷新。
func (s AgentToolsConfig) NeedToRefreshToolSetsOnRun() bool {
	for _, cfg := range s.MCPToolSets {
		if strings.EqualFold(strings.TrimSpace(cfg.Type), constant.MCPTypeBKAIDev) {
			return true
		}
	}
	return false
}

// AgentPromptEntry configures a single BKAIDev-hosted prompt entry.
type AgentPromptEntry struct {
	// ID is the prompt_id on the BKAIDev platform used to retrieve content.
	ID int `yaml:"id"`
	// Code is the local key used to retrieve content from PromptStore.
	// 建议BKAIDev平台的prompt_code对应
	Code string `yaml:"code"`
	// Required: if true, the initial sync of this prompt must succeed before the agent
	// is marked prompt-ready. Non-required failures are logged as warnings and skipped.
	Required bool `yaml:"required"`
}

// AgentPromptConfig configures agent prompts: local file mode or BKAIDev sync mode.
// When Enabled is true, BKAIDev sync mode is active and the file fields are ignored.
// Gateway credentials are shared via AgentServerSetting.BKAIDevSyncAPIGateway.
type AgentPromptConfig struct {
	// SystemPromptFile is the path to a Markdown/text file whose content becomes the
	// GlobalInstruction. Prepended to every LLM request.
	SystemPromptFile string `yaml:"systemPromptFile"`
	// InstructionFile is the path to a Markdown/text file whose content becomes the
	// Instruction. Appended to every LLM request.
	InstructionFile string `yaml:"instructionFile"`
	// SystemPrompt is the system prompt content loaded from SystemPromptFile at startup.
	SystemPrompt string
	// Instruction is the instruction content loaded from InstructionFile at startup.
	Instruction string

	// BKAIDev sync mode fields (ignored when Enabled=false).
	// Enabled turns on BKAIDev prompt sync. When true, file fields above are ignored.
	Enabled bool `yaml:"enabled"`
	// SpaceID is the BKAIDev space identifier.
	SpaceID string `yaml:"spaceID"`
	// SyncInterval is the cron sync interval, e.g. "5m". Default: "5m".
	SyncInterval string `yaml:"syncInterval"`
	// StorePath is the local store file path that persists prompt content and MD5
	// across restarts. Default: "prompt-store.json".
	StorePath string `yaml:"storePath"`
	// Entries is the list of prompts to sync from BKAIDev.
	Entries []AgentPromptEntry `yaml:"entries"`
}

// BKAIDevSyncEnabled reports whether BKAIDev prompt sync is enabled.
func (s *AgentPromptConfig) BKAIDevSyncEnabled() bool {
	return s.Enabled
}

func (s *AgentPromptConfig) trySetDefault() {
	if s.BKAIDevSyncEnabled() {
		if s.SyncInterval == "" {
			s.SyncInterval = "5m"
		}
		if s.StorePath == "" {
			s.StorePath = "prompt-store.json"
		}
		// 远程模式下不加载文件，内容由 Syncer 异步填充
		return
	}
	s.SystemPrompt = loadPromptFile(s.SystemPromptFile)
	s.Instruction = loadPromptFile(s.InstructionFile)
}

// Validate validates the agent prompt config.
func (s AgentPromptConfig) Validate() error {
	if !s.BKAIDevSyncEnabled() {
		if s.SystemPrompt == "" {
			return errors.New("SystemPrompt is required for local file mode")
		}
		return nil
	}

	if s.SpaceID == "" {
		return errors.New("spaceID is not set")
	}
	names := make(map[string]struct{}, len(s.Entries))
	for i, p := range s.Entries {
		if p.ID <= 0 {
			logs.Warnf("prompts[%d]: id must be positive, id: %d", i, p.ID)
			if !p.Required {
				continue
			}
			return errf.Newf(errf.InvalidParameter, "prompts[%d]: id must be positive", i)
		}
		if p.Code == "" {
			logs.Warnf("prompts[%d]: name must not be empty, id: %d", i, p.ID)
			if !p.Required {
				continue
			}
			return errf.Newf(errf.InvalidParameter, "prompts[%d]: name must not be empty, id: %d", i, p.ID)
		}
		if _, dup := names[p.Code]; dup {
			logs.Warnf("prompts[%d]: duplicate name %s skip, id: %d", i, p.Code, p.ID)
			continue
		}
		names[p.Code] = struct{}{}
	}
	return nil
}

// loadPromptFile reads a prompt text file and returns its trimmed content.
// Returns an empty string when path is empty or the file cannot be read.
func loadPromptFile(path string) string {
	if path = strings.TrimSpace(path); path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// trySetDefault 中不可以使用 logs，会导致 logfile 提前创建
		fmt.Fprintf(os.Stderr, "failed to load prompt file %q: %v", path, err)
		return ""
	}
	return strings.TrimSpace(string(data))
}

// AgentModelProvider defines a named LLM provider endpoint.
// Each provider represents an independent OpenAI-compatible API gateway
// with its own base URL and authentication credentials.
// Models reference a provider by name via AgentModelConfig.Provider.
type AgentModelProvider struct {
	ApiGateway `yaml:",inline"`

	// Name uniquely identifies this provider (e.g. "aidev", "deepseek").
	Name string `yaml:"name"`
	// Type is the type of the provider.
	Type enumor.AgentModelProviderType `yaml:"type"`
	// BaseURL is the base URL of the OpenAI-compatible endpoint.
	BaseURL string `yaml:"baseURL"`
	// APIKey is the optional Bearer token for providers that use API-key auth.
	APIKey string `yaml:"apiKey"`
}

// Validate validates the agent model provider.
func (a *AgentModelProvider) Validate() error {
	if err := a.Type.Validate(); err != nil {
		return err
	}

	switch a.Type {
	case enumor.AgentModelProviderTypeBKAPIGW:
		return a.ApiGateway.validate()
	case enumor.AgentModelProviderTypeOpenAI:
		if a.BaseURL == "" {
			return fmt.Errorf("baseURL should not be empty")
		}
		if a.APIKey == "" {
			return fmt.Errorf("apiKey should not be empty")
		}
	}

	return nil
}

// IsBKAPIProvider checks if the model provider is a BK API gateway provider.
func (a *AgentModelProvider) IsBKAPIProvider() bool {
	return a.Type == enumor.AgentModelProviderTypeBKAPIGW
}

// ConvertToBKAPIProvider converts the model provider to a BK API gateway provider.
func (a *AgentModelProvider) ConvertToBKAPIProvider() *AgentModelProvider {
	baseURL := a.BaseURL
	if len(a.Endpoints) > 0 {
		baseURL = a.Endpoints[0]
	}
	return &AgentModelProvider{
		ApiGateway: a.ApiGateway,
		Name:       a.Name,
		Type:       enumor.AgentModelProviderTypeBKAPIGW,
		BaseURL:    baseURL,
	}
}

// IsOpenAIProvider checks if the model provider is an OpenAI provider.
func (a *AgentModelProvider) IsOpenAIProvider() bool {
	return a.Type == enumor.AgentModelProviderTypeOpenAI
}

// ConvertToOpenAIProvider converts the model provider to an OpenAI provider.
func (a *AgentModelProvider) ConvertToOpenAIProvider() *AgentModelProvider {
	return &AgentModelProvider{
		Name:    a.Name,
		Type:    enumor.AgentModelProviderTypeOpenAI,
		BaseURL: a.BaseURL,
		APIKey:  a.APIKey,
	}
}

// AgentModelConfig describes one allowed AI model with its context window size.
type AgentModelConfig struct {
	// Name is the model identifier (e.g. "deepseek-v3").
	Name string `yaml:"name"`
	// Provider references an AgentModelProvider.Name to select which LLM endpoint to use.
	// Empty means use the default provider ("aidev" section).
	Provider string `yaml:"provider"`
	// ContextWindow is the model's context window size in tokens.
	// 0 means use the framework's built-in lookup table or the default (8192).
	ContextWindow int `yaml:"contextWindow"`
}

// AgentModelGeneralConfig describes the model config for the AGUI agent.
type AgentModelGeneralConfig struct {
	// DefaultModel is the default LLM model identifier used by the agent.
	// When empty, the first model in AllowedModels is used as default.
	DefaultModel string `yaml:"defaultModel"`
	// Stream enables token-level streaming when calling the upstream LLM.
	// When true, each token is forwarded to the client as a separate TEXT_MESSAGE_CONTENT
	// SSE event, producing a real-time typewriter effect.
	// When false (default), the LLM response is returned as a single event after completion.
	Stream bool `yaml:"stream"`
	// DisplayReasoning enables the display of reasoning content in the response.
	DisplayReasoning bool `yaml:"displayReasoning"`
	// MaxTokens is the maximum number of tokens in the LLM response.
	MaxTokens int `yaml:"maxTokens"`
	// Temperature is the temperature of the LLM response.
	Temperature float64 `yaml:"temperature"`
	// Mode selects the agent implementation: "agent" (default) uses llmagent, "graph" uses graphagent.
	Mode enumor.AgentMode `yaml:"mode"`
}

// Validate validates the agent AGUI model configuration.
func (a *AgentModelGeneralConfig) Validate() error {
	if a.DefaultModel == "" {
		return fmt.Errorf("defaultModel must not be empty")
	}
	if err := a.Mode.Validate(); err != nil {
		return err
	}
	if a.MaxTokens <= 0 {
		return fmt.Errorf("maxTokens must be greater than 0")
	}
	if a.Temperature < 0 || a.Temperature > 1 {
		return fmt.Errorf("temperature must be between 0 and 1")
	}
	return nil
}

func (a *AgentModelGeneralConfig) trySetDefault() {
	if a.MaxTokens == 0 {
		a.MaxTokens = 38000
	}
	if a.Temperature == 0 {
		a.Temperature = 0.7
	}
	if a.Mode == "" {
		a.Mode = enumor.AgentModeAgent
	}
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
	// AllowedModels is the list of permitted AI models.
	// When empty, the platform default list (pkg/criteria/enumor.DefaultAllowedAIModels) is used.
	AllowedModels []AgentModelConfig `yaml:"allowedModels"`
	// Model is the model config for the AGUI agent.
	Model AgentModelGeneralConfig `yaml:"model"`
}

// Validate validates the agent AGUI configuration.
func (a *AgentAGUI) Validate() error {
	if err := a.Model.Validate(); err != nil {
		return err
	}
	return nil
}

func (a *AgentAGUI) trySetDefault() {
	a.Model.trySetDefault()
}

// AllowedModelNames returns the plain model name list (for backward-compatible call sites).
func (a AgentAGUI) AllowedModelNames() []string {
	// if no allowed models configured, use default allowed models
	if len(a.AllowedModels) == 0 {
		defaults := enumor.DefaultAllowedAIModels
		allowedModels := make([]string, len(defaults))
		for i, m := range defaults {
			allowedModels[i] = string(m)
		}
		return allowedModels
	}

	names := make([]string, len(a.AllowedModels))
	for i, m := range a.AllowedModels {
		names[i] = m.Name
	}
	return names
}

// ModelProviderMapping returns a map of model name → provider name for entries
// that have an explicit provider configured.
func (a AgentAGUI) ModelProviderMapping() map[string]string {
	m := make(map[string]string)
	for _, cfg := range a.AllowedModels {
		if cfg.Provider != "" {
			m[cfg.Name] = cfg.Provider
		}
	}
	return m
}

// ModelContextWindows returns a map of model name → context window for entries
// that have an explicit contextWindow > 0 configured.
func (a AgentAGUI) ModelContextWindows() map[string]int {
	m := make(map[string]int)
	for _, cfg := range a.AllowedModels {
		if cfg.ContextWindow > 0 {
			m[cfg.Name] = cfg.ContextWindow
		}
	}
	return m
}

// AgentServerSetting defines agent server used setting options.
type AgentServerSetting struct {
	Network   Network              `yaml:"network"`
	Service   Service              `yaml:"service"`
	Log       LogOption            `yaml:"log"`
	Providers []AgentModelProvider `yaml:"providers"`
	Storage   AgentStorage         `yaml:"storage"`
	Tools     AgentToolsConfig     `yaml:"tools"`
	AGUI      AgentAGUI            `yaml:"agui"`
	// Skills holds all skill configuration: filesystem paths and BKAIDev sync parameters.
	Skills *AgentBKAIDevSyncSkillsConfig `yaml:"skills"`
	// Prompt configures prompt files or BKAIDev-hosted prompt sync.
	Prompt AgentPromptConfig `yaml:"prompt"`
	// BKAIDevSyncAPIGateway holds the BKAIDev API gateway credentials shared by all
	// sync domains (skills, prompts, etc.).
	BKAIDevSyncAPIGateway ApiGateway `yaml:"bkaidevSyncApiGateway"`
}

// SkillSyncEnabled reports whether BKAIDev skill sync is turned on.
func (s *AgentServerSetting) SkillSyncEnabled() bool {
	return s.Skills != nil && s.Skills.Enabled
}

// PromptSyncEnabled reports whether BKAIDev prompt sync is turned on.
func (s *AgentServerSetting) PromptSyncEnabled() bool {
	return s.Prompt.BKAIDevSyncEnabled()
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
	s.Storage.trySetDefault()
	s.Tools.trySetDefault()
	if s.SkillSyncEnabled() {
		s.Skills.trySetDefault()
	}
	s.Prompt.trySetDefault()
}

// Validate AgentServerSetting option.
func (s AgentServerSetting) Validate() error {
	if err := s.Network.validate(); err != nil {
		return err
	}

	if err := s.Service.validate(); err != nil {
		return err
	}

	if err := s.AGUI.Validate(); err != nil {
		return err
	}

	if s.SkillSyncEnabled() || s.PromptSyncEnabled() {
		if err := s.BKAIDevSyncAPIGateway.validate(); err != nil {
			return err
		}
	}

	if s.SkillSyncEnabled() {
		if err := s.Skills.Validate(); err != nil {
			return err
		}
	}

	if err := s.Prompt.Validate(); err != nil {
		return fmt.Errorf("prompt: %w", err)
	}

	return nil
}

// TenantEnable returns false as agent-server does not support multi-tenancy.
func (s AgentServerSetting) TenantEnable() bool {
	return false
}

// GetProviders returns a map of provider name → provider config.
func (s AgentServerSetting) GetProviders() map[string]*AgentModelProvider {
	m := make(map[string]*AgentModelProvider)

	// Explicit providers.
	for _, p := range s.Providers {
		switch p.Type {
		case enumor.AgentModelProviderTypeOpenAI:
			m[p.Name] = p.ConvertToOpenAIProvider()
		default:
			// default to BK API gateway provider
			m[p.Name] = p.ConvertToBKAPIProvider()
		}
	}
	return m
}

// GetProvider returns the provider config by name.
func (s AgentServerSetting) GetProvider(providerName string) (*AgentModelProvider, error) {
	if providerName == "" {
		return nil, fmt.Errorf("provider name is empty")
	}

	if cfg, ok := s.GetProviders()[providerName]; ok {
		return cfg, nil
	}
	return nil, fmt.Errorf("provider %q not found", providerName)
}
