import {
  BlocksIcon,
  GaugeIcon,
  StackIcon,
} from "../builder/Icons";
import type { RailItem } from "./LeftRail";

/**
 * Workflows is the app's place: monitoring and editing are its modes
 * (the Monitor | Edit switch in the top bar), and every run lives in
 * the monitor. Tasks is parked — visible but switched off — until it
 * gets a real purpose again.
 */
export function appRailItems(): RailItem[] {
  return [
    {
      key: "tasks",
      label: "Tasks",
      icon: <StackIcon />,
      disabled: true,
    },
    {
      key: "workflows",
      label: "Workflows",
      icon: <BlocksIcon />,
      to: "/workflows",
    },
    {
      key: "benchmark",
      label: "Benchmark",
      icon: <GaugeIcon />,
      disabled: true,
    },
  ];
}
