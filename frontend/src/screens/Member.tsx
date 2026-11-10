import { createContext, useCallback, useContext, useEffect, useState } from 'react';
import { api } from '../api';
import type { CorrectableLesson, Me, Member } from '../types';
import { Icon } from '../icons';
import { Empty, Ring, Skeleton, humanError, useToast } from '../ui';
import { haptic, useTelegramBack } from '../telegram';
import { plural } from '../util';
import { LessonWeeks } from './LessonWeeks';

/** Opens a member's attendance page from anywhere (lesson roster, group list, stats, log). */
// `version` grows whenever marks or settings change on a member page, so the screens kept
// mounted underneath can reload what they show.
export const OpenMemberContext = createContext<{ open: (userId: number) => void; version: number }>({ open: () => {}, version: 0 });
export const useOpenMember = () => useContext(OpenMemberContext).open;
export const useMemberDataVersion = () => useContext(OpenMemberContext).version;

export default function MemberScreen({ me, userId, onBack, onChanged }: {
  me: Me;
  userId: number;
  onBack: () => void;
  onChanged: () => void;
}) {
  const isHeadman = me.member!.role === 'headman';
  const toast = useToast();
  const [member, setMember] = useState<Member | null>(null);
  const [lessons, setLessons] = useState<CorrectableLesson[] | null>(null);
  const [subject, setSubject] = useState<string | null>(null);

  const load = useCallback(() => {
    api.memberAttendance(userId).then((r) => { setMember(r.member); setLessons(r.lessons); }, (e) => toast(humanError(e)));
  }, [userId, toast]);
  useEffect(load, [load]);
  useTelegramBack(onBack);

  async function mark(l: CorrectableLesson, status: 'present' | 'absent') {
    if (l.status === status && !l.auto) return;
    haptic.tap();
    try {
      await api.mark({ slot_id: l.slot_id, date: l.date, status, user_id: userId });
      haptic.ok();
      load();
      onChanged();
    } catch (e) {
      haptic.err();
      toast(humanError(e));
    }
  }

  async function setBlocked(blocked: boolean) {
    try {
      await api.setSelfMarkBlocked(userId, blocked);
      haptic.ok();
      toast(blocked ? 'Теперь отмечает только староста' : 'Студент снова отмечается сам', 'ok');
      load();
      onChanged();
    } catch (e) {
      toast(humanError(e));
    }
  }

  // Counted lessons are those with a result: marked, or automatically absent.
  const counted = (lessons ?? []).filter((l) => l.status);
  const present = counted.filter((l) => l.status === 'present').length;
  const absent = counted.length - present;
  const auto = counted.filter((l) => l.auto).length;
  const pct = counted.length ? present / counted.length : 0;
  const bySubject = new Map<string, { p: number; t: number }>();
  for (const l of counted) {
    const s = bySubject.get(l.subject) ?? { p: 0, t: 0 };
    s.t++;
    if (l.status === 'present') s.p++;
    bySubject.set(l.subject, s);
  }
  const subjects = [...bySubject.entries()].sort((a, b) => a[1].p / a[1].t - b[1].p / b[1].t);
  const shown = subject ? (lessons ?? []).filter((l) => l.subject === subject) : lessons ?? [];
  const isSelf = userId === me.member!.user_id;

  return (
    <div className="page">
      <header className="top">
        <button className="icon-btn" onClick={onBack} aria-label="Назад"><Icon name="left" /></button>
        <div className="grow">
          <div className="eyebrow">{member ? (member.role === 'headman' ? 'Староста' : member.username ? '@' + member.username : 'Студент') : me.group!.name}</div>
          <h1 className="display sm">{member?.full_name ?? '…'}</h1>
        </div>
      </header>

      {!lessons ? <Skeleton h={150} /> : (
        <div className="hero-card">
          <Ring value={pct} size={104} stroke={10}><b>{Math.round(pct * 100)}%</b></Ring>
          <div>
            <div className="hero-k">{isSelf ? 'Ваша посещаемость' : 'Посещаемость'}</div>
            <div className="hero-v">{counted.length} {plural(counted.length, 'пара', 'пары', 'пар')}</div>
            <div className="hero-chips">
              <span>✓ {present}</span><span>✕ {absent}</span>{auto > 0 && <span>авто {auto}</span>}
            </div>
          </div>
        </div>
      )}

      {isHeadman && member && member.role !== 'headman' && (
        <div className="card form">
          <div className="row-title">Кто отмечает</div>
          <div className="segmented">
            <button className={!member.self_mark_blocked ? 'active' : ''} onClick={() => member.self_mark_blocked && setBlocked(false)}>Сам</button>
            <button className={member.self_mark_blocked ? 'active' : ''} onClick={() => !member.self_mark_blocked && setBlocked(true)}>Только староста</button>
          </div>
          <p className="hint">«Только староста» — студент не сможет отмечаться ни в приложении, ни кнопками в боте, и не получит напоминаний.</p>
        </div>
      )}
      {!isHeadman && member?.self_mark_blocked && (
        <div className="corr-banner"><Icon name="ban" size={18} /><span>Вас отмечает староста.</span></div>
      )}

      {subjects.length > 1 && (
        <section>
          <h3 className="section-title">По предметам</h3>
          <div className="card list">
            {subjects.map(([name, s]) => (
              <button key={name} className={'list-row subject-row' + (subject === name ? ' active' : '')}
                onClick={() => { haptic.select(); setSubject(subject === name ? null : name); }}>
                <div className="grow">
                  <div className="row-title">{name}</div>
                  <div className="meta"><span className="c-ok">✓ {s.p}</span><span className="c-bad">✕ {s.t - s.p}</span></div>
                </div>
                <Ring value={s.p / s.t} size={40} stroke={5}><small>{Math.round((s.p / s.t) * 100)}</small></Ring>
              </button>
            ))}
          </div>
        </section>
      )}

      {lessons && (
        <>
          <h3 className="section-title">
            {subject ? <>Пары: {subject} <button className="chip sm" onClick={() => setSubject(null)}>все</button></> : 'Все пары'}
          </h3>
          {shown.length === 0 ? <Empty title="Прошедших пар нет" /> : <LessonWeeks lessons={shown} canEdit={isHeadman} readOnly={!isHeadman} onMark={mark} />}
        </>
      )}
    </div>
  );
}
