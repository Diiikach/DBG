/** Убирает из объекта параметров undefined, null и пустые строки. */
export function cleanParams(
  params: Record<string, unknown>,
): Record<string, string | number | boolean> {
  const out: Record<string, string | number | boolean> = {};
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null) continue;
    if (typeof v === "string" && v.trim() === "") continue;
    out[k] = v as string | number | boolean;
  }
  return out;
}
