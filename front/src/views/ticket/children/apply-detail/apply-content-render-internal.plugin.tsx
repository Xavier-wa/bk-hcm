import { Ref } from 'vue';
import { ACCOUNT_TYPES } from '../../constants';
import AccountApplyDetail from './account-apply-detail';
import CommonApplyDetail from './common-apply-detail/index.vue';
import BpassApplyDetail from '@/views/service/my-apply/components/bpass-apply-detail';

export const applyContentRender = (
  currentApplyData: Ref<any>,
  curApplyKey: Ref<string>,
  applyDetailProps: any,
  bpaasProps: any,
) => {
  if (currentApplyData.value.source === 'bpaas') {
    return <BpassApplyDetail params={currentApplyData.value} key={curApplyKey.value} {...bpaasProps} />;
  }

  if (ACCOUNT_TYPES.includes(currentApplyData.value.operation)) {
    return <AccountApplyDetail detail={currentApplyData.value} />;
  }
  return <CommonApplyDetail details={currentApplyData.value} key={curApplyKey.value} {...applyDetailProps} />;
};
