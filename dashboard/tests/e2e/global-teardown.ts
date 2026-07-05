import * as fs from 'node:fs';

export default async function globalTeardown(): Promise<void> {
  const pid = process.env.COMAX_E2E_PID;
  if (pid) {
    try {
      process.kill(Number(pid));
    } catch {
      /* already dead */
    }
  }
  const tmp = process.env.COMAX_E2E_TMP;
  if (tmp && fs.existsSync(tmp)) {
    try {
      fs.rmSync(tmp, { recursive: true, force: true });
    } catch {
      /* best-effort cleanup */
    }
  }
}
