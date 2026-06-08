import { PropType, defineComponent, onMounted, ref, watch } from 'vue';
import { Select } from 'bkui-vue';
import { useZiyanScrStore } from '@/store';
import './index.scss';

interface IdcZone {
  id: number;
  cmdb_region_name: string;
  cmdb_zone_name: string;
  cmdb_zone_id: number;
}

export default defineComponent({
  name: 'ScrIdcZoneSelector',
  props: { cmdbRegionName: [Array, String] as PropType<string[] | string>, multiple: { type: Boolean, default: true } },
  setup(props) {
    const ziyanScrStore = useZiyanScrStore();

    const list = ref([]);
    const selected = ref(props.multiple ? [] : '');

    const queryIdcZoneList = async (cmdb_region_name: string[] | string) => {
      const res = await ziyanScrStore.queryIdcZoneList({
        cmdb_region_name: Array.isArray(cmdb_region_name) ? cmdb_region_name : [cmdb_region_name],
      });
      list.value = res.data.info || [];
    };

    onMounted(() => {
      queryIdcZoneList([]);
    });

    watch(
      () => props.cmdbRegionName,
      (val) => {
        queryIdcZoneList(val);
      },
      {
        deep: true,
      },
    );

    return () => (
      <Select
        v-model={selected.value}
        multiple={props.multiple}
        multipleMode={props.multiple ? 'tag' : null}
        collapseTags
        filterable>
        {list.value.map(({ id, cmdb_zone_name }: IdcZone) => (
          <Select.Option key={id} id={cmdb_zone_name} name={cmdb_zone_name} />
        ))}
      </Select>
    );
  },
});
