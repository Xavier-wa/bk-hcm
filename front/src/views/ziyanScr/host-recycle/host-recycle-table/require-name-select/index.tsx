import { defineComponent, ref, watch, onMounted } from 'vue';
import { getRequireTypes } from '@/api/host/task';
export default defineComponent({
  props: {
    modelValue: {
      type: String,
      default: '',
    },
  },
  emits: ['update:modelValue'],
  setup(props, { attrs, emit }) {
    const options = ref([]);
    const selectedValue = ref('');
    watch(
      () => props.modelValue,
      (val) => {
        selectedValue.value = val;
      },
      { immediate: true },
    );
    const updateSelectedValue = (value) => {
      emit('update:modelValue', value);
    };
    const fetchOptions = () => {
      getRequireTypes()
        .then((res) => {
          options.value = res.data?.info?.map((item) => ({
            label: item.require_name,
            value: item.require_name,
          }));
        })
        .catch(() => {
          options.value = [];
        });
    };
    onMounted(() => {
      fetchOptions();
    });
    return () => (
      <bk-select modelValue={selectedValue} onUpdate:modelValue={updateSelectedValue} v-bind={attrs} filterable>
        {options.value.map(({ value, label }) => {
          return <bk-option key={value} label={label} value={value} />;
        })}
      </bk-select>
    );
  },
});
