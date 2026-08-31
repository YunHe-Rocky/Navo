export interface RouteAdmissionSnapshot {
  runtime: {
    mode: string;
    selected_id: string;
  };
  capture: {
    committed_mode: string;
  };
}

export type PrimaryConnectionAction =
  | { kind: "select_route"; label: "选择 Navo 线路" }
  | { kind: "start"; label: "启动 Navo 接管" }
  | { kind: "stop"; label: "停止 Navo 接管" };

export function requiresNavoRoute(snapshot: RouteAdmissionSnapshot): boolean {
  return snapshot.runtime.mode !== "direct" && !snapshot.runtime.selected_id;
}

export function derivePrimaryConnectionAction(snapshot: RouteAdmissionSnapshot): PrimaryConnectionAction {
  if (snapshot.capture.committed_mode !== "off") {
    return { kind: "stop", label: "停止 Navo 接管" };
  }
  if (requiresNavoRoute(snapshot)) {
    return { kind: "select_route", label: "选择 Navo 线路" };
  }
  return { kind: "start", label: "启动 Navo 接管" };
}
