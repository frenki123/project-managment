(() => {
  // Reconnect after Air replaces the development server.
  // The initial connection does not reload the page.
  let connected = false;

  const connect = () => {
    const source = new EventSource("/__dev/reload");

    source.onopen = () => {
      if (connected) {
        window.location.reload();
        return;
      }
      connected = true;
    };

    source.onerror = () => {
      source.close();
      window.setTimeout(connect, 250);
    };
  };

  connect();
})();
