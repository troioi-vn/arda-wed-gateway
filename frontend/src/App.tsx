function App() {
  return (
    <main className="layout">
      <section className="terminal-pane">
        <header>Arda Terminal (Phase 0 scaffold)</header>
        <div className="terminal-body">
          WebSocket stream will be rendered here in Milestone 1.
        </div>
      </section>
      <aside className="map-pane">
        <header>Map / Context</header>
        <div className="panel-body">Placeholder panel</div>
      </aside>
      <section className="suggestion-pane">
        <header>Suggestions</header>
        <div className="panel-body">Action buttons will appear here.</div>
      </section>
    </main>
  );
}

export default App;
