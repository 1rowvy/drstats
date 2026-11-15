import { useEffect, useState } from "react";
import { AppService } from "../bindings/github.com/1rowvy/drstats";

export default function App() {
  const [version, setVersion] = useState("");
  useEffect(() => {
    AppService.Version().then(setVersion);
  }, []);
  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="logo">drstats</div>
        <div className="version">{version}</div>
      </aside>
      <main className="content">
        <h1>Matches</h1>
      </main>
    </div>
  );
}
