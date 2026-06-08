import { PropType, defineComponent, onMounted, ref, watch } from 'vue';
import { Select } from 'bkui-vue';
import './index.scss';
import { lodashGet } from '@/utils/scr/lodashGet';

export default defineComponent({
  name: 'ScrCreateFilterSelector',
  props: {
    modelValue: {
      type: Array as PropType<string[]>,
    },
    api: {
      type: Function as PropType<() => Promise<any>>,
      required: true,
    },
    multiple: {
      type: Boolean,
      default: true,
    },
    optionIdPath: {
      type: String,
      default: '',
    },
    optionNamePath: {
      type: String,
      default: '',
    },
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const list = ref([]);
    const selected = ref(props.modelValue);

    onMounted(() => {
      const getOptionList = async () => {
        const res = await props.api();
        list.value = res.data.info || [];
      };

      getOptionList();
    });

    watch(
      selected,
      (val) => {
        emit('update:modelValue', val);
      },
      { deep: true },
    );

    watch(
      () => props.modelValue,
      (val) => {
        selected.value = val;
      },
      {
        deep: true,
      },
    );

    return () => (
      <Select
        v-model={selected.value}
        multiple={props.multiple}
        multipleMode={props.multiple ? 'tag' : undefined}
        collapseTags
        clearable
        filterable>
        {list.value.map((item) => (
          <Select.Option
            key={lodashGet(item, props.optionIdPath)}
            id={lodashGet(item, props.optionIdPath)}
            name={lodashGet(item, props.optionNamePath)}
          />
        ))}
      </Select>
    );
  },
});
