import {
  BlocksIcon,
  GaugeIcon,
  StackIcon,
  TerminalIcon,
} from "../builder/Icons";
import type { RailItem } from "./LeftRail";

/**
 * Sessions leads: a workspace per project, the place you sit in all
 * day. Workflows is the app's other place — monitoring and editing are
 * its modes (the Monitor | Edit switch in the top bar), and every run
 * lives in the monitor. Tasks is parked — visible but switched off —
 * until it gets a real purpose again.
 */
export function appRailItems(): RailItem[] {
  return [
    {
      key: "sessions",
      label: "Sessions",
      icon: <TerminalIcon />,
      to: "/sessions",
    },
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
