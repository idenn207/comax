import { spawn } from 'node:child_process';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';

const BOOT_LINE = /Comax Secrets: bootstrap admin token \(shown ONCE\):\s*\n\s*(\S+)/;

export default async function globalSetup(): Promise<void> {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const workspaceRoot = path.resolve(here, '..', '..', '..', '..');
  const binSuffix = process.platform === 'win32' ? '.exe' : '';
  const serverBin = path.join(workspaceRoot, 'bin', `secret-server${binSuffix}`);
  if (!fs.existsSync(serverBin)) {
    throw new Error(
      `secret-server not built at ${serverBin}. Run: go build -tags embed_dashboard -o bin/secret-server ./cmd/server`,
    );
  }

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'comax-e2e-'));
  const port = process.env.COMAX_E2E_PORT ?? '9090';

  const child = spawn(serverBin, [], {
    env: {
      ...process.env,
      COMAX_LISTEN: `:${port}`,
      COMAX_DB_PATH: path.join(tmpDir, 'secrets.db'),
      COMAX_MASTER_KEY_FILE: path.join(tmpDir, 'master.key'),
      COMAX_DASHBOARD_ENABLED: 'true',
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  let stdoutBuf = '';
  let onExit!: (code: number | null) => void;
  let timer!: NodeJS.Timeout;
  const tokenPromise = new Promise<string>((resolve, reject) => {
    onExit = (code) =>
      reject(new Error(`secret-server exited early with code ${code}\nstdout:\n${stdoutBuf}`));
    timer = setTimeout(
      () => reject(new Error(`timed out waiting for bootstrap token\nstdout:\n${stdoutBuf}`)),
      30_000,
    );
    child.on('exit', onExit);
    child.stdout!.on('data', (chunk: Buffer) => {
      stdoutBuf += chunk.toString();
      const m = stdoutBuf.match(BOOT_LINE);
      if (m) resolve(m[1]);
    });
    child.stderr!.on('data', (chunk: Buffer) => {
      process.stderr.write(`[secret-server stderr] ${chunk}`);
    });
  });

  let token!: string;
  try {
    token = await tokenPromise;
  } finally {
    clearTimeout(timer);
    child.off('exit', onExit);
  }

  const healthURL = `http://localhost:${port}/healthz`;
  for (let i = 0; i < 60; i++) {
    try {
      const res = await fetch(healthURL);
      if (res.ok) break;
    } catch {
      /* not ready */
    }
    await new Promise((r) => setTimeout(r, 250));
  }

  process.env.DASHBOARD_TOKEN = token;
  process.env.COMAX_E2E_PID = String(child.pid);
  process.env.COMAX_E2E_TMP = tmpDir;
  child.unref();
}
