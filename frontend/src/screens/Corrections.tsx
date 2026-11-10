import { useCallback, useEffect, useState } from 'react';
import { api } from '../api';
import type { CorrectableLesson, CorrectionEntry, Me } from '../types';
import { Icon } from '../icons';
import { Empty, Skeleton, humanError, useToast } from '../ui';
import { haptic } from '../telegram';
import { humanDate, shortDate } from '../util';

const STATUS: Record<string, string> = { present: 'был', absent: 'не был', auto: 'н (авто)' };

export default function CorrectionsScreen({ me, onBack, onChanged }: { me: Me; onBack: () => void; onChanged: () => void }) {
  const isHeadman = me.member!.role === 'headman';
  const toast = useToast();
  const [open, setOpen] = useState(me.group!.corrections_open);
  const [lessons, setLessons] = useState<CorrectableLesson[] | null>(null);
  const [log, setLog] = useState<CorrectionEntry[] | null>(null);
  const [view, setView] = useState<'mine' | 'log'>('mine');

  const load = useCallback(() => {
    api.myCorrectableLessons().then((r) => { setLessons(r.lessons); setOpen(r.corrections_open); }, (e) => toast(humanError(e)));
    if (isHeadman) api.correctionsLog().then(setLog, (e) => toast(humanError(e)));
  }, [isHeadman, toast]);
  useEffect(load, [load]);

  async function toggle(next: boolean) {
    try {
      await api.setCorrectionsOpen(next);
      setOpen(next);
      haptic.ok();
      toast(next ? 'Исправления включены' : 'Исправления выключены', 'ok');
      onChanged();
    } catch (e) {
      toast(humanError(e));
    }
  }

  async function mark(l: CorrectableLesson, status: 'present' | 'absent') {
    if (l.status === status && !l.auto) return;
    haptic.tap();
    try {
      await api.mark({ slot_id: l.slot_id, date: l.date, status });
      haptic.ok();
      load();
    } catch (e) {
      haptic.err();
      toast(humanError(e));
    }
  }

  const canEdit = isHeadman || open;
  const days = lessons ? [...new Set(lessons.map((l) => l.date))] : [];

  return (
    <div className="page">
      <header className="top">
        <button className="icon-btn" onClick={onBack} aria-label="Назад"><Icon name="left" /></button>
        <div className="grow">
          <div className="eyebrow">{me.group!.name}</div>
          <h1 className="display sm">Исправления</h1>
        </div>
      </header>

      {isHeadman ? (
        <div className="card form">
          <div className="row-title">Студенты могут исправлять свои отметки</div>
          <div className="segmented">
            <button className={!open ? 'active' : ''} onClick={() => open && toggle(false)}>Выключено</button>
            <button className={open ? 'active' : ''} onClick={() => !open && toggle(true)}>Включено</button>
          </div>
          <p className="hint">Пока включено, каждый может поменять «был / не был» за прошедшие пары. Все изменения видны в журнале.</p>
        </div>
      ) : (
        <div className={'corr-banner' + (open ? ' on' : '')}>
          <Icon name={open ? 'edit' : 'ban'} size={18} />
          <span>{open ? 'Староста включил исправления: можно поправить свои отметки за прошлые пары.' : 'Сейчас исправления выключены старостой.'}</span>
        </div>
      )}

      {isHeadman && (
        <div className="segmented">
          <button className={view === 'mine' ? 'active' : ''} onClick={() => setView('mine')}>Мои пары</button>
          <button className={view === 'log' ? 'active' : ''} onClick={() => setView('log')}>
            Журнал{log && log.length > 0 ? ` · ${log.length}` : ''}
          </button>
        </div>
      )}

      {view === 'mine' && (
        lessons === null ? <Skeleton h={160} /> : lessons.length === 0 ? <Empty title="Прошедших пар нет" /> : days.map((d) => (
          <section key={d} className="day-section">
            <h3 className="section-title">{humanDate(d)}</h3>
            <div className="card list">
              {lessons.filter((l) => l.date === d).map((l) => (
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
                  <button className={'round yes' + (l.status === 'present' ? ' on' : '')} disabled={!canEdit} onClick={() => mark(l, 'present')} aria-label="Был"><Icon name="check" size={16} /></button>
                  <button className={'round no' + (l.status === 'absent' && !l.auto ? ' on' : '')} disabled={!canEdit} onClick={() => mark(l, 'absent')} aria-label="Не был"><Icon name="x" size={16} /></button>
                </div>
              ))}
            </div>
          </section>
        ))
      )}

      {view === 'log' && isHeadman && (
        log === null ? <Skeleton h={160} /> : log.length === 0 ? <Empty title="Никто ничего не менял" /> : (
          <div className="card list">
            {log.map((e) => (
              <div key={e.id} className="list-row">
                <div className="grow">
                  <div className="row-title">{e.full_name}</div>
                  <div className="meta wrap">
                    <span>{e.subject} · {shortDate(e.date)} {e.start_time}</span>
                    <span className="c-bad">{STATUS[e.old_status]}</span>
                    <span>→</span>
                    <span className={e.new_status === 'present' ? 'c-ok' : 'c-bad'}>{STATUS[e.new_status]}</span>
                  </div>
                </div>
                <span className="hint">{new Date(e.changed_at).toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}</span>
              </div>
            ))}
          </div>
        )
      )}
    </div>
  );
}
