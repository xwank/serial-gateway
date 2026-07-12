let scanDevices = [];
let stagingSlots = [];
let gatewayRunning = false;

async function api(path, opts) {
  const res = await fetch(path, opts);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

function setStatus(msg) {
  document.getElementById('status').textContent = msg;
}

function esc(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/"/g,'&quot;');
}

function renderScan() {
  const body = document.getElementById('scanBody');
  const hint = document.getElementById('scanHint');
  body.innerHTML = '';
  if (!scanDevices.length) {
    hint.style.display = 'block';
    return;
  }
  hint.style.display = 'none';
  scanDevices.forEach((d, i) => {
    const tr = document.createElement('tr');
    const status = d.in_staging
      ? '<span class="tag-ok">已在列表</span>'
      : '<span class="tag-warn">未加入</span>';
    tr.innerHTML = `
      <td class="col-check"><input type="checkbox" class="pick" data-i="${i}" ${d.in_staging ? 'disabled' : ''} /></td>
      <td>${esc(d.com)}</td>
      <td><code>${esc(d.location)}</code></td>
      <td>${esc(d.vid_pid)}</td>
      <td>${status}</td>
    `;
    body.appendChild(tr);
  });
}

function renderStaging() {
  const body = document.getElementById('stagingBody');
  const hint = document.getElementById('stagingHint');
  document.getElementById('stagingCount').textContent = stagingSlots.length;
  body.innerHTML = '';
  if (!stagingSlots.length) {
    hint.style.display = 'block';
    return;
  }
  hint.style.display = 'none';
  stagingSlots.forEach((s) => {
    const tr = document.createElement('tr');
    tr.innerHTML = `
      <td>${s.id}</td>
      <td>${esc(s.com)}</td>
      <td><code>${esc(s.location)}</code></td>
      <td><input type="text" class="st-desc" data-id="${s.id}" value="${esc(s.description)}" /></td>
      <td><input type="number" class="st-port" data-id="${s.id}" min="1" max="65535" value="${s.tcp_port}" /></td>
      <td><button type="button" class="btn-rm small danger" data-id="${s.id}">移除</button></td>
    `;
    body.appendChild(tr);
  });
  body.querySelectorAll('.btn-rm').forEach(btn => {
    btn.onclick = () => removeStaging(parseInt(btn.dataset.id, 10));
  });
}

function collectStagingFromUI() {
  return stagingSlots.map(s => {
    const descEl = document.querySelector(`.st-desc[data-id="${s.id}"]`);
    const portEl = document.querySelector(`.st-port[data-id="${s.id}"]`);
    return {
      id: s.id,
      match_location: s.match_location,
      com: s.com,
      location: s.location,
      description: descEl ? descEl.value.trim() : s.description,
      tcp_port: portEl ? parseInt(portEl.value, 10) : s.tcp_port,
    };
  });
}

const DEFAULT_TCP_PORT = 2001;

function collectSelectedScan() {
  const picks = document.querySelectorAll('.pick:checked');
  const slots = [];
  picks.forEach(cb => {
    const i = parseInt(cb.dataset.i, 10);
    const d = scanDevices[i];
    if (!d || d.in_staging) return;
    slots.push({
      match_location: d.match_location,
      com: d.com,
      location: d.location,
      description: d.description,
      tcp_port: DEFAULT_TCP_PORT,
    });
  });
  return slots;
}

async function loadIPs() {
  const data = await api('/api/ips');
  const sel = document.getElementById('serverIp');
  sel.innerHTML = '';
  data.ips.forEach(ip => {
    const o = document.createElement('option');
    o.value = ip; o.textContent = ip;
    sel.appendChild(o);
  });
  if (data.selected) sel.value = data.selected;
}

async function loadStaging() {
  const data = await api('/api/staging');
  stagingSlots = data.slots || [];
  renderStaging();
}

async function scan() {
  setStatus('正在扫描在线设备...');
  const data = await api('/api/scan');
  scanDevices = data.devices || [];
  renderScan();
  setStatus(data.message || '');
}

async function addStaging() {
  const slots = collectSelectedScan();
  if (!slots.length) {
    setStatus('请先勾选要加入的在线设备');
    return;
  }
  const data = await api('/api/staging/add', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      preserve: collectStagingFromUI(),
      slots,
    }),
  });
  stagingSlots = data.slots || [];
  renderStaging();
  await scan();
  setStatus(data.message);
}

async function removeStaging(id) {
  const data = await api('/api/staging/remove', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id, preserve: collectStagingFromUI() }),
  });
  stagingSlots = data.slots || [];
  renderStaging();
  await scan();
  setStatus(data.message);
}

async function clearStaging() {
  if (!confirm('确定清空服务启动列表？')) return;
  const data = await api('/api/staging/clear', { method: 'POST' });
  stagingSlots = [];
  renderStaging();
  await scan();
  setStatus(data.message);
}

async function saveConfig() {
  const lan_ip = document.getElementById('serverIp').value;
  const slots = collectStagingFromUI();
  if (!slots.length) {
    setStatus('服务列表为空，请先加入 Hub 槽位');
    return;
  }
  const data = await api('/api/config', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ lan_ip, slots }),
  });
  setStatus(data.message);
}

async function startGateway() {
  const lan_ip = document.getElementById('serverIp').value;
  const slots = collectStagingFromUI();
  if (!slots.length) {
    setStatus('服务列表为空，无法启动');
    return;
  }
  const data = await api('/api/gateway/start', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ lan_ip, slots }),
  });
  gatewayRunning = true;
  updateButtons();
  setStatus(data.message);
}

async function stopGateway() {
  const data = await api('/api/gateway/stop', { method: 'POST' });
  gatewayRunning = false;
  updateButtons();
  setStatus(data.message);
}

async function refreshStatus() {
  try {
    const data = await api('/api/status');
    gatewayRunning = data.running;
    updateButtons();
    if (data.log_tail) {
      document.getElementById('logView').textContent = data.log_tail;
    }
  } catch (_) {}
}

function updateButtons() {
  document.getElementById('btnStart').disabled = gatewayRunning;
  document.getElementById('btnStop').disabled = !gatewayRunning;
}

document.getElementById('btnRefreshIp').onclick = () => loadIPs().catch(e => setStatus(e.message));
document.getElementById('btnScan').onclick = () => scan().catch(e => setStatus(e.message));
document.getElementById('btnAddStaging').onclick = () => addStaging().catch(e => setStatus(e.message));
document.getElementById('btnClearStaging').onclick = () => clearStaging().catch(e => setStatus(e.message));
document.getElementById('btnSave').onclick = () => saveConfig().catch(e => setStatus(e.message));
document.getElementById('btnStart').onclick = () => startGateway().catch(e => setStatus(e.message));
document.getElementById('btnStop').onclick = () => stopGateway().catch(e => setStatus(e.message));

loadIPs().catch(e => setStatus(e.message));
loadStaging().catch(e => setStatus(e.message));
refreshStatus();
setInterval(refreshStatus, 3000);
