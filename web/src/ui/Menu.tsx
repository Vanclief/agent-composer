import { DropdownMenu } from "radix-ui";
import type { ReactNode } from "react";

/**
 * App dropdown: existing menu visuals over Radix DropdownMenu behavior
 * (keyboard nav, collision-aware positioning, outside-click, ARIA).
 * The trigger must be a single element that forwards its ref.
 */
export function Menu({
  trigger,
  align = "end",
  className = "builder-runmenu",
  children,
}: {
  trigger: ReactNode;
  align?: "start" | "end";
  className?: string;
  children: ReactNode;
}) {
  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>{trigger}</DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          className={className}
          align={align}
          sideOffset={5}
          loop
        >
          {children}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}

export function MenuItem({
  className,
  onSelect,
  children,
}: {
  className?: string;
  onSelect: () => void;
  children: ReactNode;
}) {
  return (
    <DropdownMenu.Item asChild onSelect={onSelect}>
      <button type="button" className={className}>
        {children}
      </button>
    </DropdownMenu.Item>
  );
}
