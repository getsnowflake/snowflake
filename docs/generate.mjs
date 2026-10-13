import { readFile, writeFile, rename, rm } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { promisify } from 'node:util';
import swagger2openapi from 'swagger2openapi';
import { merge, isErrorResult } from 'openapi-merge';
import { enrich, prepareForMerge } from './enrich.mjs';
import { lint } from './lint.mjs';

const [command, input, output] = process.argv.slice(2);
const readJson = async (file) => JSON.parse(await readFile(file, 'utf8'));
if (command === 'convert' && input && output) {
  const result = await promisify(swagger2openapi.convertObj)(await readJson(input), {});
  await writeFile(output, JSON.stringify(result.openapi, null, 2) + '\n');
} else if (command === 'merge' && input) {
  const config = await readJson(input);
  const base = dirname(resolve(input));
  const inputs = [];
  for (const entry of config.inputs) {
    const spec = await readJson(resolve(base, entry.inputFile));
    inputs.push({ oas: prepareForMerge(spec) });
  }
  const result = merge(inputs, { securitySchemesStrategy: 'error' });
  if (isErrorResult(result)) throw new Error(result.message);
  const file = resolve(base, config.output);
  // Only publish the new artifact after it passes validation and linting.
  const candidate = file + '.tmp.json';
  try {
    await writeFile(candidate, JSON.stringify(enrich(result.output), null, 2) + '\n');
    lint(candidate);
    await rename(candidate, file);
  } finally {
    await rm(candidate, { force: true });
  }
} else {
  throw new Error('Usage: node generate.mjs convert <swagger.json> <openapi.json> | merge <config.json>');
}
