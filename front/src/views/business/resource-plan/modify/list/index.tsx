import { defineComponent, computed, type PropType, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { Plus as PlusIcon } from 'bkui-vue/lib/icon';
import Panel from '@/components/panel';
import useColumns from '@/views/resource/resource-manage/hooks/use-scr-columns';
import cssModule from './index.module.scss';

import type { IPlanTicket } from '@/typings/resourcePlan';

export default defineComponent({
  props: {
    modelValue: Object as PropType<IPlanTicket>,
  },

  emits: ['show-add', 'show-modify', 'update:modelValue'],

  setup(props, { emit, expose }) {
    const { t } = useI18n();
    const { columns, settings } = useColumns('forecastList');

    const isValidateError = ref(false);

    const totalData = computed(() => {
      const totalData = [
        {
          label: t('CPU总核数：'),
          value: 0,
        },
        {
          label: t('内存总量：'),
          value: 0,
          unit: 'GB',
        },
        {
          label: t('云盘总量：'),
          value: 0,
          unit: 'GB',
        },
      ];

      props.modelValue.demands.forEach((demand) => {
        totalData[0].value += demand.cvm.cpu_core;
        totalData[1].value += demand.cvm.memory;
        totalData[2].value += demand.cbs.disk_size;
      });

      return totalData;
    });
    const tableColumns = computed(() => {
      return [
        ...columns,
        {
          label: t('操作'),
          width: 120,
          render: ({ index }: { index: number }) => (
            <>
              <bk-button text theme={'primary'} onClick={() => handleCopy(index)} class={cssModule['mr-5']}>
                {t('克隆')}
              </bk-button>
              <bk-button text theme={'primary'} onClick={() => handleModify(index)} class={cssModule['mr-5']}>
                {t('修改')}
              </bk-button>
              <bk-button text theme={'primary'} onClick={() => handleDelete(index)}>
                {t('移除')}
              </bk-button>
            </>
          ),
        },
      ];
    });

    const handleToAdd = () => {
      emit('show-add');
    };

    const handleModify = (index: number) => {
      emit('show-modify', props.modelValue.demands[index]);
    };

    const handleCopy = (index: number) => {
      const demand = JSON.parse(JSON.stringify(props.modelValue.demands[index]));
      emit('update:modelValue', {
        ...props.modelValue,
        demands: [
          ...props.modelValue.demands.slice(0, index + 1),
          demand,
          ...props.modelValue.demands.slice(index + 1),
        ],
      });
    };

    const handleDelete = (index: number) => {
      emit('update:modelValue', {
        ...props.modelValue,
        demands: [...props.modelValue.demands.slice(0, index), ...props.modelValue.demands.slice(index + 1)],
      });
    };

    const validate = () => {
      isValidateError.value = props.modelValue.demands.length <= 0;
      if (props.modelValue.demands.length > 0) {
        return Promise.resolve();
      }
      return Promise.reject(t('预测清单不能为空'));
    };

    expose({
      validate,
    });

    return () => (
      // 预测清单
      <Panel
        title={() => (
          <div>
            <span class='mr5'>{t('预测清单')}</span>
            <i
              class={'hcm-icon bkhcm-icon-info-line'}
              v-bk-tooltips={{
                content: (
                  <div>
                    <div>
                      请按业务需求，评估合理的资源需求量，领取资源超出或低于预测额度20%，会产生资源量1个月的成本罚金。
                    </div>
                    <div>如实际领取/资源预测=130%，则产生10%的罚金</div>
                    <div>如实际领取/资源预测=65%， 则产生15%的罚金</div>
                  </div>
                ),
              }}></i>
          </div>
        )}>
        <section class={cssModule.header}>
          <bk-button theme='primary' outline onClick={handleToAdd} class={cssModule.button}>
            <PlusIcon class={cssModule['plus-icon']} />
            {t('添加')}
          </bk-button>
          {totalData.value.map((item) => {
            return (
              <>
                <span>{item.label}</span>
                <span class={cssModule['total-number']}>{item.value || '--'}</span>
                <span class={cssModule['total-unit']}>{item.unit}</span>
              </>
            );
          })}
        </section>
        <bk-table
          class='table-container'
          showOverflowTooltip
          data={props.modelValue.demands}
          columns={tableColumns.value}
          settings={settings.value}
        />
        {isValidateError.value && props.modelValue.demands.length <= 0 ? (
          <span class={cssModule['error-txt']}>{t('预测清单不能为空')}</span>
        ) : (
          ''
        )}
      </Panel>
    );
  },
});
