import { useState } from 'react';
import type { CorrectableLesson } from '../types';
import { Icon } from '../icons';
import { haptic } from '../telegram';
import { addDays, humanDate, mondayOf, parseDate, plural } from '../util';

/** Week label "3–9 ноября": day + month together gives the genitive; month alone would be "ноябрь". */
export function weekLabel(monday: string) {
  const a = parseDate(monday), b = parseDate(addDays(monday, 6));
  const dm = (x: Date) => x.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' });
  return a.getMonth() === b.getMonth() ? `${a.getDate()}–${dm(b)}` : `${dm(a)} – ${dm(b)}`;
}

/**
 * Past lessons (newest first) grouped into collapsible weeks, then days, each with ✓/✕ buttons.
 * The two most recent weeks start open.
 */
export function LessonWeeks({ lessons, canEdit, onMark, readOnly = false }: {
  lessons: CorrectableLesson[];
  canEdit: boolean;
  /** Hide the buttons entirely (a view-only history) instead of showing them disabled. */
  readOnly?: boolean;
  onMark: (l: CorrectableLesson, status: 'present' | 'absent') => void;
}) {
  const [toggled, setToggled] = useState<Set<string>>(new Set());
  const weeks: { monday: string; days: { date: string; items: CorrectableLesson[] }[]; absent: number; total: number }[] = [];
  for (const l of lessons) {
    const monday = mondayOf(l.date);
    let w = weeks[weeks.length - 1];
    if (!w || w.monday !== monday) weeks.push((w = { monday, days: [], absent: 0, total: 0 }));
    let d = w.days[w.days.length - 1];
    if (!d || d.date !== l.date) w.days.push((d = { date: l.date, items: [] }));
    d.items.push(l);
    w.total++;
    if (l.status === 'absent') w.absent++;
  }
  const isOpen = (i: number, monday: string) => (i < 2) !== toggled.has(monday);
  const flip = (monday: string) => setToggled((s) => {
    const n = new Set(s);
    if (n.has(monday)) n.delete(monday);
    else n.add(monday);
    return n;
  });

  return (
    <>
      {weeks.map((w, i) => (
        <section key={w.monday} className="week">
          <button className={'week-head' + (isOpen(i, w.monday) ? ' open' : '')} onClick={() => { haptic.select(); flip(w.monday); }}>
            <span className="grow">
              <b>{weekLabel(w.monday)}</b>
              <small>{w.total} {plural(w.total, 'пара', 'пары', 'пар')}{w.absent > 0 && <> · <span className="c-bad">{w.absent} н</span></>}</small>
            </span>
            <Icon name="right" size={16} className="week-chev" />
          </button>
          {isOpen(i, w.monday) && w.days.map(({ date: d, items }) => (
            <div key={d} className="day-section">
              <h3 className="section-title">{humanDate(d)}</h3>
              <div className="card list">
                {items.map((l) => (
                  <div key={l.slot_id + l.date} className="list-row">
                    <div className="slot-time"><b>{l.start_time}</b><span>{l.end_time}</span></div>
                    <div className="grow">
                      <div className="row-title">{l.subject}</div>
                      <div className="meta">
                        <span className={l.status === 'present' ? 'c-ok' : l.status === 'absent' ? 'c-bad' : ''}>
                          {l.status === 'present' ? 'был' : l.status === 'absent' ? (l.auto ? 'н (не отметился)' : 'не был') : 'не отмечено'}
                        </span>
                        {l.corrected && <span className="auto-tag">испр.</span>}
                      </div>
                    </div>
                    {!readOnly && <>
                    <button className={'round yes' + (l.status === 'present' ? ' on' : '')} disabled={!canEdit} onClick={() => onMark(l, 'present')} aria-label="Был"><Icon name="check" size={16} /></button>
                    <button className={'round no' + (l.status === 'absent' && !l.auto ? ' on' : '')} disabled={!canEdit} onClick={() => onMark(l, 'absent')} aria-label="Не был"><Icon name="x" size={16} /></button>
                    </>}
                  </div>
                ))}
              </div>
            </div>
          ))}
        </section>
      ))}
    </>
  );
}
