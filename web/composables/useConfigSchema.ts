/**
 * Initialize config values based on a schema definition.
 * Mutates the config object in place, coercing types and applying defaults.
 */
export function initializeConfigFromSchema(
  config: Record<string, any>,
  fields: any[],
): void {
  fields.forEach((field: any) => {
    if (field.type === "bool") {
      if (config[field.name] !== undefined) {
        if (typeof config[field.name] === "string") {
          config[field.name] =
            config[field.name] === "true" || config[field.name] === "1";
        } else {
          config[field.name] = Boolean(config[field.name]);
        }
      } else {
        config[field.name] =
          field.default !== undefined ? Boolean(field.default) : false;
      }
    } else if (field.sensitive && config[field.name] === "***MASKED***") {
      config[field.name] = "";
    } else if (field.type === "arraystring") {
      if (config[field.name] && !Array.isArray(config[field.name])) {
        if (typeof config[field.name] === "string") {
          config[field.name] = config[field.name]
            .split(",")
            .map((s: string) => s.trim())
            .filter((s: string) => s.length > 0);
        }
      } else if (!config[field.name]) {
        config[field.name] = field.default || [];
      }
    } else if (field.type === "arrayint") {
      if (config[field.name] && !Array.isArray(config[field.name])) {
        if (typeof config[field.name] === "string") {
          config[field.name] = config[field.name]
            .split(",")
            .map((s: string) => parseInt(s.trim()))
            .filter((n: number) => !isNaN(n));
        }
      } else if (!config[field.name]) {
        config[field.name] = field.default || [];
      }
    } else if (field.type === "arraybool") {
      if (!config[field.name]) {
        config[field.name] = field.default || [];
      }
    } else if (field.type === "arrayobject") {
      if (!config[field.name]) {
        config[field.name] = field.default || [];
      }
      // Initialize nested objects in array
      if (
        Array.isArray(config[field.name]) &&
        field.nested &&
        field.nested.length > 0
      ) {
        config[field.name].forEach((item: any) => {
          if (typeof item === "object" && item !== null) {
            initializeConfigFromSchema(item, field.nested);
          }
        });
      }
    } else if (field.type === "object") {
      if (!config[field.name]) {
        config[field.name] = field.default || {};
      }
      if (field.nested && field.nested.length > 0) {
        initializeConfigFromSchema(config[field.name], field.nested);
      }
    } else if (field.type === "int") {
      if (config[field.name] === undefined) {
        config[field.name] =
          field.default !== undefined ? field.default : 0;
      }
    } else if (field.type === "string") {
      if (config[field.name] === undefined) {
        config[field.name] =
          field.default !== undefined ? field.default : "";
      }
    }
  });
}
