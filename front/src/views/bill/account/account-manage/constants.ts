import { convertToIdNameMap } from './util';

export enum AccountLevelEnum {
  FirstLevel = 'firstAccount',
  SecondLevel = 'secondaryAccount',
}

export const tabs = [
  {
    key: AccountLevelEnum.FirstLevel,
    label: '一级账号',
    component: '',
  },
  {
    key: AccountLevelEnum.SecondLevel,
    label: '二级账号',
  },
];

// 云厂商
export const BILL_VENDORS = [
  {
    id: 'tcloud',
    name: '腾讯云',
  },
  {
    id: 'aws',
    name: '亚马逊云',
  },
  {
    id: 'azure',
    name: '微软云',
  },
  {
    id: 'gcp',
    name: '谷歌云',
  },
  {
    id: 'huawei',
    name: '华为云',
  },
  {
    id: 'zenlayer',
    name: 'zenlayer',
  },
  {
    id: 'kaopu',
    name: '靠谱云',
  },
];

export const BILL_VENDORS_MAP = convertToIdNameMap(BILL_VENDORS);

export const searchData = [
  {
    name: '一级帐号名称',
    id: 'name',
    async: false,
  },
  {
    name: '一级帐号ID',
    id: 'cloud_id',
    async: false,
  },
  {
    name: '云厂商',
    id: 'vendor',
    children: BILL_VENDORS,
    async: false,
  },
  {
    name: '帐号邮箱',
    id: 'email',
    async: false,
  },
  {
    name: '主负责人',
    id: 'managers',
    multiple: true,
    async: false,
  },
];

export const BILL_SITE_TYPES = [
  {
    name: '中国站',
    id: 'china',
  },
  {
    name: '国际站',
    id: 'international',
  },
];

export const BILL_SITE_TYPES_MAP = convertToIdNameMap(BILL_SITE_TYPES);

export const secondarySearchData = [
  {
    name: '二级帐号名称',
    id: 'name',
    async: false,
  },
  {
    name: '二级帐号ID',
    id: 'cloud_id',
    async: false,
  },
  {
    name: '所属一级帐号名称',
    id: 'parent_account_name',
    async: false,
  },
  {
    name: '云厂商',
    id: 'vendor',
    async: false,
  },
  {
    name: '站点类型',
    id: 'site',
    children: BILL_SITE_TYPES,
    async: false,
  },
  {
    name: '帐号邮箱',
    id: 'accountEmail',
    async: false,
  },
  {
    name: '运营产品',
    id: 'op_product_id',
    multiple: true,
    async: true,
  },
];
