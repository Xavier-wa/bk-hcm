import { VendorEnum } from '@/common/constant';
import { getModel } from '@/model/manager';
import { TableColumnTcloud } from './data-list-column-tcloud';
// TODO: 支持其他云厂商时取消注释
// import { TableColumnAws } from './data-list-column-aws';
// import { TableColumnAzure } from './data-list-column-azure';

/**
 * 表格列工厂
 *
 * 根据当前云厂商创建对应的表格列模型。
 * 多资源类型场景（如 operation-log）可将 vendor 替换为 resourceType。
 */
export class TableColumnFactory {
  static createModel(vendor: VendorEnum) {
    switch (vendor) {
      case VendorEnum.TCLOUD:
        return getModel(TableColumnTcloud);
      // TODO: 添加其他云厂商
      // case VendorEnum.AWS:
      //   return getModel(TableColumnAws);
      // case VendorEnum.AZURE:
      //   return getModel(TableColumnAzure);
      default:
        throw new Error(`Unsupported vendor: ${vendor}`);
    }
  }
}
