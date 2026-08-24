import type { CaptureMode } from "../types";
import type { BackendProvider } from "./client";

export function createCaptureApi(backend: BackendProvider) {
  return {
    switchMode: (mode: CaptureMode) => backend().SetCaptureMode(mode),
    verify: () => backend().VerifyCapture(),
  };
}
