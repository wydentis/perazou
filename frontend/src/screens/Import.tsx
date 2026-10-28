import { useMemo, useState } from 'react';
import { api } from '../api';
import type { ImportGroup, ImportLesson, Slot } from '../types';
import { WEEKDAYS, plural, rangeLabel } from '../util';
import { confirmDialog, haptic } from '../telegram';
import { Icon } from '../icons';
import { humanError, useToast } from '../ui';
import { Field } from './Onboarding';

type Sub = '' | 'А' | 'Б';
const uniqJoin = (xs: string[]) => [...new Set(xs.filter(Boolean))].join(' / ');

/** Turns a parsed lesson into a slot for the chosen subgroup, or null if it is not theirs. */
function toSlot(l: ImportLesson, sub: Sub): Omit<Slot, 'id'> | null {
  if (sub && l.sub && l.sub !== sub) return null;
  // A lesson for one subgroup uses that subgroup's teacher and room, whatever the view.
  const side = l.sub || sub;
  const teacher = side === 'А' ? l.teacher_a : side === 'Б' ? l.teacher_b : uniqJoin([l.teacher_a, l.teacher_b]);
  const room = side === 'А' ? l.room_a : side === 'Б' ? l.room_b : uniqJoin([l.room_a, l.room_b]);
  return {
    weekday: l.weekday, start_time: l.start_time, end_time: l.end_time,
    subject: !sub && l.sub ? `${l.subject} (подгр. ${l.sub})` : l.subject,
    teacher, room, parity: l.parity, valid_from: l.valid_from, valid_to: l.valid_to,
  };
}

