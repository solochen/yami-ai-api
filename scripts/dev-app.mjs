import http from "node:http";
import { createRequire } from "node:module";
import path from "node:path";

const [dir, socketPath] = process.argv.slice(2);
const require = createRequire(path.join(dir, "package.json"));
const next = require("next");
const app = next({ dev: true, dir, hostname: "localhost", port: Number(process.env.DEV_PORT || 3000) });

try {
  await app.prepare();
  const handler = app.getRequestHandler();
  // Each Next app uses a local socket, so neither needs its own TCP port.
  const server = http.createServer((req, res) => {
    Promise.resolve(handler(req, res)).catch(error => {
      console.error(error);
      if (!res.headersSent) res.writeHead(500);
      res.end("Local development render failed");
    });
  });
  server.on("error", error => { console.error(error); process.exit(1); });
  server.listen(socketPath, () => process.send?.({ type: "ready" }));
  const stop = async () => {
    const timer = setTimeout(() => process.exit(0), 4000);
    timer.unref();
    server.closeAllConnections();
    server.close();
    await app.close();
    process.exit(0);
  };
  process.once("SIGINT", stop);
  process.once("SIGTERM", stop);
} catch (error) {
  console.error(error);
  process.exit(1);
}
