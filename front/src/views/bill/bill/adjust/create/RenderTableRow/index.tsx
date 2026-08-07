import { PropType, defineComponent, ref, watch } from 'vue';
import { reqBillsAdjustmentApiBrands, reqBillsAdjustmentGpuCards } from '@/api/bill';
import { InputColumn, OperationColumn, SelectColumn, TextPlainColumn } from '@blueking/ediatable';
import AdjustTypeSelector, { AdjustTypeEnum } from './components/AdjustTypeSelector';
import { BILL_ADJUSTMENT_SUPPORTED_VENDORS, ResClassEnum, ResClassList } from '@/constants';
import SubAccountSelector from '../../../components/search/sub-account-selector';
import { VendorEnum } from '@/common/constant';
import { useOperationProducts } from '@/hooks/useOperationProducts';
import useFormModel from '@/hooks/useFormModel';

export default defineComponent({
  props: {
    removeable: {
      required: true,
      type: Boolean,
      default: false,
    },
    vendor: {
      required: true,
      type: String as PropType<VendorEnum>,
    },
    rootAccountId: {
      required: true,
      type: String,
    },
    editData: {
      required: true,
      type: Object,
      default: {},
    },
    edit: {
      required: true,
      type: Boolean,
    },
  },
  emits: ['add', 'remove', 'copy', 'change'],
  setup(props, { emit, expose }) {
    const { formModel, resetForm, setFormValues } = useFormModel({
      type: AdjustTypeEnum.Increase,
      res_class: ResClassEnum.Cpu,
      res_sub_class: undefined as string | undefined,
      product_id: undefined as string | undefined,
      main_account_id: undefined as string | undefined,
      cost: '',
      memo: '',
    });

    const costRef = ref();
    const memoRef = ref();
    const productRef = ref();
    const mainAccountRef = ref();
    const subClassRef = ref();
    const subClassList = ref<{ label: string; value: string }[]>([]);
    const subClassLoading = ref(false);
    let subClassRequestId = 0;

    const { OperationProductsSelector, getAppendixList } = useOperationProducts(!props.edit);

    const canLoadSubClass = () =>
      BILL_ADJUSTMENT_SUPPORTED_VENDORS.includes(props.vendor) &&
      [ResClassEnum.GpuCard, ResClassEnum.GpuApi].includes(formModel.res_class);

    const loadSubClassList = async () => {
      subClassRequestId += 1;
      const requestId = subClassRequestId;
      if (!canLoadSubClass()) {
        subClassList.value = [];
        subClassLoading.value = false;
        return;
      }

      subClassLoading.value = true;
      try {
        const res =
          formModel.res_class === ResClassEnum.GpuCard
            ? await reqBillsAdjustmentGpuCards(props.vendor)
            : await reqBillsAdjustmentApiBrands(props.vendor);
        if (requestId === subClassRequestId) {
          const list: string[] = res?.data?.details ?? [];
          subClassList.value = list.map((value) => ({ label: value, value }));
        }
      } catch {
        if (requestId === subClassRequestId) subClassList.value = [];
      } finally {
        if (requestId === subClassRequestId) subClassLoading.value = false;
      }
    };

    const normalizeResClass = (value: unknown): ResClassEnum =>
      ResClassList.some((item) => item.value === value) ? (value as ResClassEnum) : ResClassEnum.Cpu;

    const normalizeSelectValue = (value: unknown) =>
      value === '' || value === null || value === undefined ? undefined : value;

    const handleResClassChange = (value: unknown) => {
      formModel.res_class = normalizeResClass(value);
      formModel.res_sub_class = undefined;
      loadSubClassList();
    };

    const handleVendorChange = () => {
      formModel.res_sub_class = undefined;
      loadSubClassList();
    };

    const handleAdd = () => {
      emit('add');
    };

    const handleRemove = () => {
      emit('remove');
    };

    const handleCopy = () => {
      emit('copy', formModel);
    };

    watch(
      () => props.editData,
      (data) => {
        if (data.product_id) getAppendixList(data.product_id);
        setFormValues({
          ...data,
          res_sub_class: normalizeSelectValue(data.res_sub_class) as string | undefined,
          product_id: normalizeSelectValue(data.product_id) as string | undefined,
          main_account_id: normalizeSelectValue(data.main_account_id) as string | undefined,
        });
        loadSubClassList();
      },
      {
        deep: true,
        immediate: true,
      },
    );

    watch(
      () => props.vendor,
      (vendor, oldVendor) => {
        if (vendor !== oldVendor) handleVendorChange();
      },
    );

    watch(
      () => formModel,
      (val) => {
        emit('change', val);
      },
      {
        deep: true,
      },
    );

    watch(
      () => props.rootAccountId,
      () => {
        formModel.main_account_id = undefined;
      },
    );

    const resetRow = () => {
      subClassRequestId += 1;
      subClassList.value = [];
      subClassLoading.value = false;
      resetForm();
    };

    expose({
      getValue: async () => {
        return await Promise.all([
          costRef.value!.getValue(),
          memoRef.value!.getValue(),
          productRef.value!.getValue(),
          mainAccountRef.value!.getValue(),
          ...([ResClassEnum.GpuCard, ResClassEnum.GpuApi].includes(formModel.res_class)
            ? [subClassRef.value!.getValue()]
            : []),
        ]).then(() => {
          const resSubClass = [ResClassEnum.Cpu, ResClassEnum.GpuOther].includes(formModel.res_class)
            ? ''
            : formModel.res_sub_class || '';
          return { ...formModel, res_sub_class: resSubClass };
        });
      },
      reset: resetRow,
      getRowValue: () => {
        return formModel;
      },
    });

    return () => (
      <>
        <tr>
          <td>
            <AdjustTypeSelector v-model={formModel.type} />
          </td>
          <td>
            <OperationProductsSelector v-model={formModel.product_id} ref={productRef} isEdiatable />
          </td>
          <td>
            <SubAccountSelector
              isEditable={true}
              v-model={formModel.main_account_id}
              ref={mainAccountRef}
              vendor={[props.vendor]}
              productId={formModel.product_id ? [formModel.product_id] : []}
              rootAccountId={props.rootAccountId ? [props.rootAccountId] : []}
            />
          </td>
          <td>
            <TextPlainColumn>人工调账</TextPlainColumn>
          </td>
          <td>
            <SelectColumn
              list={ResClassList}
              v-model={formModel.res_class}
              onUpdate:modelValue={handleResClassChange}
            />
          </td>
          <td>
            {[ResClassEnum.GpuCard, ResClassEnum.GpuApi].includes(formModel.res_class) ? (
              <SelectColumn
                key={`subclass-${props.vendor}-${formModel.res_class}`}
                ref={subClassRef}
                v-model={formModel.res_sub_class}
                list={subClassList.value}
                clearable
                placeholder='请选择'
                rules={[{ validator: (value: string) => Boolean(value), message: '资源子类不能为空' }]}
                {...({ loading: subClassLoading.value, filterable: true } as Record<string, unknown>)}
              />
            ) : (
              <TextPlainColumn>--</TextPlainColumn>
            )}
          </td>
          <td>
            <InputColumn
              type='number'
              min={0}
              precision={3}
              ref={costRef}
              v-model_number={formModel.cost}
              rules={[
                {
                  validator: (value: string) => Boolean(value),
                  message: '金额不能为空',
                },
              ]}
            />
          </td>
          <td>
            <InputColumn ref={memoRef} v-model={formModel.memo} />
          </td>
          {!props.edit && (
            <OperationColumn
              removeable={props.removeable}
              onAdd={handleAdd}
              onRemove={handleRemove}
              showCopy
              onCopy={handleCopy}
            />
          )}
        </tr>
      </>
    );
  },
});
