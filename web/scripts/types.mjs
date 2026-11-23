// Writes src/lib/api.gen.ts, TypeScript types for API v2, from docs/api/v2/schema.json.
// Covers the JSON Schema the API uses: objects, arrays and tuples, enums, consts,
// $ref and oneOf. Anything else fails loudly rather than turning into `any`.
import { readFileSync, writeFileSync } from "node:fs";

const schema = JSON.parse(readFileSync(new URL("../../docs/api/v2/schema.json", import.meta.url), "utf8"));
const known = new Set(["type", "properties", "required", "additionalProperties", "items", "prefixItems", "minItems", "enum", "const", "$ref", "oneOf", "description", "format", "pattern", "minimum", "maximum", "minLength", "minProperties", "maxProperties"]);

function type(s, at) {
  for (const k of Object.keys(s)) if (!known.has(k)) throw new Error(`${at}: unsupported keyword ${k}`);
  if (s.$ref) return s.$ref.replace("#/$defs/", "");
  if (s.const !== undefined) return JSON.stringify(s.const);
  if (s.enum) return s.enum.map((v) => JSON.stringify(v)).join(" | ");
  if (s.oneOf) {
    // Each branch narrows the base object's properties.
    return s.oneOf.map((b, i) => object({ ...s, ...b, oneOf: undefined, properties: { ...s.properties, ...b.properties } }, `${at}.oneOf[${i}]`)).join(" | ");
  }
  switch (s.type) {
    case "string": return "string";
    case "number": case "integer": return "number";
    case "boolean": return "boolean";
    case "array":
      if (s.prefixItems) return `[${s.prefixItems.map((p, i) => type(p, `${at}[${i}]`)).join(", ")}]`;
      return s.items ? `${wrap(type(s.items, `${at}[]`))}[]` : "unknown[]";
    case "object": return object(s, at);
  }
  throw new Error(`${at}: no type`);
}

const wrap = (t) => (t.includes(" | ") ? `(${t})` : t);

function object(s, at) {
  const req = new Set(s.required ?? []);
  const fields = Object.entries(s.properties ?? {}).map(([k, v]) => {
    const doc = v.description ? `  /** ${v.description} */\n` : "";
    return `${doc}  ${k}${req.has(k) ? "" : "?"}: ${type(v, `${at}.${k}`)};`;
  });
  return `{\n${fields.join("\n")}\n}`;
}

const out = ["// Generated from docs/api/v2/schema.json by scripts/types.mjs. Do not edit.", ""];
for (const [name, def] of Object.entries(schema.$defs)) {
  if (def.description) out.push(`/** ${def.description} */`);
  out.push(`export type ${name} = ${type(def, name)};`, "");
}
writeFileSync(new URL("../src/lib/api.gen.ts", import.meta.url), out.join("\n"));
