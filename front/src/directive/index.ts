import { bkTooltips } from 'bkui-vue';
import safeHtml from './safeHtml';
// import overflowTitle from './overflowTitle';
import clipboard from './clipboard';
const directives: Record<string, any> = {
  // 指令对象
  bkTooltips,
  safeHtml,
  // overflowTitle,
  clipboard,
};

export default {
  install(app: any) {
    Object.keys(directives).forEach((key) => {
      app.directive(key, directives[key]);
    });
  },
};
