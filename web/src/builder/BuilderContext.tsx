import {
  createContext,
  type ReactNode,
  useContext,
} from "react";
import type { RunEntry } from "./runData";

interface BuilderRuntime {
  currentRun: RunEntry | null;
  expandedGroups: Set<string>;
  runs: RunEntry[];
  onSelectRun: (fullId: string) => void;
  onToggleGroup: (nodeId: string) => void;
  onViewRuns: () => void;
}

const BuilderRuntimeContext = createContext<BuilderRuntime | null>(null);

export function BuilderRuntimeProvider({
  children,
  value,
}: {
  children: ReactNode;
  value: BuilderRuntime;
}) {
  return (
    <BuilderRuntimeContext.Provider value={value}>
      {children}
    </BuilderRuntimeContext.Provider>
  );
}

export function useBuilderRuntime() {
  const value = useContext(BuilderRuntimeContext);
  if (!value) {
    throw new Error("Builder runtime is unavailable");
  }
  return value;
}
