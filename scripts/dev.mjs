import { fork } from "node:child_process";
import { once } from "node:events";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { randomUUID } from "node:crypto";
import { fileURLToPath } from "node:url";
import { setTimeout as delay } from "node:timers/promises";
import { prepareDocker } from "./dev-docker.mjs";
import { createDevGateway } from "./dev-proxy.mjs";

const root = fileURLToPath(new URL("../", import.meta.url));
const stateFile = path.join(root, ".codex-tmp/dev/state.json");
const host = process.env.DEV_HOST || "127.0.0.1";
const port = Number(process.env.DEV_PORT || 3000);
const children = [];
let gateway, socketDir, stopping = false;

async function readState() {
  try { return JSON.parse(await readFile(stateFile, "utf8")); } catch { return null; }
}

async function isRunning(state) {
  if (!state) return false;
  try {
    const response = await fetch(`http://127.0.0.1:${state.port}/__dev/status`, { signal: AbortSignal.timeout(1500) });
    const live = await response.json();
    return live.pid === state.pid && live.id === state.id;
  } catch { return false; }
}

async function stop() {
  if (stopping) return;
  stopping = true;
  gateway?.close();
  gateway?.closeAllConnections();
  await Promise.all(children.map(child => new Promise(resolve => {
    if (child.exitCode !== null || child.signalCode !== null) return resolve();
    child.once("exit", resolve);
    child.kill("SIGTERM");
    const timer = setTimeout(() => child.kill("SIGKILL"), 5000);
    timer.unref();
  })));
  if (socketDir) await rm(socketDir, { recursive: true, force: true });
  const state = await readState();
  if (state?.pid === process.pid) await rm(stateFile, { force: true });
}

async function start() {
  const previous = await readState();
  if (await isRunning(previous)) {
    console.log(`开发服务已运行：http://localhost:${previous.port}`);
    return;
  }
  if (!Number.isInteger(port) || port < 1 || port > 65535) throw new Error("DEV_PORT 必须是有效端口");
  prepareDocker();
  // Check before starting the compilers; never terminate an unrelated listener.
  const probe = net.createServer();
  await new Promise((resolve, reject) => { probe.once("error", reject); probe.listen(port, host, resolve); });
  await new Promise(resolve => probe.close(resolve));
  socketDir = await mkdtemp(path.join(process.platform === "darwin" ? "/tmp" : os.tmpdir(), "yihang-dev-"));
  const targets = { api: process.env.DEV_API_TARGET || "http://127.0.0.1:8080", media: process.env.DEV_MEDIA_TARGET || "http://127.0.0.1:9000" };
  for (const name of ["web", "admin"]) {
    const socketPath = process.platform === "win32" ? `\\\\.\\pipe\\yihang-${process.pid}-${name}` : path.join(socketDir, `${name}.sock`);
    targets[name] = { socketPath };
    const child = fork(fileURLToPath(new URL("./dev-app.mjs", import.meta.url)), [path.join(root, "apps", name), socketPath], {
      cwd: path.join(root, "apps", name),
      env: { ...process.env, NODE_ENV: "development", DEV_PORT: String(port), NEXT_PUBLIC_API_URL: "", INTERNAL_API_URL: targets.api, MEDIA_PROXY_TARGET: targets.media, ADMIN_ASSET_PREFIX: name === "admin" ? "/admin-assets" : "", DEV_DIST_DIR: ".next-dev" },
      stdio: ["ignore", "inherit", "inherit", "ipc"],
    });
    children.push(child);
  }
  await Promise.all(children.map(child => new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error("Next.js 开发进程启动超时，请检查上方日志")), 90_000);
    timer.unref();
    child.once("error", error => { clearTimeout(timer); reject(error); });
    child.once("exit", code => {
      clearTimeout(timer);
      if (stopping) return;
      reject(new Error(`Next.js 开发进程退出 (${code})`));
      if (gateway) void stop().then(() => process.exit(1));
    });
    child.on("message", message => { if (message.type === "ready") { clearTimeout(timer); resolve(); } });
  })));
  const state = { pid: process.pid, port, id: randomUUID() };
  gateway = createDevGateway(targets, state);
  gateway.on("error", error => { console.error(error); void stop().then(() => process.exit(1)); });
  gateway.listen(port, host);
  await once(gateway, "listening");
  await mkdir(path.dirname(stateFile), { recursive: true });
  await writeFile(stateFile, JSON.stringify(state));
  console.log(`\n前台：http://localhost:${port}\n后台：http://localhost:${port}/admin\n已启用热更新；停止服务运行 pnpm dev:stop。\n`);
}

process.once("SIGINT", () => { void stop().then(() => process.exit(0)); });
process.once("SIGTERM", () => { void stop().then(() => process.exit(0)); });
try {
  if (process.argv.includes("--stop")) {
    const state = await readState();
    if (await isRunning(state)) {
      process.kill(state.pid, "SIGTERM");
      for (let attempt = 0; attempt < 50; attempt++) {
        try { process.kill(state.pid, 0); } catch { break; }
        await delay(100);
      }
      console.log("已停止本地前台和后台开发服务；Docker 后端继续运行。");
    } else {
      console.log("没有正在运行的本项目开发入口。");
    }
  } else {
    await start();
  }
} catch (error) {
  console.error(error.message);
  await stop();
  process.exitCode = 1;
}
