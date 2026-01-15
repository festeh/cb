import { existsSync, mkdirSync, readFileSync, writeFileSync } from "fs";
import { dirname } from "path";
import { execSync, spawn } from "child_process";
import arg from "arg";

const STATE_FILE = `${process.env.HOME}/.config/cb/screen-state.json`;
const SCREEN_BIN = "/home/dima/projects/cb/screen/screen";

interface State {
  tab: number;
  position: string;
}

function loadState(): State {
  mkdirSync(dirname(STATE_FILE), { recursive: true });

  if (existsSync(STATE_FILE)) {
    try {
      const state = JSON.parse(readFileSync(STATE_FILE, "utf-8"));
      return {
        tab: state.tab ?? 0,
        position: state.position ?? "right",
      };
    } catch {}
  }

  return { tab: 0, position: "right" };
}

function saveState(state: State): void {
  writeFileSync(STATE_FILE, JSON.stringify(state));
}

function parseArgs(savedState: State) {
  const args = arg({
    "--tab": Number,
    "--pos": String,
    "--server": String,
    "-t": "--tab",
    "-p": "--pos",
    "-s": "--server",
  });

  return {
    tab: args["--tab"] ?? savedState.tab,
    pos: args["--pos"] ?? savedState.position,
    server: args["--server"] ?? "http://localhost:8082",
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

async function notifyTabChange(server: string, tab: number): Promise<void> {
  try {
    await fetch(`${server}/tab`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ tab }),
    });
  } catch {}
}

async function ensureFlowContent(server: string): Promise<void> {
  try {
    const res = await fetch(`${server}/flow/has-content`);
    const data = await res.json();

    if (!data.has_content) {
      fetch(`${server}/flow/trigger`, { method: "POST" }).catch(() => {});

      for (let attempts = 0; attempts < 20; attempts++) {
        await new Promise((resolve) => setTimeout(resolve, 5000));
        const checkRes = await fetch(`${server}/flow/has-content`);
        const checkData = await checkRes.json();
        if (checkData.has_content) break;
      }
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
  const savedState = loadState();
  const { tab, pos, server } = parseArgs(savedState);

  // Toggle: if running with same params, kill and exit
  if (isScreenRunning() && tab === savedState.tab && pos === savedState.position) {
    killScreen();
    process.exit(0);
  }

  saveState({ tab, position: pos });
  await notifyTabChange(server, tab);
  await ensureFlowContent(server);

  killScreen();
  launchScreen(server, tab);
}

main();
