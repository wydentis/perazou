import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Me } from '../types';
import { Icon } from '../icons';
import { haptic } from '../telegram';
import { humanError } from '../ui';

export default function Onboarding({ me, initialCode, onDone }: { me: Me; initialCode: string; onDone: () => void }) {
  const [mode, setMode] = useState<'join' | 'create'>(initialCode ? 'join' : 'create');
  const [code, setCode] = useState(initialCode);
  const [groupName, setGroupName] = useState<string | null>(null);
  const [fullName, setFullName] = useState('');
  const [name, setName] = useState('');
  const [week, setWeek] = useState<'odd' | 'even'>('odd');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || 'Europe/Moscow';

  useEffect(() => {
    setGroupName(null);
    if (mode !== 'join' || code.length < 8) return;
    api.preview(code).then((r) => setGroupName(r.name), () => setGroupName(null));
  }, [code, mode]);

  async function submit() {
    setBusy(true);
    setErr('');
    try {
      if (mode === 'join') await api.join({ code, full_name: fullName });
      else await api.createGroup({ name, full_name: fullName, timezone: tz, current_week: week });
      haptic.ok();
      onDone();
    } catch (e) {
      haptic.err();
      setErr(humanError(e));
    } finally {
      setBusy(false);
    }
  }

  const canSubmit = fullName.trim() && (mode === 'join' ? groupName : name.trim());
  return (
    <div className="onboarding">
      <div className="hero-art">
        <div className="logo big"><Icon name="check" size={44} strokeWidth={3} /></div>
        <div className="ticker" aria-hidden><span>БЫЛ · НЕ БЫЛ · БЫЛ · НЕ БЫЛ · БЫЛ · НЕ БЫЛ · БЫЛ · НЕ БЫЛ ·&nbsp;</span><span>БЫЛ · НЕ БЫЛ · БЫЛ · НЕ БЫЛ · БЫЛ · НЕ БЫЛ · БЫЛ · НЕ БЫЛ ·&nbsp;</span></div>
      </div>
      <div className="page">
        <h1 className="display">Привет,<br />{me.user.first_name}</h1>
        <p className="lead">Отмечайте посещаемость пар в&nbsp;пару касаний — всей группой.</p>

        <div className="role-cards">
          <button className={'role ' + (mode === 'join' ? 'active' : '')} onClick={() => { haptic.select(); setMode('join'); }}>
            <Icon name="users" size={22} />
            <b>Я студент</b>
            <span>Вступить по коду</span>
          </button>
          <button className={'role ' + (mode === 'create' ? 'active' : '')} onClick={() => { haptic.select(); setMode('create'); }}>
            <Icon name="crown" size={22} />
            <b>Я староста</b>
            <span>Создать группу</span>
          </button>
        </div>

        <div className="form card">
          {mode === 'join' ? (
            <>
              <Field label="Код приглашения">
                <input className="code-input" value={code} onChange={(e) => setCode(e.target.value.toUpperCase().trim())} maxLength={8} placeholder="ABCD2345" />
              </Field>
              {code.length >= 8 && (groupName
                ? <div className="pill ok"><Icon name="check" size={14} /> {groupName}</div>
                : <div className="pill bad"><Icon name="x" size={14} /> Группа не найдена</div>)}
            </>
          ) : (
            <>
              <Field label="Название группы">
                <input value={name} onChange={(e) => setName(e.target.value)} maxLength={64} placeholder="ИВТ-21" />
              </Field>
              <Field label="Сейчас идёт неделя">
                <div className="segmented">
                  <button className={week === 'odd' ? 'active' : ''} onClick={() => setWeek('odd')}>Нечётная</button>
                  <button className={week === 'even' ? 'active' : ''} onClick={() => setWeek('even')}>Чётная</button>
                </div>
              </Field>
            </>
          )}
          <Field label="Фамилия и имя">
            <input value={fullName} onChange={(e) => setFullName(e.target.value)} maxLength={100} placeholder="Иванов Иван" />
          </Field>
          {mode === 'create' && <p className="hint">Часовой пояс: {tz}</p>}
          {err && <p className="error-text">{err}</p>}
        </div>

        <button className="btn primary xl" disabled={!canSubmit || busy} onClick={submit}>
          {busy ? 'Секунду…' : mode === 'join' ? 'Вступить в группу' : 'Создать группу'}
        </button>
      </div>
    </div>
  );
}

export function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
    </label>
  );
}
