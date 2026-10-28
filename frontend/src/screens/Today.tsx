import { useCallback, useEffect, useState } from 'react';
import { api } from '../api';
import type { AttendanceRow, Lesson, Me } from '../types';
import { addDays, humanDate, todayIn } from '../util';
import { haptic } from '../telegram';

export default function Today({ me }: { me: Me }) {
  const isHeadman = me.member!.role === 'headman';
  const today = todayIn(me.group!.timezone);
  const [date, setDate] = useState(today);
  const [lessons, setLessons] = useState<Lesson[] | null>(null);
  const [err, setErr] = useState('');
  const [expanded, setExpanded] = useState<number | null>(null);

  const load = useCallback(() => {
    api.lessons(date).then(setLessons, (e: Error) => setErr(e.message));
  }, [date]);
  useEffect(() => {
    setLessons(null);
    setExpanded(null);
    load();
  }, [load]);

  async function mark(l: Lesson, status: 'present' | 'absent') {
    setErr('');
    try {
      await api.mark({ slot_id: l.id, date: l.date, status: l.my_status === status ? 'none' : status });
      haptic.ok();
      load();
    } catch (e) {
      haptic.err();
      setErr((e as Error).message);
      load();
    }
  }

  return (
    <div className="page">
      <div className="datebar">
        <button onClick={() => setDate(addDays(date, -1))}>‹</button>
        <div>
          <b>{humanDate(date)}</b>
          {date !== today && <a onClick={() => setDate(today)}>Сегодня</a>}
        </div>
        <button onClick={() => setDate(addDays(date, 1))}>›</button>
      </div>
      {err && <p className="err">{err}</p>}
      {lessons === null && <p className="muted center">Загрузка…</p>}
      {lessons?.length === 0 && <p className="muted center">В этот день пар нет</p>}
      {lessons?.map((l) => (
        <div key={l.id} className={'card' + (l.cancelled ? ' cancelled' : '')}>
          <div className="row">
            <div className="time">{l.start_time}<br /><span className="muted">{l.end_time}</span></div>
            <div className="grow">
              <b>{l.subject}</b>
              <div className="muted small">{l.room && `ауд. ${l.room} · `}{l.cancelled ? 'отменена' : `${l.present} был(и) · ${l.absent} не был(и) · из ${l.total}`}</div>
            </div>
          </div>
          {!l.cancelled && (
            <div className="actions">
              <button className={'yes' + (l.my_status === 'present' ? ' on' : '')} disabled={!l.open} onClick={() => mark(l, 'present')}>✅ Был</button>
              <button className={'no' + (l.my_status === 'absent' ? ' on' : '')} disabled={!l.open} onClick={() => mark(l, 'absent')}>❌ Не был</button>
            </div>
          )}
          {!l.cancelled && !l.open && <div className="muted small">{l.started ? 'Отметка закрыта' : 'Отметка откроется с началом пары'}</div>}
          {isHeadman && (
            <div className="actions">
              {l.started && !l.cancelled && <button onClick={() => setExpanded(expanded === l.id ? null : l.id)}>{expanded === l.id ? 'Скрыть список' : 'Отметить студентов'}</button>}
              <button onClick={async () => { await api.cancelLesson(l.id, l.date, !l.cancelled); load(); }}>{l.cancelled ? 'Вернуть пару' : 'Отменить пару'}</button>
            </div>
          )}
          {expanded === l.id && <Roster lesson={l} onChange={load} />}
        </div>
      ))}
    </div>
  );
}

function Roster({ lesson, onChange }: { lesson: Lesson; onChange: () => void }) {
  const [rows, setRows] = useState<AttendanceRow[]>([]);
  const [err, setErr] = useState('');
  const load = useCallback(() => {
    api.lessonAttendance(lesson.id, lesson.date).then(setRows, (e: Error) => setErr(e.message));
  }, [lesson.id, lesson.date]);
  useEffect(load, [load]);

  async function set(r: AttendanceRow, status: 'present' | 'absent') {
    try {
      await api.mark({ slot_id: lesson.id, date: lesson.date, user_id: r.user_id, status: r.status === status ? 'none' : status });
      load();
      onChange();
    } catch (e) {
      setErr((e as Error).message);
    }
  }
  return (
    <div className="roster">
      {err && <p className="err">{err}</p>}
      {rows.map((r) => (
        <div key={r.user_id} className="row roster-row">
          <span className="grow">{r.full_name}</span>
          <button className={'mini yes' + (r.status === 'present' ? ' on' : '')} onClick={() => set(r, 'present')}>✅</button>
          <button className={'mini no' + (r.status === 'absent' ? ' on' : '')} onClick={() => set(r, 'absent')}>❌</button>
        </div>
      ))}
    </div>
  );
}
