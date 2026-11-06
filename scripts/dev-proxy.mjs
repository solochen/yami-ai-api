import http from "node:http";
import https from "node:https";

const within = (pathname, prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`);

export function selectTarget(url, targets) {
  const pathname = new URL(url, "http://localhost").pathname;
  if (pathname !== "/api/favicon" && ["/api", "/admin/api", "/v1", "/v1beta", "/uploads-local", "/health", "/metrics"].some(prefix => within(pathname, prefix))) {
    return { target: targets.api, path: url };
  }
  if (within(pathname, "/media")) {
    const mediaPath = url.replace(/^\/media(?=\/|\?|$)/, "");
    return { target: targets.media, path: mediaPath.startsWith("/") ? mediaPath : `/${mediaPath}` };
  }
  return { target: within(pathname, "/admin") || within(pathname, "/admin-assets") ? targets.admin : targets.web, path: url };
}

function requestOptions(req, selected) {
  const target = selected.target;
  const headers = {
    ...req.headers,
    "x-forwarded-host": req.headers.host,
    "x-forwarded-proto": "http",
    "x-forwarded-for": req.socket.remoteAddress,
  };
  if (typeof target === "string") {
    const upstream = new URL(target);
    return { transport: upstream.protocol === "https:" ? https : http, options: {
      hostname: upstream.hostname, port: upstream.port, method: req.method, path: selected.path, headers,
    } };
  }
  return { transport: http, options: { socketPath: target.socketPath, method: req.method, path: selected.path, headers } };
}

export function createDevGateway(targets, status) {
  const server = http.createServer((req, res) => {
    if (status && req.url === "/__dev/status") {
      res.writeHead(200, { "content-type": "application/json", "cache-control": "no-store" });
      res.end(JSON.stringify(status));
      return;
    }
    const selected = selectTarget(req.url, targets);
    const { transport, options } = requestOptions(req, selected);
    const upstream = transport.request(options, response => {
      res.writeHead(response.statusCode, response.headers);
      // Flush SSE headers and pipe bytes without buffering uploads or responses.
      res.flushHeaders();
      response.on("error", () => res.destroy());
      response.pipe(res);
    });
    upstream.on("error", error => {
      if (res.destroyed) return;
      if (res.headersSent) return res.destroy();
      res.writeHead(502, { "content-type": "text/plain; charset=utf-8" });
      res.end(`本地开发服务暂不可用：${error.code || "upstream error"}`);
    });
    req.on("aborted", () => upstream.destroy());
    res.on("close", () => { if (!res.writableEnded) upstream.destroy(); });
    req.pipe(upstream);
  });
  server.requestTimeout = 0;
  server.on("upgrade", (req, socket, head) => {
    const { transport, options } = requestOptions(req, selectTarget(req.url, targets));
    const upstream = transport.request(options);
    upstream.on("upgrade", (response, upstreamSocket, upstreamHead) => {
      const rawHeaders = response.rawHeaders;
      socket.write(`HTTP/1.1 ${response.statusCode} ${response.statusMessage}\r\n`);
      for (let i = 0; i < rawHeaders.length; i += 2) socket.write(`${rawHeaders[i]}: ${rawHeaders[i + 1]}\r\n`);
      socket.write("\r\n");
      if (upstreamHead.length) socket.write(upstreamHead);
      if (head.length) upstreamSocket.write(head);
      upstreamSocket.on("error", () => socket.destroy());
      socket.on("error", () => upstreamSocket.destroy());
      socket.on("close", () => upstreamSocket.destroy());
      upstreamSocket.on("close", () => socket.destroy());
      socket.pipe(upstreamSocket).pipe(socket);
    });
    upstream.on("response", response => { response.resume(); socket.end(`HTTP/1.1 ${response.statusCode}\r\nConnection: close\r\n\r\n`); });
    upstream.on("error", () => socket.destroy());
    socket.on("close", () => upstream.destroy());
    upstream.end();
  });
  return server;
}
