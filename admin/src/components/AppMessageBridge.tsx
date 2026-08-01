import { App, message as staticMessage } from 'antd';
import type { MessageInstance } from 'antd/es/message/interface';
import { useLayoutEffect } from 'react';

function patchStaticMessage(instance: MessageInstance): () => void {
  const orig = {
    info: staticMessage.info,
    success: staticMessage.success,
    error: staticMessage.error,
    warning: staticMessage.warning,
    loading: staticMessage.loading,
    open: staticMessage.open,
    destroy: staticMessage.destroy,
  };
  staticMessage.info = (...args) => instance.info(...args);
  staticMessage.success = (...args) => instance.success(...args);
  staticMessage.error = (...args) => instance.error(...args);
  staticMessage.warning = (...args) => instance.warning(...args);
  staticMessage.loading = (...args) => instance.loading(...args);
  staticMessage.open = (...args) => instance.open(...args);
  staticMessage.destroy = (...args) => instance.destroy(...args);
  return () => {
    staticMessage.info = orig.info;
    staticMessage.success = orig.success;
    staticMessage.error = orig.error;
    staticMessage.warning = orig.warning;
    staticMessage.loading = orig.loading;
    staticMessage.open = orig.open;
    staticMessage.destroy = orig.destroy;
  };
}

/** Patches antd static `message.*` to use App context (theme + cssVar). Mount inside `<App>`. */
export default function AppMessageBridge() {
  const { message } = App.useApp();
  useLayoutEffect(() => patchStaticMessage(message), [message]);
  return null;
}
