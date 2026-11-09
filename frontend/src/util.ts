export const WEEKDAYS = ['Понедельник', 'Вторник', 'Среда', 'Четверг', 'Пятница', 'Суббота', 'Воскресенье'];

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
/** Mirrors backend slot.occursOn. */
export function occursOn(slot: { weekday: number; valid_from?: string; valid_to?: string }, date: string): boolean {
  if (slot.weekday !== isoWeekday(date)) return false;
  return !(slot.valid_from && date < slot.valid_from) && !(slot.valid_to && date > slot.valid_to);
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

/**
 * When the lessons screen should refresh next, in ms, or null if nothing will change today.
 * While any window is open, poll so counters stay fresh; otherwise wake once at the next
 * moment a lesson's state flips (window opens, lesson starts, window closes).
 */
export function lessonsRefreshDelay(
  lessons: { cancelled: boolean; open: boolean; opens_at: string; start_time: string; closes_at: string }[],
  nowMin: number,
  nowSec: number,
): number | null {
  const live = lessons.filter((l) => !l.cancelled);
  if (live.some((l) => l.open)) return 30_000;
  const next = live
    .flatMap((l) => [l.opens_at, l.start_time, l.closes_at].map(toMinutes))
    .filter((m) => m > nowMin)
    .sort((a, b) => a - b)[0];
  if (next === undefined) return null;
  return ((next - nowMin) * 60 - nowSec) * 1000 + 1500; // just after the flip
}
