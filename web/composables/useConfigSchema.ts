import type { ConfigSchemaField } from "~/types";

/**
 * Returns a new config object with values coerced to the types declared by the
 * schema and any missing fields populated from `field.default`.
 */
export function initializeConfigFromSchema(
  config: Record<string, unknown>,
  fields: ConfigSchemaField[],
): Record<string, unknown> {
  const result: Record<string, unknown> = { ...config };

  fields.forEach((field) => {
    const current = result[field.name];

    if (field.type === "bool") {
      if (current !== undefined) {
        if (typeof current === "string") {
          result[field.name] = current === "true" || current === "1";
        } else {
          result[field.name] = Boolean(current);
        }
      } else {
        result[field.name] =
          field.default !== undefined ? Boolean(field.default) : false;
      }
    } else if (field.sensitive && current === "***MASKED***") {
      result[field.name] = "";
    } else if (field.type === "arraystring") {
      if (current && !Array.isArray(current)) {
        if (typeof current === "string") {
          result[field.name] = current
            .split(",")
            .map((s) => s.trim())
            .filter((s) => s.length > 0);
        }
      } else if (!current) {
        result[field.name] = field.default ?? [];
      }
    } else if (field.type === "arrayint") {
      if (current && !Array.isArray(current)) {
        if (typeof current === "string") {
          result[field.name] = current
            .split(",")
            .map((s) => parseInt(s.trim()))
            .filter((n) => !isNaN(n));
        }
      } else if (!current) {
        result[field.name] = field.default ?? [];
      }
    } else if (field.type === "arraybool") {
      if (!current) {
        result[field.name] = field.default ?? [];
      }
    } else if (field.type === "arrayobject") {
      const seeded = current ?? field.default ?? [];
      if (Array.isArray(seeded) && field.nested && field.nested.length > 0) {
        result[field.name] = seeded.map((item) =>
          typeof item === "object" && item !== null
            ? initializeConfigFromSchema(
                item as Record<string, unknown>,
                field.nested!,
              )
            : item,
        );
      } else {
        result[field.name] = seeded;
      }
    } else if (field.type === "object") {
      const seeded =
        (current as Record<string, unknown> | undefined) ??
        (field.default as Record<string, unknown> | undefined) ??
        {};
      if (field.nested && field.nested.length > 0) {
        result[field.name] = initializeConfigFromSchema(seeded, field.nested);
      } else {
        result[field.name] = seeded;
      }
    } else if (field.type === "int") {
      if (current === undefined) {
        result[field.name] =
          field.default !== undefined ? field.default : 0;
      }
    } else if (field.type === "string") {
      if (current === undefined) {
        result[field.name] =
          field.default !== undefined ? field.default : "";
      }
    }
  });

  return result;
}
