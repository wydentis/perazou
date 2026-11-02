import { fmtDate } from './util';

/**
 * Seasonal themes, switched on by the device's local date. Each runs for whole days
 * [from, to] inclusive and needs no deploy to turn off. `?theme=<name>` previews one
 * outside its dates.
 */
const HOLIDAYS = [{ name: 'halloween', from: '2026-11-03', to: '2026-11-03' }] as const;

export type Holiday = (typeof HOLIDAYS)[number]['name'];

export const holiday: Holiday | null = (() => {
  const forced = new URLSearchParams(location.search).get('theme');
  const match = HOLIDAYS.find((h) => h.name === forced);
  if (match) return match.name;
  const today = fmtDate(new Date());
  return HOLIDAYS.find((h) => today >= h.from && today <= h.to)?.name ?? null;
})();

/** Attendance wording; the Halloween theme swaps in playful verbs. */
export const words =
  holiday === 'halloween'
    ? {
        present: 'Явился',
        absent: 'Испарился',
        presentCount: (n: number) => `${n} ${n % 10 === 1 && n % 100 !== 11 ? 'явился' : 'явились'}`,
        absentCount: (n: number) => `${n} ${n % 10 === 1 && n % 100 !== 11 ? 'испарился' : 'испарились'}`,
        absentQuoted: '«испарился»',
        presentEmoji: '🎃',
        absentEmoji: '👻',
      }
    : {
        present: 'Был',
        absent: 'Не был',
        presentCount: (n: number) => `${n} ${n % 10 === 1 && n % 100 !== 11 ? 'был' : 'были'}`,
        absentCount: (n: number) => `${n} не ${n % 10 === 1 && n % 100 !== 11 ? 'был' : 'были'}`,
        absentQuoted: '«не был»',
        presentEmoji: '',
        absentEmoji: '',
      };
