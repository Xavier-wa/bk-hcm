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

package es

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/tools/json"
	"hcm/pkg/tools/ssl"

	"github.com/olivere/elastic/v7"
)

// EsCli elasticsearch client
type EsCli struct {
	client *elastic.Client
}

// NewEsClient create es client
func NewEsClient(esConf cc.Es) (*EsCli, error) {
	httpClient := &http.Client{}
	if esConf.TLS.Enable() {
		tlsC, err := ssl.ClientTLSConfVerify(esConf.TLS.InsecureSkipVerify, esConf.TLS.CAFile, esConf.TLS.CertFile,
			esConf.TLS.KeyFile, esConf.TLS.Password)
		if err != nil {
			return nil, fmt.Errorf("init es tls config failed, err: %v", err)
		}

		httpClient.Transport = &http.Transport{TLSClientConfig: tlsC}
	}

	client, err := elastic.NewClient(
		elastic.SetHttpClient(httpClient),
		elastic.SetURL(esConf.Url),
		elastic.SetSniff(false),
		elastic.SetBasicAuth(esConf.User, esConf.Password))
	if err != nil {
		logs.Errorf("create new es client error, err: %v", err)
		return nil, err
	}

	return &EsCli{client: client}, nil
}

// SearchWithCond search with condition
func (es *EsCli) SearchWithCond(ctx context.Context, cond map[string][]interface{}, index string, start, limit int,
	sort string, fields []string) ([]Host, error) {

	query, err := es.buildQuery(cond)
	if err != nil {
		return nil, err
	}
	search, err := es.search(ctx, query, index, start, limit, sort, fields)
	if err != nil {
		return nil, err
	}

	result := make([]Host, len(search.Hits.Hits))
	for idx, hit := range search.Hits.Hits {
		var data Host
		err = json.Unmarshal(hit.Source, &data)
		if err != nil {
			return nil, err
		}

		data.AppName = cutCCPrefix(data.AppName)
		data.Module = cutCCPrefix(data.Module)
		result[idx] = data
	}

	return result, nil
}

// CountWithCond count with condition
func (es *EsCli) CountWithCond(ctx context.Context, cond map[string][]interface{}, index string) (int64, error) {
	query, err := es.buildQuery(cond)
	if err != nil {
		return 0, err
	}
	count, err := es.count(ctx, query, index)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (es *EsCli) buildQuery(cond map[string][]interface{}) (elastic.Query, error) {
	query := elastic.NewBoolQuery()
	query.Must(elastic.NewTermsQuery("cluster_id", 0))
	subOrQuery := elastic.NewBoolQuery()
	for k, v := range cond {
		// 当查询条件为操作人时，可以对主维护人或者备份维护人进行匹配
		if k == Operator {
			subQuery := elastic.NewBoolQuery()
			subQuery.Should(elastic.NewTermsQuery(ServerOperator, v...), elastic.NewTermsQuery(ServerBakOperator, v...))
			query.Must(subQuery)
			continue
		}

		if k == ModuleName || k == AssetID {
			subOrQuery.Should(elastic.NewTermsQuery(k, v...))
			continue
		}

		query.Must(elastic.NewTermsQuery(k, v...))
	}
	query.Must(subOrQuery)

	return query, nil
}

func (es *EsCli) search(ctx context.Context, query elastic.Query, index string, start, limit int, sort string,
	fields []string) (
	*elastic.SearchResult, error) {

	searchSource := elastic.NewSearchSource()
	searchSource.From(start)
	searchSource.Size(limit)
	searchSource.Sort(sort, true)

	// 指定要返回的字段
	if len(fields) > 0 {
		searchSource.FetchSourceContext(elastic.NewFetchSourceContext(true).Include(fields...))
	}

	searchResult, err := es.client.Search().
		Index(index).
		SearchSource(searchSource).
		Query(query).
		Collapse(elastic.NewCollapseBuilder("server_asset_id")). // 通过固资号去重，确保只返回同一台主机的一条数据
		Pretty(true).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	return searchResult, nil
}

// Count data in elastic with target conditions.
func (es *EsCli) count(ctx context.Context, query elastic.Query, index string) (int64, error) {
	searchResult, err := es.client.Search().
		Index(index).
		Query(query).
		Collapse(elastic.NewCollapseBuilder("server_asset_id")). // 通过固资号去重，确保只返回同一台主机的一条数据
		Aggregation("count", elastic.NewCardinalityAggregation().Field("server_asset_id")).
		Do(ctx)
	if err != nil {
		return 0, err
	}

	count, found := searchResult.Aggregations.Cardinality("count")
	if !found {
		return 0, nil
	}

	return int64(*count.Value), nil
}

// GetLatestIndex get elasticsearch lastest index
func GetLatestIndex() string {
	syncDate := time.Now().AddDate(0, 0, -1).Format(constant.DateLayout)
	return GetIndex(strings.Replace(syncDate, "-", "", -1))
}

// GetIndex get elasticsearch index
func GetIndex(date string) string {
	return indexPrefix + date
}

// cutCCPrefix cut off CC_ prefix
func cutCCPrefix(val string) string {
	val, _ = strings.CutPrefix(val, "CC_")
	return val
}

// AddCCPrefix add CC_ prefix
func AddCCPrefix(val string) string {
	return fmt.Sprintf("CC_%s", val)
}

type organization struct {
	department string
	center     string
	group      string
}

// getOrganization 根据传入的组织架构字符串，切割成对应组织架构的字段
// 如：A公司/B事业群/C部门/D中心/F组 会根据"/"进行拆分
// 要求切割的至少有3段，到“部门”纬度；最多有5段，到“组”纬度
func getOrganization(val string) (*organization, error) {
	arr := strings.Split(val, "/")
	if len(arr) < 3 || len(arr) > 5 {
		return nil, fmt.Errorf("organization: %v is invalid", val)
	}

	result := &organization{department: arr[2]}

	if len(arr) >= 4 {
		result.center = arr[3]
	}

	if len(arr) == 5 {
		result.group = arr[4]
	}

	return result, nil
}
