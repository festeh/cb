#!/usr/bin/env bun
import { existsSync, mkdirSync, readFileSync, writeFileSync, appendFileSync } from "fs";
import { dirname } from "path";
import { execSync, spawn } from "child_process";
import arg from "arg";

const STATE_FILE = `${process.env.HOME}/.config/cb/screen-state.json`;
const SCREEN_BIN = "/home/dima/projects/cb/screen/screen";
const LOG_FILE = "/tmp/cb-screen.log";

function log(message: string): void {
  const now = new Date();
  const pad = (n: number, len = 2) => String(n).padStart(len, "0");
  const timestamp = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} ${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}.${pad(now.getMilliseconds(), 3)}`;
  appendFileSync(LOG_FILE, `${timestamp} ${message}\n`);
}

interface State {
  tab: number;
  position: string;
  server: string;
}

function loadState(): State {
  mkdirSync(dirname(STATE_FILE), { recursive: true });

  if (existsSync(STATE_FILE)) {
    try {
      const state = JSON.parse(readFileSync(STATE_FILE, "utf-8"));
      return {
        tab: state.tab ?? 0,
        position: state.position ?? "right",
        server: state.server ?? "http://localhost:8080",
      };
    } catch {}
  }

  return { tab: 0, position: "right", server: "http://localhost:8080" };
}

function saveState(state: State): void {
  writeFileSync(STATE_FILE, JSON.stringify(state));
}

function parseArgs(savedState: State) {
  const args = arg({
    "--tab": Number,
    "--pos": String,
    "--server": String,
    "--scroll": String,
    "-t": "--tab",
    "-p": "--pos",
    "-s": "--server",
  });

  return {
    tab: args["--tab"] ?? savedState.tab,
    pos: args["--pos"] ?? savedState.position,
    server: args["--server"] ?? savedState.server,
    scroll: args["--scroll"] as "up" | "down" | undefined,
  };
}

function isScreenRunning(): boolean {
  try {
    execSync("pgrep -f 'cb/screen/screen'", { stdio: "pipe" });
    return true;
  } catch {
    return false;
  }
}

function killScreen(): void {
  try {
    execSync("pkill -f 'cb/screen/screen'", { stdio: "pipe" });
  } catch {}
}

async function scrollBoth(server: string, dir: "up" | "down"): Promise<void> {
  // Send signal to screen overlay (SIGUSR1=up, SIGUSR2=down)
  const signal = dir === "up" ? "SIGUSR1" : "SIGUSR2";
  try {
    execSync(`pkill -${signal} -f 'cb/screen/screen'`, { stdio: "pipe" });
  } catch {}

  // Send HTTP request to server for frontend SSE
  try {
    await fetch(`${server}/scroll?dir=${dir}`, { method: "POST" });
  } catch {}
}

async function notifyTabChange(server: string, tab: number): Promise<void> {
  try {
    await fetch(`${server}/tab`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ tab }),
    });
  } catch {}
}

async function triggerFlowIfNeeded(server: string): Promise<void> {
  try {
    const res = await fetch(`${server}/flow/has-content`);
    const data = await res.json();

    if (!data.has_content) {
      fetch(`${server}/flow/trigger`, { method: "POST" }).catch(() => {});
    }
  } catch {}
}

function launchScreen(server: string, tab: number): void {
  const child = spawn(SCREEN_BIN, ["-server", server, "-tab", String(tab)], {
    stdio: "inherit",
    detached: false,
  });

  process.on("SIGINT", () => child.kill("SIGINT"));
  process.on("SIGTERM", () => child.kill("SIGTERM"));
  child.on("exit", (code) => process.exit(code ?? 0));
}

async function main() {
  log("Starting launch script");
  const savedState = loadState();
  const { tab, pos, server, scroll } = parseArgs(savedState);
  log(`Args: tab=${tab} pos=${pos} server=${server} scroll=${scroll}`);

  // Handle scroll command
  if (scroll) {
    log(`Scrolling ${scroll}`);
    await scrollBoth(server, scroll);
    process.exit(0);
  }

  log(`Saved state: tab=${savedState.tab} pos=${savedState.position}`);

  // Toggle: if running with same params, kill and exit
  if (isScreenRunning() && tab === savedState.tab && pos === savedState.position) {
    log("Screen running with same params, killing and exiting");
    killScreen();
    process.exit(0);
  }

  saveState({ tab, position: pos, server });
  log("Notifying tab change");
  await notifyTabChange(server, tab);
  log("Triggering flow if needed");
  await triggerFlowIfNeeded(server);

  log("Launching screen");
  killScreen();
  launchScreen(server, tab);
}

main();
