import { computed, defineComponent, onUnmounted, ref, watch } from 'vue';
import './index.scss';
import DetailHeader from '@/views/resource/resource-manage/common/header/detail-header';
import { useAccountStore } from '@/store';
import { useRoute } from 'vue-router';
import { APPLICATION_TYPE_MAP } from '@/views/ticket/constants';
import Clb from './clb.vue';
import useFormModel from '@/hooks/useFormModel';
import routerAction from '@/router/utils/action';
import { GLOBAL_BIZS_KEY, VendorEnum } from '@/common/constant';
import { BpaasEndStatus } from '@/views/service/my-apply/components/bpass-apply-detail';
import { applyContentRender } from './apply-content-render.plugin';

export enum ApplicationStatus {
  pending = 'pending',
  pass = 'pass',
  rejected = 'rejected',
  cancelled = 'cancelled',
  delivering = 'delivering',
  completed = 'completed',
  deliver_partial = 'deliver_partial',
  deliver_error = 'deliver_error',
}

export interface IApplicationDetail {
  id: string;
  source: string;
  sn: string;
  // 被替换为使用 operation 字段，接口还是会返回，但不再使用了
  type: string;
  // 新增 operation 字段，用于替代 type 字段，解决共用同一个审批流(之前的type字段)但需要区分不同的操作类型
  operation: string;
  status: ApplicationStatus;
  applicant: string;
  content: string;
  delivery_detail: string;
  memo: string;
  creator: string;
  reviser: string;
  created_at: string;
  updated_at: string;
  ticket_url: string;
  [key: string]: any;
}

export default defineComponent({
  setup() {
    const accountStore = useAccountStore();
    const isLoading = ref(false);
    const currentApplyData = ref<IApplicationDetail & { BpaasName?: string }>({});
    const curApplyKey = ref('');
    const isCancelBtnLoading = ref(false);
    const route = useRoute();

    const { formModel: bpaasPayload, setFormValues: setBpaasPayload } = useFormModel({
      bpaas_sn: 0,
      account_id: '',
      id: '',
      applicant: '',
    });

    let interval: NodeJS.Timeout;
    const bizId = computed(() => Number(route.query.bizs));

    // 获取单据详情
    const getMyApplyDetail = async (id: string) => {
      isLoading.value = true;
      try {
        const res = await accountStore.getApplyAccountDetail(id, bizId.value);
        currentApplyData.value = res.data;
        curApplyKey.value = res.data.id;

        if (res.data.source === 'bpaas') {
          resolveBpaasDetail(res);
        }

        if ([ApplicationStatus.pending, ApplicationStatus.delivering].includes(res.data.status)) {
          // 避免重复创建 interval
          if (!interval) {
            interval = setInterval(() => getMyApplyDetail(route.query.id as string), 5000);
          }
        } else {
          clearTimer();
        }
      } finally {
        isLoading.value = false;
      }
    };

    const resolveBpaasDetail = async (res: any) => {
      setBpaasPayload({
        applicant: res.data.applicant,
        account_id: JSON.parse(res.data.content).account_id,
        id: res.data.id,
        bpaas_sn: +res.data.sn,
      });
      const bpaasRes = await accountStore.getBpassDetail(bpaasPayload);
      currentApplyData.value = { ...currentApplyData.value, ...bpaasRes.data };
      if (BpaasEndStatus.includes(bpaasRes.data.Status)) clearInterval(interval);
    };

    const clearTimer = () => {
      clearInterval(interval);
      interval = null as unknown as NodeJS.Timeout;
    };

    onUnmounted(clearTimer);

    // 撤销单据
    const handleCancel = async (id: string) => {
      isCancelBtnLoading.value = true;
      try {
        await accountStore.cancelApplyAccount(id);
        getMyApplyDetail(id);
      } finally {
        isCancelBtnLoading.value = false;
      }
    };

    watch(
      () => route.query.id,
      (id) => {
        if (id) {
          getMyApplyDetail(id as string);
        }
      },
      {
        immediate: true,
      },
    );

    const subTitle = computed(() => {
      return APPLICATION_TYPE_MAP[currentApplyData.value?.operation] || currentApplyData.value?.BpaasName;
    });

    const currentApplyDataContent = computed<any>(() => {
      let content = {};
      try {
        content = JSON.parse(currentApplyData.value?.content ?? null);
      } catch {
        console.error('parse currentApplyData.content error');
      }
      return content;
    });

    const isGotoSecurityRuleShow = computed(() =>
      [
        'create_security_group',
        'create_security_group_rule',
        'update_security_group_rule',
        'delete_security_group_rule',
      ].includes(currentApplyData.value?.type),
    );
    const isGotoSecurityRuleDisabled = computed(() => !currentApplyDataContent.value?.sg_id);
    const gotoSecurityRule = () => {
      routerAction.open({
        path: '/business/security/detail',
        query: {
          [GLOBAL_BIZS_KEY]: accountStore.bizs,
          id: currentApplyDataContent.value?.sg_id,
          vendor: currentApplyDataContent.value?.vendor ?? VendorEnum.ZIYAN,
          active: 'rule',
        },
      });
    };

    const render = () => {
      // 负载均衡详情
      if (!currentApplyData.value?.operation) return null;
      if (['create_load_balancer'].includes(currentApplyData.value.operation)) {
        return <Clb applicationDetail={currentApplyData.value} loading={isLoading.value} />;
      }
      return (
        <div class={'apply-detail-container'}>
          <DetailHeader>
            {{
              default: () => (
                <>
                  <span class={'title'}>申请单详情</span>
                  <span class={'sub-title'}>
                    &nbsp;-&nbsp;
                    {subTitle.value}
                  </span>
                </>
              ),
              right: () =>
                isGotoSecurityRuleShow.value ? (
                  <bk-button theme='primary' disabled={isGotoSecurityRuleDisabled.value} onClick={gotoSecurityRule}>
                    跳转至安全组规则
                  </bk-button>
                ) : undefined,
            }}
          </DetailHeader>
          <div class={'apply-content-wrapper'}>
            {applyContentRender(
              currentApplyData,
              curApplyKey,
              {
                cancelLoading: isCancelBtnLoading.value,
                onCancel: handleCancel,
              },
              {
                loading: isLoading.value,
                getBpaasDetail: getMyApplyDetail,
                bpaasPayload,
                bpaasJsonContent: currentApplyDataContent.value,
                isGotoSecurityRuleShow: isGotoSecurityRuleShow.value,
              },
            )}
          </div>
        </div>
      );
    };

    return render;
  },
});
