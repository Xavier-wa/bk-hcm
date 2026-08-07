import { Model, Column } from '@/decorator';

@Model()
export class FieldTcloud {
  // ---- apiOnly 字段：仅用于 API 提交，不在表单展示 ----
  @Column('string', { apiOnly: true })
  id: string;

  // ---- 基础字段 ----
  @Column('string', {
    name: '名称',
    required: true,
    rules: [
      {
        validator: (value: string) => value?.length >= 1 && value?.length <= 128,
        message: '长度为1~128个字符',
        trigger: 'blur',
      },
    ],
  })
  name: string;

  @Column('enum', {
    name: '类型',
    required: true,
    option: {
      '1': { label: '类型A', disabled: false },
      '2': { label: '类型B', disabled: false },
    },
    meta: {
      display: { appearance: 'radio' },
    },
  })
  type: string;

  @Column('string', {
    name: '描述',
    meta: {
      display: {
        props: { type: 'textarea', rows: 3, maxlength: 100 },
      },
    },
  })
  memo: string;

  // ---- 选择类字段 ----
  @Column('list', {
    name: '关联资源',
    required: true,
    meta: {
      display: {
        props: { idKey: 'id', displayKey: 'name' },
      },
    },
  })
  resource_id: string;

  // ---- 只读/预览字段 ----
  @Column('string', {
    name: '配置预览',
    meta: {
      display: {
        props: {
          type: 'textarea',
          readonly: true,
          rows: 10,
          placeholder: '内容由关联资源自动生成，不可手动编辑',
        },
      },
    },
  })
  config_preview: string;

  // ---- 数值字段 ----
  @Column('number', {
    name: '配额',
    required: true,
  })
  quota: number;

  // ---- 布尔字段 ----
  @Column('bool', {
    name: '是否启用',
  })
  enabled: boolean;
}
