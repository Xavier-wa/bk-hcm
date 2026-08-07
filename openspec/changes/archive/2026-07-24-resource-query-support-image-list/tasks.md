## 1. 请求/响应类型与归一化

- [x] 1.1 在 cloud-server（或对应 api 包）定义业务维度镜像查询的类型化请求结构体（字段 `Platform`/`Name`/`Type`/`Region`/`Page`；`vendor` 为 path）与响应结构体，并补充 validator tag；**不**接受透传 filter
- [x] 1.2 实现 type 统一为 `public`/`private`/`shared`：同步入库归一化；查询只认统一值；`shared` 独立不算 public
- [x] 1.3 为归一化映射编写单元测试，覆盖 tcloud 大写枚举（含 `SHARED_IMAGE`→`shared`）与其他厂商 `public`

## 2. cloud-server 业务维度查询接口

- [x] 2.1 在 `cmd/cloud-server/service/image/` 新增 handler（如 `biz.go`），复用 `imageSvc`，实现业务维度镜像查询
- [x] 2.2 在 `init.go` 注册路由 `POST /bizs/{bk_biz_id}/vendors/{vendor}/images/list`
- [x] 2.3 组装 filter：vendor/platform/region 精确匹配、name 模糊匹配、type 经统一语义匹配
- [x] 2.4 实现业务可见性过滤：`type==public OR type==shared OR (type==private AND bk_biz_id)`
- [x] 2.5 实现 enable_cvm：仅当 vendor 为自研云时附加 `extension.enable_cvm==true`
- [x] 2.6 底层调用 data-service `Global.ListImage`，接入标准分页
- [x] 2.7 接入业务访问鉴权（复用 `imageSvc.authorizer`），防止越权查看他业务私有镜像

## 3. 接口测试

- [x] 3.1 补充接口单元测试，覆盖：公共/共享全可见、shared≠public、本业务私有可见、他业务私有不可见、自研云 enable_cvm 生效、其他厂商不受 enable_cvm 影响、各过滤维度、空数据返回空列表

## 4. 接口文档

- [x] 4.1 按 api-principle 新增/更新业务维度镜像查询接口文档；说明 type=`public|private|shared`、shared 全业务可见且不算 public

## 5. AI 助手能力接入（本仓库范围）

- [x] 5.1 在 `hcm-resource-search` skill references 新增 `list_biz_image.md`
- [x] 5.2 在 `SKILL.md` 的「references 索引」「计算」分类下登记 `list_biz_image.md`
- [x] 5.3 （另轨 / 非本仓）`hcm-res-manager` MCP 工具注册由负责人另行处理，不纳入本 change 完成标准

## 6. 一致性验证

- [x] 6.1 验证系统提示词「镜像」职责描述与实际能力一致（提示词无需改动；端到端依赖 MCP 另轨就绪后联调）
- [x] 6.2 验证「列举可查询资源」与「实际查询可用镜像列表」两个场景回复一致

## 7. 存量数据（本迭代仅记录）

- [ ] 7.1 存量镜像 type 刷数：`PUBLIC_IMAGE`→`public`，`PRIVATE_IMAGE`→`private`，`SHARED_IMAGE`→`shared`，历史小写保持（**上线前需完成；本迭代不实现脚本，仅记 task**）
