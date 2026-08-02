import { App, Modal as staticModal, message as staticMessage } from 'antd';
import type { MessageInstance } from 'antd/es/message/interface';
import type { HookAPI as ModalHookAPI } from 'antd/es/modal/useModal';
import { useLayoutEffect } from 'react';

function patchStaticModal(instance: ModalHookAPI): () => void {
  const orig = {
    info: staticModal.info,
    success: staticModal.success,
    error: staticModal.error,
    warning: staticModal.warning,
    confirm: staticModal.confirm,
  };
  staticModal.info = (...args) => instance.info(...args);
  staticModal.success = (...args) => instance.success(...args);
  staticModal.error = (...args) => instance.error(...args);
  staticModal.warning = (...args) => instance.warning(...args);
  staticModal.confirm = (...args) => instance.confirm(...args);
  return () => {
    staticModal.info = orig.info;
    staticModal.success = orig.success;
    staticModal.error = orig.error;
    staticModal.warning = orig.warning;
    staticModal.confirm = orig.confirm;
  };
}

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

/** Patches antd static `message.*` / `Modal.*` to use App context (theme + cssVar). Mount inside `<App>`. */
export default function AppMessageBridge() {
  const { message, modal } = App.useApp();
  useLayoutEffect(() => patchStaticMessage(message), [message]);
  useLayoutEffect(() => patchStaticModal(modal), [modal]);
  return null;
}
