import assert from "node:assert/strict";
import { once } from "node:events";
import http from "node:http";
import net from "node:net";
import test from "node:test";
import { createDevGateway, selectTarget } from "./dev-proxy.mjs";

const targets = { web: { socketPath: "web" }, admin: { socketPath: "admin" }, api: "http://api:8080", media: "http://minio:9000" };

test("routes pages, API, media and both HMR paths to the correct service", () => {
  for (const url of ["/", "/app/agents/renovation_budget_agent", "/_next/webpack-hmr", "/api/favicon"]) {
    assert.equal(selectTarget(url, targets).target, targets.web);
  }
  for (const url of ["/admin", "/admin/login", "/admin-assets/_next/webpack-hmr"]) {
    assert.equal(selectTarget(url, targets).target, targets.admin);
  }
  for (const url of ["/api/upload", "/admin/api/login", "/v1/chat/completions", "/v1beta/models/test", "/uploads-local/image.png", "/health"]) {
    assert.equal(selectTarget(url, targets).target, targets.api);
  }
  assert.deepEqual(selectTarget("/media/bucket/a%20b.mp4?x=1", targets), { target: targets.media, path: "/bucket/a%20b.mp4?x=1" });
  assert.deepEqual(selectTarget("/media?x=1", targets), { target: targets.media, path: "/?x=1" });
  assert.equal(selectTarget("/administrator", targets).target, targets.web);
  assert.equal(selectTarget("/media-library", targets).target, targets.web);
});

async function listen(t, server) {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(() => { server.closeAllConnections(); server.close(); });
  return `http://127.0.0.1:${server.address().port}`;
}

test("preserves uploads, cookies, query strings and authentication responses", async t => {
  const body = Buffer.from([0, 255, 13, 10, 128, 42]);
  const upstream = http.createServer(async (req, res) => {
    const chunks = [];
    for await (const chunk of req) chunks.push(chunk);
    assert.deepEqual(Buffer.concat(chunks), body);
    assert.equal(req.url, "/admin/api/upload?mode=test");
    assert.equal(req.headers.cookie, "starai_admin_session=test");
    res.writeHead(201, { "set-cookie": "starai_admin_session=updated; Path=/admin; HttpOnly" });
    res.end("uploaded");
  });
  const address = await listen(t, upstream);
  const gateway = await listen(t, createDevGateway({ ...targets, api: address }));
  const response = await fetch(`${gateway}/admin/api/upload?mode=test`, { method: "POST", headers: { cookie: "starai_admin_session=test" }, body });
  assert.equal(response.status, 201);
  assert.match(response.headers.get("set-cookie"), /Path=\/admin; HttpOnly/);
  assert.equal(await response.text(), "uploaded");
});

test("passes range requests and HEAD responses through the media path", async t => {
  const upstream = http.createServer((req, res) => {
    assert.equal(req.url, "/bucket/video.mp4");
    assert.equal(req.headers.range, "bytes=2-4");
    res.writeHead(206, { "content-range": "bytes 2-4/10", "content-length": "3" });
    res.end(req.method === "HEAD" ? undefined : "234");
  });
  const address = await listen(t, upstream);
  const gateway = await listen(t, createDevGateway({ ...targets, media: address }));
  for (const method of ["GET", "HEAD"]) {
    const response = await fetch(`${gateway}/media/bucket/video.mp4`, { method, headers: { range: "bytes=2-4" } });
    assert.equal(response.status, 206);
    assert.equal(response.headers.get("content-range"), "bytes 2-4/10");
    assert.equal(await response.text(), method === "HEAD" ? "" : "234");
  }
});

test("delivers SSE before the upstream response finishes", async t => {
  let finish;
  const released = new Promise(resolve => { finish = resolve; });
  const upstream = http.createServer(async (_req, res) => {
    res.writeHead(200, { "content-type": "text/event-stream" });
    res.write("data: first\n\n");
    await released;
    res.end("data: [DONE]\n\n");
  });
  const address = await listen(t, upstream);
  const gateway = await listen(t, createDevGateway({ ...targets, api: address }));
  try {
    const response = await fetch(`${gateway}/api/chat`, { signal: AbortSignal.timeout(3000) });
    const reader = response.body.getReader();
    assert.equal(new TextDecoder().decode((await reader.read()).value), "data: first\n\n");
    finish();
    assert.equal(new TextDecoder().decode((await reader.read()).value), "data: [DONE]\n\n");
  } finally { finish(); }
});

test("forwards WebSocket upgrades for frontend and admin hot reload", async t => {
  const sockets = new Set();
  t.after(() => { for (const socket of sockets) socket.destroy(); });
  const upstream = http.createServer();
  upstream.on("upgrade", (req, socket) => {
    sockets.add(socket);
    socket.write("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n");
    socket.write(req.url);
  });
  const address = await listen(t, upstream);
  const gateway = await listen(t, createDevGateway({ ...targets, web: address, admin: address }));
  for (const url of ["/_next/webpack-hmr", "/admin-assets/_next/webpack-hmr"]) {
    const socket = net.connect(new URL(gateway).port, "127.0.0.1");
    sockets.add(socket);
    await once(socket, "connect");
    socket.write(`GET ${url} HTTP/1.1\r\nHost: localhost\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n`);
    let received = "";
    await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error("HMR upgrade timed out")), 3000);
      socket.on("data", chunk => {
        received += chunk.toString();
        if (received.includes(url)) { clearTimeout(timer); resolve(); }
      });
      socket.on("error", reject);
    });
    assert.match(received, /101 Switching Protocols/);
    socket.destroy();
  }
});
