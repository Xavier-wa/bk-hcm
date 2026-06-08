import { defineComponent, ref, watch, nextTick } from 'vue';
import './index.scss';
export default defineComponent({
  name: 'ResourceType',
  props: {
    updateTypes: {
      type: Function,
      default: () => {},
    },
    returnPlan: {
      type: Object,
      default: () => ({
        cvm: '',
        pm: '',
        skipConfirm: true,
      }),
    },
  },
  setup(props, { expose }) {
    const recycleForm = ref();
    const handleConfirm = async () => {
      await recycleForm.value.validate();
    };
    const clearValidate = () => {
      nextTick(() => {
        recycleForm.value?.clearValidate();
      });
    };
    expose({ handleConfirm, clearValidate });
    const rules = ref({
      cvm: [{ required: true, message: '请选择CVM回收类型', trigger: 'change' }],
      pm: [{ required: true, message: '请选择物理机回收类型', trigger: 'change' }],
      skipConfirm: [{ required: true, message: '请选择是否跳过二次确认', trigger: 'change' }],
    });

    const cvmOptions = ref([
      { id: 'IMMEDIATE', name: '立即销毁' },
      { id: 'DELAY', name: '延迟销毁(隔离15天)' },
    ]);

    const pmOptions = ref([
      { id: 'IMMEDIATE', name: '立即销毁(隔离2小时)' },
      { id: 'DELAY', name: '延迟销毁(隔离1天)' },
    ]);

    watch(
      () => props.returnPlan,
      (newVal: any) => {
        props.updateTypes(newVal);
      },
      { deep: true },
    );

    return () => (
      <div class='recycleForm'>
        <bk-form ref={recycleForm} model={props.returnPlan} rules={rules.value} label-width='220'>
          <bk-form-item label='CVM回收类型' property='cvm' required>
            <bk-select v-model={props.returnPlan.cvm} class='wid300' filterable>
              {cvmOptions.value.map((item) => (
                <bk-option key={item.id} label={item.name} value={item.id} />
              ))}
            </bk-select>
            <div style='color: #ccc; font-size: 12px;'>CVM 非立即销毁隔离15天，隔离期间费用仍由业务承担</div>
          </bk-form-item>
          <bk-form-item label='物理机回收类型' property='pm' required>
            <bk-select v-model={props.returnPlan.pm} class='wid300' filterable>
              {pmOptions.value.map((item) => (
                <bk-option key={item.id} label={item.name} value={item.id} />
              ))}
            </bk-select>
            <div style='color:#ccc; font-size: 12px;'>
              物理机非立即销毁隔离1天，立即销毁隔离2小时，隔离期间费用仍由业务承担
            </div>
          </bk-form-item>
          <bk-form-item label='是否跳过“非空负载二次确认”' property='skipConfirm' required>
            <bk-radio-group v-model={props.returnPlan.skipConfirm}>
              <bk-radio label={true}>是</bk-radio>
              <bk-radio label={false}>否</bk-radio>
            </bk-radio-group>
            <div style='color: #ccc; font-size: 12px; line-height: 20px;'>
              <p>
                公司回收流程会通过检查 CPU
                负载判断设备是否已空闲。若检测为非空负载，会暂停回收，并邮件通知维护人再次确认。
              </p>
              <p>若该选项选为“否”,请留意题为“非空负载设备退回二次确认”的邮件。</p>
            </div>
          </bk-form-item>
        </bk-form>
      </div>
    );
  },
});
