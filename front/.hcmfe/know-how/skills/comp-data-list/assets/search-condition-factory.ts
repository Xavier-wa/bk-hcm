import { VendorEnum } from '@/common/constant';
import { getModel } from '@/model/manager';
import { SearchConditionTcloud } from './search-condition-tcloud';
// TODO: 支持其他云厂商时取消注释
// import { SearchConditionAws } from './search-condition-aws';
// import { SearchConditionAzure } from './search-condition-azure';

/**
 * 搜索条件工厂
 *
 * 根据当前云厂商创建对应的搜索条件模型。
 * 多资源类型场景（如 operation-log）可将 vendor 替换为 resourceType。
 */
export class SearchConditionFactory {
  static createModel(vendor: VendorEnum) {
    switch (vendor) {
      case VendorEnum.TCLOUD:
        return getModel(SearchConditionTcloud);
      // TODO: 添加其他云厂商
      // case VendorEnum.AWS:
      //   return getModel(SearchConditionAws);
      // case VendorEnum.AZURE:
      //   return getModel(SearchConditionAzure);
      default:
        throw new Error(`Unsupported vendor: ${vendor}`);
    }
  }
}
