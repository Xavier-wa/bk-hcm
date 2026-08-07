/**
 * 复制到业务模块后，按用途改名：
 * - FormFieldFactory
 * - SearchConditionFactory
 * - TableColumnFactory
 * - DetailsFieldFactory
 *
 * 同时将 FieldTcloud / FieldAws 改为用途明确的模型名。
 */
import { VendorEnum } from '@/common/constant';
import { getModel } from '@/model/manager';
import { FieldAws } from './field-aws';
import { FieldTcloud } from './field-tcloud';

export class FieldFactory {
  static createModel(vendor: VendorEnum) {
    switch (vendor) {
      case VendorEnum.TCLOUD:
        return getModel(FieldTcloud);
      case VendorEnum.AWS:
        return getModel(FieldAws);
      default:
        throw new Error(`Unsupported vendor: ${vendor}`);
    }
  }
}
