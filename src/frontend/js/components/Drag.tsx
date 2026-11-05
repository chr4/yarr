import { useSignal } from "@preact/signals";
import { useRef } from "preact/hooks";

interface DragProps {
  onResize: (width: number) => void;
  onReset: () => void;
}

export default function Drag({ onResize, onReset }: DragProps) {
  const active = useSignal(false);
  const startX = useRef(0);
  const initW = useRef(0);

  const onPointerDown = (e: PointerEvent) => {
    if (e.button !== 0) return;
    e.preventDefault();
    const el = e.currentTarget as HTMLElement;
    startX.current = e.clientX;
    // measure the rendered column rather than relying on the stored width
    initW.current = el.parentElement!.getBoundingClientRect().width;
    active.value = true;
    document.body.classList.add("is-resizing");
    el.setPointerCapture(e.pointerId);
  };

  const onPointerMove = (e: PointerEvent) => {
    if (!active.value) return;
    onResize(Math.round(initW.current + e.clientX - startX.current));
  };

  const onPointerUp = (e: PointerEvent) => {
    if (!active.value) return;
    active.value = false;
    document.body.classList.remove("is-resizing");
    (e.currentTarget as HTMLElement).releasePointerCapture(e.pointerId);
  };

  return (
    <div
      className={`drag ${active.value ? "is-active" : ""}`}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      onDblClick={onReset}
    />
  );
}
