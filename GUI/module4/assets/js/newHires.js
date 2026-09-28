// assets/js/newHires.js
async function api(action, payload = {}) {
  const res = await fetch(`/module/proxy/module4/api?action=${encodeURIComponent(action)}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

function makeCard(person) {


  const addDone = !!person.web_user_id;
  const deskDone = !!person.desk_ticket_sent;
  const accDone  = !!person.account_ticket_sent;

  // Si todo hecho, no mostramos
  if (addDone && deskDone && accDone) return null;

  const col = document.createElement("div");
  col.className = "col";

  const fullName = `${person.first_name ?? ""} ${person.last_name ?? ""}`.trim();

  col.innerHTML = `
    <div class="card shadow-sm h-100">
      <div class="card-body d-flex flex-column">
        <h5 class="card-title mb-3">${fullName || "(No name)"}</h5>

        <div class="d-flex flex-wrap gap-2 mt-auto">
          <button class="btn btn-primary btn-sm" data-action="add_intranet" ${addDone ? "hidden" : ""}>
            Add to intranet
          </button>
          <button class="btn btn-outline-primary btn-sm" data-action="desk_ticket" ${deskDone ? "hidden" : ""}>
            Request desk assignment
          </button>
          <button class="btn btn-outline-primary btn-sm" data-action="account_ticket" ${accDone ? "hidden" : ""}>
            Request account creation
          </button>
        </div>
      </div>
    </div>
  `;

  // handlers
  col.querySelectorAll("button[data-action]").forEach(btn => {
    btn.addEventListener("click", async () => {
      const action = btn.dataset.action;
      btn.disabled = true;

      try {
        if (action === "add_intranet") {
          const ok = await confirmAddToIntranet();
          if (!ok) {
            btn.disabled = false; // cancelado => reactivar botón
            return;
          }
          await api("newHiresAddToIntranet", { people_id: person.people_id });
        } else if (action === "desk_ticket") {
          await api("newHiresRequestDesk", { people_id: person.people_id });
        } else if (action === "account_ticket") {
          await api("newHiresRequestAccount", { people_id: person.people_id });
        }

        // refrescamos lista tras acción (para bloquear/ocultar si ya está todo hecho)
        await loadNewHires();
      } catch (e) {
        console.error(e);
        alert("Operation failed: " + (e?.message || e));
        btn.disabled = false;
      }
    });
  });

  return col;
}

function confirmAddToIntranet() {
  return new Promise((resolve) => {
    const modalEl = document.getElementById("confirmAddIntranetModal");
    const yesBtn = document.getElementById("confirmAddIntranetYesBtn");

    if (!modalEl || !yesBtn || !window.bootstrap?.Modal) {
      // fallback si no está el modal/Bootstrap: confirm nativo
      resolve(window.confirm("Are you sure you have added all intranet relevant information for this worker?"));
      return;
    }

    const modal = bootstrap.Modal.getOrCreateInstance(modalEl);

    const cleanup = () => {
      yesBtn.removeEventListener("click", onYes);
      modalEl.removeEventListener("hidden.bs.modal", onHidden);
    };

    const onYes = () => {
      cleanup();
      modal.hide();
      resolve(true);
    };

    const onHidden = () => {
      // si se cierra sin darle Yes => No
      cleanup();
      resolve(false);
    };

    yesBtn.addEventListener("click", onYes, { once: true });
    modalEl.addEventListener("hidden.bs.modal", onHidden, { once: true });

    modal.show();
  });
}

export async function loadNewHires() {
  const listEl = document.getElementById("newhires-list");
  const countEl = document.getElementById("newhires-count");
  if (!listEl) return;

  listEl.innerHTML = `<div class="col"><div class="text-muted">Loading...</div></div>`;

  const data = await api("getNewHires", {}); 

  console.log("New hires data:", data);

  let rows = [];
  if (Array.isArray(data)) rows = data;
  else if (Array.isArray(data?.rows)) rows = data.rows;
  else if (data && typeof data === "object") rows = Object.values(data);

  const normalized = rows
    .filter(Boolean)
    .map(p => ({
      people_id: p.id, 
      first_name: p.name ?? "",
      last_name: p.surname ?? "",
      second_last_name: p.secondSurname ?? "",
      web_user_id: p.people_idExternal ?? null,
      desk_ticket_sent: p.desk_request ?? false,
      account_ticket_sent: p.account_request ?? false,
    }));

  listEl.innerHTML = "";
  let shown = 0;

  normalized.forEach(p => {
    const card = makeCard({
      ...p,
      last_name: `${p.last_name} ${p.second_last_name}`.trim(),
    });
    if (card) {
      listEl.appendChild(card);
      shown++;
    }
  });
  if (shown === 0) {
    listEl.innerHTML = `<div class="col"><div class="text-muted">No new hires pending actions</div></div>`;
  }

  // badge count
  if (shown > 0) {
    countEl.style.display = "inline-block";
    countEl.textContent = String(shown);
  } else {
    countEl.style.display = "none";
  }
}


let newHiresLoaded = false;

export function initNewHiresUI() {
  // Load once on page entry so the badge is visible from the Users tab
  loadNewHires()
    .then(() => {
      newHiresLoaded = true;
    })
    .catch(console.error);

  const tabBtn = document.getElementById("tab-newhires");
  if (tabBtn) {
    tabBtn.addEventListener("shown.bs.tab", () => {
      if (newHiresLoaded) return;

      loadNewHires()
        .then(() => {
          newHiresLoaded = true;
        })
        .catch(console.error);
    });
  }

  const histBtn = document.getElementById("newHiresHistoryBtn");
  if (histBtn) {
    histBtn.addEventListener("click", () => {
      openNewHiresHistory().catch(err => {
        console.error(err);
        alert("Failed to load history: " + (err?.message || err));
      });
    });
  }
}


function fmtTs(v) {
  if (!v) return "—";

  if (v instanceof Date) {
    if (Number.isNaN(v.getTime())) return "—";
    const y = v.getFullYear();
    const m = String(v.getMonth() + 1).padStart(2, "0");
    const d = String(v.getDate()).padStart(2, "0");
    const hh = String(v.getHours()).padStart(2, "0");
    const mm = String(v.getMinutes()).padStart(2, "0");
    return `${y}-${m}-${d} ${hh}:${mm}`;
  }

  let s = String(v).trim();
  if (!s) return "—";

  // "2026-03-02T11:35:00Z" -> "2026-03-02 11:35:00"
  s = s.replace("T", " ").replace("Z", "");

  // Recorta a "YYYY-MM-DD HH:MM"
  if (/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}/.test(s)) return s.slice(0, 16);
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return s;

  const d = parseTs(s);
  return d ? fmtTs(d) : "—";
}

function parseTs(v) {
  if (!v) return null;

  if (typeof v === "number") return null;

  let s = String(v).trim();
  if (!s) return null;

  if (s.startsWith("1900-01-01")) return null;

  // "YYYY-MM-DD HH:MM"  -> "YYYY-MM-DDTHH:MM:00"
  // "YYYY-MM-DD HH:MM:SS" -> "YYYY-MM-DDTHH:MM:SS"
  if (/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}(:\d{2})?$/.test(s)) {
    s = s.replace(" ", "T");
    if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(s)) s += ":00";
  }

  // "YYYY-MM-DD" -> "YYYY-MM-DDT00:00:00"
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) {
    s += "T00:00:00";
  }

  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return null;
  return d;
}

function lastActivityOf(row) {
  const ds = [
    parseTs(row.added),
    parseTs(row.intra_req_time),
    parseTs(row.desk_req_time),
    parseTs(row.acc_req_time),
  ].filter(Boolean);

  if (ds.length === 0) return null;
  ds.sort((a, b) => b - a);
  return ds[0];
}

function badge(done, ts) {
  if (!done) return `<span class="badge bg-secondary">No</span>`;
  return `<span class="badge bg-success">Yes</span><div class="small text-muted">${fmtTs(ts)}</div>`;
}

async function openNewHiresHistory() {
  const modalEl = document.getElementById("newHiresHistoryModal");
  const bodyEl = document.getElementById("newHiresHistoryBody");
  if (!modalEl || !bodyEl || !window.bootstrap?.Modal) {
    alert("History modal not available.");
    return;
  }

  bodyEl.innerHTML = `<tr><td colspan="7" class="text-muted">Loading...</td></tr>`;
  const modal = bootstrap.Modal.getOrCreateInstance(modalEl);
  modal.show();

  // endpoint global
  const data = await api("getNewHiresHistory", {});
  let rows = [];
  if (Array.isArray(data)) rows = data;
  else if (Array.isArray(data?.rows)) rows = data.rows;
  else if (data && typeof data === "object") rows = Object.values(data);

  console.log("New hires history data:", data);
  // normaliza y calcula last_activity
  const norm = rows.filter(Boolean).map(r => {
    const la = lastActivityOf(r);
    return {
      ...r,
      last_activity: la ? fmtTs(la.toISOString()) : "—",
    };
  });

  // orden por última actividad desc (y si no, por people_id desc)
  norm.sort((a, b) => {
    const da = a.last_activity === "—" ? 0 : Date.parse(a.last_activity);
    const db = b.last_activity === "—" ? 0 : Date.parse(b.last_activity);
    if (db !== da) return db - da;
    return (b.people_id || 0) - (a.people_id || 0);
  });

  if (norm.length === 0) {
    bodyEl.innerHTML = `<tr><td colspan="7" class="text-muted">No history</td></tr>`;
    return;
  }

  bodyEl.innerHTML = norm.map(r => `
    <tr>
      <td>${r.people_id ?? "—"}</td>
      <td>${[r.name, r.surname, r.secondSurname].filter(Boolean).join(" ") || "—"}</td>
      <td>${r.added ? fmtTs(r.added) : "—"}</td>
      <td>${badge(!!r.intra_req, r.intra_req_time)}</td>
      <td>${badge(!!r.desk_request, r.desk_req_time)}</td>
      <td>${badge(!!r.account_request, r.acc_req_time)}</td>
      <td>${r.last_activity}</td>
    </tr>
  `).join("");
}