import { useBusinessGlobalStore } from './business-global';
import { useRollingServerStore } from './rolling-server';

export const preload = async () => {
  const { getFullBusiness, getAuthorizedBusiness } = useBusinessGlobalStore();
  const { getResPollBusinessList } = useRollingServerStore();

  return Promise.all([getFullBusiness(), getAuthorizedBusiness(), getResPollBusinessList()]);
};

export * from './staff';
export * from './user';
export * from './account';
export * from './departments';
export * from './business';
export * from './ziyanScr';
export * from './resource';
export * from './resourcePlan';
export * from './common';
export * from './host';
export * from './scheme';
export * from './loadbalancer';
export * from './task';
export * from './rolling-server-usage';
export * from './cloud-account-manage/secondary-account';
export * from './cloud-account-manage/tertiary-account';
export * from './cloud-account-manage/cloud-secret';
