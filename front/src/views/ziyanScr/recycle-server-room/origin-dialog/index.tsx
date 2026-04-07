import { defineComponent, computed, PropType, watch, ref } from 'vue';
import ExportToExcelButton from '@/components/export-to-excel-button';
import useColumns from '@/views/resource/resource-manage/hooks/use-scr-columns';
import cssModule from '../dialog.module.scss';
import { useI18n } from 'vue-i18n';

import { IDissolve, IDissolveHostCurrentListParam } from '@/typings/ziyanScr';
import { useZiyanScrStore } from '@/store/ziyanScr';

export default defineComponent({
  components: {
    ExportToExcelButton,
  },
  props: {
    isShow: {
      type: Boolean,
    },
    searchParams: {
      type: Object as PropType<IDissolveHostCurrentListParam>,
    },
    rowData: {
      type: Object as PropType<IDissolve>,
    },
  },
  emits: ['update:is-show'],
  setup(props, { emit }) {
    const { t } = useI18n();
    const { columns: rawColumns, generateColumnsSettings } = useColumns('decommissionDetails');
    const ziyanScrStore = useZiyanScrStore();

    // origin-dialog 中 project_name 和 svr_type_name 默认不展示
    const columns = rawColumns.map((col: any) => ({
      ...col,
      isDefaultShow: ['project_name', 'svr_type_name'].includes(col.field) ? false : col.isDefaultShow,
    }));
    const settings = generateColumnsSettings(columns);

    const isLoading = ref(false);
    const tableData = ref();
    const pagination = ref({
      current: 1,
      limit: 10,
      count: 0,
    });
    const totalCount = ref(0);
    const activeModule = ref('');

    const title = computed(() =>
      t('{title}（原始）_设备详情', { title: props?.searchParams?.bk_biz_names?.[0] || '' }),
    );

    const moduleHostCountMap = ref(new Map<string, number>());

    const handleModuleSearch = (module?: string) => {
      activeModule.value = module;
      getData(module ? [module] : undefined);
    };

    const getData = async (moduleNames?: string[]) => {
      try {
        isLoading.value = true;
        const res = await ziyanScrStore.dissolveHostOriginList({
          page: {
            count: false,
            start: 0,
            limit: 50000,
          },
          ...props.searchParams,
          module_names: moduleNames ?? props.searchParams.module_names,
        });
        tableData.value = res.data.details;
        pagination.value.count = tableData.value?.length || 0;

        // 未指定module时认为是“全部”
        if (!moduleNames) {
          // 根据列表数据按module_name分组并统计每个分组的机器数量
          tableData.value.forEach((item: any) => {
            moduleHostCountMap.value.set(item.module_name, (moduleHostCountMap.value.get(item.module_name) ?? 0) + 1);
          });
          totalCount.value = pagination.value.count;
        }
      } catch (error) {
        console.error(error, 'error'); // eslint-disable-line no-console
      } finally {
        isLoading.value = false;
      }
    };

    const handleClose = () => {
      emit('update:is-show', false);
    };
    watch(
      () => props.searchParams,
      () => {
        if (props.isShow) {
          moduleHostCountMap.value.clear();
          getData();
          activeModule.value = '';
        }
      },
    );
    return () => (
      <bk-dialog
        class='step-dialog'
        width={'80%'}
        dialog-type='show'
        theme='primary'
        headerAlign='left'
        title={title.value}
        isShow={props.isShow}
        onClosed={() => handleClose()}>
        <div class={cssModule['dialog-content']}>
          <div class={cssModule['module-list']}>
            <div
              class={[cssModule['module-item'], { [cssModule.active]: !activeModule.value }]}
              onClick={() => handleModuleSearch()}>
              全部 <em class={cssModule['item-count']}>{totalCount.value}</em>
            </div>
            {[...moduleHostCountMap.value]
              .sort((a, b) => b[1] - a[1])
              .map(([name, count]) => (
                <div
                  key={name}
                  class={[cssModule['module-item'], { [cssModule.active]: activeModule.value === name }]}
                  onClick={() => handleModuleSearch(name)}>
                  {name} <em class={cssModule['item-count']}>{count}</em>
                </div>
              ))}
          </div>
          <div class={cssModule['data-table']} v-bkloading={{ loading: isLoading.value }}>
            <div class={cssModule.title}>
              <export-to-excel-button data={tableData.value} columns={columns} filename={title.value} theme='primary' />
              <span class={cssModule['total-num']}>
                {t('总条数：')} {pagination.value.count}
              </span>
            </div>
            <bk-table
              show-overflow-tooltip
              data={tableData.value}
              columns={columns}
              pagination={pagination.value}
              settings={settings.value}
              max-height={'calc(100% - 52px)'}
            />
          </div>
        </div>
      </bk-dialog>
    );
  },
});
