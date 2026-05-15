import { defineComponent, ref, watch } from 'vue';
import apiService from '../../../../api/scrApi';
import isEqual from 'lodash/isEqual';
export default defineComponent({
  name: 'AntiAffinityLevelSelect',
  props: {
    modelValue: {
      type: [String, Array],
      default: '',
    },
    params: {
      type: Object,
      default: () => ({}),
    },
  },
  emits: ['affinitychange'],
  setup(props, { emit }) {
    const options = ref([]);
    const optionsRequestId = Symbol();
    const selectedValue = ref();
    const handleSelectorChange = (value: string) => {
      emit('affinitychange', value);
    };
    const initValue = (newVal) => {
      selectedValue.value = newVal;
    };
    const fetchOptions = async () => {
      const { resourceType, hasZone } = props.params;

      try {
        const { info } = await apiService.getAntiAffinityLevels(resourceType, hasZone, {
          requestId: optionsRequestId,
        });
        options.value =
          info.map((item: any) => ({
            value: item.level,
            label: item.description,
          })) || [];
      } catch {
        options.value = [];
      }
    };
    watch(
      () => props.modelValue,
      (newVal) => {
        initValue(newVal);
      },
      { immediate: true },
    );
    watch(
      () => props.params.resourceType,
      (newVal, oldVal) => {
        if (!isEqual(newVal, oldVal)) {
          fetchOptions();
        }
      },
      { immediate: true },
    );
    watch(
      () => props.params.hasZone,
      (newVal, oldVal) => {
        if (!isEqual(newVal, oldVal)) {
          fetchOptions();
        }
      },
      { immediate: true },
    );
    return () => (
      <div>
        <bk-select v-model={selectedValue.value} onChange={handleSelectorChange} filterable>
          {options.value.map((opt) => (
            <bk-option key={opt.value} label={opt.label} value={opt.value} />
          ))}
        </bk-select>
      </div>
    );
  },
});
