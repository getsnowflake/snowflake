import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

type Problem = { ruleId: string; message: string };

export function lint(file: string) {
  const cli = fileURLToPath(new URL('./node_modules/@scalar/cli/index.js', import.meta.url));
  const validate = spawnSync(process.execPath, [cli, 'document', 'validate', file], { encoding: 'utf8' });
  if (validate.error || validate.status !== 0) throw new Error(validate.error?.message ?? validate.stdout + validate.stderr);
  const result = spawnSync(process.execPath, [cli, 'document', 'lint', file, '--format', 'json', '--max-problems', '10000'], { encoding: 'utf8' });
  if (result.error) throw result.error;
  if (!result.stdout.trim()) throw new Error(result.stderr || 'Scalar returned no lint report.');
  const report = JSON.parse(result.stdout) as { problems: Problem[] };
  // Preserve the established wire format. This is Scalar's naming advice,
  // not an invalid contract, and changing just the spec would mislead clients.
  const findings = report.problems.filter((p) => p.ruleId !== 'camel-case-properties');
  // Scalar normally exits successfully on warnings. Documentation generation
  // treats warnings and info findings as failures too, so they cannot regress.
  if (result.status !== 0 || findings.length) {
    throw new Error(findings.map((p) => `${p.ruleId}: ${p.message}`).join('\n') || result.stderr);
  }
  console.log(`Scalar validation and lint passed: 0 contract findings; ${report.problems.length} naming advisories for existing JSON fields.`);
}

if (import.meta.main) lint(process.argv[2]);
