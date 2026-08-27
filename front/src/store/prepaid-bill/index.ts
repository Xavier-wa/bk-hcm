import { shallowRef, ref } from 'vue';
import { defineStore } from 'pinia';
import rollRequest from '@blueking/roll-request';
import http from '@/http';
import { QueryRuleOPEnum, type IListResData, type IPageQuery, type QueryFilterType } from '@/typings';
import { enableCount, onePageParams } from '@/utils/search';
import type { ListGeneratorFactory } from '@/components/form/list.vue';
import type {
  IPrepaidBillItem,
  IPrepaidSplitItem,
  IMainAccountItem,
  IRootAccountOption,
  IOperationProductItem,
} from '@/views/prepaid-bill/typings';
import { formatBillPeriod } from '@/views/prepaid-bill/utils';

const PREPAID_ITEMS_API = '/api/v1/account/bills/prepaid_items/list';
const MAIN_ACCOUNTS_API = '/api/v1/account/main_accounts/list';
const OPERATION_PRODUCTS_API = '/api/v1/account/operation_products/list';
const splitItemsApi = (id: string) => `/api/v1/account/bills/prepaid_items/${id}/split_items/list`;

const MAIN_ACCOUNT_BATCH = 100;
const OPERATION_PRODUCT_BATCH = 500;
const PREPAID_EXPORT_PAGE_LIMIT = 500;

const emptyFilter = (): QueryFilterType => ({ op: 'and', rules: [] });

const writeCache = <K, V>(cache: { value: Map<K, V> }, entries: Array<[K, V]>) => {
  if (!entries.length) return;
  const next = new Map(cache.value);
  entries.forEach(([key, value]) => next.set(key, value));
  cache.value = next;
};

