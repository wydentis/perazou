import { useCallback, useEffect, useState } from 'react';
import { api } from '../api';
import type { Group, Slot } from '../types';
import { PARITY, WEEKDAYS } from '../util';
import { confirmDialog, haptic } from '../telegram';
import { Icon } from '../icons';
import { Empty, Skeleton, humanError, useToast } from '../ui';
import { Field } from './Onboarding';

const EMPTY: Omit<Slot, 'id'> = { weekday: 1, start_time: '09:00', end_time: '10:30', subject: '', room: '', parity: 0 };
const PARITY_SHORT = ['', 'нечёт', 'чёт'];

export default function Schedule({ group, isHeadman }: { group: Group; isHeadman: boolean }) {
  const toast = useToast();
  const [slots, setSlots] = useState<Slot[] | null>(null);
  const [editing, setEditing] = useState<{ id?: number; slot: Omit<Slot, 'id'> } | null>(null);
  const load = useCallback(() => {
    api.schedule().then(setSlots, (e) => toast(humanError(e)));
  }, [toast]);
  useEffect(load, [load]);

  if (editing) {
    return <SlotForm initial={editing.slot} id={editing.id} onClose={() => { setEditing(null); load(); }} />;
  }
  return (
    <div className="page">
      <header className="top">
        <div>
          <div className="eyebrow">Сейчас {group.current_week === 'odd' ? 'нечётная' : 'чётная'} неделя</div>
          <h1 className="display">Расписание</h1>
        </div>
        {isHeadman && (
          <button className="fab" onClick={() => { haptic.tap(); setEditing({ slot: EMPTY }); }} aria-label="Добавить пару">
            <Icon name="plus" size={22} />
          </button>
        )}
      </header>

      {slots === null && <><Skeleton h={80} /><Skeleton h={80} /></>}
      {slots?.length === 0 && (
        <Empty title="Пока пусто" text={isHeadman ? 'Добавьте первую пару кнопкой «+»' : 'Староста ещё не заполнил расписание'} />
      )}
      {WEEKDAYS.map((name, i) => {
        const day = slots?.filter((s) => s.weekday === i + 1) ?? [];
        if (!day.length) return null;
        return (
          <section key={name} className="day-section">
            <h3 className="section-title">{name}</h3>
            <div className="card list">
              {day.map((s) => (
                <button key={s.id} className="list-row" disabled={!isHeadman} onClick={() => setEditing({ id: s.id, slot: s })}>
                  <div className="slot-time"><b>{s.start_time}</b><span>{s.end_time}</span></div>
                  <div className="grow">
                    <div className="row-title">{s.subject}</div>
                    {s.room && <div className="meta"><Icon name="pin" size={13} /> {s.room}</div>}
                  </div>
                  {s.parity > 0 && <span className={'badge ' + (s.parity === 1 ? 'violet' : 'pink')}>{PARITY_SHORT[s.parity]}</span>}
                  {isHeadman && <Icon name="right" size={16} className="chev" />}
                </button>
              ))}
            </div>
          </section>
        );
      })}
    </div>
  );
}

function SlotForm({ initial, id, onClose }: { initial: Omit<Slot, 'id'>; id?: number; onClose: () => void }) {
  const toast = useToast();
  const [s, setS] = useState(initial);
  const set = <K extends keyof typeof s>(k: K, v: (typeof s)[K]) => setS({ ...s, [k]: v });

  async function save() {
    try {
      await api.saveSlot(s, id);
      haptic.ok();
      toast('Сохранено', 'ok');
      onClose();
    } catch (e) {
      haptic.err();
      toast(humanError(e));
    }
  }
  async function remove() {
    if (!(await confirmDialog('Удалить пару вместе со всей историей посещаемости по ней?'))) return;
    try {
      await api.deleteSlot(id!);
      onClose();
    } catch (e) {
      toast(humanError(e));
    }
  }
  return (
    <div className="page">
      <header className="top">
        <button className="icon-btn" onClick={onClose} aria-label="Назад"><Icon name="left" /></button>
        <h1 className="display sm grow">{id ? 'Пара' : 'Новая пара'}</h1>
      </header>
      <div className="card form">
        <Field label="Предмет">
          <input value={s.subject} onChange={(e) => set('subject', e.target.value)} maxLength={100} placeholder="Математический анализ" />
        </Field>
        <Field label="День недели">
          <div className="weekday-picker">
            {WEEKDAYS.map((n, i) => (
              <button key={n} className={s.weekday === i + 1 ? 'active' : ''} onClick={() => set('weekday', i + 1)}>{n.slice(0, 2)}</button>
            ))}
          </div>
        </Field>
        <div className="row2">
          <Field label="Начало"><input type="time" value={s.start_time} onChange={(e) => set('start_time', e.target.value)} /></Field>
          <Field label="Конец"><input type="time" value={s.end_time} onChange={(e) => set('end_time', e.target.value)} /></Field>
        </div>
        <Field label="Аудитория">
          <input value={s.room} onChange={(e) => set('room', e.target.value)} maxLength={30} placeholder="301" />
        </Field>
        <Field label="Повторяется">
          <div className="segmented">
            {PARITY.map((n, i) => (
              <button key={n} className={s.parity === i ? 'active' : ''} onClick={() => set('parity', i as 0 | 1 | 2)}>
                {['Всегда', 'Нечёт', 'Чёт'][i]}
              </button>
            ))}
          </div>
        </Field>
      </div>
      <button className="btn primary xl" disabled={!s.subject.trim() || s.end_time <= s.start_time} onClick={save}>Сохранить</button>
      {id && <button className="btn danger" onClick={remove}><Icon name="x" size={18} /> Удалить пару</button>}
    </div>
  );
}
