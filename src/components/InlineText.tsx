"use client";
import { useEffect, useState } from "react";

/** One line of text edited where it is shown: it keeps what is typed, and saves when the cursor leaves it or Enter is pressed. */
export default function InlineText(props: {
  label: string;
  value: string;
  onSave: (v: string) => void;
  placeholder?: string;
  arabic?: boolean;
  className?: string;
}) {
  const [v, setV] = useState(props.value);
  useEffect(() => setV(props.value), [props.value]);
  return (
    <input
      aria-label={props.label}
      value={v}
      placeholder={props.placeholder}
      onChange={(e) => setV(e.target.value)}
      onBlur={() => v !== props.value && props.onSave(v)}
      onKeyDown={(e) => e.key === "Enter" && !e.nativeEvent.isComposing && e.currentTarget.blur()}
      className={`edit ${props.arabic ? "ar" : ""} ${props.className ?? ""}`}
      {...(props.arabic ? { lang: "ar", dir: "rtl" } : {})}
    />
  );
}
