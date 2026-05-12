/* eslint-disable */
import { saveAs } from 'file-saver';
import { Workbook, Worksheet } from 'exceljs';

interface CellRange {
  s: { r: number; c: number };
  e: { r: number; c: number };
}

const SHEET_NAME = 'SheetJS';
const XLSX_MIME_TYPE = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
const MIN_COLUMN_WIDTH = 10;
const COLUMN_WIDTH_PADDING = 2;

function generateArray(table: HTMLTableElement): [any[][], CellRange[]] {
  var out = [];
  var rows = table.querySelectorAll('tr');
  var ranges = [];
  for (var R = 0; R < rows.length; ++R) {
    var outRow = [];
    var row = rows[R];
    var columns = row.querySelectorAll('td');
    for (var C = 0; C < columns.length; ++C) {
      var cell = columns[C];
      var colspan = cell.getAttribute('colspan');
      var rowspan = cell.getAttribute('rowspan');
      var cellValue: string | number = cell.innerText;
      // 如果是有效数字字符串，转换为数字类型
      if (cellValue !== '' && !isNaN(Number(cellValue))) cellValue = Number(cellValue);

      //Skip ranges
      ranges.forEach(function (range) {
        if (R >= range.s.r && R <= range.e.r && outRow.length >= range.s.c && outRow.length <= range.e.c) {
          for (var i = 0; i <= range.e.c - range.s.c; ++i) outRow.push(null);
        }
      });

      //Handle Row Span
      const colspanNum = Number(colspan) || 1;
      if (rowspan || colspan) {
        const rowspanNum = Number(rowspan) || 1;
        ranges.push({
          s: {
            r: R,
            c: outRow.length,
          },
          e: {
            r: R + rowspanNum - 1,
            c: outRow.length + colspanNum - 1,
          },
        });
      }

      //Handle Value
      outRow.push(cellValue !== '' ? cellValue : null);

      //Handle Colspan
      if (colspanNum > 1) for (var k = 0; k < colspanNum - 1; ++k) outRow.push(null);
    }
    out.push(outRow);
  }
  return [out, ranges];
}

function createWorksheet(data: any[][], merges: CellRange[] = [], autoWidth = false, headerIndex = 0) {
  const workbook = new Workbook();
  const worksheet = workbook.addWorksheet(SHEET_NAME);

  worksheet.addRows(data);
  setDateCellFormat(worksheet, data);
  setWorksheetMerges(worksheet, merges);

  if (autoWidth) {
    setWorksheetAutoWidth(worksheet, data, headerIndex);
  }

  return workbook;
}

function setDateCellFormat(worksheet: Worksheet, data: any[][]) {
  data.forEach((row, rowIndex) => {
    row.forEach((val, columnIndex) => {
      if (val instanceof Date) {
        worksheet.getRow(rowIndex + 1).getCell(columnIndex + 1).numFmt = 'm/d/yy';
      }
    });
  });
}

function setWorksheetMerges(worksheet: Worksheet, merges: CellRange[]) {
  merges.forEach((range) => {
    worksheet.mergeCells(range.s.r + 1, range.s.c + 1, range.e.r + 1, range.e.c + 1);
  });
}

function getCellWidth(val: any) {
  if (val == null) {
    return MIN_COLUMN_WIDTH;
  }

  const cellText = val.toString();
  if (cellText.charCodeAt(0) > 255) {
    return cellText.length * 2;
  }
  return cellText.length;
}

function setWorksheetAutoWidth(worksheet: Worksheet, data: any[][], headerIndex = 0) {
  const colWidth = data.map((row) => row.map((val) => getCellWidth(val)));
  const result = [...(colWidth[headerIndex] || [])];

  for (let i = 0; i < colWidth.length; i++) {
    for (let j = 0; j < colWidth[i].length; j++) {
      if (!result[j]) {
        result[j] = MIN_COLUMN_WIDTH;
      }

      if (result[j] < colWidth[i][j]) {
        result[j] = colWidth[i][j];
      }
    }
  }

  result.forEach((width, index) => {
    worksheet.getColumn(index + 1).width = Math.max(width + COLUMN_WIDTH_PADDING, MIN_COLUMN_WIDTH);
  });
}

async function saveWorkbook(workbook: Workbook, filename: string) {
  const buffer = await workbook.xlsx.writeBuffer();
  saveAs(
    new Blob([buffer], {
      type: XLSX_MIME_TYPE,
    }),
    `${filename}.xlsx`,
  );
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

export async function export_table_to_excel(id: string): Promise<void> {
  var theTable = document.getElementById(id) as HTMLTableElement | null;
  if (!theTable) {
    console.error(`Table element with id "${id}" not found`);
    return;
  }
  var oo = generateArray(theTable);
  var ranges = oo[1];

  /* original data */
  var data = oo[0];
  const workbook = createWorksheet(data, ranges);
  await saveWorkbook(workbook, 'test');
}
export interface ExportOptions {
  header: string[] | string[][];
  data: any[][];
  filename: string;
  autoWidth?: boolean;
  mutipleHeader?: boolean;
  merges?: any[];
  headerIndex?: number;
  maxRowsPerFile?: number;
}

// 单个文件导出的核心逻辑
function exportSingleExcel({
  header,
  data,
  filename,
  autoWidth = true,
  mutipleHeader = false,
  merges = [],
  headerIndex = 0,
}: Omit<ExportOptions, 'maxRowsPerFile'>): Promise<void> {
  data = [...data];
  if (mutipleHeader) {
    // 多行表头时，header 应为 string[][]
    const multiHeader = header as string[][];
    const reversedHeader = [...multiHeader].reverse();
    reversedHeader.forEach((hd) => {
      data.unshift(hd);
    });
  } else {
    data.unshift(header as string[]);
  }
  const workbook = createWorksheet(data, merges, autoWidth, headerIndex);
  return saveWorkbook(workbook, filename);
}

// 每批最大行数（15万行，避免内存溢出）
export const MAX_ROWS_PER_FILE = 150000;

export async function export_json_to_excel(
  opts: ExportOptions = { header: [], data: [], filename: '' },
): Promise<void> {
  let {
    header,
    data,
    filename,
    autoWidth = true,
    mutipleHeader = false, // 是否包含多行 header
    merges = [], // 合并单元格选项
    headerIndex = 0, //以 header 第几列为基础
    maxRowsPerFile = MAX_ROWS_PER_FILE,
  } = opts;
  /* original data */
  filename = filename || 'excel-list';

  // 如果数据量不大，直接导出
  if (data.length <= maxRowsPerFile) {
    await exportSingleExcel({
      header,
      data,
      filename,
      autoWidth,
      mutipleHeader,
      merges,
      headerIndex,
    });
    return;
  }

  // 数据量大，分批导出多个文件
  const totalParts = Math.ceil(data.length / maxRowsPerFile);
  console.log(`数据量较大（${data.length} 行），将分 ${totalParts} 个文件导出`);

  for (let i = 0; i < totalParts; i++) {
    const start = i * maxRowsPerFile;
    const end = Math.min(start + maxRowsPerFile, data.length);
    const chunk = data.slice(start, end);

    if (i > 0) {
      // 延迟导出，避免连续创建多个大文件导致内存问题
      await delay(500);
    }

    await exportSingleExcel({
      header,
      data: chunk,
      filename: `${filename}_第${i + 1}部分_共${totalParts}部分`,
      autoWidth,
      mutipleHeader,
      merges: i === 0 ? merges : [], // 只有第一个文件保留合并单元格
      headerIndex,
    });
  }
}
