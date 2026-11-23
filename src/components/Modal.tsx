"use client";
import { useEffect, useRef, type ReactNode } from "react";

export default function Modal(props: {
  open: boolean;
  onClose: () => void;
  onOpen?: () => void;
  label?: string;
  labelledBy?: string;
  className: string;
  children: ReactNode;
}) {
  const dialog = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const el = dialog.current;
    if (!el) return;
    if (props.open && !el.open) {
      el.showModal();
      props.onOpen?.();
    }
    if (!props.open && el.open) el.close();
  }, [props.open]);

  return (
    <dialog
      ref={dialog}
      aria-label={props.label}
      aria-labelledby={props.labelledBy}
      onClose={props.onClose}
      onClick={(e) => e.target === dialog.current && props.onClose()}
      className={`${props.className} w-[calc(100%-2.5rem)] rounded-modal border border-rule bg-surface text-ink shadow-modal backdrop:bg-black/70`}
    >
      {props.children}
    </dialog>
  );
}
