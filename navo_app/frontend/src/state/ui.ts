import type { Page } from "../types";

export type ThemeMode = "day" | "night";
export type DialogName = "none" | "tun_fault" | "close";

export interface UIState {
  page: Page;
  theme: ThemeMode;
  dialog: DialogName;
  loading: boolean;
  activityLabel: string;
  notice: string;
  failure: string;
}

export function createInitialUIState(): UIState {
  return {
    page: "overview",
    theme: "night",
    dialog: "none",
    loading: false,
    activityLabel: "",
    notice: "",
    failure: "",
  };
}

export function selectPage(state: UIState, page: Page): UIState {
  return { ...state, page };
}

export function openDialog(state: UIState, dialog: Exclude<DialogName, "none">): UIState {
  return { ...state, dialog };
}

export function closeDialog(state: UIState): UIState {
  return { ...state, dialog: "none" };
}

export function beginOperation(state: UIState, activityLabel: string): UIState {
  return { ...state, loading: true, activityLabel, notice: "", failure: "" };
}

export function finishOperation(state: UIState, notice = ""): UIState {
  return { ...state, loading: false, activityLabel: "", notice, failure: "" };
}

export function failOperation(state: UIState, failure: string): UIState {
  return { ...state, loading: false, activityLabel: "", notice: "", failure };
}
