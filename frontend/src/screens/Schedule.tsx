import { useCallback, useEffect, useState } from 'react';
import { api } from '../api';
import type { Group, Slot } from '../types';
import { PARITY, WEEKDAYS } from '../util';
import { confirmDialog } from '../telegram';

const EMPTY: Omit<Slot, 'id'> = { weekday: 1, start_time: '09:00', end_time: '10:30', subject: '', room: '', parity: 0 };

export default function Schedule({ group, isHeadman }: { group: Group; isHeadman: boolean }) {
  const [slots, setSlots] = useState<Slot[] | null>(null);
  const [editing, setEditing] = useState<{ id?: number; slot: Omit<Slot, 'id'> } | null>(null);
  const [err, setErr] = useState('');
  const load = useCallback(() => {
    api.schedule().then(setSlots, (e: Error) => setErr(e.message));
  }, []);
  useEffect(load, [load]);

  if (editing) {
    return <SlotForm initial={editing.slot} id={editing.id} onClose={() => { setEditing(null); load(); }} />;
  }
  return (
    <div className="page">
      <h1>Расписание</h1>
      <p className="muted small">Сейчас идёт {group.current_week === 'odd' ? 'нечётная' : 'чётная'} неделя</p>
      {err && <p className="err">{err}</p>}
      {isHeadman && <button className="primary" onClick={() => setEditing({ slot: EMPTY })}>+ Добавить пару</button>}
      {slots?.length === 0 && <p className="muted center">Расписание пусто</p>}
      {WEEKDAYS.map((name, i) => {
        const day = slots?.filter((s) => s.weekday === i + 1) ?? [];
        if (!day.length) return null;
        return (
          <section key={name}>
            <h3>{name}</h3>
            {day.map((s) => (
              <div key={s.id} className="card row" onClick={() => isHeadman && setEditing({ id: s.id, slot: s })}>
                <div className="time">{s.start_time}<br /><span className="muted">{s.end_time}</span></div>
                <div className="grow">
                  <b>{s.subject}</b>
                  <div className="muted small">{[s.room && `ауд. ${s.room}`, s.parity ? PARITY[s.parity] : ''].filter(Boolean).join(' · ')}</div>
                </div>
              </div>
            ))}
          </section>
        );
      })}
    </div>
  );
}

function SlotForm({ initial, id, onClose }: { initial: Omit<Slot, 'id'>; id?: number; onClose: () => void }) {
  const [s, setS] = useState(initial);
  const [err, setErr] = useState('');
  const set = <K extends keyof typeof s>(k: K, v: (typeof s)[K]) => setS({ ...s, [k]: v });

  async function save() {
    try {
      await api.saveSlot(s, id);
      onClose();
    } catch (e) {
      setErr((e as Error).message);
    }
  }
  async function remove() {
    if (!(await confirmDialog('Удалить пару вместе со всей историей посещаемости по ней?'))) return;
    try {
      await api.deleteSlot(id!);
      onClose();
    } catch (e) {
      setErr((e as Error).message);
    }
  }
  return (
    <div className="page">
      <h1>{id ? 'Редактировать пару' : 'Новая пара'}</h1>
      <label>Предмет</label>
      <input value={s.subject} onChange={(e) => set('subject', e.target.value)} maxLength={100} />
      <label>День недели</label>
      <select value={s.weekday} onChange={(e) => set('weekday', Number(e.target.value))}>
        {WEEKDAYS.map((n, i) => <option key={n} value={i + 1}>{n}</option>)}
      </select>
      <div className="row">
        <div className="grow"><label>Начало</label><input type="time" value={s.start_time} onChange={(e) => set('start_time', e.target.value)} /></div>
        <div className="grow"><label>Конец</label><input type="time" value={s.end_time} onChange={(e) => set('end_time', e.target.value)} /></div>
      </div>
      <label>Аудитория</label>
      <input value={s.room} onChange={(e) => set('room', e.target.value)} maxLength={30} />
      <label>Повторяется</label>
      <select value={s.parity} onChange={(e) => set('parity', Number(e.target.value) as 0 | 1 | 2)}>
        {PARITY.map((n, i) => <option key={n} value={i}>{n}</option>)}
      </select>
      {err && <p className="err">{err}</p>}
      <button className="primary" disabled={!s.subject.trim()} onClick={save}>Сохранить</button>
      {id && <button className="danger" onClick={remove}>Удалить</button>}
      <button onClick={onClose}>Назад</button>
    </div>
  );
}
