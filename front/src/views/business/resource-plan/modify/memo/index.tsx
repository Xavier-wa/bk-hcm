import { defineComponent, ref, PropType } from 'vue';
import { useI18n } from 'vue-i18n';
import Panel from '@/components/panel';

import planRemark from '../plan-remark.js';

import cssModule from './index.module.scss';

import type { IPlanTicket } from '@/typings/resourcePlan';

export default defineComponent({
  props: {
    modelValue: Object as PropType<IPlanTicket>,
  },

  emits: ['update:modelValue'],

  setup(props, { emit, expose }) {
    const { t } = useI18n();
    const rules = {
      remark: [
        {
          validator: (value: string) => value.length >= 20,
          message: t('字数不少于20字'),
          trigger: 'change',
        },
        {
          validator: (value: string) => value !== planRemark,
          message: t('需要对预测说明进行修改'),
          trigger: 'change',
        },
      ],
    };
    const fromRef = ref();

    const updateModelValue = (value: string) => {
      emit('update:modelValue', {
        ...props.modelValue,
        remark: value,
      });
    };

    const validate = () => {
      return fromRef.value.validate();
    };

    expose({
      validate,
    });

    return () => (
      <Panel title={t('预测信息')}>
        <bk-form form-type='vertical' ref={fromRef} rules={rules} model={props.modelValue} class={cssModule.home}>
          <bk-form-item label={t('预测说明')} property='remark' required>
            <bk-input
              type='textarea'
              clearable
              maxlength={1024}
              showWordLimit
              placeholder={planRemark}
              modelValue={props.modelValue.remark}
              onChange={updateModelValue}
            />
          </bk-form-item>
        </bk-form>
      </Panel>
    );
  },
});
