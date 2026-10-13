import { sample } from 'openapi-sampler';
import type { OpenAPIV3 } from 'openapi-types';
import type { OpenApiDocument } from 'openapi-merge';

const methods = new Set(['get', 'post', 'put', 'patch', 'delete', 'head', 'options', 'trace']);
const humanize = (name: string) => name.replace(/^models\./, '').replace(/([a-z])([A-Z])/g, '$1 $2');

export function* operations(spec: OpenAPIV3.Document) {
  for (const [path, item] of Object.entries(spec.paths ?? {})) {
    if (!item) continue;
    for (const [method, operation] of Object.entries(item)) {
      if (methods.has(method) && operation) yield { path, method, operation: operation as OpenAPIV3.OperationObject };
    }
  }
}

// Materialize each service's security before merging. A service-wide requirement
// must never leak onto another service's public login or health operations.
export function prepareForMerge(spec: OpenAPIV3.Document): OpenApiDocument {
  for (const { operation } of operations(spec)) {
    operation.security ??= structuredClone(spec.security ?? []);
  }
  delete spec.security;
  const bearer = spec.components?.securitySchemes?.Bearer;
  if (bearer) {
    spec.components!.securitySchemes!.Bearer = {
      type: 'http', scheme: 'bearer', bearerFormat: 'JWT',
      description: 'JWT access token returned by the authentication service.',
    };
  }
  // The merge library uses its own OpenAPI types, with required path items.
  return spec as OpenApiDocument;
}

function schemaExample(schema: OpenAPIV3.SchemaObject | OpenAPIV3.ReferenceObject, spec: OpenAPIV3.Document, request?: boolean) {
  // The sampler accepts JSON Schema types; OpenAPI 3.0 adds nullable and uses
  // boolean exclusive bounds, which its runtime also supports.
  return sample(schema as Parameters<typeof sample>[0], {
    skipReadOnly: request === true, skipWriteOnly: request === false,
  }, spec);
}

function addExamples(content: Record<string, OpenAPIV3.MediaTypeObject> | undefined, spec: OpenAPIV3.Document, request: boolean) {
  for (const media of Object.values(content ?? {})) {
    if (!Object.hasOwn(media, 'example') && !Object.keys(media.examples ?? {}).length && media.schema) {
      media.example = schemaExample(media.schema, spec, request);
    }
  }
}

function descriptionContext(path: string[]) {
  if (path[0] === 'paths') {
    const location = `${path[2].toUpperCase()} ${path[1]}`;
    if (path[3] === 'responses') return `Response ${path[4]} for ${location}${path.length > 5 ? `, ${path.slice(5).join(' ')}` : ''}`;
    if (path[3] === 'parameters') return `Parameter ${path[4]} for ${location}`;
    return `${location}${path.length > 3 ? `, ${path.slice(3).join(' ')}` : ''}`;
  }
  return path.filter((part) => !['components', 'schemas', 'properties'].includes(part)).join('.');
}

function distinguishDescriptions(spec: OpenAPIV3.Document) {
  const descriptions = new Map<string, Array<{ value: Record<string, unknown>; path: string[] }>>();
  function visit(value: unknown, path: string[] = []) {
    if (!value || typeof value !== 'object') return;
    const object = value as Record<string, unknown>;
    if (typeof object.description === 'string' && object.description.trim()) {
      const matches = descriptions.get(object.description) ?? [];
      matches.push({ value: object, path });
      descriptions.set(object.description, matches);
    }
    for (const [key, child] of Object.entries(value)) {
      // Payload examples/defaults are API data, not documentation metadata.
      if (!['example', 'default', 'enum', 'value'].includes(key)) visit(child, [...path, key]);
    }
  }
  visit(spec);
  for (const [description, matches] of descriptions) {
    if (matches.length < 2) continue;
    for (const { value, path } of matches) {
      value.description = `${description.replace(/\.$/, '')}. ${descriptionContext(path)}.`;
    }
  }
}

export function enrich(spec: OpenAPIV3.Document) {
  const tags = new Map((spec.tags ?? []).map((tag) => [tag.name, tag]));
  for (const { path, method, operation } of operations(spec)) {
    const words = `${method} ${path.replaceAll('{', ' by ').replaceAll('}', '')}`.match(/[a-zA-Z0-9]+/g)!;
    operation.operationId ||= words.map((word, i) => i ? word[0].toUpperCase() + word.slice(1) : word).join('');
    if (!operation.description && !operation.summary) operation.summary = `${method.toUpperCase()} ${path}`;
    for (const name of operation.tags ?? []) {
      if (!tags.has(name)) tags.set(name, { name, description: `${humanize(name)} operations.` });
    }
    if (operation.requestBody && !('$ref' in operation.requestBody)) {
      operation.requestBody.description ||= `Request fields for ${operation.summary || operation.description}`.replace(/\.?$/, '.');
      addExamples(operation.requestBody.content, spec, true);
    }
    for (const response of Object.values(operation.responses ?? {})) {
      if (!('$ref' in response)) addExamples(response.content, spec, false);
    }
  }
  spec.tags = [...tags.values()];
  for (const tag of spec.tags) tag.description ||= `${humanize(tag.name)} operations.`;
  for (const [kind, components] of Object.entries(spec.components ?? {})) {
    for (const [name, component] of Object.entries(components)) {
      const metadata = component as { $ref?: string; description?: string; enum?: unknown[]; example?: unknown };
      if (metadata.$ref) continue;
      metadata.description ||= `${humanize(name)}${metadata.enum ? ' values' : ' fields'}.`;
      if (kind === 'schemas' && !Object.hasOwn(metadata, 'example')) {
        (component as OpenAPIV3.SchemaObject).example = schemaExample(component as OpenAPIV3.SchemaObject, spec);
      }
    }
  }
  distinguishDescriptions(spec);
  return spec;
}
