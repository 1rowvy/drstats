import { useEffect, useState } from "react";
import { Window } from "@wailsio/runtime";

/** Minimise / maximise / close buttons of the custom (frameless) title bar.
 *  In the HTTP server build there is no native window: calls just fail quietly. */
export function WindowControls() {
  const [max, setMax] = useState(false);
  useEffect(() => {
    const update = () => Window.IsMaximised().then(setMax).catch(() => {});
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, []);
  const call = (f: () => Promise<void>) => () => { f().catch(() => {}); };
  return (
    <div className="win-ctl">
      <button title="Свернуть" onClick={call(() => Window.Minimise())}>
        <svg width="10" height="10" viewBox="0 0 10 10"><path d="M0 5.5h10" stroke="currentColor" /></svg>
      </button>
      <button title={max ? "Восстановить" : "Развернуть"} onClick={call(() => Window.ToggleMaximise())}>
        {max ? (
          <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor">
            <rect x="0.5" y="2.5" width="7" height="7" /><path d="M2.5 2.5v-2h7v7h-2" />
          </svg>
        ) : (
          <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor"><rect x="0.5" y="0.5" width="9" height="9" /></svg>
        )}
      </button>
      <button className="close" title="Закрыть" onClick={call(() => Window.Close())}>
        <svg width="10" height="10" viewBox="0 0 10 10" stroke="currentColor"><path d="M0.5 0.5l9 9M9.5 0.5l-9 9" /></svg>
      </button>
    </div>
  );
}
