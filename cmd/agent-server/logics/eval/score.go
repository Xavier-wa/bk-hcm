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

package eval

import (
	"math"
	"sort"

	"hcm/pkg/cc"
)

const dimMax = 5.0

// Compute maps dimension scores to process / outcome / quality percents using yaml weights.
// Model-filled percent totals MUST be ignored.
//
// 判官没给分的维度按「本轮不适用」处理，从分子和分母里一起剔除。
// 若按 0 分计入，一次没有工具失败、没有中断的正常对话会因为漏掉 error_handling
// 与 guidance_at_interrupt 两维，被压到过程维 67 分以下，永远达不到 passThreshold。
func Compute(dims DimScores, processW, outcomeW map[string]float64) ScoreResult {
	processW, outcomeW = resolveWeights(processW, outcomeW)
	process := weightedPercent(dims, processW)
	outcome := weightedPercent(dims, outcomeW)
	quality := int(math.Round(float64(process)*0.5 + float64(outcome)*0.5))
	return ScoreResult{Process: process, Outcome: outcome, Quality: quality}
}

// MissingDims 返回权重表里有、但判官没有给分的维度名（字典序）。
// 这些维度被当作本轮不适用，调用方应当留日志，便于发现判官漏输出而非真的不适用。
func MissingDims(dims DimScores, processW, outcomeW map[string]float64) []string {
	processW, outcomeW = resolveWeights(processW, outcomeW)
	missing := make([]string, 0)
	for _, weights := range []map[string]float64{processW, outcomeW} {
		for name := range weights {
			if _, ok := dims[name]; !ok {
				missing = append(missing, name)
			}
		}
	}
	sort.Strings(missing)
	return missing
}

func resolveWeights(processW, outcomeW map[string]float64) (map[string]float64, map[string]float64) {
	if len(processW) == 0 {
		processW = cc.DefaultEvalProcessWeights
	}
	if len(outcomeW) == 0 {
		outcomeW = cc.DefaultEvalOutcomeWeights
	}
	return processW, outcomeW
}

func weightedPercent(dims DimScores, weights map[string]float64) int {
	var sumW, sumWD float64
	for name, weight := range weights {
		score, ok := dims[name]
		if !ok {
			continue
		}
		sumW += weight
		sumWD += weight * clampDim(score)
	}
	if sumW == 0 {
		return 0
	}
	return int(math.Round(sumWD / sumW / dimMax * 100))
}

func clampDim(v int) float64 {
	if v < 0 {
		return 0
	}
	if v > 5 {
		return 5
	}
	return float64(v)
}
