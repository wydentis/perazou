export const WEEKDAYS = ['Понедельник', 'Вторник', 'Среда', 'Четверг', 'Пятница', 'Суббота', 'Воскресенье'];
export const PARITY = ['Каждую неделю', 'Нечётные недели', 'Чётные недели'];

export function fmtDate(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}
export function parseDate(s: string): Date {
  const [y, m, d] = s.split('-').map(Number);
  return new Date(y, m - 1, d);
}
export function addDays(s: string, n: number): string {
  const d = parseDate(s);
  d.setDate(d.getDate() + n);
  return fmtDate(d);
}
export function humanDate(s: string): string {
  return parseDate(s).toLocaleDateString('ru-RU', { weekday: 'long', day: 'numeric', month: 'long' });
}
/** Today's date in the group's timezone (so the day switches when the group's day does). */
export function todayIn(tz: string): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone: tz, year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date());
}

export const WEEKDAYS_SHORT = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];

export function isoWeekday(s: string): number {
  return ((parseDate(s).getDay() + 6) % 7) + 1;
}
export function mondayOf(s: string): string {
  return addDays(s, 1 - isoWeekday(s));
}
/** Mirrors backend isOddWeek: week1Monday is the Monday of an odd week. */
export function isOddWeek(week1Monday: string, date: string): boolean {
  const days = Math.round((parseDate(mondayOf(date)).getTime() - parseDate(week1Monday).getTime()) / 86400000);
  const weeks = Math.floor(days / 7);
  return ((weeks % 2) + 2) % 2 === 0;
}
export function occursOn(
  slot: { weekday: number; parity: number; valid_from?: string; valid_to?: string },
  week1Monday: string,
  date: string,
): boolean {
  if (slot.weekday !== isoWeekday(date)) return false;
  if ((slot.valid_from && date < slot.valid_from) || (slot.valid_to && date > slot.valid_to)) return false;
  if (slot.parity === 1) return isOddWeek(week1Monday, date);
  if (slot.parity === 2) return !isOddWeek(week1Monday, date);
  return true;
}
/** Minutes since midnight in the group's timezone. */
export function nowMinutesIn(tz: string): number {
  const [h, m] = new Intl.DateTimeFormat('en-GB', { timeZone: tz, hour: '2-digit', minute: '2-digit', hour12: false })
    .format(new Date())
    .split(':')
    .map(Number);
  return (h % 24) * 60 + m;
}
export function toMinutes(hhmm: string): number {
  const [h, m] = hhmm.split(':').map(Number);
  return h * 60 + m;
}
/** "2026-10-24" -> "24.10" */
export function shortDate(s: string): string {
  return s ? `${s.slice(8, 10)}.${s.slice(5, 7)}` : '';
}
export function rangeLabel(from: string, to: string): string {
  if (from && to) return from === to ? shortDate(from) : `${shortDate(from)}–${shortDate(to)}`;
  if (from) return `с ${shortDate(from)}`;
  if (to) return `до ${shortDate(to)}`;
  return '';
}
export function plural(n: number, one: string, few: string, many: string): string {
  const m10 = n % 10, m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return one;
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return few;
  return many;
}
