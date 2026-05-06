import { defineComponent, ref, type PropType, onBeforeMount, watch, computed, nextTick } from 'vue';
import Panel from '@/components/panel';
import { useI18n } from 'vue-i18n';
import { useResourcePlanStore } from '@/store';
import cssModule from './index.module.scss';

import type { IPlanTicketDemand, IDiskType } from '@/typings/resourcePlan';
import { AdjustType } from '@/typings/plan';
import { isEqual } from 'lodash';

export default defineComponent({
  props: {
    planTicketDemand: Object as PropType<IPlanTicketDemand>,
    resourceType: String,
    type: String as PropType<AdjustType>,
    originPlanTicketDemand: Object as PropType<IPlanTicketDemand>,
  },

  emits: ['update:planTicketDemand', 'disableTypeChange'],

  setup(props, { emit, expose }) {
    const { t } = useI18n();
    const resourcePlanStore = useResourcePlanStore();

    const formRef = ref();
    const isLoadingDiskTypes = ref(false);
    const diskTypes = ref<IDiskType[]>([]);

    const isCVM = computed(() => props.resourceType === 'cvm');

    const rules = computed(() => ({
      disk_io: [
        {
          validator: (value: number) => {
            return value > 0;
          },
          message: t('单实例磁盘IO应大于0'),
          trigger: 'change',
        },
      ],
      disk_per_size: [
        {
          validator: (value: number) => {
            return value >= 0;
          },
          message: isCVM.value ? t('云磁盘容量/实例应大于等于0') : t('云磁盘容量/块应大于等于0'),
          trigger: 'change',
        },
      ],
      disk_num: [
        {
          validator: (value: number) => {
            return props.resourceType === 'cbs' ? value > 0 : true;
          },
          message: t('所需数量应大于0'),
          trigger: 'change',
        },
      ],
    }));

    const handleUpdatePlanTicketDemand = (key: string, value: unknown) => {
      const newDemand = { ...props.planTicketDemand, cbs: { ...props.planTicketDemand.cbs, [key]: value } };

      if (props.type === AdjustType.none) {
        emit('update:planTicketDemand', newDemand);
        return;
      }

      // 这里只变更cbs信息，可以简化比较逻辑
      if (!isEqual(props.originPlanTicketDemand.cbs, newDemand.cbs)) {
        emit('disableTypeChange', AdjustType.time);
      } else if (isEqual(newDemand, props.originPlanTicketDemand)) {
        emit('disableTypeChange', AdjustType.none);
      }

      emit('update:planTicketDemand', newDemand);
    };

    const handleUpdateDiskType = (val: string) => {
      const diskType = diskTypes.value.find((diskType) => diskType.disk_type === val);
      handleUpdatePlanTicketDemand('disk_type', diskType?.disk_type || '');
      nextTick(() => {
        handleUpdatePlanTicketDemand('disk_type_name', diskType.disk_type_name || '');

        nextTick(() => {
          handleUpdatePlanTicketDemand('disk_io', 15);
        });
      });
    };

    const validate = () => {
      return formRef.value?.validate();
    };

    const clearValidate = () => {
      return formRef.value?.clearValidate();
    };

    const getDiskTypes = () => {
      isLoadingDiskTypes.value = true;
      resourcePlanStore
        .getDiskTypes()
        .then((data: { data: { details: IDiskType[] } }) => {
          diskTypes.value = data?.data?.details || [];
        })
        .finally(() => {
          isLoadingDiskTypes.value = false;
        });
    };

    const calcDiskSize = () => {
      const num =
        (props.resourceType === 'cvm' ? props.planTicketDemand.cvm.os : props.planTicketDemand.cbs.disk_num) || 0;
      const perSize = props.planTicketDemand.cbs.disk_per_size || 0;
      handleUpdatePlanTicketDemand('disk_size', num * perSize);
    };

    watch(
      [
        () => props.planTicketDemand.cbs?.disk_num,
        () => props.planTicketDemand.cbs?.disk_per_size,
        () => props.planTicketDemand.cvm.os,
        () => props.resourceType,
      ],
      ([cbsDiskNum, cbsDiskPerSize]) => {
        if (cbsDiskNum !== undefined || cbsDiskPerSize !== undefined) {
          calcDiskSize();
        }
      },
    );

    onBeforeMount(getDiskTypes);

    expose({
      validate,
      clearValidate,
    });

    return () =>
      props.planTicketDemand.cbs && (
        <Panel title={t('CBS云磁盘信息')} noShadow>
          <bk-form
            form-type='vertical'
            ref={formRef}
            rules={rules.value}
            model={props.planTicketDemand.cbs}
            class={cssModule.home}>
            <bk-form-item label={t('云盘类型')} property='disk_type' required class={cssModule['span-line']}>
              <bk-select
                disabled={props.type === AdjustType.time}
                filterable
                clearable
                loading={isLoadingDiskTypes.value}
                modelValue={props.planTicketDemand.cbs.disk_type}
                popoverOptions={{ boundary: 'parent' }}
                onChange={(val: string) => handleUpdateDiskType(val)}>
                {diskTypes.value.map((diskType) => (
                  <bk-option id={diskType.disk_type} name={diskType.disk_type_name}></bk-option>
                ))}
              </bk-select>
            </bk-form-item>
            {/* 编辑CBS时仅可调整云盘总量 */}
            {!isCVM.value && AdjustType.none !== props.type ? (
              <bk-form-item label={t('云盘总量')} property='disk_size' required class={cssModule['span-half-line']}>
                <bk-input
                  disabled={props.type === AdjustType.time}
                  type='number'
                  suffix={'GB'}
                  min={0}
                  modelValue={props.planTicketDemand.cbs.disk_size}
                  onChange={(val: number) => handleUpdatePlanTicketDemand('disk_size', val || 0)}
                  clearable
                />
              </bk-form-item>
            ) : (
              <>
                <bk-form-item
                  label={isCVM.value ? t('云磁盘容量/实例') : t('云磁盘容量/块')}
                  property='disk_per_size'
                  required
                  class={cssModule['span-half-line']}>
                  <bk-input
                    disabled={props.type === AdjustType.time}
                    type='number'
                    suffix={'GB'}
                    min={0}
                    modelValue={props.planTicketDemand.cbs.disk_per_size}
                    onChange={(val: number) => handleUpdatePlanTicketDemand('disk_per_size', val || 0)}
                    clearable
                  />
                </bk-form-item>
                <bk-form-item
                  label={t('云盘总量')}
                  description={
                    props.resourceType === 'cbs' ? t('需要的云磁盘总量') : t('所有实例的系统盘，数据盘总容量')
                  }
                  property='name'>
                  <span class={cssModule.number}>{props.planTicketDemand.cbs.disk_size} GB</span>
                </bk-form-item>
                {props.resourceType === 'cbs' ? (
                  <bk-form-item
                    label={t('所需数量')}
                    description={t('需要的云磁盘块数')}
                    property='disk_num'
                    required
                    class={cssModule['span-line']}>
                    <bk-input
                      disabled={props.type === AdjustType.time}
                      type='number'
                      min={0}
                      suffix={t('块')}
                      modelValue={props.planTicketDemand.cbs.disk_num}
                      onChange={(val: number) => handleUpdatePlanTicketDemand('disk_num', val || 0)}
                      clearable
                    />
                  </bk-form-item>
                ) : (
                  ''
                )}
              </>
            )}

            <bk-form-item
              label={t('单实例磁盘IO')}
              description={t('磁盘IO吞吐需求，无特殊要求填写15；高性能云盘上限150，SSD云硬盘上限260')}
              property='disk_io'
              class={cssModule['span-line']}>
              <bk-input
                type='number'
                min={0}
                max={props.planTicketDemand.cbs.disk_type === 'CLOUD_PREMIUM' ? 150 : 260}
                disabled={!props.planTicketDemand.cbs.disk_type || props.type === AdjustType.time}
                modelValue={props.planTicketDemand.cbs.disk_io}
                onChange={(val: number) => handleUpdatePlanTicketDemand('disk_io', val || 0)}
                clearable
              />
            </bk-form-item>
          </bk-form>
        </Panel>
      );
  },
});
