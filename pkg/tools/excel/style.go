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

package excel

import (
	"fmt"

	"hcm/pkg/criteria/errf"

	"github.com/xuri/excelize/v2"
)

const (
	// runeASCIIMax is the max code point treated as half-width for width estimation.
	runeASCIIMax = 127
	// ColWidthCJKUnit is the Excel column width unit per CJK (full-width) character (header).
	ColWidthCJKUnit = 2.2
	// ColWidthASCIIUnit is the Excel column width unit per ASCII character (header).
	ColWidthASCIIUnit = 1.2
	// ColWidthDataCJKUnit is the width unit per CJK character for data cells.
	ColWidthDataCJKUnit = 2.0
	// ColWidthDataASCIIUnit is the width unit per ASCII character for data cells.
	ColWidthDataASCIIUnit = 1.0
	// ColWidthPadding is the extra padding added to each column width.
	ColWidthPadding = 2.0
	// ColWidthMin is the minimum column width to prevent overly narrow columns.
	ColWidthMin = 10.0
	// ColWidthMax is the maximum column width to avoid excessively wide columns.
	ColWidthMax = 50.0
)

func colWidthFromRunes(text string, cjkUnit, asciiUnit float64) float64 {
	var w float64
	for _, r := range text {
		if r > runeASCIIMax {
			w += cjkUnit
		} else {
			w += asciiUnit
		}
	}
	return w
}

func clampColWidth(w float64) float64 {
	if w < ColWidthMin {
		return ColWidthMin
	}
	if w > ColWidthMax {
		return ColWidthMax
	}
	return w
}

// ColWidthFromText estimates the Excel column width for data cell text.
// CJK characters count as ColWidthDataCJKUnit, ASCII as ColWidthDataASCIIUnit.
func ColWidthFromText(text string) float64 {
	w := colWidthFromRunes(text, ColWidthDataCJKUnit, ColWidthDataASCIIUnit) + ColWidthPadding
	if w > ColWidthMax {
		return ColWidthMax
	}
	return w
}

// ColWidthFromHeader estimates column width for header text (bold).
// CJK characters count as ColWidthCJKUnit, ASCII as ColWidthASCIIUnit.
func ColWidthFromHeader(text string) float64 {
	w := colWidthFromRunes(text, ColWidthCJKUnit, ColWidthASCIIUnit) + ColWidthPadding
	return clampColWidth(w)
}

// ColWidthTracker tracks the maximum column width while building a sheet.
type ColWidthTracker struct {
	widths []float64
}

// NewColWidthTracker creates a tracker for the given number of columns.
func NewColWidthTracker(colCount int) *ColWidthTracker {
	return &ColWidthTracker{widths: make([]float64, colCount)}
}

// UpdateHeader records header text width for a column.
func (t *ColWidthTracker) UpdateHeader(col int, text string) {
	t.update(col, ColWidthFromHeader(text))
}

// Update records data cell text width for a column.
func (t *ColWidthTracker) Update(col int, text string) {
	t.update(col, ColWidthFromText(text))
}

func (t *ColWidthTracker) update(col int, w float64) {
	if t == nil || col < 0 || col >= len(t.widths) {
		return
	}
	if w > t.widths[col] {
		t.widths[col] = w
	}
}

// Apply sets column widths on the sheet from tracked maximums.
func (t *ColWidthTracker) Apply(f *excelize.File, sheet string) error {
	if f == nil {
		return errf.Newf(errf.InvalidParameter, "file is nil")
	}
	if t == nil {
		return errf.Newf(errf.InvalidParameter, "width tracker is nil")
	}
	for i, w := range t.widths {
		if w <= 0 {
			w = ColWidthMin
		}
		colName, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return fmt.Errorf("get column name for col %d failed: %w", i+1, err)
		}
		if err = f.SetColWidth(sheet, colName, colName, w); err != nil {
			return fmt.Errorf("set col %s width failed: %w", colName, err)
		}
	}
	return nil
}

// SetHeaderStyle applies bold font and light-blue background to the first header row.
func SetHeaderStyle(f *excelize.File, sheet string, headerCols int) error {
	if f == nil {
		return errf.Newf(errf.InvalidParameter, "file is nil")
	}
	if headerCols <= 0 {
		return errf.Newf(errf.InvalidParameter, "header column count must be positive")
	}
	style, err := f.NewStyle(&excelize.Style{
		// 表头加粗
		Font: &excelize.Font{Bold: true},
		// 设置浅蓝色表头方便区分
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
	})
	if err != nil {
		return errf.Newf(errf.Aborted, "create header style failed: %v", err)
	}
	lastCol, err := excelize.ColumnNumberToName(headerCols)
	if err != nil {
		return errf.Newf(errf.Aborted, "get last column name failed: %v", err)
	}
	if err = f.SetCellStyle(sheet, "A1", lastCol+"1", style); err != nil {
		return errf.Newf(errf.Aborted, "set header style failed: %v", err)
	}

	return nil
}
