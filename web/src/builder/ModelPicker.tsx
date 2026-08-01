import { useEffect, useRef, useState } from "react";

/**
 * Model input: free text plus a dropdown of the harness's known
 * models. A native datalist filters by the typed value, which hides
 * every option the moment a model from another harness is in the box —
 * this menu always shows the full list.
 */
export function ModelPicker({
  id,
  value,
  models,
  disabled,
  onChange,
}: {
  id?: string;
  value: string;
  models: string[];
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) {
      return;
    }
    const close = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    window.addEventListener("mousedown", close);
    return () => window.removeEventListener("mousedown", close);
  }, [open]);

  return (
    <div className="model-picker" ref={rootRef}>
      <input
        id={id}
        className="builder-input mono"
        value={value}
        disabled={disabled}
        placeholder="model id"
        onChange={(event) => onChange(event.target.value)}
      />
      {models.length > 0 && (
        <button
          type="button"
          className="model-picker__toggle"
          aria-label="Show known models"
          disabled={disabled}
          onClick={() => setOpen((current) => !current)}
        >
          ▾
        </button>
      )}
      {open && (
        <ul className="model-picker__menu scrollnice">
          {models.map((name) => (
            <li key={name}>
              <button
                type="button"
                className={name === value ? "active" : ""}
                onClick={() => {
                  onChange(name);
                  setOpen(false);
                }}
              >
                {name}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
