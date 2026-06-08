import { Button, Input, Popover, Select } from 'bkui-vue';
import { defineComponent, ref, watch, onMounted } from 'vue';
import './index.scss';
import { EditLine } from 'bkui-vue/lib/icon';
export default defineComponent({
  name: 'EditItem',
  props: {
    modelValue: {
      type: [String, Number, Boolean],
      default: '',
    },
    type: {
      type: String,
      default: 'text',
    },
    save: {
      type: [Function, Promise],
      default: null,
    },
    controlAttrs: {
      type: Object,
      default: () => ({}),
    },
  },
  emits: ['update:modelValue'],
  setup(props) {
    const content = ref();
    const options = ref([
      {
        label: '是',
        value: true,
      },
      {
        label: '否',
        value: false,
      },
    ]);
    const isEdit = ref(false);
    watch(
      () => props.modelValue,
      () => {
        content.value = props.modelValue;
      },
      {
        immediate: true,
      },
    );
    const handleEdit = () => {
      isEdit.value = true;
    };
    const handleSave = async () => {
      try {
        await props.save(content.value);
        isEdit.value = false;
      } catch (error) {}
    };
    const handleCancel = () => {
      content.value = props.modelValue;
      isEdit.value = false;
    };

    onMounted(() => {});
    return () => (
      <Popover trigger='manual' isShow={isEdit.value} theme='light'>
        {{
          default: () => (
            <div class='pop-btn'>
              {props.type === 'boolean' ? <div class='bool-color'>{props.modelValue ? '是' : '否'}</div> : null}
              {props.type === 'number' ? <div>{props.modelValue}</div> : null}
              {!['boolean', 'number'].includes(props.type) ? <div>{props.modelValue || '-'}</div> : null}
              <Button text theme='primary' onClick={handleEdit}>
                <EditLine />
              </Button>
            </div>
          ),
          content: () => (
            <div class='pop-content-container'>
              {props.type === 'textarea' ? (
                <Input v-bind={props.controlAttrs} v-model={content.value} type='textarea' />
              ) : null}
              {props.type === 'boolean' ? (
                <Select v-model={content.value} filterable v-bind={props.controlAttrs}>
                  {options.value.map((item) => (
                    <Select.Option id={item.value} name={item.label} />
                  ))}
                </Select>
              ) : null}
              {props.type === 'number' ? (
                <Input v-bind={props.controlAttrs} type='number' v-model={content.value} />
              ) : null}
              {props.type === 'text' ? <Input v-bind={props.controlAttrs} v-model={content.value} /> : null}
              <Button title='保存' text theme='primary' onClick={handleSave}>
                保存
              </Button>
              <Button title='取消' text theme='primary' onClick={handleCancel}>
                取消
              </Button>
            </div>
          ),
        }}
      </Popover>
    );
  },
});
