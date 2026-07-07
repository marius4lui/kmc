import net from "node:net";

export const DEFAULT_PORT_RANGE = { start: 43100, end: 43999 };

export function isPortFree(port, host = "127.0.0.1") {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.once("error", () => resolve(false));
    server.once("listening", () => {
      server.close(() => resolve(true));
    });
    server.listen(port, host);
  });
}

export async function findFreePort(range = DEFAULT_PORT_RANGE) {
  for (let port = range.start; port <= range.end; port += 1) {
    if (await isPortFree(port)) return port;
  }
  throw new Error(`No free port found in ${range.start}-${range.end}.`);
}
