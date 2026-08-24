import { inject, provide, type InjectionKey } from "vue";
import type { useNavoApplication } from "./useNavoApplication";

export type NavoApplication = ReturnType<typeof useNavoApplication>;

const navoApplicationKey: InjectionKey<NavoApplication> = Symbol("navo-application");

export function provideNavoApplication(application: NavoApplication) {
  provide(navoApplicationKey, application);
}

export function useNavoApplicationContext() {
  const application = inject(navoApplicationKey);
  if (!application) throw new Error("Navo application context is unavailable");
  return application;
}
