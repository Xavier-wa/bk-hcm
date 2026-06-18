import { VendorEnum, VendorMap } from '@/common/constant';
import tcloudVendorIcon from '@/assets/image/vendor-tcloud.svg';
import awsVendorIcon from '@/assets/image/vendor-aws.svg';
import azureVendorIcon from '@/assets/image/vendor-azure.svg';
import gcpVendorIcon from '@/assets/image/vendor-gcp.svg';
import huaweiVendorIcon from '@/assets/image/vendor-huawei.svg';

// vendor 标识 → 厂商图标 svg 文件映射（直接用图标资源文件，非复用 vendorProperty 代码对象）
// 自研云（tcloud-ziyan）复用腾讯云图标
const VENDOR_ICON_MAP: Record<string, string> = {
  [VendorEnum.TCLOUD]: tcloudVendorIcon,
  [VendorEnum.ZIYAN]: tcloudVendorIcon,
  [VendorEnum.AWS]: awsVendorIcon,
  [VendorEnum.AZURE]: azureVendorIcon,
  [VendorEnum.GCP]: gcpVendorIcon,
  [VendorEnum.HUAWEI]: huaweiVendorIcon,
};

export interface VendorDisplay {
  name: string;
  icon: string;
}

// 取 vendor 的展示名与图标。未知 vendor 兜底：名称用原值、图标用腾讯云图标，保证不报错
export const getVendorDisplay = (vendor: string): VendorDisplay => ({
  name: VendorMap[vendor] || vendor,
  icon: VENDOR_ICON_MAP[vendor] || tcloudVendorIcon,
});
