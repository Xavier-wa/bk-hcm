import { ref } from 'vue';
import { getCvmProduceOrderStatusOpts } from '@/api/host/cvm';
export const useCvmProduceStatus = () => {
  const statusList = ref([]);
  const fetchCvmProduceStatus = async () => {
    try {
      const res = await getCvmProduceOrderStatusOpts();
      statusList.value = res?.data?.info || [];
    } catch (e) {
      console.error('[CvmProduceStatus] fetch failed:', e);
    }
  };
  fetchCvmProduceStatus();
  const getCvmProduceStatus = (status: number | string) => {
    const matched = statusList.value.find((item) => item.status === status);
    return matched?.description || status;
  };
  return { statusList, getCvmProduceStatus };
};
