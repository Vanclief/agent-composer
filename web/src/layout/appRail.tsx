import {
  BlocksIcon,
  CogIcon,
  StackIcon,
} from "../builder/Icons";
import type { RailItem } from "./LeftRail";

/**
 * The app's only navigation. Every view is a real route: Tasks at /,
 * Workflows at /workflows, the Library (builder) at /build.
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
    {
      key: "library",
      label: "Library",
      icon: <CogIcon />,
      to: "/build",
    },
  ];
}
