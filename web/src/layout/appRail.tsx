import {
  BlocksIcon,
  StackIcon,
} from "../builder/Icons";
import type { RailItem } from "./LeftRail";

/**
 * The app's only navigation: Tasks at /, Workflows at /workflows.
 * Editing is a mode of Workflows (the Monitor | Edit switch in the
 * top bar), not a place of its own.
 */
export function appRailItems(): RailItem[] {
  return [
    {
      key: "tasks",
      label: "Tasks",
      icon: <StackIcon />,
      to: "/",
    },
    {
      key: "workflows",
      label: "Workflows",
      icon: <BlocksIcon />,
      to: "/workflows",
    },
  ];
}
