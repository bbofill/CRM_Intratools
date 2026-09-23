// modules.js
const API_URL = "/module/proxy/module0/api/module"; 

const moduleBody = document.getElementById("moduleBody");
let modules = []; 


const STATE_UI = {
  running: { text: "Running", class: "text-success" },
  stopped: { text: "Stopped", class: "text-danger" },
  unknown: { text: "?", class: "" },
};

// build table
function buildTable(data) {
  moduleBody.innerHTML = "";
  modules = Object.keys(data);
  modules.forEach((mod) => {
    const state = data[mod] || "unknown";
    const row = document.createElement("tr");
    row.innerHTML = `
      <td class="fw-medium">${mod}</td>
      <td class="text-nowrap">
        <button class="btn btn-success btn-sm me-1" data-module="${mod}" data-action="start">Start</button>
        <button class="btn btn-danger btn-sm me-1" data-module="${mod}" data-action="stop">Stop</button>
        <button class="btn btn-warning btn-sm text-white" data-module="${mod}" data-action="restart">Restart</button>
      </td>
      <td id="status-${mod}">—</td>`;
    moduleBody.appendChild(row);
    updateStatusCell(mod, state);
  });
}

// button events
moduleBody.addEventListener("click", (e) => {
  const btn = e.target.closest("button");
  if (!btn) return;
  const module = btn.dataset.module;
  const action = btn.dataset.action;
  sendAction(module, action);
});

// POST start|stop|restart & refresh
function sendAction(module, action) {
  fetch(API_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ module, action }),
  })
    .then((res) => {
      if (!res.ok) throw new Error(`Error ${res.status}`);
      return res.json();
    })
    .then(() => refreshStatus(module))
    .catch((err) => alert(`Error: ${err.message}`));
}

function refreshStatus(module) {
  fetch(`${API_URL}?module=${encodeURIComponent(module)}`)
    .then((res) => {
      if (!res.ok) throw new Error(`Error ${res.status}`);
      return res.json();
    })
    .then((data) => {
      const state = data.status || "unknown";
      updateStatusCell(module, state);
    })
    .catch(() => updateStatusCell(module, "unknown"));
}

function updateStatusCell(module, state) {
  const cell = document.getElementById(`status-${module}`);
  if (!cell) return; 
  const ui = STATE_UI[state] || STATE_UI.unknown;
  cell.textContent = ui.text;
  cell.className = ui.class;
}

// Reload full table
function refreshAll() {
  fetch(API_URL)
    .then((res) => {
      if (!res.ok) throw new Error(`Error ${res.status}`);
      return res.json();
    })
    .then((data) => {
      const keys = Object.keys(data);
      const sameSet =
        keys.length === modules.length && keys.every((k) => modules.includes(k));
      if (!sameSet) {
        buildTable(data);
      } else {
        keys.forEach((mod) => updateStatusCell(mod, data[mod]));
      }
    })
    .catch(() => modules.forEach((m) => updateStatusCell(m, "unknown")));
}

// firstload 
fetch(API_URL)
  .then((res) => {
    if (!res.ok) throw new Error(`Error ${res.status}`);
    return res.json();
  })
  .then((data) => buildTable(data))
  .catch((err) => {
    console.error(err);
    moduleBody.innerHTML =
      '<tr><td colspan="3" class="text-danger">Unable to get Modules</td></tr>';
  });

setInterval(refreshAll, 10_000);