export default function ImportScreen({ groupName, onClose }: { groupName: string; onClose: () => void }) {
  const toast = useToast();
  const [url, setUrl] = useState('');
  const [busy, setBusy] = useState(false);
  const [groups, setGroups] = useState<ImportGroup[] | null>(null);
  const [gi, setGi] = useState(0);
  const [sub, setSub] = useState<Sub>('');
  const [off, setOff] = useState<Set<number>>(new Set());
  const [replace, setReplace] = useState(false);

  async function load(p: Promise<{ groups: ImportGroup[] }>) {
    setBusy(true);
    try {
      const r = await p;
      setGroups(r.groups);
      // Preselect the group whose number matches ours ("ИВТ-4" -> "4 группа").
      const num = groupName.match(/\d+/)?.[0];
      const guess = num ? r.groups.findIndex((g) => g.name.startsWith(num + ' ')) : -1;
      setGi(guess >= 0 ? guess : 0);
      setOff(new Set());
      haptic.ok();
    } catch (e) {
      haptic.err();
      toast(humanError(e));
    } finally {
      setBusy(false);
    }
  }

  const rows = useMemo(() => {
    if (!groups) return [];
    return groups[gi].lessons
      .map((l, i) => ({ i, l, slot: toSlot(l, sub) }))
      .filter((x): x is { i: number; l: ImportLesson; slot: Omit<Slot, 'id'> } => x.slot !== null);
  }, [groups, gi, sub]);
  const selected = rows.filter((r) => !off.has(r.i));
  const warnings = rows.filter((r) => r.l.note).length;

  async function apply() {
    if (replace && !(await confirmDialog('Текущее расписание и вся посещаемость по нему будут удалены. Продолжить?'))) return;
    setBusy(true);
    try {
      const r = await api.importApply(selected.map((x) => x.slot), replace);
      haptic.ok();
      toast(`Добавлено ${r.created} ${plural(r.created, 'пара', 'пары', 'пар')}` + (r.skipped ? `, ${r.skipped} обновлено` : ''), 'ok');
      onClose();
    } catch (e) {
      haptic.err();
      toast(humanError(e));
    } finally {
      setBusy(false);
    }
  }

  const toggle = (i: number) => {
    haptic.select();
    setOff((s) => {
      const n = new Set(s);
      if (n.has(i)) n.delete(i);
      else n.add(i);
      return n;
    });
  };

  return (
    <div className="page">
      <header className="top">
        <button className="icon-btn" onClick={groups ? () => setGroups(null) : onClose} aria-label="Назад"><Icon name="left" /></button>
        <h1 className="display sm grow">Импорт</h1>
      </header>

      {!groups ? (
        <>
          <p className="lead">Загрузите расписание факультета — мы найдём в нём вашу группу.</p>
          <div className="card import-source">
            <Field label="Ссылка на Google Таблицу">
              <input value={url} onChange={(e) => setUrl(e.target.value.trim())} placeholder="https://docs.google.com/spreadsheets/d/…" inputMode="url" />
            </Field>
            <button className="btn primary" disabled={!url || busy} onClick={() => load(api.importFromUrl(url))}>
              <Icon name="link" size={18} /> {busy ? 'Читаем…' : 'Загрузить по ссылке'}
            </button>
            <div className="or">или</div>
            <label className={'btn ghost file-btn' + (busy ? ' disabled' : '')}>
              <Icon name="upload" size={18} /> Выбрать .xlsx
              <input type="file" accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" disabled={busy}
                onChange={(e) => { const f = e.target.files?.[0]; if (f) load(api.importFromFile(f)); e.target.value = ''; }} />
            </label>
          </div>
          <p className="hint">По ссылке файл должен быть открыт для всех, у кого есть ссылка. Иначе скачайте его и загрузите вручную.</p>
        </>
      ) : (
        <>
          <Field label="Группа в файле">
            <div className="group-chips">
              {groups.map((g, i) => (
                <button key={g.name} className={i === gi ? 'active' : ''} onClick={() => { haptic.select(); setGi(i); setOff(new Set()); }}>{g.name}</button>
              ))}
            </div>
          </Field>
          <Field label="Подгруппа">
            <div className="segmented">
              {(['', 'А', 'Б'] as Sub[]).map((s) => (
                <button key={s || 'all'} className={sub === s ? 'active' : ''} onClick={() => { setSub(s); setOff(new Set()); }}>
                  {s ? `Подгруппа ${s}` : 'Вся группа'}
                </button>
              ))}
            </div>
          </Field>

          <div className="summary">
            <span>{selected.length} из {rows.length} {plural(rows.length, 'пары', 'пар', 'пар')}</span>
            {warnings > 0 && <span className="warn">{warnings} — проверьте пометки</span>}
          </div>

          {WEEKDAYS.map((name, wd) => {
            const day = rows.filter((r) => r.slot.weekday === wd + 1);
            if (!day.length) return null;
            return (
              <section key={name} className="day-section">
                <h3 className="section-title">{name}</h3>
                <div className="card list">
                  {day.map(({ i, l, slot }) => (
                    <div key={i} className={'list-row import-row ' + (off.has(i) ? 'off' : 'on')} onClick={() => toggle(i)}>
                      <div className="check"><Icon name="check" size={14} strokeWidth={3} /></div>
                      <div className="slot-time"><b>{slot.start_time}</b><span>{slot.end_time}</span></div>
                      <div className="grow">
                        <div className="row-title">{slot.subject}</div>
                        <div className="meta wrap">
                          {slot.room && <span className="meta-item"><Icon name="pin" size={13} /> {slot.room}</span>}
                          {slot.teacher && <span className="meta-item"><Icon name="user" size={13} /> {slot.teacher}</span>}
                        </div>
                        {(slot.valid_from || slot.valid_to || slot.parity > 0) && (
                          <div className="meta" style={{ marginTop: 6 }}>
                            {(slot.valid_from || slot.valid_to) && <span className="badge violet">{rangeLabel(slot.valid_from, slot.valid_to)}</span>}
                            {slot.parity > 0 && <span className="badge pink">{slot.parity === 1 ? 'нечёт' : 'чёт'}</span>}
                          </div>
                        )}
                        {l.note && <div className="note"><Icon name="alert" size={14} /> {l.note}</div>}
                      </div>
                    </div>
                  ))}
                </div>
              </section>
            );
          })}

          <div className="sticky-bottom">
            <div className="segmented">
              <button className={!replace ? 'active' : ''} onClick={() => setReplace(false)}>Добавить</button>
              <button className={replace ? 'active' : ''} onClick={() => setReplace(true)}>Заменить текущее</button>
            </div>
            <button className="btn primary xl" disabled={!selected.length || busy} onClick={apply}>
              {busy ? 'Сохраняем…' : `Импортировать ${selected.length} ${plural(selected.length, 'пару', 'пары', 'пар')}`}
            </button>
          </div>
        </>
      )}
    </div>
  );
}
