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

package plan

import (
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"time"

	"hcm/cmd/woa-server/dal/task/dao"
	"hcm/cmd/woa-server/dal/task/table"
	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/cmsi"
	"hcm/pkg/thirdparty/cvmapi"
	"hcm/pkg/tools/metadata"
)

// recycleReturnResPlanOrder 一条销毁返还预测单的报表行数据，全部字段取自 CRP QueryOrderInfo / CvmData。
type recycleReturnResPlanOrder struct {
	planProduct  string
	opProduct    string
	orderType    string
	useTime      string
	deviceFamily string
	city         string
	cpuCore      int
	memoryGB     float64
	diskGB       int
	orderURL     string
}

// formatCityCell 渲染城市列单元格，无数据时展示 "-"。
func formatCityCell(city string) string {
	if city == "" {
		city = "-"
	}
	return fmt.Sprintf(`<td style="padding:0 16px;border:1px solid #dcdee5;font-size:12px;
									 color:#4d4f56;white-space:nowrap">%s</td>`,
		html.EscapeString(city))
}

// calcRecycleReturnResPlanLastWeekRange 根据参考时间计算上一自然周统计周期：上周一 00:00:00 ~ 上周日 23:59:59。
func calcRecycleReturnResPlanLastWeekRange(now time.Time) (start time.Time, end time.Time) {
	loc := now.Location()
	daysSinceMonday := (int(now.Weekday()) - int(time.Monday) + 7) % 7
	thisMonday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).
		AddDate(0, 0, -daysSinceMonday)
	lastMonday := thisMonday.AddDate(0, 0, -7)
	start = lastMonday
	lastSunday := lastMonday.AddDate(0, 0, 6)
	end = time.Date(lastSunday.Year(), lastSunday.Month(), lastSunday.Day(), 23, 59, 59, 0, loc)
	return start, end
}

// generateAndSendRecycleReturnResPlanReport 取数 -> 组装 -> 发送销毁返还预测周报邮件。
func (c *Controller) generateAndSendRecycleReturnResPlanReport(kt *kit.Kit, sendDate, start, end time.Time) error {
	startTime := time.Now()
	logs.Infof("start to generate recycle return res plan report, period: %s ~ %s, rid: %s",
		start.Format(constant.TimeStdFormat), end.Format(constant.TimeStdFormat), kt.Rid)

	// 1. 取数：筛选候选集（status=SUCCESS + update_at 在周期内）
	candidates, err := c.fetchRecycleReturnResPlanCandidates(kt, start, end)
	if err != nil {
		return err
	}

	// 2. 逐条调 CRP 取预测返还单明细
	orders, err := c.collectRecycleReturnResPlanOrders(kt, candidates)
	if err != nil {
		return err
	}

	// 3. 组装报表
	title, content := buildRecycleReturnResPlanReport(sendDate, start, end, c.bkHcmURL, orders)

	// 4. 发送邮件
	if err := c.sendRecycleReturnResPlanReport(kt, title, content); err != nil {
		return err
	}

	logs.Infof("end to generate recycle return res plan report, candidate: %d, order: %d, cost: %fs, rid: %s",
		len(candidates), len(orders), time.Since(startTime).Seconds(), kt.Rid)
	return nil
}

