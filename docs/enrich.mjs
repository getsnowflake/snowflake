import { sample } from 'openapi-sampler';

const methods = new Set(['get', 'post', 'put', 'patch', 'delete', 'head', 'options', 'trace']);
const humanize = (name) => name.replace(/^models\./, '').replace(/([a-z])([A-Z])/g, '$1 $2');

export function* operations(spec) {
  for (const [path, item] of Object.entries(spec.paths ?? {})) {
    for (const [method, operation] of Object.entries(item)) {
      if (methods.has(method)) yield { path, method, operation };
    }
  }
}

// Materialize each service's security before merging. A service-wide requirement
// must never leak onto another service's public login or health operations.
export function prepareForMerge(spec) {
  for (const { operation } of operations(spec)) {
    operation.security ??= structuredClone(spec.security ?? []);
  }
  delete spec.security;
  const bearer = spec.components?.securitySchemes?.Bearer;
  if (bearer) {
    spec.components.securitySchemes.Bearer = {
      type: 'http', scheme: 'bearer', bearerFormat: 'JWT',
      description: 'JWT access token returned by the authentication service.',
    };
  }
  return spec;
}

function addExamples(content, spec, request) {
  for (const media of Object.values(content ?? {})) {
    if (!Object.hasOwn(media, 'example') && !Object.keys(media.examples ?? {}).length && media.schema) {
      media.example = sample(media.schema, { skipReadOnly: request, skipWriteOnly: !request }, spec);
    }
  }
}

function descriptionContext(path) {
  if (path[0] === 'paths') {
    const location = `${path[2].toUpperCase()} ${path[1]}`;
    if (path[3] === 'responses') return `Response ${path[4]} for ${location}${path.length > 5 ? `, ${path.slice(5).join(' ')}` : ''}`;
    if (path[3] === 'parameters') return `Parameter ${path[4]} for ${location}`;
    return `${location}${path.length > 3 ? `, ${path.slice(3).join(' ')}` : ''}`;
  }
  return path.filter((part) => !['components', 'schemas', 'properties'].includes(part)).join('.');
}

function distinguishDescriptions(spec) {
  const descriptions = new Map();
  function visit(value, path = []) {
    if (!value || typeof value !== 'object') return;
    if (typeof value.description === 'string' && value.description.trim()) {
      const matches = descriptions.get(value.description) ?? [];
      matches.push({ value, path });
      descriptions.set(value.description, matches);
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

export function enrich(spec) {
  const tags = new Map((spec.tags ?? []).map((tag) => [tag.name, tag]));
  for (const { path, method, operation } of operations(spec)) {
    const words = `${method} ${path.replaceAll('{', ' by ').replaceAll('}', '')}`.match(/[a-zA-Z0-9]+/g);
    operation.operationId ||= words.map((word, i) => i ? word[0].toUpperCase() + word.slice(1) : word).join('');
    if (!operation.description && !operation.summary) operation.summary = `${method.toUpperCase()} ${path}`;
    for (const name of operation.tags ?? []) {
      if (!tags.has(name)) tags.set(name, { name, description: `${humanize(name)} operations.` });
    }
    if (operation.requestBody && !operation.requestBody.$ref) {
      operation.requestBody.description ||= `Request fields for ${operation.summary || operation.description}`.replace(/\.?$/, '.');
      addExamples(operation.requestBody.content, spec, true);
    }
    for (const response of Object.values(operation.responses ?? {})) addExamples(response.content, spec, false);
  }
  spec.tags = [...tags.values()];
  for (const tag of spec.tags) tag.description ||= `${humanize(tag.name)} operations.`;
  for (const [kind, components] of Object.entries(spec.components ?? {})) {
    for (const [name, component] of Object.entries(components)) {
      if (!component.$ref) component.description ||= `${humanize(name)}${component.enum ? ' values' : ' fields'}.`;
      if (kind === 'schemas' && !Object.hasOwn(component, 'example')) {
        component.example = sample(component, {}, spec);
      }
    }
  }
  distinguishDescriptions(spec);
  return spec;
}
