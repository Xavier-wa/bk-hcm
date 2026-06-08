### 描述

- 该接口提供版本：v1.6.1+。
- 该接口所需权限：业务访问。
- 该接口功能描述：资源回收单据预览。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/task/preview/recycle/order

### 输入参数

| 参数名称                  | 参数类型         | 必选 | 描述                                                                 |
|-----------------------|--------------|----|--------------------------------------------------------------------|
| ips	                  | string array | 否	 | 要查询的ip列表，数量最大500                                                   |
| asset_ids	            | string array | 否	 | 要查询的固资号列表，数量最大500                                                  |
| bk_host_ids           | int array	   | 否	 | 要查询的CC主机ID，数量最大500                                                 |
| remark	               | string	      | 否	 | 备注                                                                 |
| return_plan           | object	      | 否	 | 资源退回策略                                                             |
| skip_confirm          | bool	        | 否	 | 是否跳过二次确认。默认为false，不跳过                                              |
| recycle_type_sequence | string array | 否  | 回收项目的匹配优先级。例如：\['滚服项目', '短租项目'\] 表示优先匹配滚服项目，有剩余才匹配短租项目。默认即优先匹配滚服项目 |

#### return_plan

| 参数名称 | 参数类型    | 必选 | 描述                                                                           |
|------|---------|----|------------------------------------------------------------------------------|
| cvm	 | string	 | 否	 | cvm退回策略，默认值"IMMEDIATE"。"IMMEDIATE": 立即销毁, "DELAY": 延迟销毁（隔离观察15天，隔离期间费用仍由业务承担） |
| pm	  | string	 | 否  | 物理机退回策略，默认值"IMMEDIATE"。"IMMEDIATE": 立即销毁, "DELAY": 延迟销毁（隔离观察1天，隔离期间费用仍由业务承担） |

说明：

- ips、asset_ids和bk_host_ids不能同时为空

### 调用示例

#### 获取详细信息请求参数示例

```json
{
  "ips": [
    "10.0.0.1"
  ],
  "remark": "老旧替换",
  "return_plan": {
    "cvm": "IMMEDIATE",
    "pm": "IMMEDIATE"
  },
  "skip_confirm": true,
  "recycle_type_sequence": ["短租项目", "滚服项目"]
}
```

### 响应示例

#### 获取详细信息返回结果示例

```json
{
  "result": true,
  "code": 0,
  "message": "success",
  "data": {
    "count": 1,
    "info": [
      {
        "order_id": 1,
        "suborder_id": "1-1",
        "bk_biz_id": 2,
        "resource_type": "QCLOUDCVM",
        "recycle_type": "常规项目",
        "return_plan": "IMMEDIATE",
        "skip_confirm": true,
        "total_num": 20,
        "cost_concerned": true,
        "sum_cpu_core": 100,
        "remark": "老旧替换"
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型         | 描述                         |
|---------|--------------|----------------------------|
| result  | bool         | 请求成功与否。true:请求成功；false请求失败 |
| code    | int          | 错误编码。 0表示success，>0表示失败错误  |
| message | string       | 请求失败返回的错误信息                |
| data	   | object array | 响应数据                       |

#### data

| 参数名称 | 参数类型         | 描述         |
|------|--------------|------------|
| info | object array | 资源回收单据信息列表 |

#### data.info

| 参数名称           | 参数类型     | 描述                                                      |
|----------------|----------|---------------------------------------------------------|
| order_id       | int      | 资源回收单号                                                  |
| suborder_id	   | string	  | 资源回收子单号                                                 |
| bk_biz_id      | 	int     | CC业务ID                                                  |
| resource_type  | 	string  | 资源类型。"QCLOUDCVM": 腾讯云虚拟机, "IDCPM": IDC物理机, "OTHERS": 其他 |
| recycle_type   | 	string	 | 回收类型。"常规项目", "机房裁撤", "过保裁撤", "不区分"                      |
| return_plan    | 	string	 | 退回策略。"IMMEDIATE": 立即销毁, "DELAY": 延迟销毁                   |
| total_num      | 	int	    | 资源总数                                                    |
| cost_concerned | 	bool	   | 是否涉及回收成本                                                |
| sum_cpu_core   | 	int	    | 资源总CPU核数                                                |
| remark	        | string	  | 备注                                                      |
