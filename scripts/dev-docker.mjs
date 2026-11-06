import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../", import.meta.url));
const compose = ["compose", "--ansi", "never", "--progress", "plain", "-f", "infra/docker/docker-compose.yml", "-f", "infra/docker/docker-compose.dev.yml"];

function docker(args, capture = false) {
  return execFileSync("docker", args, { cwd: root, stdio: capture ? ["ignore", "pipe", "inherit"] : "inherit", encoding: "utf8" });
}

export function prepareDocker() {
  // Reuse the existing Compose project and volumes, including Redis's /data.
  const running = docker([...compose, "ps", "--status", "running", "--services"], true).trim().split(/\s+/);
  if (running.includes("redis")) {
    docker([...compose, "exec", "-T", "redis", "redis-cli", "SAVE"]);
  }
  docker([...compose, "stop", "web", "admin"]);
  const services = ["postgres", "redis", "minio", "mock-new-api", "api", "worker"];
  if (running.includes("searxng")) services.push("searxng");
  docker([...compose, "up", "-d", "--no-build", "--pull", "never", "--wait", "--wait-timeout", "60", ...services]);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { prepareDocker(); } catch { process.exitCode = 1; }
}