export const usePrepaidBillStore = defineStore('prepaid-bill', () => {
  const listLoading = ref(false);
  const detailLoading = ref(false);
  const splitLoading = ref(false);
  const mainCache = shallowRef(new Map<string, IMainAccountItem>());
  const parentCache = shallowRef(new Map<string, IRootAccountOption>());
  const productCache = shallowRef(new Map<number, IOperationProductItem>());

  const rememberMains = (list: IMainAccountItem[]) => {
    writeCache(
      mainCache,
      list.filter((item) => item.id).map((item) => [item.id, item] as [string, IMainAccountItem]),
    );
    writeCache(
      parentCache,
      list
        .filter((item) => item.parent_account_id)
        .map(
          (item) =>
            [
              item.parent_account_id,
              { id: item.parent_account_id, name: item.parent_account_name || item.parent_account_id },
            ] as [string, IRootAccountOption],
        ),
    );
  };

  const rememberProducts = (list: IOperationProductItem[]) => {
    writeCache(
      productCache,
      list
        .filter((item) => item.op_product_id !== null && item.op_product_id !== undefined)
        .map((item) => [item.op_product_id, item] as [number, IOperationProductItem]),
    );
  };

  const listPrepaidItems = async (filter: QueryFilterType, page: IPageQuery) => {
    listLoading.value = true;
    try {
      const [listRes, countRes] = await Promise.all<
        [Promise<IListResData<IPrepaidBillItem[]>>, Promise<IListResData<IPrepaidBillItem[]>>]
      >([
        http.post(PREPAID_ITEMS_API, {
          filter,
          page: { count: false, start: page.start, limit: page.limit, sort: page.sort, order: page.order },
        }),
        http.post(PREPAID_ITEMS_API, { filter, page: { count: true, start: 0, limit: 0 } }),
      ]);
      return {
        list: listRes?.data?.details ?? [],
        count: countRes?.data?.count ?? 0,
      };
    } finally {
      listLoading.value = false;
    }
  };

  const normalizeSplitItems = (list: IPrepaidSplitItem[] = []) =>
    list.map((item) => ({
      ...item,
      bill_period: formatBillPeriod(item.bill_year, item.bill_month),
    }));

  const getPrepaidItem = async (id: string) => {
    const filter: QueryFilterType = {
      op: 'and',
      rules: [{ field: 'id', op: QueryRuleOPEnum.EQ, value: id }],
    };
    detailLoading.value = true;
    try {
      const res: IListResData<IPrepaidBillItem[]> = await http.post(
        PREPAID_ITEMS_API,
        enableCount({ filter, page: onePageParams() }, false),
      );
      return res?.data?.details?.[0] ?? null;
    } finally {
      detailLoading.value = false;
    }
  };

  const listSplitItems = async (id: string) => {
    splitLoading.value = true;
    try {
      const res: IListResData<IPrepaidSplitItem[]> = await http.post(splitItemsApi(id));
      return normalizeSplitItems(res?.data?.details ?? []);
    } finally {
      splitLoading.value = false;
    }
  };

  const listMainAccounts = async (filter: QueryFilterType, page: { start: number; limit: number }) => {
    const res: IListResData<IMainAccountItem[]> = await http.post(MAIN_ACCOUNTS_API, {
      filter,
      page: { count: false, start: page.start, limit: page.limit },
    });
    const list = res?.data?.details ?? [];
    rememberMains(list);
    return list;
  };

  const getMainAccountsByIds = async (ids: string[]) => {
    const uniqueIds = [...new Set(ids.filter(Boolean))];
    const missing = uniqueIds.filter((id) => !mainCache.value.has(id));
    while (missing.length) {
      const batch = missing.splice(0, MAIN_ACCOUNT_BATCH);
      await listMainAccounts(
        { op: 'and', rules: [{ field: 'id', op: QueryRuleOPEnum.IN, value: batch }] },
        { start: 0, limit: batch.length },
      );
    }
    return uniqueIds.map((id) => mainCache.value.get(id)).filter(Boolean);
  };

  const getRootAccountLabel = (id: string) => parentCache.value.get(id);

  const getRootAccountsByIds = async (ids: string[]) => {
    const uniqueIds = [...new Set(ids.filter(Boolean))];
    const missing = uniqueIds.filter((id) => !parentCache.value.has(id));
    if (missing.length) {
      await listMainAccounts(
        { op: 'and', rules: [{ field: 'parent_account_id', op: QueryRuleOPEnum.IN, value: missing }] },
        { start: 0, limit: MAIN_ACCOUNT_BATCH },
      );
    }
    return uniqueIds.map((id) => parentCache.value.get(id)).filter(Boolean);
  };

  const listOperationProducts = async (params: {
    op_product_ids?: number[];
    op_product_name?: string;
    page: { start: number; limit: number };
  }) => {
    const res: IListResData<IOperationProductItem[]> = await http.post(OPERATION_PRODUCTS_API, {
      op_product_ids: params.op_product_ids,
      op_product_name: params.op_product_name,
      page: { count: false, start: params.page.start, limit: params.page.limit },
    });
    const list = res?.data?.details ?? [];
    rememberProducts(list);
    return list;
  };

  const getOperationProductsByIds = async (ids: Array<string | number>) => {
    const uniqueIds = [...new Set(ids.map((id) => Number(id)).filter((id) => !Number.isNaN(id)))];
    const missing = uniqueIds.filter((id) => !productCache.value.has(id));
    while (missing.length) {
      const batch = missing.splice(0, OPERATION_PRODUCT_BATCH);
      await listOperationProducts({ op_product_ids: batch, page: { start: 0, limit: batch.length } });
    }
    return uniqueIds.map((id) => productCache.value.get(id)).filter(Boolean);
  };

  const listPrepaidItemsForExport = async (
    filter: QueryFilterType,
    page: { sort?: string; order?: string; total: number },
    signal: AbortSignal,
  ) => {
    const list = await rollRequest({
      httpClient: http,
      pageEnableCountKey: 'count',
    }).rollReqUseTotalCount<IPrepaidBillItem>(
      PREPAID_ITEMS_API,
      {
        filter,
        page: { sort: page.sort, order: page.order },
      },
      {
        limit: PREPAID_EXPORT_PAGE_LIMIT,
        total: page.total,
        listGetter: (res: IListResData<IPrepaidBillItem[]>) => res.data?.details ?? [],
        countGetter: (res: IListResData<IPrepaidBillItem[]>) => res.data?.count ?? 0,
      },
      { signal },
    );

    await Promise.all([
      getMainAccountsByIds(list.map((item) => item.main_account_id)),
      getRootAccountsByIds(list.map((item) => item.root_account_id)),
      getOperationProductsByIds(list.map((item) => item.product_id)),
    ]);

    return list;
  };

  const createMainAccountListGenerator = (): ListGeneratorFactory<IMainAccountItem> => {
    return async function* (keywordOrOptions) {
      const keyword = typeof keywordOrOptions === 'string' ? keywordOrOptions : undefined;
      const options = typeof keywordOrOptions === 'object' ? keywordOrOptions : undefined;
      let filter = emptyFilter();
      if (options?.ids?.length) {
        filter = { op: 'and', rules: [{ field: 'id', op: QueryRuleOPEnum.IN, value: options.ids.map(String) }] };
      } else if (keyword) {
        filter = { op: 'and', rules: [{ field: 'name', op: QueryRuleOPEnum.CS, value: keyword }] };
      }

      const gen = await rollRequest({ httpClient: http, pageEnableCountKey: 'count' }).rollReqUseCount<
        IListResData<IMainAccountItem[]>
      >(
        MAIN_ACCOUNTS_API,
        { filter },
        {
          limit: MAIN_ACCOUNT_BATCH,
          countGetter: (res) => res.data.count,
          listGetter: (res) => res.data.details,
          generator: true,
        },
        true,
      );

      for (const promise of gen) {
        const res = await promise;
        const list = res?.data?.details ?? [];
        rememberMains(list);
        yield list;
      }
    };
  };

  const createRootAccountListGenerator = (): ListGeneratorFactory<IRootAccountOption> => {
    return async function* (keywordOrOptions) {
      const keyword = typeof keywordOrOptions === 'string' ? keywordOrOptions : undefined;
      const options = typeof keywordOrOptions === 'object' ? keywordOrOptions : undefined;
      const seen = new Set<string>();

      if (options?.ids?.length) {
        const list = (await getRootAccountsByIds(options.ids.map(String))) as IRootAccountOption[];
        yield list;
        return;
      }

      const gen = createMainAccountListGenerator()(keyword);
      for await (const mains of gen) {
        const parents: IRootAccountOption[] = [];
        mains.forEach((item) => {
          if (!item.parent_account_id || seen.has(item.parent_account_id)) return;
          const name = item.parent_account_name || item.parent_account_id;
          if (keyword && !name.includes(keyword) && !item.parent_account_id.includes(keyword)) return;
          seen.add(item.parent_account_id);
          parents.push({ id: item.parent_account_id, name });
        });
        if (parents.length) yield parents;
      }
    };
  };

  const createOperationProductListGenerator = (): ListGeneratorFactory<IOperationProductItem> => {
    return async function* (keywordOrOptions) {
      const keyword = typeof keywordOrOptions === 'string' ? keywordOrOptions : undefined;
      const options = typeof keywordOrOptions === 'object' ? keywordOrOptions : undefined;
      const params: Record<string, any> = {};
      if (options?.ids?.length) {
        params.op_product_ids = options.ids.map((id) => Number(id));
      } else if (keyword) {
        params.op_product_name = keyword;
      }

      const gen = await rollRequest({ httpClient: http, pageEnableCountKey: 'count' }).rollReqUseCount<
        IListResData<IOperationProductItem[]>
      >(
        OPERATION_PRODUCTS_API,
        params,
        {
          limit: OPERATION_PRODUCT_BATCH,
          countGetter: (res) => res.data.count,
          listGetter: (res) => res.data.details,
          generator: true,
        },
        true,
      );

      for (const promise of gen) {
        const res = await promise;
        const list = res?.data?.details ?? [];
        rememberProducts(list);
        yield list;
      }
    };
  };

  return {
    listLoading,
    detailLoading,
    splitLoading,
    mainCache,
    parentCache,
    productCache,
    listPrepaidItems,
    listPrepaidItemsForExport,
    getPrepaidItem,
    listSplitItems,
    listMainAccounts,
    getMainAccountsByIds,
    listRootAccountsForSelect: createRootAccountListGenerator,
    getRootAccountLabel,
    getRootAccountsByIds,
    listOperationProducts,
    getOperationProductsByIds,
    createMainAccountListGenerator,
    createRootAccountListGenerator,
    createOperationProductListGenerator,
  };
});
