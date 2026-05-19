import dayjs from "dayjs";
import "dayjs/locale/ru";

dayjs.locale("ru");

/** Форматирует дату из ISO/YYYY-MM-DD в DD.MM.YYYY. */
export function formatDate(value?: string | null): string {
  if (!value) return "—";
  const d = dayjs(value);
  if (!d.isValid()) return value;
  return d.format("DD.MM.YYYY");
}

/** Форматирует timestamp в DD.MM.YYYY HH:mm. */
export function formatDateTime(value?: string | null): string {
  if (!value) return "—";
  const d = dayjs(value);
  if (!d.isValid()) return value;
  return d.format("DD.MM.YYYY HH:mm");
}

/** Возвращает "—" если значение пустое/undefined. */
export function dash<T>(value: T | null | undefined): T | string {
  if (value === null || value === undefined) return "—";
  if (typeof value === "string" && value.trim() === "") return "—";
  return value;
}

/** Формат числа с локалью ru-RU. */
export function formatNumber(value?: number | null, fractionDigits = 2): string {
  if (value === null || value === undefined || Number.isNaN(value)) return "—";
  return new Intl.NumberFormat("ru-RU", {
    minimumFractionDigits: 0,
    maximumFractionDigits: fractionDigits,
  }).format(value);
}
