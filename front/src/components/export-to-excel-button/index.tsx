import { defineComponent } from 'vue';
import { exportTableToExcel } from '@/utils';
export default defineComponent({
  props: {
    data: {
      type: Array,
      default: () => [],
      required: true,
    },
    columns: {
      type: Array,
      required: true,
    },
    filename: {
      type: String,
      default: `导出文件`,
    },
    text: {
      type: String,
      default: `导出`,
    },
  },
  emits: ['update:modelValue'],
  setup(props, { attrs }) {
    const exportToExcel = () => {
      exportTableToExcel(props.data, props.columns, props.filename);
    };
    return () => (
      <bk-button v-bind={attrs} disabled={!props.data.length} onClick={exportToExcel}>
        <i class='hcm-icon bkhcm-icon-download mr8'></i>
        {props.text}
      </bk-button>
    );
  },
});
