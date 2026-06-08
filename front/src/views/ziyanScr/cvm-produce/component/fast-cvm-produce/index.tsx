import { defineComponent, ref, computed, watch, onMounted } from 'vue';
import useColumns from '@/views/resource/resource-manage/hooks/use-scr-columns';
import { useTable } from '@/hooks/useTable/useTable';
import { getRestrict } from '@/api/host/cvm';
import AreaSelector from '@/views/ziyanScr/hostApplication/components/AreaSelector';
import ZoneSelector from '@/views/ziyanScr/hostApplication/components/ZoneSelector';
import { HelpFill, Search } from 'bkui-vue/lib/icon';
import { Button, Form, Select, Sideslider } from 'bkui-vue';
import DevicetypeSelector from '@/views/ziyanScr/components/devicetype-selector/index.vue';
import { type IDeviceFamilyItem } from '@/store/config/device-family';
import HcmFormDeviceFamily from '@/components/form/device-family.vue';
import { VendorEnum } from '@/common/constant';
const { FormItem } = Form;

export default defineComponent({
  props: {
    modelValue: {
      type: Boolean,
      default: false,
    },
    title: {
      type: String,
      default: '快速生产',
    },
    actionText: {
      type: String,
      default: '快速生产',
    },
  },
  emits: ['update:modelValue'],
  setup(props, { attrs, emit }) {
    const isDisplay = ref(false);
    watch(
      () => props.modelValue,
      (val) => {
        isDisplay.value = val;
      },
      {
        immediate: true,
      },
    );
    const updateShowValue = () => {
      emit('update:modelValue', false);
    };
    const deviceTypeDisabled = ref(false);

    // 选中的机型族
    const selectedDeviceFamily = ref<IDeviceFamilyItem[]>([]);

    const filterDeviceFamily = computed(() => {
      return selectedDeviceFamily.value.flatMap((item) =>
        item.children?.length ? item.children?.map((child) => child.id) : [item.id],
      );
    });

    const defaultFilterForm = () => ({
      region: [],
      zone: [],
      device_type: [],
      device_group: [],
      cpu: '',
      mem: '',
    });
    const filterForm = ref(defaultFilterForm());
    const pageInfo = ref({
      start: 0,
      limit: 10,
    });
    const defaultFilter = () => ({
      op: 'and',
      rules: [],
    });
    const requestListParams = ref({
      filter: defaultFilter(),
      page: pageInfo.value,
    });
    const paramTableRules = computed(() => {
      const rules = [];
      ['region', 'zone', 'device_type'].map((item) => {
        if (Array.isArray(filterForm.value[item]) && filterForm.value[item].length) {
          const fieldNameMap: Record<string, string> = {
            region: 'dc.region',
            zone: 'dc.zone',
            device_type: 'dc.device_type',
          };
          rules.push({
            field: fieldNameMap[item],
            op: 'in',
            value: filterForm.value[item],
          });
        }
        return null;
      });

      if (filterDeviceFamily.value.length) {
        rules.push({
          field: 'device_family',
          op: 'in',
          value: filterDeviceFamily.value,
        });
      }

      if (filterForm.value.cpu) {
        rules.push({
          field: 'cpu_core',
          op: 'eq',
          value: filterForm.value.cpu,
        });
      }
      if (filterForm.value.mem) {
        rules.push({
          field: 'memory',
          op: 'eq',
          value: filterForm.value.mem,
        });
      }
      return rules;
    });
    const loadOrders = () => {
      const filter = paramTableRules.value.length
        ? {
            op: 'and',
            rules: paramTableRules.value,
          }
        : {
            op: 'and',
            rules: [],
          };
      const params = {
        filter,
        page: pageInfo.value,
      };
      requestListParams.value = { ...params };
      getListData();
    };
    const filterOrders = () => {
      pageInfo.value.start = 0;
      loadOrders();
    };
    const clearFilter = () => {
      filterForm.value = defaultFilterForm();
      deviceConfigDisabled.value = false;
      deviceTypeDisabled.value = false;
      selectedDeviceFamily.value = [];
      filterOrders();
    };
    const { columns } = useColumns('cvmFastProduceQuery');
    const tableColumns = [...columns];
    const { CommonTable, getListData } = useTable({
      tableOptions: {
        columns: tableColumns,
      },
      requestOption: {
        sortOption: {
          sort: 'capacity',
          order: 'DESC',
          legacy: false,
        },
      },
      scrConfig: () => {
        return {
          url: '/api/v1/woa/config/capacity/list_with_device_info',
          payload: {
            ...requestListParams.value,
          },
          pageEnableCountKey: 'count',
          clearRules: false,
        };
      },
    });
    const cpuList = ref([]);
    const memList = ref([]);
    const fetchCpuOrMem = async () => {
      const res = await getRestrict();
      const { cpu, mem } = res?.data || {};
      cpuList.value = cpu || [];
      memList.value = mem || [];
    };
    // CVM机型
    const cvmDevicetypeParams = computed(() => {
      const { region, zone, cpu, mem } = filterForm.value;
      return {
        vendor: VendorEnum.ZIYAN,
        region,
        zone,
        device_family: filterDeviceFamily.value,
        cpu,
        mem,
        disable: false,
      };
    });

    const handleDeviceGroupChange = (items: IDeviceFamilyItem[]) => {
      filterForm.value.cpu = '';
      filterForm.value.mem = '';
      filterForm.value.device_type = [];
      selectedDeviceFamily.value = items;
    };
    const deviceConfigDisabled = ref(false);
    const handleDeviceTypeChange = () => {
      filterForm.value.cpu = '';
      filterForm.value.mem = '';
      deviceConfigDisabled.value = filterForm.value.device_type.length > 0;
    };
    const handleDeviceConfigChange = () => {
      filterForm.value.device_type = [];

      const { cpu, mem } = filterForm.value;

      deviceTypeDisabled.value = Boolean(cpu || mem);
    };
    onMounted(() => {
      fetchCpuOrMem();
    });
    return () => (
      <Sideslider
        class='common-sideslider'
        v-bind={attrs}
        width='1080'
        v-model:isShow={isDisplay.value}
        title={props.title}
        before-close={updateShowValue}>
        {{
          default: () => (
            <div class='apply-list-container common-sideslider-content'>
              <div class={'filter-container'}>
                <Form formType='vertical' class='scr-form-wrapper' model={filterForm}>
                  <FormItem label='地域'>
                    <AreaSelector multiple v-model={filterForm.value.region} params={{ resourceType: 'QCLOUDCVM' }} />
                  </FormItem>
                  <FormItem label='园区'>
                    <ZoneSelector
                      multiple
                      v-model={filterForm.value.zone}
                      params={{ resourceType: 'QCLOUDCVM', region: filterForm.value.region }}
                    />
                  </FormItem>
                  <FormItem label='实例族'>
                    <HcmFormDeviceFamily
                      v-model={filterForm.value.device_group}
                      filterable
                      multiple
                      clearable
                      collapse-tags
                      onChange={handleDeviceGroupChange}
                    />
                    <div
                      class='tool-pos'
                      v-bk-tooltips={{
                        theme: 'light',
                        content: (
                          <div>
                            实例族相关概念请
                            <a
                              class='link-type'
                              href='https://cloud.tencent.com/document/product/213/11518'
                              target='_blank'>
                              查看文档
                            </a>
                          </div>
                        ),
                      }}>
                      <HelpFill />
                    </div>
                  </FormItem>
                  <FormItem label='机型'>
                    <DevicetypeSelector
                      v-model={filterForm.value.device_type}
                      resourceType='cvm'
                      params={cvmDevicetypeParams.value}
                      multiple
                      disabled={deviceTypeDisabled.value}
                      onChange={handleDeviceTypeChange}
                    />
                  </FormItem>
                  <FormItem label='CPU(核)'>
                    <Select
                      v-model={filterForm.value.cpu}
                      clearable
                      filterable
                      placeholder='请选择'
                      disabled={deviceConfigDisabled.value}
                      onChange={handleDeviceConfigChange}>
                      {cpuList.value.map((item) => {
                        return <Select.Option key={item} name={item} id={item} />;
                      })}
                    </Select>
                  </FormItem>
                  <FormItem label='内存(G)'>
                    <Select
                      v-model={filterForm.value.mem}
                      clearable
                      filterable
                      placeholder='请选择'
                      disabled={deviceConfigDisabled.value}
                      onChange={handleDeviceConfigChange}>
                      {memList.value.map((item) => {
                        return <Select.Option key={item} name={item} id={item} />;
                      })}
                    </Select>
                  </FormItem>
                </Form>
                <div class='btn-container'>
                  <Button theme='primary' onClick={filterOrders}>
                    <Search />
                    查询
                  </Button>
                  <Button onClick={() => clearFilter()}>重置</Button>
                </div>
              </div>
              <CommonTable />
            </div>
          ),
        }}
      </Sideslider>
    );
  },
});
