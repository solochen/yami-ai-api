"use client";

import { useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { VideoMediaItem } from "@starai/shared-types";
import { useI18n } from "@/i18n/I18nProvider";

export function VideoPromptField({
  value,
  onChange,
  onSubmit,
  placeholder,
  rows = 5,
  className,
  fill = false,
  images,
  videos,
  audios,
}: {
  value: string;
  onChange: (value: string) => void;
  onSubmit: () => void;
  placeholder?: string;
  rows?: number;
  className?: string;
  fill?: boolean;
  images: VideoMediaItem[];
  videos: VideoMediaItem[];
  audios: VideoMediaItem[];
}) {
  const { t } = useI18n();
  const fieldRef = useRef<HTMLTextAreaElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const [cursor, setCursor] = useState(0);
  const [active, setActive] = useState(0);
  const [menuBox, setMenuBox] = useState<{ left: number; bottom: number; maxHeight: number } | null>(null);
  const mentions = useMemo(() => {
    const items: string[] = [];
    images.forEach((_, index) => items.push(`@${t("canvas.kind.image")}${index + 1}`));
    videos.forEach((_, index) => items.push(`@${t("canvas.kind.video")}${index + 1}`));
    audios.forEach((_, index) => items.push(`@${t("canvas.kind.audio")}${index + 1}`));
    return items;
  }, [audios, images, t, videos]);
  const before = value.slice(0, cursor);
  const match = before.match(/@([^\s@]*)$/);
  const query = match ? match[1] : null;
  const shown = query === null
    ? []
    : mentions.filter((label) => label.slice(1).toLowerCase().includes(query.toLowerCase()));
  const current = shown[Math.min(active, Math.max(shown.length - 1, 0))];

  useLayoutEffect(() => {
    if (shown.length === 0) return;
    const place = () => {
      const field = fieldRef.current;
      if (!field) return;
      const rect = field.getBoundingClientRect();
      const available = Math.max(120, rect.top - 12);
      setMenuBox({
        left: Math.max(8, rect.left + 12),
        bottom: Math.max(8, window.innerHeight - rect.top + 6),
        maxHeight: Math.min(360, available),
      });
    };
    place();
    window.addEventListener("resize", place);
    window.addEventListener("scroll", place, true);
    return () => {
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", place, true);
    };
  }, [shown.length, value, cursor]);

  useLayoutEffect(() => {
    const menu = menuRef.current;
    const selected = menu?.querySelector<HTMLButtonElement>("[data-active='true']");
    if (!menu || !selected) return;
    const top = selected.offsetTop;
    const bottom = top + selected.offsetHeight;
    if (top < menu.scrollTop) menu.scrollTop = top;
    else if (bottom > menu.scrollTop + menu.clientHeight) menu.scrollTop = bottom - menu.clientHeight;
  }, [current]);

  const insert = (label: string) => {
    const nextBefore = before.replace(/@([^\s@]*)$/, `${label} `);
    const next = nextBefore + value.slice(cursor);
    onChange(next);
    const position = nextBefore.length;
    setCursor(position);
    setActive(0);
    requestAnimationFrame(() => {
      fieldRef.current?.focus();
      fieldRef.current?.setSelectionRange(position, position);
    });
  };

  return (
    <div className={`relative min-w-0 ${fill ? "flex-1" : "w-full"}`}>
      {shown.length > 0 && menuBox && createPortal(
        <div
          ref={menuRef}
          className="fixed z-[80] w-44 overflow-y-auto rounded-xl border border-gray-200 bg-white py-1 shadow-lg dark:border-white/10 dark:bg-gray-900"
          style={{ left: menuBox.left, bottom: menuBox.bottom, maxHeight: menuBox.maxHeight }}
        >
          {shown.map((label) => (
            <button
              key={label}
              type="button"
              data-active={label === current}
              className={`block w-full px-3 py-1.5 text-left text-xs ${label === current ? "bg-cyan-50 text-cyan-800 dark:bg-cyan-400/10 dark:text-cyan-100" : "text-gray-700 dark:text-gray-200"}`}
              onMouseDown={(event) => {
                event.preventDefault();
                insert(label);
              }}
            >
              {label}
            </button>
          ))}
        </div>,
        document.body,
      )}
      <textarea
        ref={fieldRef}
        value={value}
        rows={rows}
        placeholder={placeholder}
        className={className}
        onChange={(event) => {
          onChange(event.target.value);
          setCursor(event.target.selectionStart ?? event.target.value.length);
          setActive(0);
        }}
        onClick={(event) => setCursor(event.currentTarget.selectionStart ?? 0)}
        onKeyUp={(event) => {
          if (!["ArrowDown", "ArrowUp", "Enter", "Tab"].includes(event.key)) {
            setCursor(event.currentTarget.selectionStart ?? 0);
          }
        }}
        onKeyDown={(event) => {
          if (shown.length > 0 && (event.key === "ArrowDown" || event.key === "ArrowUp")) {
            event.preventDefault();
            setActive((index) => (event.key === "ArrowDown" ? index + 1 : index - 1 + shown.length) % shown.length);
            return;
          }
          if (shown.length > 0 && current && (event.key === "Enter" || event.key === "Tab")) {
            event.preventDefault();
            insert(current);
            return;
          }
          if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing && event.keyCode !== 229) {
            event.preventDefault();
            onSubmit();
          }
        }}
      />
    </div>
  );
}
