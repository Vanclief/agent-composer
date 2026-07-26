import { Dialog } from "radix-ui";
import type { FormEvent, ReactNode } from "react";

/**
 * App modal: builder-modal visuals over Radix Dialog behavior
 * (focus trap, ESC, scroll lock, outside-click, ARIA).
 * With `onSubmit` the content element is a form.
 */
export function Modal({
  title,
  onClose,
  onSubmit,
  footer,
  children,
}: {
  title: string;
  onClose: () => void;
  onSubmit?: (event: FormEvent) => void;
  footer?: ReactNode;
  children: ReactNode;
}) {
  const body = (
    <>
      <div className="builder-modal__head">
        <Dialog.Title asChild>
          <h3>{title}</h3>
        </Dialog.Title>
        <Dialog.Close asChild>
          <button
            type="button"
            className="builder-icon-button"
            aria-label="Close"
          >
            ×
          </button>
        </Dialog.Close>
      </div>
      <div className="builder-modal__body scrollnice">{children}</div>
      {footer && <div className="builder-modal__foot">{footer}</div>}
    </>
  );

  return (
    <Dialog.Root
      open
      onOpenChange={(open) => {
        if (!open) {
          onClose();
        }
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="builder-modal-overlay">
          <Dialog.Content
            aria-describedby={undefined}
            asChild={Boolean(onSubmit)}
            className={onSubmit ? undefined : "builder-modal"}
          >
            {onSubmit ? (
              <form className="builder-modal" onSubmit={onSubmit}>
                {body}
              </form>
            ) : (
              body
            )}
          </Dialog.Content>
        </Dialog.Overlay>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
