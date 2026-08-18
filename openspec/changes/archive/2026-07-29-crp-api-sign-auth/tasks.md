## 1. 配置扩展

- [x] 1.1 `pkg/cc/ziyan_types.go` 的 `CVMCliConf` 新增 `ApiKey string \`yaml:"api_key"\``、`ApiSecret string \`yaml:"api_secret"\``，并在 `validate()` 中校验二者非空
- [x] 1.2 `pkg/cc/ziyan_types.go` 的 `Crp` 新增同样两个字段，并在 `validate()` 中校验非空
- [x] 1.3 `cmd/woa-server/etc/woa_server.yaml` 的 `cvm` 块补充 `api_key`、`api_secret` 样例值
- [x] 1.4 `cmd/hc-service/etc/hc_service.yaml` 的 `crp` 块补充 `api_key`、`api_secret` 样例值
- [ ] 1.5 【部署侧，随发版执行】各环境 values 的 `cvm` 块补充 `api_key`、`api_secret`。chart 默认 `values.yaml` 不含 `cvm` 等自研云第三方配置块，一律由环境 values 提供，本次不改动默认 values

## 2. 签名鉴权实现

- [x] 2.1 `pkg/thirdparty/cvmapi/constvar.go` 删除 `CvmApiKeyVal` 常量，新增 `CvmApiTs`、`CvmApiSign` query 参数名常量
- [x] 2.2 `pkg/thirdparty/cvmapi/constvar.go` 的 `CVMCli` 新增 `ApiKey`、`ApiSecret` 字段
- [x] 2.3 新增 `pkg/thirdparty/cvmapi/sign.go`，实现 `authParams()`（返回 api_key/api_ts/api_sign）与 `hmacSHA1Hex()`
- [x] 2.4 `cvmApi` 结构体持有 `apiKey`、`apiSecret`，`NewCVMClientInterface` 校验密钥非空后注入

## 3. 接口调用改造

- [x] 3.1 `pkg/thirdparty/cvmapi/cvmapi.go` 全部 34 处 `WithParam(CvmApiKey, CvmApiKeyVal)` 替换为 `WithParams(c.authParams())`

## 4. 调用方接线

- [x] 4.1 `pkg/thirdparty/thirdparty.go` 为 `CVM`、`OldCVM` 两个客户端注入 `opts.CvmOpt.ApiKey`、`opts.CvmOpt.ApiSecret`
- [x] 4.2 `pkg/thirdparty/thirdparty.go` 创建失败日志改为只打印地址，避免 secret 落盘
- [x] 4.3 `cmd/hc-service/service/service.go` 为 `crpCli` 注入 `cc.HCService().Crp.ApiKey`、`ApiSecret`
- [x] 4.4 `cmd/woa-server/logics/plan/fetcher/crp.go` 的 `UserName` 改为读取 `cc.WoaServer().ClientConfig.CvmOpt.ApiKey`

## 5. 测试与验证

- [x] 5.1 新增 `pkg/thirdparty/cvmapi/sign_test.go`，表驱动覆盖签名算法正确性（含已知向量）、参数完整性、密钥校验失败场景
- [x] 5.2 `go build ./...` 与 `go test ./pkg/thirdparty/cvmapi/...` 通过
