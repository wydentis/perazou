import { useCallback, useEffect, useState } from 'react';
import { api } from '../api';
import type { GroupDetails, Me } from '../types';
import { confirmDialog, tg } from '../telegram';

export default function GroupScreen({ me, onChanged }: { me: Me; onChanged: () => void }) {
  const isHeadman = me.member!.role === 'headman';
  const [d, setD] = useState<GroupDetails | null>(null);
  const [err, setErr] = useState('');
  const [name, setName] = useState(me.group!.name);
  const [week, setWeek] = useState(me.group!.current_week);
  const [myName, setMyName] = useState(me.member!.full_name);
  const load = useCallback(() => {
    api.group().then(setD, (e: Error) => setErr(e.message));
  }, []);
  useEffect(load, [load]);

  const run = (fn: () => Promise<unknown>, after?: () => void) => async () => {
    setErr('');
    try {
      await fn();
      (after ?? load)();
    } catch (e) {
      setErr((e as Error).message);
    }
  };

  function share() {
    if (!d?.invite_link) return;
    const text = encodeURIComponent(`Вступай в группу ${me.group!.name}: отмечай посещаемость в приложении`);
    const url = `https://t.me/share/url?url=${encodeURIComponent(d.invite_link)}&text=${text}`;
    if (tg) tg.openTelegramLink(url);
    else window.open(url, '_blank');
  }

  return (
    <div className="page">
      <h1>{me.group!.name}</h1>
      {err && <p className="err">{err}</p>}

      {isHeadman && d?.invite_code && (
        <section className="card">
          <b>Приглашение</b>
          <p className="muted small">Код: <code>{d.invite_code}</code></p>
          {d.invite_link ? (
            <button className="primary" onClick={share}>Поделиться ссылкой</button>
          ) : (
            <p className="muted small">Задайте BOT_USERNAME на сервере, чтобы получить ссылку.</p>
          )}
          <button onClick={run(async () => {
            if (await confirmDialog('Старая ссылка перестанет работать. Продолжить?')) await api.regenerateInvite();
          })}>Сменить код</button>
        </section>
      )}

      <section>
        <h3>Моё имя</h3>
        <input value={myName} onChange={(e) => setMyName(e.target.value)} maxLength={100} />
        <button disabled={!myName.trim() || myName === me.member!.full_name} onClick={run(() => api.rename(me.member!.user_id, myName), onChanged)}>Сохранить</button>
      </section>

      <section>
        <h3>Участники ({d?.members.length ?? '…'})</h3>
        {d?.members.map((m) => (
          <div key={m.user_id} className="card row">
            <span className="grow">{m.full_name}{m.role === 'headman' && ' 👑'}{m.user_id === me.member!.user_id && ' (вы)'}</span>
            {isHeadman && m.role !== 'headman' && (
              <button className="mini danger" onClick={run(async () => {
                if (await confirmDialog(`Исключить ${m.full_name}?`)) await api.removeMember(m.user_id);
              })}>✕</button>
            )}
          </div>
        ))}
      </section>

      {isHeadman ? (
        <section>
          <h3>Настройки группы</h3>
          <label>Название</label>
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={64} />
          <label>Сейчас идёт неделя</label>
          <div className="segmented">
            <button className={week === 'odd' ? 'active' : ''} onClick={() => setWeek('odd')}>Нечётная</button>
            <button className={week === 'even' ? 'active' : ''} onClick={() => setWeek('even')}>Чётная</button>
          </div>
          <button onClick={run(() => api.updateGroup({ name, timezone: me.group!.timezone, current_week: week }), onChanged)}>Сохранить</button>
        </section>
      ) : (
        <button className="danger" onClick={run(async () => {
          if (await confirmDialog('Выйти из группы? Ваши отметки будут удалены.')) await api.leave();
        }, onChanged)}>Выйти из группы</button>
      )}
    </div>
  );
}
