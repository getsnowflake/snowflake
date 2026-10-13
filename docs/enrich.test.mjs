import assert from 'node:assert/strict';
import test from 'node:test';
import { merge } from 'openapi-merge';
import { enrich, prepareForMerge } from './enrich.mjs';

function fixture() {
  return {
    openapi: '3.0.3', info: { title: 'Test', version: '1' },
    paths: {
      '/score': {
        post: {
          summary: 'Calculate credit score', tags: ['Scorer'],
          requestBody: { content: { 'application/json': { schema: { $ref: '#/components/schemas/Profile' } } } },
          responses: { '200': { description: 'OK', content: { 'application/json': { schema: { $ref: '#/components/schemas/Result' } } } } },
        },
        get: { summary: 'Get credit score', responses: { '200': { description: 'OK' } } },
      },
    },
    components: { schemas: {
      Profile: { type: 'object', required: ['income'], properties: {
        income: { type: 'number', minimum: 1, example: 5000 },
        description: { type: 'string', example: 'OK' },
        secret: { type: 'string', readOnly: true },
      } },
      Result: { type: 'object', properties: {
        score: { type: 'integer', minimum: 0, maximum: 900, example: 750 },
        secret: { type: 'string', writeOnly: true },
      } },
    } },
  };
}

test('generates media examples from referenced schemas without exposing read/write-only fields', () => {
  const spec = enrich(fixture());
  const post = spec.paths['/score'].post;
  assert.equal(post.operationId, 'postScore');
  assert.ok(post.requestBody.description);
  assert.deepEqual(post.requestBody.content['application/json'].example, { income: 5000, description: 'OK' });
  assert.deepEqual(post.responses['200'].content['application/json'].example, { score: 750 });
  assert.equal(spec.tags[0].name, 'Scorer');
  assert.ok(spec.components.schemas.Profile.description);
  assert.notEqual(post.responses['200'].description, spec.paths['/score'].get.responses['200'].description);
});

test('preserves authored examples, IDs, field names and payload descriptions; enrichment is idempotent', () => {
  const spec = fixture();
  const post = spec.paths['/score'].post;
  post.operationId = 'calculateScore';
  const example = { annual_income: 12000, description: 'OK' };
  post.requestBody.content['application/json'].example = structuredClone(example);
  enrich(spec);
  assert.equal(post.operationId, 'calculateScore');
  assert.deepEqual(post.requestBody.content['application/json'].example, example);
  assert.deepEqual(enrich(structuredClone(spec)), spec);
});

test('merging service-wide auth does not make other services public endpoints require JWT', () => {
  const protectedService = fixture();
  protectedService.security = [{ Bearer: [] }];
  protectedService.components.securitySchemes = { Bearer: { type: 'http', scheme: 'bearer' } };
  protectedService.paths['/score'].get.security = [{}];
  const publicService = {
    openapi: '3.0.3', info: { title: 'Auth', version: '1' },
    paths: { '/auth/login': { post: { responses: { '200': { description: 'Tokens' } } } } },
  };
  const result = merge([protectedService, publicService].map((spec) => ({ oas: prepareForMerge(spec) })));
  assert.ok(result.output);
  assert.deepEqual(result.output.paths['/score'].post.security, [{ Bearer: [] }]);
  assert.deepEqual(result.output.paths['/score'].get.security, [{}]);
  assert.deepEqual(result.output.paths['/auth/login'].post.security, []);
});