// fetchRecycleReturnResPlanCandidates 从 cr_ReturnTask 按 status=SUCCESS + update_at 在统计周期内取候选集，分页拉全。
func (c *Controller) fetchRecycleReturnResPlanCandidates(kt *kit.Kit, start, end time.Time) (
	[]*table.ReturnTask, error) {

	filter := map[string]interface{}{
		"status": string(table.ReturnStatusSuccess),
		"update_at": map[string]interface{}{
			"$gte": start,
			"$lte": end,
		},
	}

	candidates := make([]*table.ReturnTask, 0)
	page := metadata.BasePage{Limit: 500, Sort: "update_at"}
	for {
		tasks, err := dao.Set().ReturnTask().FindManyReturnTask(kt.Ctx, page, filter)
		if err != nil {
			logs.Errorf("failed to list return task for recycle return res plan report, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		candidates = append(candidates, tasks...)
		if len(tasks) < page.Limit {
			break
		}
		page.Start += page.Limit
	}

	logs.Infof("fetched %d return task candidates for recycle return res plan report, rid: %s", len(candidates), kt.Rid)
	return candidates, nil
}

// collectRecycleReturnResPlanOrders 逐条以 task_id 调 CRP queryOrderList 取预测返还单，仅保留 Status=0 的有效单据；
// 任一条 CRP 查询失败则整体失败。
func (c *Controller) collectRecycleReturnResPlanOrders(kt *kit.Kit, candidates []*table.ReturnTask) (
	[]recycleReturnResPlanOrder, error) {

	orders := make([]recycleReturnResPlanOrder, 0, len(candidates))

	for _, task := range candidates {
		queryOrders, err := c.resFetcher.GetOrderList(kt, task.TaskID)
		if err != nil {
			logs.Errorf("failed to query crp order list for recycle return res plan report, err: %v, task_id: %s, "+
				"rid: %s", err, task.TaskID, kt.Rid)
			return nil, fmt.Errorf("failed to query crp order list for task_id: %s, err: %w", task.TaskID, err)
		}

		orders = append(orders, convertValidRecycleReturnResPlanOrders(queryOrders)...)
	}

	logs.Infof("collected %d recycle return res plan orders, rid: %s", len(orders), kt.Rid)
	return orders, nil
}

// convertValidRecycleReturnResPlanOrders 仅保留 CRP 返回 Status=0 的有效预测返还单。
func convertValidRecycleReturnResPlanOrders(queryOrders []*cvmapi.QueryOrderInfo) []recycleReturnResPlanOrder {
	orders := make([]recycleReturnResPlanOrder, 0, len(queryOrders))
	for _, qo := range queryOrders {
		if qo == nil || qo.Status != enumor.QueryOrderInfoStatusSuccess {
			continue
		}
		orders = append(orders, toRecycleReturnResPlanOrder(qo))
	}
	return orders
}

// toRecycleReturnResPlanOrder 将 CRP QueryOrderInfo 映射为报表行。
// 接口返回字段说明：
// - planProductName: 规划产品（需求预测单的规划产品，可能为空）
// - toPlanProductName: 运营产品（需求追加单的目标规划产品，可能为空）
// - orderTypeName: 项目类型（如"年度追加"）
// - createTime: 创建时间
// - cvmData: 包含机型族名称、城市及 CPU/内存明细
// - allCoreAmount: CPU总核数
// - allDiskAmount: 磁盘总量
func toRecycleReturnResPlanOrder(qo *cvmapi.QueryOrderInfo) recycleReturnResPlanOrder {
	deviceFamily := make([]string, 0, len(qo.CvmData))
	var memoryGB float64
	for _, cd := range qo.CvmData {
		if cd.Name != "" {
			deviceFamily = append(deviceFamily, cd.Name)
		}
		if cd.Unit == "GB" {
			memoryGB += cd.Value
		}
	}

	useTime := qo.UseTime
	if useTime == "" {
		useTime = qo.CreateTime
	}

	// 获取规划产品：优先使用 PlanProductName，如果为空则使用 DeptName
	planProduct := qo.PlanProductName
	if planProduct == "" {
		planProduct = qo.DeptName
	}
	// 获取运营产品：使用 ToPlanProductName，如果为空则使用 ToDeptName
	opProduct := qo.ToPlanProductName
	if opProduct == "" {
		opProduct = qo.ToDeptName
	}

	return recycleReturnResPlanOrder{
		planProduct:  planProduct,
		opProduct:    opProduct,
		orderType:    qo.OrderTypeName,
		useTime:      useTime,
		deviceFamily: strings.Join(deviceFamily, "、"),
		city:         extractRecycleReturnResPlanCity(qo.CvmData),
		cpuCore:      qo.AllCoreAmount,
		memoryGB:     memoryGB,
		diskGB:       qo.AllDiskAmount,
		orderURL:     cvmapi.CvmPlanLinkPrefix + qo.OrderID,
	}
}

// extractRecycleReturnResPlanCity 从 CvmData 提取城市，去重后以顿号拼接；无数据时返回 "-"。
func extractRecycleReturnResPlanCity(cvmData []cvmapi.CvmData) string {
	cities := make([]string, 0, len(cvmData))
	seen := make(map[string]struct{}, len(cvmData))
	for _, item := range cvmData {
		city := strings.TrimSpace(item.CityName)
		if city == "" {
			continue
		}
		if _, ok := seen[city]; ok {
			continue
		}
		seen[city] = struct{}{}
		cities = append(cities, city)
	}
	if len(cities) == 0 {
		return "-"
	}
	return strings.Join(cities, "、")
}

// buildRecycleReturnResPlanReport 根据命中记录数组装邮件标题与正文（有单据 / 无单据两种样式）。
func buildRecycleReturnResPlanReport(sendDate time.Time, start, end time.Time, bkHcmURL string,
	orders []recycleReturnResPlanOrder) (string, string) {

	title := fmt.Sprintf("IEG-销毁返还预测统计周报-%s", sendDate.Format("2006-01-02"))

	if len(orders) == 0 {
		return title, buildRecycleReturnResPlanEmptyHTML(title, start, end, bkHcmURL)
	}
	return title, buildRecycleReturnResPlanHTML(title, start, end, bkHcmURL, orders)
}

// buildRecycleReturnResPlanHTML 组装有单据样式正文：标题 + 汇总头 + 明细表 + 备注。
// 支持合并单元格：规划产品、运营产品（运营产品合并需要规划产品一致）
func buildRecycleReturnResPlanHTML(title string, start, end time.Time, bkHcmURL string,
	orders []recycleReturnResPlanOrder) string {
	var totalCPU int64
	var totalMem float64

	// 构建带 rowspan 合并的表格行
	rowsHTML := buildTableRowsWithMerge(orders)

	for _, order := range orders {
		totalCPU += int64(order.cpuCore)
		totalMem += order.memoryGB
	}

	// 获取 bkHcmURL 用于模板中的链接
	return fmt.Sprintf(ptypes.RecycleReturnResPlanReportContentTemplate,
		bkHcmURL, bkHcmURL, html.EscapeString(title),
		totalCPU, formatFloat(totalMem),
		start.Format(constant.DateTimeLayoutCN), end.Format(constant.DateTimeLayoutCN), rowsHTML)
}

// buildTableRowsWithMerge 构建带 rowspan 合并的表格行 HTML。
// 合并规则：
// 1. 规划产品：连续相同值合并
// 2. 运营产品：在同一规划产品组内，连续相同值合并
func buildTableRowsWithMerge(orders []recycleReturnResPlanOrder) string {
	if len(orders) == 0 {
		return ""
	}

	// 按规划产品、运营产品排序
	sort.Slice(orders, func(a, b int) bool {
		if orders[a].planProduct != orders[b].planProduct {
			return orders[a].planProduct < orders[b].planProduct
		}
		return orders[a].opProduct < orders[b].opProduct
	})

	var buf strings.Builder
	rowIndex := 0
	for rowIndex < len(orders) {
		// 找出同一规划产品的连续行范围 [rowIndex, planProductEndIndex)
		planProductName := orders[rowIndex].planProduct
		planProductEndIndex := rowIndex
		for planProductEndIndex < len(orders) && orders[planProductEndIndex].planProduct == planProductName {
			planProductEndIndex++
		}
		planProductRowSpan := planProductEndIndex - rowIndex

		// 在同一规划产品内，再按运营产品分组
		opProductStartIndex := rowIndex
		for opProductStartIndex < planProductEndIndex {
			// 找出同一运营产品的连续行范围 [opProductStartIndex, opProductEndIndex)
			opProductName := orders[opProductStartIndex].opProduct
			opProductEndIndex := opProductStartIndex
			for opProductEndIndex < planProductEndIndex && orders[opProductEndIndex].opProduct == opProductName {
				opProductEndIndex++
			}
			opProductRowSpan := opProductEndIndex - opProductStartIndex

			// 遍历该运营产品组内每一行
			for colIndex := opProductStartIndex; colIndex < opProductEndIndex; colIndex++ {
				row := orders[colIndex]

				if colIndex == opProductStartIndex {
					// 该运营产品组的第一行：需要输出规划产品或运营产品
					if colIndex == rowIndex {
						// 同时也是该规划产品组的第一行：输出规划产品+运营产品
						buf.WriteString(fmt.Sprintf(ptypes.RecycleReturnResPlanTableRowWithPlanAndOp, planProductRowSpan,
							html.EscapeString(row.planProduct), opProductRowSpan, html.EscapeString(row.opProduct),
							html.EscapeString(row.orderType), html.EscapeString(row.useTime),
							html.EscapeString(row.deviceFamily),
							formatCityCell(row.city),
							row.cpuCore, formatFloat(row.memoryGB), row.diskGB,
							html.EscapeString(row.orderURL), html.EscapeString(extractOrderIDFromURL(row.orderURL))))
					} else {
						// 该规划产品组非首行：规划产品已在上面合并，只输出运营产品
						buf.WriteString(fmt.Sprintf(ptypes.RecycleReturnResPlanTableRowWithOpOnly, opProductRowSpan,
							html.EscapeString(row.opProduct), html.EscapeString(row.orderType),
							html.EscapeString(row.useTime), html.EscapeString(row.deviceFamily),
							formatCityCell(row.city),
							row.cpuCore, formatFloat(row.memoryGB), row.diskGB, html.EscapeString(row.orderURL),
							html.EscapeString(extractOrderIDFromURL(row.orderURL))))
					}
				} else {
					// 该运营产品组非首行：规划产品、运营产品均已在上面合并，只输出其余列
					buf.WriteString(fmt.Sprintf(ptypes.RecycleReturnResPlanTableRowRestOnly,
						html.EscapeString(row.orderType), html.EscapeString(row.useTime),
						html.EscapeString(row.deviceFamily),
						formatCityCell(row.city),
						row.cpuCore, formatFloat(row.memoryGB), row.diskGB,
						html.EscapeString(row.orderURL), html.EscapeString(extractOrderIDFromURL(row.orderURL))))
				}
			}
			opProductStartIndex = opProductEndIndex
		}
		rowIndex = planProductEndIndex
	}
	return buf.String()
}

// buildRecycleReturnResPlanEmptyHTML 组装无单据样式正文（命中 0 条，不渲染空表）。
func buildRecycleReturnResPlanEmptyHTML(title string, start, end time.Time, bkHcmURL string) string {
	return fmt.Sprintf(ptypes.RecycleReturnResPlanReportEmptyTemplate,
		bkHcmURL, bkHcmURL, html.EscapeString(title),
		start.Format(constant.DateTimeLayoutCN), end.Format(constant.DateTimeLayoutCN))
}

// sendRecycleReturnResPlanReport 通过 cmsi 网关发送 HTML 邮件，主送取自配置 Receivers，为空则跳过发送。
func (c *Controller) sendRecycleReturnResPlanReport(kt *kit.Kit, title, content string) error {
	receivers := c.resPlanCfg.RecycleReturnResPlanReport.Receivers
	if len(receivers) == 0 {
		logs.Warnf("skip recycle return res plan weekly report, no receivers configured, rid: %s", kt.Rid)
		return nil
	}

	ccReceivers := c.resPlanCfg.RecycleReturnResPlanReport.CcReceivers

	mail := &cmsi.CmsiMail{
		ReceiverUserName: strings.Join(receivers, ","),
		Title:            title,
		Content:          content,
		CcUserName:       strings.Join(ccReceivers, ","),
	}
	return c.CmsiClient.SendMail(kt, mail)
}

// extractOrderIDFromURL 从 CRP 单据链接中提取订单号，用作锚文本。
func extractOrderIDFromURL(url string) string {
	// 取最后一个 '/' 后面的部分
	idx := strings.LastIndex(url, "/")
	if idx >= 0 && idx+1 < len(url) {
		return url[idx+1:]
	}
	return url
}

// formatFloat 以最简形式格式化浮点数（整数为整数形式，否则保留小数）。
func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// PushRecycleReturnResPlanReport 手动触发销毁返还预测周报
// start/end 为空时默认使用上周一 00:00:00 ~ 上周日 23:59:59
func (c *Controller) PushRecycleReturnResPlanReport(kt *kit.Kit, startStr, endStr string) (
	*ptypes.PushRecycleReturnResPlanReportResp, error) {

	now := time.Now()
	loc := now.Location()

	// 默认使用上周一 00:00:00 ~ 上周日 23:59:59
	start, end := calcRecycleReturnResPlanLastWeekRange(now)

	hasStart := len(startStr) > 0
	hasEnd := len(endStr) > 0
	if hasStart != hasEnd {
		return nil, fmt.Errorf("start and end must be both set or both empty")
	}
	if hasStart && hasEnd {
		parsedStart, err := time.ParseInLocation(constant.DateTimeLayout, startStr, loc)
		if err != nil {
			return nil, fmt.Errorf("invalid start time: %s", startStr)
		}
		parsedEnd, err := time.ParseInLocation(constant.DateTimeLayout, endStr, loc)
		if err != nil {
			return nil, fmt.Errorf("invalid end time: %s", endStr)
		}
		if parsedStart.After(parsedEnd) {
			return nil, fmt.Errorf("start must be less than or equal to end")
		}
		start, end = parsedStart, parsedEnd
	}

	startTime := time.Now()
	logs.Infof("manual trigger recycle return res plan report, period: %s ~ %s, rid: %s",
		start.Format(constant.TimeStdFormat), end.Format(constant.TimeStdFormat), kt.Rid)

	if err := c.generateAndSendRecycleReturnResPlanReport(kt, now, start, end); err != nil {
		logs.Errorf("failed to generate and send recycle return res plan report, err: %v, rid: %s", err, kt.Rid)
		return &ptypes.PushRecycleReturnResPlanReportResp{
			Success: false,
			Message: fmt.Sprintf("failed to send mail: %v", err),
		}, nil
	}

	logs.Infof("manual trigger recycle return res plan report success, cost: %fs, rid: %s",
		time.Since(startTime).Seconds(), kt.Rid)

	return &ptypes.PushRecycleReturnResPlanReportResp{
		Success: true,
		Message: "success",
	}, nil
}
