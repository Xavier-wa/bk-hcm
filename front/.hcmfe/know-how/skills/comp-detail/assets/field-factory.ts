import { VendorEnum } from '@/common/constant';
import { getModel } from '@/model/manager';
import { DetailsFieldTcloud } from './field-tcloud';

export class FieldFactory {
  static createModel(vendor: VendorEnum) {
    switch (vendor) {
      case VendorEnum.TCLOUD:
        return getModel(DetailsFieldTcloud);
      // TODO: 添加其他云厂商
      // case VendorEnum.AWS:
      //   return getModel(DetailsFieldAws);
      default:
        throw new Error(`Unsupported vendor: ${vendor}`);
    }
  }
}
