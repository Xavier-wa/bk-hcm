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

package framework

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"hcm/cmd/woa-server/service/data-migration/config"
	"hcm/cmd/woa-server/service/data-migration/converters"
	"hcm/cmd/woa-server/storage/dal"
	"hcm/pkg/client/data-service/tcloud-ziyan"
	"hcm/pkg/condition"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"

	"go.mongodb.org/mongo-driver/bson"
)

// MigrationEngine 通用迁移引擎
type MigrationEngine struct {
	mongo      dal.DB        // MongoDB客户端
	dataClient *ziyan.Client // Data-service客户端
	apiRouter  APIRouter     // API路由器接口
}

// APIRouter API路由器接口
type APIRouter interface {
	BatchCallCreateAPI(kt *kit.Kit, tableName string, dataList []interface{}) error
	BatchCallUpdateAPI(kt *kit.Kit, tableName string, dataList []interface{}) error
	BatchCallListAPI(kt *kit.Kit, tableName string, pkFields []string, pkValues []interface{}) (
		[]interface{}, error)
	BatchCallListByFilterAPI(kt *kit.Kit, tableName string, filterExpr *filter.Expression) ([]interface{}, error)
}

// NewMigrationEngine 创建迁移引擎
func NewMigrationEngine(mongo dal.DB, dataClient *ziyan.Client, apiRouter APIRouter) *MigrationEngine {
	return &MigrationEngine{
		mongo:      mongo,
		dataClient: dataClient,
		apiRouter:  apiRouter,
	}
}

