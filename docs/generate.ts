import { readFile, writeFile, rename, rm } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { promisify } from 'node:util';
import swagger2openapi from 'swagger2openapi';
import { merge, isErrorResult } from 'openapi-merge';
import type { OpenAPIV2, OpenAPIV3 } from 'openapi-types';
import { enrich, prepareForMerge } from './enrich.ts';
import { lint } from './lint.ts';

const [command, input, output] = process.argv.slice(2);
const readJson = async <T>(file: string): Promise<T> => JSON.parse(await readFile(file, 'utf8'));
if (command === 'convert' && input && output) {
  const result = await promisify(swagger2openapi.convertObj)(await readJson<OpenAPIV2.Document>(input), {});
  await writeFile(output, JSON.stringify(result.openapi, null, 2) + '\n');
} else if (command === 'merge' && input) {
  const config = await readJson<{ inputs: Array<{ inputFile: string }>; output: string }>(input);
  const base = dirname(resolve(input));
  const inputs = [];
  for (const entry of config.inputs) {
    const spec = await readJson<OpenAPIV3.Document>(resolve(base, entry.inputFile));
    inputs.push({ oas: prepareForMerge(spec) });
  }
  const result = merge(inputs, { securitySchemesStrategy: 'error' });
  if (isErrorResult(result)) throw new Error(result.message);
  const file = resolve(base, config.output);
  // Only publish the new artifact after it passes validation and linting.
  const candidate = file + '.tmp.json';
  try {
    await writeFile(candidate, JSON.stringify(enrich(result.output as OpenAPIV3.Document), null, 2) + '\n');
    lint(candidate);
    await rename(candidate, file);
  } finally {
    await rm(candidate, { force: true });
  }
} else {
  throw new Error('Usage: bun generate.ts convert <swagger.json> <openapi.json> | merge <config.json>');
}
