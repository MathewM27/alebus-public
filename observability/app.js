async function loadStatus() {
  const conn = document.getElementById('conn');
  const mode = document.getElementById('mode');
  const master = document.getElementById('master');
  const active = document.getElementById('active');
  const offline = document.getElementById('offline');
  const total = document.getElementById('total');
  const mem = document.getElementById('mem');
  const health = document.getElementById('health');
  const ts = document.getElementById('ts');

  conn.textContent = 'Loading…';

  try {
    const res = await fetch('/api/redis/status', { cache: 'no-store' });
    const data = await res.json();

    conn.textContent = data.connected ? 'Connected' : 'Disconnected';
    mode.textContent = data.mode || '—';
    master.textContent = data.master || '—';

    const m = data.metrics || {};
    active.textContent = m.ActiveBusCount ?? '—';
    offline.textContent = m.OfflineBusCount ?? '—';
    total.textContent = m.TotalBusCount ?? '—';

    const memBytes = m.UsedMemoryBytes;
    if (typeof memBytes === 'number') {
      mem.textContent = (memBytes / 1024 / 1024).toFixed(1) + ' MB';
    } else {
      mem.textContent = '—';
    }

    health.textContent = JSON.stringify(data.health || {}, null, 2);

    const now = new Date();
    ts.textContent = 'Last refresh: ' + now.toLocaleTimeString();
  } catch (e) {
    conn.textContent = 'Error';
    health.textContent = String(e);
  }
}

document.getElementById('refresh').addEventListener('click', loadStatus);
loadStatus();
