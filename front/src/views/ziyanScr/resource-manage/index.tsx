import { defineComponent, onMounted, reactive, ref, watch } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { Button, DatePicker, Select, Tab, TagInput } from 'bkui-vue';
import { Plus } from 'bkui-vue/lib/icon';
import MemberSelect from '@/components/MemberSelect';
import FilterFormItems from './filter-form-items';
import { useZiyanScrStore, useUserStore } from '@/store';
import useColumns from '@/views/resource/resource-manage/hooks/use-scr-columns';
import { useTable } from '@/hooks/useTable/useTable';
import { cleanPayload, getDate } from '@/utils';
import { getTableNewRowClass, timeFormatter } from '@/common/util';
import './index.scss';

const { TabPanel } = Tab;

interface ResourceManageFilterType {
  id?: string[];
  bk_username?: string[];
  phase?: string[];
  start: string;
  end: string;
}

/**
 * SCR - 资源上下架
 */
export default defineComponent({
  name: 'ScrResourceManage',
  setup() {
    const userStore = useUserStore();
    const router = useRouter();
    const route = useRoute();
    const ziyanScrStore = useZiyanScrStore();

    const { columns: scrResourceOnlineColumns } = useColumns('scrResourceOnline');
    const { columns: scrResourceOfflineColumns } = useColumns('scrResourceOffline');

    const {
      CommonTable: ScrResourceOnlineTable,
      getListData: reloadScrResourceOnlineTable,
      pagination: scrResourceOnlinePagination,
    } = useTable({
      tableOptions: { columns: scrResourceOnlineColumns, extra: { rowClass: getTableNewRowClass() } },
      requestOption: { dataPath: 'data.info', sortOption: { sort: 'update_at', order: 'DESC' } },
      scrConfig: () => ({
        url: '/api/v1/woa/pool/findmany/launch/task',
        payload: {
          ...cleanPayload(filter),
          id: filter.id.length ? filter.id.map((id) => Number(id)) : undefined,
          start: timeFormatter(filter.start, 'YYYY-MM-DD'),
          end: timeFormatter(filter.end, 'YYYY-MM-DD'),
        },
      }),
    });

    const {
      CommonTable: ScrResourceOfflineTable,
      getListData: reloadScrResourceOfflineTable,
      pagination: scrResourceOfflinePagination,
    } = useTable({
      tableOptions: { columns: scrResourceOfflineColumns, extra: { rowClass: getTableNewRowClass() } },
      requestOption: { dataPath: 'data.info', sortOption: { sort: 'update_at', order: 'DESC' } },
      scrConfig: () => ({
        url: '/api/v1/woa/pool/findmany/recall/task',
        payload: {
          ...cleanPayload(filter),
          id: filter.id.length ? filter.id.map((id) => Number(id)) : undefined,
          start: timeFormatter(filter.start, 'YYYY-MM-DD'),
          end: timeFormatter(filter.end, 'YYYY-MM-DD'),
        },
      }),
    });

    const reloadDataList = () => {
      if (activeType.value === 'online') {
        scrResourceOnlinePagination.start = 0;
        reloadScrResourceOnlineTable();
      } else {
        scrResourceOfflinePagination.start = 0;
        reloadScrResourceOfflineTable();
      }
    };

    const types = [
      {
        label: '资源上架',
        value: 'online',
        Component: ScrResourceOnlineTable,
      },
      {
        label: '资源下架',
        value: 'offline',
        Component: ScrResourceOfflineTable,
      },
    ];

    const activeType = ref(route.query.type || 'online');

    watch(
      activeType,
      (val) => {
        router.push({ query: { type: val } });
        reloadDataList();
      },
      {
        immediate: true,
      },
    );

    const getDefaultFilter = (): ResourceManageFilterType => ({
      id: [],
      bk_username: [userStore.username],
      phase: [],
      start: getDate('yyyy-MM-dd', -30),
      end: getDate('yyyy-MM-dd', 0),
    });
    const filter = reactive(getDefaultFilter());

    watch(
      () => userStore.username,
      (username) => {
        filter.bk_username = [username];
      },
    );

    // 路由跳转至资源上/下架页面
    const gotoCreatePage = () => {
      router.push({ name: 'scrResourceManageCreate', query: { type: activeType.value } });
    };
    const filterFormItems = [
      {
        label: '单号',
        render: () => (
          <TagInput
            v-model={filter.id}
            allow-create
            collapse-tags
            createTagValidator={(tag) => /^[1-9]\d*$/.test(tag)}
            pasteFn={(v) => v.split(/\r\n|\n|\r/).map((tag) => ({ id: tag, name: tag }))}
          />
        ),
      },
      {
        label: '创建人',
        render: () => (
          <MemberSelect
            v-model={filter.bk_username}
            defaultUserlist={[
              {
                username: userStore.username,
                display_name: userStore.username,
              },
            ]}
          />
        ),
      },
      {
        label: '创建时间',
        render: () => (
          <div class='flex-row'>
            <DatePicker v-model={filter.start} />
            <span class='m4'>-</span>
            <DatePicker v-model={filter.end} />
          </div>
        ),
      },
      {
        label: '单据状态',
        render: () => (
          <Select v-model={filter.phase} multiple filterable>
            {phaseList.value.map(({ description, status }) => (
              <Select.Option key={status} id={status} name={description} />
            ))}
          </Select>
        ),
      },
    ];

    const phaseList = ref([]);
    onMounted(() => {
      // 组件挂载完成后，请求单据状态list
      const getPhaseList = async () => {
        const res = await ziyanScrStore.getTaskStatusList();
        phaseList.value = res.data.info || [];
      };

      getPhaseList();
      reloadDataList();
    });

    const clearFilter = () => {
      Object.assign(filter, getDefaultFilter());
      reloadDataList();
    };

    return () => (
      <div class='scr-resource-manage-page'>
        <Tab v-model:active={activeType.value} type='card-grid' class='tab-wrapper'>
          {types.map(({ label, value, Component }) => (
            <TabPanel key={value} label={label} name={value} renderDirective='if'>
              <div class='manage-container'>
                <FilterFormItems config={filterFormItems} handleSearch={reloadDataList} handleClear={clearFilter} />
                <div class={'manage-com'}>
                  <Button class={'manage-com-button'} theme='primary' onClick={gotoCreatePage}>
                    <Plus class='f22' /> 发起{activeType.value === 'online' ? '上架' : '下架'}
                  </Button>
                  <Component style={{ height: 'calc(100% - 250px)' }} />
                </div>
              </div>
            </TabPanel>
          ))}
        </Tab>
      </div>
    );
  },
});
