import { useEffect, useRef, useState, type ReactNode } from "react";
export function FloatingWindow({
  id,
  title,
  open,
  close,
  children,
  initial,
}: {
  id: string;
  title: string;
  open: boolean;
  close: () => void;
  children: ReactNode;
  initial: {
    x: number;
    y: number;
    width: number;
    height: number;
  };
}) {
  const key =
    id === "settings-panel" ? "traffic-settings-window" : `traffic-${id}`;
  const [rect, setRect] = useState(() => {
    try {
      const v = JSON.parse(localStorage.getItem(key));
      if (v && [v.x, v.y, v.width, v.height].every(Number.isFinite)) return v;
    } catch {}
    return initial;
  });
  const [max, setMax] = useState(false);
  const [size, setSize] = useState({ w: innerWidth, h: innerHeight });
  const gesture = useRef(null);
  const prefix = id === "settings-panel" ? "settings" : "rail-service";
  useEffect(() => {
    const resize = () => setSize({ w: innerWidth, h: innerHeight });
    const move = (e: PointerEvent) => {
      const g = gesture.current;
      if (!g || g.id !== e.pointerId) return;
      const dx = e.clientX - g.x,
        dy = e.clientY - g.y;
      setRect(
        g.kind === "move"
          ? { ...g.rect, x: g.rect.x + dx, y: g.rect.y + dy }
          : { ...g.rect, width: g.rect.width + dx, height: g.rect.height + dy },
      );
    };
    const end = () => {
      gesture.current = null;
    };
    const escape = (e: KeyboardEvent) => {
      if (e.key === "Escape" && open) close();
    };
    window.addEventListener("resize", resize);
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", end);
    window.addEventListener("pointercancel", end);
    window.addEventListener("keydown", escape);
    return () => {
      window.removeEventListener("resize", resize);
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", end);
      window.removeEventListener("pointercancel", end);
      window.removeEventListener("keydown", escape);
    };
  }, [open, close]);
  useEffect(() => {
    try {
      localStorage.setItem(key, JSON.stringify(rect));
    } catch {}
  }, [key, rect]);
  const aw = size.w - 16,
    ah = size.h - 16,
    width = max ? aw : Math.max(Math.min(340, aw), Math.min(rect.width, aw)),
    height = max ? ah : Math.max(Math.min(300, ah), Math.min(rect.height, ah));
  const left = max ? 8 : Math.max(8, Math.min(rect.x, size.w - width - 8)),
    top = max ? 8 : Math.max(8, Math.min(rect.y, size.h - height - 8));
  const start = (e, kind) => {
    if (e.button !== 0 || (kind === "move" && e.target.closest("button")))
      return;
    e.preventDefault();
    setMax(false);
    gesture.current = {
      id: e.pointerId,
      kind,
      x: e.clientX,
      y: e.clientY,
      rect: { x: left, y: top, width, height },
    };
    try {
      e.currentTarget.setPointerCapture(e.pointerId);
    } catch {}
  };
  const keyboard = (e, kind) => {
    if (
      e.target !== e.currentTarget ||
      !["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"].includes(e.key)
    )
      return;
    e.preventDefault();
    setMax(false);
    const d = e.shiftKey ? 32 : 8,
      x = e.key === "ArrowLeft" ? -d : e.key === "ArrowRight" ? d : 0,
      y = e.key === "ArrowUp" ? -d : e.key === "ArrowDown" ? d : 0;
    setRect((r) =>
      kind === "move"
        ? { ...r, x: r.x + x, y: r.y + y }
        : { ...r, width: r.width + x, height: r.height + y },
    );
  };
  return (
    <section
      id={id}
      hidden={!open}
      role="dialog"
      aria-label={title}
      className={id === "settings-panel" ? "glass top-overlay" : "glass"}
      style={{ left, top, width, height }}
    >
      <div
        id={`${prefix}-drag`}
        className="settings-titlebar"
        tabIndex={0}
        aria-label={`Move ${title}`}
        onPointerDown={(e) => start(e, "move")}
        onKeyDown={(e) => keyboard(e, "move")}
        onDoubleClick={(e) => {
          if (!(e.target as HTMLElement).closest("button")) setMax(!max);
        }}
      >
        <h2>{title}</h2>
        <div className="settings-window-actions">
          {id === "settings-panel" && (
            <button
              id="settings-maximize"
              className="quiet"
              aria-label={max ? "Restore settings size" : "Maximize settings"}
              onClick={() => setMax(!max)}
            >
              {max ? "❐" : "□"}
            </button>
          )}
          <button
            id={`${prefix}-close`}
            className="quiet"
            aria-label={`Close ${title}`}
            onClick={close}
          >
            ×
          </button>
        </div>
      </div>
      <div
        className={
          id === "settings-panel" ? "settings-body" : "rail-service-body"
        }
      >
        {children}
      </div>
      <button
        id={`${prefix}-resize`}
        className="quiet"
        aria-label={`Resize ${title}`}
        onPointerDown={(e) => start(e, "resize")}
        onKeyDown={(e) => keyboard(e, "resize")}
      >
        ◢
      </button>
    </section>
  );
}
