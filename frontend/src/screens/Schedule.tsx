import { useCallback, useEffect, useState } from 'react';
import { api } from '../api';
import type { Group, Slot } from '../types';
import { WEEKDAYS, WEEKDAYS_SHORT, rangeLabel } from '../util';
import ImportScreen from './Import';
import { confirmDialog, haptic } from '../telegram';
import { Icon } from '../icons';
import { Empty, Skeleton, humanError, useToast } from '../ui';
import { Field } from './Onboarding';

const EMPTY: Omit<Slot, 'id'> = { weekday: 1, start_time: '09:00', end_time: '10:30', subject: '', room: '', teacher: '', valid_from: '', valid_to: '' };

export default function Schedule({ group, isHeadman }: { group: Group; isHeadman: boolean }) {
  const toast = useToast();
  const [slots, setSlots] = useState<Slot[] | null>(null);
  const [editing, setEditing] = useState<{ id?: number; slot: Omit<Slot, 'id'> } | null>(null);
  const [importing, setImporting] = useState(false);
  const load = useCallback(() => {
    api.schedule().then(setSlots, (e) => toast(humanError(e)));
  }, [toast]);
  useEffect(load, [load]);

  if (importing) {
    return <ImportScreen groupName={group.name} onClose={() => { setImporting(false); load(); }} />;
  }
  if (editing) {
    return <SlotForm initial={editing.slot} id={editing.id} onClose={() => { setEditing(null); load(); }} />;
  }
  return (
    <div className="page">
      <header className="top">
        <div>
          <div className="eyebrow">{group.name}</div>
          <h1 className="display">Расписание</h1>
        </div>
        {isHeadman && (
          <button className="fab" onClick={() => { haptic.tap(); setEditing({ slot: EMPTY }); }} aria-label="Добавить пару">
            <Icon name="plus" size={22} />
          </button>
        )}
      </header>

      {slots === null && <><Skeleton h={80} /><Skeleton h={80} /></>}
      {isHeadman && (
        <button className="btn import-cta" onClick={() => setImporting(true)}>
          <Icon name="upload" size={18} /> Импорт из таблицы факультета
        </button>
      )}
      {slots?.length === 0 && (
        <Empty title="Пока пусто" text={isHeadman ? 'Импортируйте расписание или добавьте пару кнопкой «+»' : 'Староста ещё не заполнил расписание'} />
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
                    {(s.room || s.teacher) && (
                      <div className="meta wrap">
                        {s.room && <span className="meta-item"><Icon name="pin" size={13} /> {s.room}</span>}
                        {s.teacher && <span className="meta-item"><Icon name="user" size={13} /> {s.teacher}</span>}
                      </div>
                    )}
                  </div>
                  <div className="badges">
                    {(s.valid_from || s.valid_to) && <span className="badge violet">{rangeLabel(s.valid_from, s.valid_to)}</span>}
                  </div>
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
              <button key={n} className={s.weekday === i + 1 ? 'active' : ''} onClick={() => set('weekday', i + 1)}>{WEEKDAYS_SHORT[i]}</button>
            ))}
          </div>
        </Field>
        <div className="row2">
          <Field label="Начало"><input type="time" value={s.start_time} onChange={(e) => set('start_time', e.target.value)} /></Field>
          <Field label="Конец"><input type="time" value={s.end_time} onChange={(e) => set('end_time', e.target.value)} /></Field>
        </div>
        <Field label="Аудитория">
          <input value={s.room} onChange={(e) => set('room', e.target.value)} maxLength={60} placeholder="301" />
        </Field>
        <Field label="Преподаватель">
          <input value={s.teacher} onChange={(e) => set('teacher', e.target.value)} maxLength={120} placeholder="Иванов И.И." />
        </Field>
        <div className="row2">
          <Field label="Действует с"><input type="date" value={s.valid_from} onChange={(e) => set('valid_from', e.target.value)} /></Field>
          <Field label="по"><input type="date" value={s.valid_to} onChange={(e) => set('valid_to', e.target.value)} /></Field>
        </div>
      </div>
      <button className="btn primary xl"
        disabled={!s.subject.trim() || s.end_time <= s.start_time || (!!s.valid_from && !!s.valid_to && s.valid_to < s.valid_from)}
        onClick={save}>Сохранить</button>
      {id && <button className="btn danger" onClick={remove}><Icon name="x" size={18} /> Удалить пару</button>}
    </div>
  );
}
