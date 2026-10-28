import { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../api';
import type { AttendanceRow, Lesson, Me, Slot } from '../types';
import { addDays, mondayOf, nowMinutesIn, occursOn, parseDate, plural, todayIn, toMinutes, WEEKDAYS_SHORT } from '../util';
import { haptic } from '../telegram';
import { Icon } from '../icons';
import { Avatar, Empty, Skeleton, humanError, useToast } from '../ui';

export default function Today({ me }: { me: Me }) {
  const group = me.group!;
  const isHeadman = me.member!.role === 'headman';
  const today = todayIn(group.timezone);
  const toast = useToast();
  const [date, setDate] = useState(today);
  const [lessons, setLessons] = useState<Lesson[] | null>(null);
  const [slots, setSlots] = useState<Slot[]>([]);
  const [expanded, setExpanded] = useState<number | null>(null);

  useEffect(() => { api.schedule().then(setSlots, () => {}); }, []);
  const load = useCallback(() => {
    api.lessons(date).then(setLessons, (e) => toast(humanError(e)));
  }, [date, toast]);
  useEffect(() => {
    setLessons(null);
    setExpanded(null);
    load();
  }, [load]);
  // The 15-minute marking window opens and closes on its own; keep today's view current.
  useEffect(() => {
    if (date !== today) return;
    const t = setInterval(load, 30_000);
    return () => clearInterval(t);
  }, [date, today, load]);

  const week = useMemo(() => {
    const mon = mondayOf(date);
    return Array.from({ length: 7 }, (_, i) => {
      const d = addDays(mon, i);
      return { d, has: slots.some((s) => occursOn(s, d)) };
    });
  }, [date, slots]);

  async function mark(l: Lesson, status: 'present' | 'absent') {
    const next = l.my_status === status ? 'none' : status;
    // Optimistic update so the toggle feels instant.
    setLessons((ls) => ls && ls.map((x) => x.id !== l.id ? x : {
      ...x,
      my_status: next === 'none' ? '' : next,
      present: x.present - (l.my_status === 'present' ? 1 : 0) + (next === 'present' ? 1 : 0),
      absent: x.absent - (l.my_status === 'absent' ? 1 : 0) + (next === 'absent' ? 1 : 0),
    }));
    haptic.tap();
    try {
      await api.mark({ slot_id: l.id, date: l.date, status: next });
      haptic.ok();
    } catch (e) {
      haptic.err();
      toast(humanError(e));
    }
    load();
  }

  async function toggleCancel(l: Lesson) {
    try {
      await api.cancelLesson(l.id, l.date, !l.cancelled);
      toast(l.cancelled ? 'Пара возвращена' : 'Пара отменена', 'ok');
      load();
    } catch (e) {
      toast(humanError(e));
    }
  }

  const d = parseDate(date);
  const nowMin = nowMinutesIn(group.timezone);

  return (
    <div className="page">
      <header className="top">
        <div>
          <div className="eyebrow">{group.name}</div>
          <h1 className="display">
            {d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' })}
          </h1>
          <div className="sub">{date === today ? 'Сегодня, ' : ''}{d.toLocaleDateString('ru-RU', { weekday: 'long' })}</div>
        </div>
        {date !== today && <button className="chip" onClick={() => setDate(today)}>Сегодня</button>}
      </header>

      <div className="weekstrip">
        <button className="icon-btn" onClick={() => setDate(addDays(date, -7))} aria-label="Предыдущая неделя"><Icon name="left" /></button>
        {week.map(({ d: wd, has }, i) => (
          <button key={wd} className={'day' + (wd === date ? ' active' : '') + (wd === today ? ' today' : '')}
            onClick={() => { haptic.select(); setDate(wd); }}>
            <span>{WEEKDAYS_SHORT[i]}</span>
            <b>{parseDate(wd).getDate()}</b>
            <i className={has ? 'dot' : 'dot off'} />
          </button>
        ))}
        <button className="icon-btn" onClick={() => setDate(addDays(date, 7))} aria-label="Следующая неделя"><Icon name="right" /></button>
      </div>

      {lessons === null && <><Skeleton h={168} /><Skeleton h={168} /></>}
      {lessons?.length === 0 && <Empty title="Пар нет" text="Можно выдохнуть ✨" />}

      <div className="timeline">
        {lessons?.map((l) => {
          const live = date === today && !l.cancelled && nowMin >= toMinutes(l.start_time) && nowMin < toMinutes(l.end_time);
          const pPct = l.total ? (l.present / l.total) * 100 : 0;
          const aPct = l.total ? (l.absent / l.total) * 100 : 0;
          return (
            <article key={l.id} className={'lesson' + (l.cancelled ? ' cancelled' : '') + (live ? ' live' : '')}>
              <div className="lesson-time">
                <b>{l.start_time}</b>
                <span>{l.end_time}</span>
              </div>
              <div className="lesson-body card">
                <div className="lesson-head">
                  <h2>{l.subject}</h2>
                  <LessonBadge l={l} live={live} />
                </div>
                {(l.room || l.teacher) && (
                  <div className="meta wrap">
                    {l.room && <span className="meta-item"><Icon name="pin" size={14} /> {l.room}</span>}
                    {l.teacher && <span className="meta-item"><Icon name="user" size={14} /> {l.teacher}</span>}
                  </div>
                )}

                {!l.cancelled && (
                  <>
                    <div className="mark">
                      <button className={'mark-btn yes' + (l.my_status === 'present' ? ' on' : '')} disabled={!l.open} onClick={() => mark(l, 'present')}>
                        <Icon name="check" size={18} /> Был
                      </button>
                      <button className={'mark-btn no' + (l.my_status === 'absent' ? ' on' : '')} disabled={!l.open} onClick={() => mark(l, 'absent')}>
                        <Icon name="x" size={18} /> Не был
                      </button>
                    </div>
                    <MarkHint l={l} today={date === today} isHeadman={isHeadman} />
                    <div className="split-bar"><i className="p" style={{ width: pPct + '%' }} /><i className="a" style={{ width: aPct + '%' }} /></div>
                    <div className="meta">
                      <span className="c-ok">{l.present} {plural(l.present, 'был', 'были', 'были')}</span>
                      <span className="c-bad">
                        {l.absent} не {plural(l.absent, 'был', 'были', 'были')}{l.auto_absent > 0 && ` (${l.auto_absent} авто)`}
                      </span>
                      <span>из {l.total}</span>
                    </div>
                  </>
                )}

                {isHeadman && (
                  <div className="lesson-tools">
                    {l.started && !l.cancelled && (
                      <button className="btn ghost sm" onClick={() => setExpanded(expanded === l.id ? null : l.id)}>
                        <Icon name="users" size={16} /> {expanded === l.id ? 'Скрыть' : 'Список'}
                      </button>
                    )}
                    <button className="btn ghost sm" onClick={() => toggleCancel(l)}>
                      <Icon name={l.cancelled ? 'refresh' : 'ban'} size={16} /> {l.cancelled ? 'Вернуть' : 'Отменить'}
                    </button>
                  </div>
                )}
                {expanded === l.id && <Roster lesson={l} onChange={load} />}
              </div>
            </article>
          );
        })}
      </div>
    </div>
  );
}

function LessonBadge({ l, live }: { l: Lesson; live: boolean }) {
  if (l.cancelled) return <span className="badge muted">Отменена</span>;
  if (live) return <span className="badge live"><i />Идёт</span>;
  if (!l.started) return <span className="badge muted">Скоро</span>;
  if (l.open) return <span className="badge accent">До {l.closes_at}</span>;
  return <span className="badge muted">Закрыта</span>;
}

function MarkHint({ l, today, isHeadman }: { l: Lesson; today: boolean; isHeadman: boolean }) {
  let text: string;
  if (!l.started) text = `Отметка откроется в ${l.start_time} на 15 минут`;
  else if (l.open) text = `Отметиться можно до ${l.closes_at}`;
  else if (l.my_auto) text = `Не отметились до ${l.closes_at} — автоматически «не был»`;
  else text = today ? `Отметка закрылась в ${l.closes_at}` : 'Отметка закрыта';
  if (isHeadman && l.started && !l.open) text += ' · исправить можно в «Списке»';
  return <div className={'mark-hint' + (l.open ? ' live' : '')}><Icon name="clock" size={14} /> {text}</div>;
}

function Roster({ lesson, onChange }: { lesson: Lesson; onChange: () => void }) {
  const toast = useToast();
  const [rows, setRows] = useState<AttendanceRow[] | null>(null);
  const load = useCallback(() => {
    api.lessonAttendance(lesson.id, lesson.date).then(setRows, (e) => toast(humanError(e)));
  }, [lesson.id, lesson.date, toast]);
  useEffect(load, [load]);

  async function set(r: AttendanceRow, status: 'present' | 'absent') {
    haptic.tap();
    try {
      // Tapping "absent" on an automatic mark confirms it rather than clearing it.
      await api.mark({ slot_id: lesson.id, date: lesson.date, user_id: r.user_id, status: r.status === status && !r.auto ? 'none' : status });
      load();
      onChange();
    } catch (e) {
      toast(humanError(e));
    }
  }
  async function markAllUnmarked() {
    const pending = rows?.filter((r) => !r.status) ?? [];
    try {
      await Promise.all(pending.map((r) => api.mark({ slot_id: lesson.id, date: lesson.date, user_id: r.user_id, status: 'absent' })));
      haptic.ok();
      load();
      onChange();
    } catch (e) {
      toast(humanError(e));
    }
  }
  if (!rows) return <Skeleton h={120} />;
  const unmarked = rows.filter((r) => !r.status).length;
  return (
    <div className="roster">
      {rows.map((r) => (
        <div key={r.user_id} className="roster-row">
          <Avatar name={r.full_name} id={r.user_id} size={34} />
          <span className="grow">{r.full_name}{r.auto && <span className="auto-tag">авто</span>}</span>
          <button className={'round yes' + (r.status === 'present' ? ' on' : '')} onClick={() => set(r, 'present')} aria-label="Был"><Icon name="check" size={16} /></button>
          <button className={'round no' + (r.status === 'absent' ? ' on' : '')} onClick={() => set(r, 'absent')} aria-label="Не был"><Icon name="x" size={16} /></button>
        </div>
      ))}
      {unmarked > 0 && (
        <button className="btn ghost sm" onClick={markAllUnmarked}>
          Остальным ({unmarked}) — «не был»
        </button>
      )}
    </div>
  );
}