// Migrate 执行迁移
func (e *MigrationEngine) Migrate(kt *kit.Kit, req *MigrationRequest) (*MigrationResult, error) {
	startTime := time.Now()
	direction := resolveDirection(req.Direction)
	taskID := resolveTaskID(req, startTime)
	req.TaskID = taskID

	// 1. 获取配置
	cfg, err := config.Get(req.TableName)
	if err != nil {
		return nil, fmt.Errorf("get table config failed: %w", err)
	}

	// 2. 循环依赖检测（防止无限递归）
	cleanup, err := e.checkCircularDependency(kt, req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	logs.Infof("[MigrationEngine:Migrate] start migration for table: %s (mode: %s, direction: %s, task_id: %s), "+
		"visited path: %v, cfg: %+v, rid: %s", cfg.Name, req.Mode, direction, taskID, req.getVisitedPath(),
		cvt.PtrToVal(cfg), kt.Rid)

	// 3. 获取转换器
	converter, err := converters.Get(cfg.ConverterName)
	if err != nil {
		return nil, fmt.Errorf("get converter failed: %w", err)
	}

	// 4. 查询源数据
	sourceData, err := e.querySourceDataByDirection(kt, cfg, req, converter)
	if err != nil {
		logs.Errorf("[MigrationEngine:Migrate] query source data failed, table: %s, direction: %s, task_id: %s, "+
			"err: %v, rid: %s", req.TableName, direction, taskID, err, kt.Rid)
		return nil, fmt.Errorf("query source data failed, table: %s, err: %w", req.TableName, err)
	}

	logs.Infof("[MigrationEngine:Migrate] found %d records, source_table: %s, direction: %s, task_id: %s, rid: %s",
		len(sourceData), e.getSourceTableByDirection(cfg, direction), direction, taskID, kt.Rid)

	result := &MigrationResult{
		Total:     len(sourceData),
		StartTime: startTime,
		Errors:    make([]*ErrorDetail, 0),
	}

	// 5. 分批并发处理
	batchSize := resolveBatchSize(req.BatchSize, cfg.BatchSize)
	concurrency := resolveConcurrency(req.Concurrency, cfg.Concurrency)
	batches := e.splitIntoBatches(sourceData, batchSize)

	logs.Infof("[MigrationEngine:Migrate] Processing %d batches (batch_size: %d, concurrency: %d), table: %s, rid: %s",
		len(batches), batchSize, concurrency, req.TableName, kt.Rid)

	result = e.processBatchesConcurrently(kt, batches, cfg, converter, req, concurrency)

	// 6. 处理依赖表（级联迁移）
	if direction == MigrationDirectionForward && req.IncludeDependencies && len(cfg.Dependencies) > 0 {
		logs.Infof("[MigrationEngine:Migrate] Processing %d dependencies, table: %s, rid: %s",
			len(cfg.Dependencies), req.TableName, kt.Rid)
		depResult := e.migrateDependencies(kt, cfg, sourceData, req)
		result.MergeDependencies(depResult)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	logs.Infof("[MigrationEngine:Migrate] migration completed, table: %s, direction: %s, task_id: %s, total:%d, "+
		"created:%d, updated:%d, skipped:%d, failed:%d, duration:%.3fs, rid: %s", req.TableName, direction, taskID,
		result.Total, result.Created, result.Updated, result.Skipped, result.Failed, result.Duration.Seconds(), kt.Rid)

	return result, nil
}

func resolveDirection(direction MigrationDirection) MigrationDirection {
	if len(direction) == 0 {
		return MigrationDirectionForward
	}
	return direction
}

func resolveTaskID(req *MigrationRequest, startTime time.Time) string {
	if len(req.TaskID) > 0 {
		return req.TaskID
	}
	return fmt.Sprintf("%s-%d", req.TableName, startTime.UnixNano())
}

// checkCircularDependency 循环依赖检测，返回清理函数
func (e *MigrationEngine) checkCircularDependency(kt *kit.Kit, req *MigrationRequest) (func(), error) {
	if req.visitedTables == nil {
		req.visitedTables = make(map[string]bool)
	}
	if req.visitedTables[req.TableName] {
		logs.Errorf("[MigrationEngine:Migrate] Circular dependency detected! Table: %s, visited path: %v, rid: %s",
			req.TableName, req.getVisitedPath(), kt.Rid)
		return nil, fmt.Errorf("circular dependency detected: table %s is already being migrated", req.TableName)
	}
	// 标记当前表为正在访问
	req.visitedTables[req.TableName] = true
	// 返回清理函数（支持同一个表在不同分支中被访问）
	return func() { delete(req.visitedTables, req.TableName) }, nil
}

// resolveBatchSize 确定批次大小
func resolveBatchSize(reqBatchSize, cfgBatchSize int) int {
	if reqBatchSize > 0 {
		return reqBatchSize
	}
	if cfgBatchSize > 0 {
		return cfgBatchSize
	}
	return 100
}

// resolveConcurrency 确定并发数
func resolveConcurrency(reqConcurrency, cfgConcurrency int) int {
	concurrency := reqConcurrency
	if concurrency <= 0 {
		concurrency = cfgConcurrency
	}
	if concurrency <= 0 {
		concurrency = 1 // 默认串行
	}
	if concurrency > 10 {
		concurrency = 10 // 最大并发数限制
	}
	return concurrency
}

// querySourceData 查询源数据（分页获取，避免内存溢出）
func (e *MigrationEngine) querySourceData(kt *kit.Kit, cfg *config.TableMigrationConfig,
	filter MigrationFilter, converter converters.DataConverter) ([]interface{}, error) {

	// 构建MongoDB查询条件
	mongoFilter := e.buildMongoFilter(filter)

	// 查询数据
	collection := e.mongo.Table(cfg.SourceTable)

	// 分页参数
	const pageSize = 1000 // 每页1000条数据
	var allResults []interface{}
	page := 0

	logs.Infof("[MigrationEngine:querySourceData] Start fetching data with pagination (pageSize: %d), "+
		"cfgName: %s, mongoFilter: %+v, rid: %s", pageSize, cfg.Name, mongoFilter, kt.Rid)

	for {
		// 使用转换器创建正确类型的切片
		resultsPtr := converter.NewSourceDataSlice()

		// 使用Find + Start + Limit方式分页查询
		finder := collection.Find(mongoFilter).Start(uint64(page * pageSize)).Limit(uint64(pageSize))

		if err := finder.All(kt.Ctx, resultsPtr); err != nil {
			logs.Errorf("[MigrationEngine:querySourceData] fetch data from mongo failed, cfgName: %s, "+
				"page: %d, err: %v, rid: %s", cfg.Name, page, err, kt.Rid)
			return nil, fmt.Errorf("fetch data from mongo failed: %w", err)
		}

		// 将具体类型的切片转换为 []interface{}
		pageResults := e.convertToInterfaceSlice(resultsPtr)

		// 如果当前页没有数据，说明已经查询完毕
		if len(pageResults) == 0 {
			break
		}

		// 追加到总结果集
		allResults = append(allResults, pageResults...)

		logs.Infof("[MigrationEngine:querySourceData] Fetched page %d: %d records, total so far: %d, rid: %s",
			page+1, len(pageResults), len(allResults), kt.Rid)

		if len(pageResults) < pageSize {
			break
		}
		page++
	}

	// 记录查询结果日志
	logs.Infof("[MigrationEngine:querySourceData] fetch data from mongo succeeded: %d records in %d pages, rid: %s",
		len(allResults), page+1, kt.Rid)

	return allResults, nil
}

// querySourceDataByDirection 按方向查询源数据
func (e *MigrationEngine) querySourceDataByDirection(kt *kit.Kit, cfg *config.TableMigrationConfig,
	req *MigrationRequest, converter converters.DataConverter) ([]interface{}, error) {

	direction := resolveDirection(req.Direction)
	if direction == MigrationDirectionReverse {
		return e.querySourceDataFromMySQL(kt, cfg, req.Filter)
	}
	return e.querySourceData(kt, cfg, req.Filter, converter)
}

// querySourceDataFromMySQL 查询MySQL源数据（反向同步）
func (e *MigrationEngine) querySourceDataFromMySQL(kt *kit.Kit, cfg *config.TableMigrationConfig,
	filterCondition MigrationFilter) ([]interface{}, error) {

	filterExpr, err := e.buildMySQLFilter(filterCondition)
	if err != nil {
		return nil, fmt.Errorf("build mysql filter failed: %w", err)
	}

	result, err := e.apiRouter.BatchCallListByFilterAPI(kt, cfg.TargetTable, filterExpr)
	if err != nil {
		return nil, fmt.Errorf("list mysql data failed: %w", err)
	}

	logs.Infof("[MigrationEngine:querySourceDataFromMySQL] list mysql data success, table: %s, count: %d, rid: %s",
		cfg.TargetTable, len(result), kt.Rid)
	return result, nil
}

func (e *MigrationEngine) getSourceTableByDirection(cfg *config.TableMigrationConfig,
	direction MigrationDirection) string {

	if direction == MigrationDirectionReverse {
		return cfg.TargetTable
	}
	return cfg.SourceTable
}

// buildMongoFilter 构建MongoDB查询条件
func (e *MigrationEngine) buildMongoFilter(filter MigrationFilter) bson.M {
	mongoFilter := bson.M{}

	// 时间范围过滤
	if filter.TimeRange != nil {
		timeFilter := bson.M{}
		if filter.TimeRange.StartTime != nil {
			timeFilter["$gte"] = *filter.TimeRange.StartTime
		}
		if filter.TimeRange.EndTime != nil {
			timeFilter["$lte"] = *filter.TimeRange.EndTime
		}
		if len(timeFilter) > 0 {
			mongoFilter[filter.TimeRange.Field] = timeFilter
		}
	}

	// 字段过滤
	for field, value := range filter.Fields {
		mongoFilter[field] = value
	}

	// 自定义查询（优先级最高）
	if len(filter.CustomQuery) > 0 {
		for k, v := range filter.CustomQuery {
			mongoFilter[k] = v
		}
	}

	return mongoFilter
}

// buildMySQLFilter 构建MySQL过滤条件
func (e *MigrationEngine) buildMySQLFilter(filterCondition MigrationFilter) (*filter.Expression, error) {
	rules := make([]filter.RuleFactory, 0)

	if filterCondition.TimeRange != nil {
		if filterCondition.TimeRange.StartTime != nil {
			rules = append(rules, &filter.AtomRule{
				Field: filterCondition.TimeRange.Field,
				Op:    filter.GreaterThanEqual.Factory(),
				Value: *filterCondition.TimeRange.StartTime,
			})
		}
		if filterCondition.TimeRange.EndTime != nil {
			rules = append(rules, &filter.AtomRule{
				Field: filterCondition.TimeRange.Field,
				Op:    filter.LessThanEqual.Factory(),
				Value: *filterCondition.TimeRange.EndTime,
			})
		}
	}

	for field, value := range filterCondition.Fields {
		fieldRules, err := e.buildFieldFilterRules(field, value)
		if err != nil {
			return nil, err
		}
		rules = append(rules, fieldRules...)
	}

	if len(rules) == 0 {
		return nil, nil
	}
	return &filter.Expression{Op: filter.And, Rules: rules}, nil
}

func (e *MigrationEngine) buildFieldFilterRules(field string, value interface{}) ([]filter.RuleFactory, error) {
	if opMap, ok := value.(map[string]interface{}); ok {
		rules := make([]filter.RuleFactory, 0, len(opMap))
		for op, opValue := range opMap {
			switch op {
			case condition.BKDBGTE:
				rules = append(rules, &filter.AtomRule{
					Field: field, Op: filter.GreaterThanEqual.Factory(), Value: opValue,
				})
			case condition.BKDBLTE:
				rules = append(rules, &filter.AtomRule{
					Field: field, Op: filter.LessThanEqual.Factory(), Value: opValue,
				})
			case condition.BKDBGT:
				rules = append(rules, &filter.AtomRule{Field: field, Op: filter.GreaterThan.Factory(), Value: opValue})
			case condition.BKDBLT:
				rules = append(rules, &filter.AtomRule{Field: field, Op: filter.LessThan.Factory(), Value: opValue})
			case condition.BKDBNE:
				rules = append(rules, &filter.AtomRule{Field: field, Op: filter.NotEqual.Factory(), Value: opValue})
			case condition.BKDBIN:
				rules = append(rules, &filter.AtomRule{Field: field, Op: filter.In.Factory(), Value: opValue})
			default:
				return nil, fmt.Errorf("unsupported filter operator %s for field %s", op, field)
			}
		}
		return rules, nil
	}

	if reflect.TypeOf(value) != nil && reflect.TypeOf(value).Kind() == reflect.Slice {
		return []filter.RuleFactory{
			&filter.AtomRule{Field: field, Op: filter.In.Factory(), Value: value},
		}, nil
	}

	return []filter.RuleFactory{
		&filter.AtomRule{Field: field, Op: filter.Equal.Factory(), Value: value},
	}, nil
}

// processBatch 处理一批数据
func (e *MigrationEngine) processBatch(kt *kit.Kit, batch []interface{}, cfg *config.TableMigrationConfig,
	converter converters.DataConverter, req *MigrationRequest) *MigrationResult {

	result := &MigrationResult{
		Errors: make([]*ErrorDetail, 0),
	}

	// 1. 提取所有主键
	pkValues, extractResult := e.extractPrimaryKeys(kt, batch, cfg, converter)
	result.Merge(extractResult)
	if len(pkValues) == 0 {
		return result
	}

	// 2. 批量查询已存在的数据
	existingMap, err := e.queryExistingData(kt, cfg, pkValues, converter)
	if err != nil {
		result.Failed += len(batch)
		result.Errors = append(result.Errors, &ErrorDetail{
			Table:  cfg.TargetTable,
			Action: "batch_list",
			Error:  fmt.Sprintf("batch query failed: %v", err),
		})
		return result
	}

	// 3. 分类处理数据
	toCreate, toUpdate := e.classifyBatchData(kt, batch, cfg, converter, req, existingMap, result)

	// 记录操作的数据量
	logs.Infof("[MigrationEngine:Migrate] processBatch:classifyBatchData, existingMap size: %d, toCreate count: %d, "+
		"toUpdate count: %d, rid: %s", len(existingMap), len(toCreate), len(toUpdate), kt.Rid)

	// 4. 执行批量创建
	e.executeBatchCreate(kt, cfg, req, toCreate, result)

	// 5. 执行批量更新
	e.executeBatchUpdate(kt, cfg, req, toUpdate, result)

	result.Success = result.Created + result.Updated + result.Skipped

	return result
}

// processReverseBatch 处理一批反向同步数据（mysql -> mongodb）
func (e *MigrationEngine) processReverseBatch(kt *kit.Kit, batch []interface{}, cfg *config.TableMigrationConfig,
	converter converters.DataConverter, req *MigrationRequest) *MigrationResult {

	result := &MigrationResult{Errors: make([]*ErrorDetail, 0)}
	for _, sourceItem := range batch {
		e.processSingleReverseItem(kt, sourceItem, cfg, converter, req, result)
	}

	result.Success = result.Created + result.Updated + result.Skipped
	logs.Infof("[MigrationEngine:processReverseBatch] batch processed, table: %s, task_id: %s, created:%d, "+
		"updated:%d, skipped:%d, failed:%d, rid: %s", cfg.SourceTable, req.TaskID, result.Created, result.Updated,
		result.Skipped, result.Failed, kt.Rid)
	return result
}

func (e *MigrationEngine) processSingleReverseItem(kt *kit.Kit, sourceItem interface{},
	cfg *config.TableMigrationConfig, converter converters.DataConverter, req *MigrationRequest,
	result *MigrationResult) {

	pk, err := converter.ExtractPrimaryKey(sourceItem)
	if err != nil {
		e.appendReverseError(kt, result, cfg.SourceTable, "extract_pk", err, nil)
		return
	}

	exists, shouldSkip := e.checkReverseItemState(kt, sourceItem, cfg, converter, req, result, pk)
	if shouldSkip {
		return
	}

	mongoDoc, err := e.buildMongoDocFromMySQL(cfg, sourceItem)
	if err != nil {
		e.appendReverseError(kt, result, cfg.SourceTable, "convert_to_mongo", err, pk)
		return
	}

	if req.DryRun {
		e.increaseReverseResult(result, exists)
		return
	}

	pkFilter := e.buildMongoPKFilter(pk, cfg)
	if err = e.mongo.Table(cfg.SourceTable).Upsert(kt.Ctx, pkFilter, mongoDoc); err != nil {
		e.appendReverseError(kt, result, cfg.SourceTable, "upsert_mongo", err, pk)
		return
	}

	e.increaseReverseResult(result, exists)
}

func (e *MigrationEngine) checkReverseItemState(kt *kit.Kit, sourceItem interface{}, cfg *config.TableMigrationConfig,
	converter converters.DataConverter, req *MigrationRequest, result *MigrationResult,
	pk interface{}) (bool, bool) {

	existingItem, exists, err := e.queryExistingMongoByPrimaryKey(kt, cfg, converter, pk)
	if err != nil {
		e.appendReverseError(kt, result, cfg.SourceTable, "query_existing", err, pk)
		return false, true
	}

	if exists {
		if req.Mode == MigrationModeCreateOnly {
			result.Skipped++
			return true, true
		}

		isEqual, _ := converter.CompareData(existingItem, sourceItem, cfg.CompareFields)
		if isEqual {
			result.Skipped++
			return true, true
		}
		return true, false
	}

	if req.Mode == MigrationModeUpdateOnly {
		result.Skipped++
		return false, true
	}
	return false, false
}

func (e *MigrationEngine) appendReverseError(kt *kit.Kit, result *MigrationResult, tableName, action string,
	err error, pk interface{}) {

	if action == "extract_pk" {
		logs.Errorf("[MigrationEngine:processReverseBatch] extract primary key failed, err: %v, rid: %s", err, kt.Rid)
	} else {
		logs.Errorf("[MigrationEngine:processReverseBatch] %s failed, pk: %v, err: %v, rid: %s",
			action, pk, err, kt.Rid)
	}

	result.Failed++
	errorDetail := &ErrorDetail{Table: tableName, Action: action, Error: err.Error()}
	if pk != nil {
		errorDetail.PrimaryKey = pk
	}
	result.Errors = append(result.Errors, errorDetail)
}

func (e *MigrationEngine) increaseReverseResult(result *MigrationResult, exists bool) {
	if exists {
		result.Updated++
		return
	}
	result.Created++
}

func (e *MigrationEngine) queryExistingMongoByPrimaryKey(kt *kit.Kit, cfg *config.TableMigrationConfig,
	converter converters.DataConverter, pk interface{}) (interface{}, bool, error) {
	filterCondition := e.buildMongoPKFilter(pk, cfg)
	itemPtr, err := newSourceDataItem(converter)
	if err != nil {
		return nil, false, err
	}

	err = e.mongo.Table(cfg.SourceTable).Find(filterCondition).One(kt.Ctx, itemPtr)
	if err != nil {
		if e.mongo.IsNotFoundError(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return itemPtr, true, nil
}

func (e *MigrationEngine) buildMongoPKFilter(pk interface{}, cfg *config.TableMigrationConfig) bson.M {
	conditions := make(bson.M)
	if pkMap, ok := pk.(map[string]interface{}); ok {
		for _, field := range cfg.GetSourcePrimaryKeys() {
			conditions[field] = pkMap[field]
		}
		return conditions
	}

	pkFields := cfg.GetSourcePrimaryKeys()
	if len(pkFields) > 0 {
		conditions[pkFields[0]] = pk
	}
	return conditions
}

func newSourceDataItem(converter converters.DataConverter) (interface{}, error) {
	slicePtr := converter.NewSourceDataSlice()
	sliceType := reflect.TypeOf(slicePtr)
	if sliceType.Kind() != reflect.Ptr || sliceType.Elem().Kind() != reflect.Slice {
		return nil, fmt.Errorf("invalid NewSourceDataSlice type: %T", slicePtr)
	}

	elemType := sliceType.Elem().Elem()
	if elemType.Kind() == reflect.Ptr {
		return reflect.New(elemType.Elem()).Interface(), nil
	}
	return reflect.New(elemType).Interface(), nil
}

// extractPrimaryKeys 提取所有主键
func (e *MigrationEngine) extractPrimaryKeys(kt *kit.Kit, batch []interface{},
	cfg *config.TableMigrationConfig, converter converters.DataConverter) ([]interface{}, *MigrationResult) {

	result := &MigrationResult{
		Errors: make([]*ErrorDetail, 0),
	}

	pkValues := make([]interface{}, 0, len(batch))
	for _, item := range batch {
		pk, err := converter.ExtractPrimaryKey(item)
		if err != nil {
			logs.Errorf("[MigrationEngine:Migrate] extractPrimaryKeys extract primary key failed, "+
				"err: %v, cfgName: %s, item: %+v, rid: %s", err, cfg.Name, item, kt.Rid)
			result.Failed++
			result.Errors = append(result.Errors, &ErrorDetail{
				Table:  cfg.TargetTable,
				Action: "extract_pk",
				Error:  err.Error(),
			})
			continue
		}
		pkValues = append(pkValues, pk)
		logs.Infof("[MigrationEngine:Migrate] extractPrimaryKeys, pk: %v, cfgName: %s, item: %+v, rid: %s",
			pk, cfg.Name, item, kt.Rid)
	}

	return pkValues, result
}

// queryExistingData 查询已存在的数据（支持单字段和复合主键）
func (e *MigrationEngine) queryExistingData(kt *kit.Kit, cfg *config.TableMigrationConfig,
	pkValues []interface{}, converter converters.DataConverter) (map[string]interface{}, error) {

	// 支持复合主键查询
	pkFields := cfg.GetTargetPrimaryKeys()
	existingDataList, err := e.apiRouter.BatchCallListAPI(kt, cfg.TargetTable, pkFields, pkValues)
	if err != nil {
		logs.Errorf("[MigrationEngine:queryExistingData] BatchCallListAPI failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return e.buildExistingDataMap(kt, existingDataList, cfg, converter), nil
}

// classifyBatchData 分类处理数据（判断创建还是更新）
func (e *MigrationEngine) classifyBatchData(kt *kit.Kit, batch []interface{},
	cfg *config.TableMigrationConfig, converter converters.DataConverter,
	req *MigrationRequest, existingMap map[string]interface{},
	result *MigrationResult) ([]interface{}, []interface{}) {

	toCreate := make([]interface{}, 0)
	toUpdate := make([]interface{}, 0)

	for _, sourceItem := range batch {
		pk, err := converter.ExtractPrimaryKey(sourceItem)
		if err != nil {
			logs.Warnf("[MigrationEngine:classifyBatchData] extract primary key failed, err: %v, sourceItem: %+v, "+
				"rid: %s", err, sourceItem, kt.Rid)
			continue
		}

		// 构建Map的key（支持单字段和复合主键）
		pkKey := e.buildPrimaryKeyString(pk, cfg)

		if targetItem, exists := existingMap[pkKey]; exists {
			e.handleExistingItem(kt, cfg, converter, req, sourceItem, targetItem, pk, &toUpdate, result)
		} else {
			e.handleNewItem(kt, cfg, converter, req, sourceItem, &toCreate, result)
		}
	}

	return toCreate, toUpdate
}

// handleExistingItem 处理已存在的数据项
func (e *MigrationEngine) handleExistingItem(kt *kit.Kit, cfg *config.TableMigrationConfig,
	converter converters.DataConverter, req *MigrationRequest,
	sourceItem, targetItem interface{}, pk interface{},
	toUpdate *[]interface{}, result *MigrationResult) {

	// 已存在，判断是否需要更新
	if req.Mode == MigrationModeCreateOnly {
		result.Skipped++
		return
	}

	// 对比数据
	isEqual, diffFields := converter.CompareData(sourceItem, targetItem, cfg.CompareFields)
	if !isEqual {
		updateReq, err := converter.ConvertToUpdate(sourceItem, targetItem)
		if err != nil {
			logs.Errorf("[MigrationEngine:handleExistingItem] convert to update failed: %v, rid: %s", err, kt.Rid)
			result.Failed++
			return
		}
		*toUpdate = append(*toUpdate, updateReq)
		logs.Infof("[MigrationEngine:handleExistingItem] need update, pk: %v, diff_fields: %v, rid: %s",
			pk, diffFields, kt.Rid)
	} else {
		result.Skipped++
	}
}

// handleNewItem 处理新数据项
func (e *MigrationEngine) handleNewItem(kt *kit.Kit, cfg *config.TableMigrationConfig,
	converter converters.DataConverter, req *MigrationRequest,
	sourceItem interface{}, toCreate *[]interface{}, result *MigrationResult) {

	// 不存在，需要创建
	if req.Mode == MigrationModeUpdateOnly {
		result.Skipped++
		return
	}

	createReq, err := converter.ConvertToCreate(sourceItem)
	if err != nil {
		logs.Errorf("[MigrationEngine:handleNewItem] convert to create failed: %v, rid: %s", err, kt.Rid)
		result.Failed++
		return
	}
	*toCreate = append(*toCreate, createReq)
}

// executeBatchCreate 执行批量创建
func (e *MigrationEngine) executeBatchCreate(kt *kit.Kit, cfg *config.TableMigrationConfig,
	req *MigrationRequest, toCreate []interface{}, result *MigrationResult) {

	if len(toCreate) == 0 {
		logs.Infof("[MigrationEngine:executeBatchCreate] no data to need create, targetTable: %s, rid: %s",
			cfg.TargetTable, kt.Rid)
		return
	}

	if req.DryRun {
		logs.Infof("[MigrationEngine:executeBatchCreate] [DryRun] Would create %d records for table: %s, rid: %s",
			len(toCreate), cfg.TargetTable, kt.Rid)
		result.Created += len(toCreate)
		return
	}

	logs.Infof("[MigrationEngine:executeBatchCreate] Batch creating %d records for table: %s, toCreate: %+v, rid: %s",
		len(toCreate), cfg.TargetTable, toCreate, kt.Rid)

	if err := e.apiRouter.BatchCallCreateAPI(kt, cfg.TargetTable, toCreate); err != nil {
		logs.Errorf("[MigrationEngine:executeBatchCreate] BatchCallCreateAPI failed, err: %v, rid: %s", err, kt.Rid)
		result.Failed += len(toCreate)
		result.Errors = append(result.Errors, &ErrorDetail{
			Table:  cfg.TargetTable,
			Action: "batch_create",
			Error:  err.Error(),
		})
	} else {
		result.Created += len(toCreate)
	}
}

// executeBatchUpdate 执行批量更新
func (e *MigrationEngine) executeBatchUpdate(kt *kit.Kit, cfg *config.TableMigrationConfig,
	req *MigrationRequest, toUpdate []interface{}, result *MigrationResult) {

	if len(toUpdate) == 0 {
		logs.Infof("[MigrationEngine:executeBatchUpdate] no data to need update, targetTable %s, rid: %s",
			cfg.TargetTable, kt.Rid)
		return
	}

	if req.DryRun {
		logs.Infof("[MigrationEngine:executeBatchUpdate] [DryRun] Would update %d records for table %s, rid: %s",
			len(toUpdate), cfg.TargetTable, kt.Rid)
		result.Updated += len(toUpdate)
		return
	}

	logs.Infof("[MigrationEngine:executeBatchUpdate] Batch updating %d records for table %s, rid: %s",
		len(toUpdate), cfg.TargetTable, kt.Rid)

	if err := e.apiRouter.BatchCallUpdateAPI(kt, cfg.TargetTable, toUpdate); err != nil {
		logs.Errorf("[MigrationEngine:executeBatchUpdate] BatchCallUpdateAPI failed, err: %v, rid: %s", err, kt.Rid)
		result.Failed += len(toUpdate)
		result.Errors = append(result.Errors, &ErrorDetail{
			Table:  cfg.TargetTable,
			Action: "batch_update",
			Error:  err.Error(),
		})
	} else {
		result.Updated += len(toUpdate)
	}
}

// processBatchesConcurrently 并发处理多个批次
func (e *MigrationEngine) processBatchesConcurrently(kt *kit.Kit, batches [][]interface{},
	cfg *config.TableMigrationConfig, converter converters.DataConverter,
	req *MigrationRequest, concurrency int) *MigrationResult {

	totalResult := &MigrationResult{
		TableName: cfg.TargetTable,
		StartTime: time.Now(),
	}

	if len(batches) == 0 {
		return totalResult
	}

	// 1. 创建任务通道和结果通道
	type batchTask struct {
		index int
		batch []interface{}
	}
	taskChan := make(chan batchTask, len(batches))
	resultChan := make(chan *MigrationResult, len(batches))

	// 2. 启动工作协程池
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for task := range taskChan {
				logs.Infof("[MigrationEngine:processBatchesConcurrently] worker-%d processing batch %d/%d "+
					"(size: %d, task_id: %s), rid: %s", workerID, task.index+1, len(batches), len(task.batch),
					req.TaskID, kt.Rid)

				// 处理批次
				var batchResult *MigrationResult
				if resolveDirection(req.Direction) == MigrationDirectionReverse {
					batchResult = e.processReverseBatch(kt, task.batch, cfg, converter, req)
				} else {
					batchResult = e.processBatch(kt, task.batch, cfg, converter, req)
				}
				batchResult.BatchIndex = task.index + 1
				resultChan <- batchResult

				logs.Infof("[MigrationEngine:processBatchesConcurrently] worker-%d completed batch %d/%d "+
					"(task_id: %s), rid: %s", workerID, task.index+1, len(batches), req.TaskID, kt.Rid)
			}
		}(i)
	}

	// 3. 分发任务
	go func() {
		for i, batch := range batches {
			taskChan <- batchTask{index: i, batch: batch}
		}
		close(taskChan)
	}()

	// 4. 等待所有工作协程完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	logs.Infof("[MigrationEngine:processBatchesConcurrently] Waiting for %d batches to complete, rid: %s",
		len(batches), kt.Rid)
	// 5. 收集结果
	var resultMutex sync.Mutex
	for batchResult := range resultChan {
		resultMutex.Lock()
		totalResult.Merge(batchResult)
		resultMutex.Unlock()
	}

	return totalResult
}

// migrateDependencies 迁移依赖表（级联）
func (e *MigrationEngine) migrateDependencies(kt *kit.Kit, parentCfg *config.TableMigrationConfig,
	parentSourceData []interface{}, req *MigrationRequest) map[string]*MigrationResult {

	dependencyResults := make(map[string]*MigrationResult)

	for _, dep := range parentCfg.Dependencies {
		logs.Infof("[MigrationEngine:migrateDependencies] Processing dependency: %s (relation: %s), rid: %s",
			dep.ConfigName, dep.RelationType, kt.Rid)

		// 1. 根据关联类型构建过滤条件
		depFilter := e.buildDependencyFilter(parentSourceData, dep, parentCfg)

		// 2. 递归迁移依赖表
		depReq := &MigrationRequest{
			TableName:           dep.ConfigName,
			Filter:              depFilter,
			Mode:                req.Mode,
			DryRun:              req.DryRun,
			IncludeDependencies: true, // 支持多级依赖
			BatchSize:           req.BatchSize,
			Concurrency:         req.Concurrency,
			visitedTables:       req.visitedTables, // 传递已访问表的追踪信息（关键：支持循环依赖检测）
		}

		depResult, err := e.Migrate(kt, depReq)
		if err != nil {
			logs.Errorf("[MigrationEngine:migrateDependencies] Failed to migrate dependency %s: %v, rid: %s",
				dep.ConfigName, err, kt.Rid)
			// 记录错误但继续处理其他依赖
			dependencyResults[dep.ConfigName] = &MigrationResult{
				TableName: dep.ConfigName,
				Failed:    1,
				Errors: []*ErrorDetail{{
					Table:  dep.ConfigName,
					Action: "migrate_dependency",
					Error:  err.Error(),
				}},
			}
			continue
		}

		dependencyResults[dep.ConfigName] = depResult
	}

	return dependencyResults
}

// buildDependencyFilter 根据主表数据构建子表过滤条件
func (e *MigrationEngine) buildDependencyFilter(parentData []interface{},
	dep *config.DependencyConfig,
	parentCfg *config.TableMigrationConfig) MigrationFilter {

	switch dep.RelationType {
	case "one_to_many":
		// 一对多：主表的主键 = 子表的外键
		foreignKeyValues := make([]interface{}, 0, len(parentData))

		converter, err := converters.Get(parentCfg.ConverterName)
		if err != nil {
			logs.Errorf("[MigrationEngine:migrateDependencies] get converter failed: %v", err)
			return MigrationFilter{}
		}

		for _, item := range parentData {
			pkValue, err := converter.ExtractPrimaryKey(item)
			if err != nil {
				continue
			}
			foreignKeyValues = append(foreignKeyValues, pkValue)
		}

		return MigrationFilter{
			Fields: map[string]interface{}{
				dep.ForeignKey: bson.M{"$in": foreignKeyValues},
			},
		}

	default:
		return MigrationFilter{}
	}
}

// splitIntoBatches 分批
func (e *MigrationEngine) splitIntoBatches(items []interface{}, batchSize int) [][]interface{} {
	if len(items) == 0 {
		return [][]interface{}{}
	}

	batches := make([][]interface{}, 0)

	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	return batches
}

// buildExistingDataMap 构建已存在数据的映射（支持单字段和复合主键）
func (e *MigrationEngine) buildExistingDataMap(kt *kit.Kit, dataList []interface{}, cfg *config.TableMigrationConfig,
	converter converters.DataConverter) map[string]interface{} {

	m := make(map[string]interface{})

	for _, item := range dataList {
		pk, err := converter.ExtractPrimaryKey(item)
		if err != nil {
			logs.Errorf("[MigrationEngine:Migrate] buildExistingDataMap:failed, pk: %+v, err: %+v, item: %+v, rid: %s",
				pk, err, item, kt.Rid)
			continue
		}
		// 构建Map的key（支持单字段和复合主键）
		key := e.buildPrimaryKeyString(pk, cfg)
		m[key] = item
	}

	return m
}

// buildPrimaryKeyString 构建主键字符串（用作Map的key）
func (e *MigrationEngine) buildPrimaryKeyString(pk interface{}, cfg *config.TableMigrationConfig) string {
	// 如果是复合主键（map类型）
	if pkMap, ok := pk.(map[string]interface{}); ok {
		// 按配置的主键字段顺序拼接
		pkFields := cfg.GetTargetPrimaryKeys()
		values := make([]string, 0, len(pkFields))
		for _, field := range pkFields {
			if val, exists := pkMap[field]; exists {
				values = append(values, fmt.Sprintf("%v", val))
			} else {
				values = append(values, "")
			}
		}
		// 用特殊分隔符连接（避免与数据值冲突）
		return "PK:" + fmt.Sprintf("%v", values)
	}
	// 单字段主键，直接转字符串
	return fmt.Sprintf("%v", pk)
}

// convertToInterfaceSlice 将具体类型的切片指针转换为 []interface{}
func (e *MigrationEngine) convertToInterfaceSlice(slicePtr interface{}) []interface{} {
	// 使用反射将 *[]*T 转换为 []interface{}
	sliceValue := reflect.ValueOf(slicePtr).Elem()
	result := make([]interface{}, sliceValue.Len())
	for i := 0; i < sliceValue.Len(); i++ {
		result[i] = sliceValue.Index(i).Interface()
	}
	return result
}
