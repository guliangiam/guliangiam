import { message as staticMessage } from "antd";
import type { MessageInstance } from "antd/es/message/interface";

// Plain-function call sites (e.g. handleMutationError) cannot use hooks, so
// AppShell injects the context-bound App.useApp() instance here at mount.
// The static fallback keeps unit tests and pre-mount calls working.
let instance: MessageInstance = staticMessage;

export function setAppMessage(next: MessageInstance): void {
  instance = next;
}

export function appMessage(): MessageInstance {
  return instance;
}
