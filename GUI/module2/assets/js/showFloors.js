
const floorCanvas = document.getElementById("floorCanvas");

let currentSvg = null;
let originalViewBox = null;
let selectedDeskId = null;
let selectedDeskSvgId = null;

let currentFloor = 0;

let filterStart = null;
let filterEnd = null;

const deskDataById = new Map();


const container = document.getElementById("floorCanvas");

function normalizeText(str) {
  return str
    .toLowerCase()
    .normalize("NFD")              
    .replace(/[\u0300-\u036f]/g, ""); 
}


async function loadDeskStatus(floor) {
  const qs = new URLSearchParams({ action: "desks", floor: String(floor) });

  if (filterStart) qs.set("start_date", filterStart);
  if (filterEnd) qs.set("end_date", filterEnd); // si es null, no se manda

  const url = `/module/proxy/module2/api?${qs.toString()}`;
  console.log("FETCH:", url);

  const res = await fetch(url);
  if (!res.ok) {
    const text = await res.text();
    console.error("API error:", res.status, text);
    return;
  }

  const desks = await res.json();
  deskDataById.clear();

  console.log("Loaded desks data:", desks);
  const rangeStartDay = filterStart ? parseDayNum(filterStart) : null;
  const rangeEndDay = filterEnd ? parseDayNum(filterEnd) : null;

  const rangeStart = filterStart;
  const rangeEnd = filterEnd || filterStart;

  const hasStart = rangeStartDay != null;
  const hasEnd = rangeEndDay != null;


  const hasFiniteRange = hasStart && hasEnd && rangeEndDay >= rangeStartDay;
  const rangeLen = hasFiniteRange ? (rangeEndDay - rangeStartDay + 1) : 0;

  desks.forEach(desk => {
    deskDataById.set(String(desk.id), desk);

    //console.log("Processing desk:", desk);
    const safeName = desk.name.replace(/\./g, "\\.");
    const el = document.querySelector(`#desk_${safeName}`);
    if (!el) {
      console.warn("Desk not found in SVG:", desk.name);
      return;
    }

    const status = computeStatus(desk, rangeStart, rangeEnd);

    el.classList.remove("available", "reserved", "assigned", "disabled", "partial");
    el.classList.add(status);

    el.dataset.deskId = desk.id;
    el.dataset.status = status;
    el.dataset.worker = ""; 
    el.dataset.floor = desk.floor;
    el.dataset.roomUabCode = desk.room_uab_code || "";
  });
}

function parseDayNum(yyyy_mm_dd) {
  if (!yyyy_mm_dd) return null;
  const [y, m, d] = yyyy_mm_dd.split("-").map(Number);
  return Math.floor(Date.UTC(y, m - 1, d) / 86400000);
}

function dayNumToStr(dayNum) {
  const dt = new Date(dayNum * 86400000);
  const y = dt.getUTCFullYear();
  const m = String(dt.getUTCMonth() + 1).padStart(2, "0");
  const d = String(dt.getUTCDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

async function loadFloor(floor) {
    floorCanvas.innerHTML = "";

    const response = await fetch(`./assets/svg/floor-${floor}.svg`);
    const svgText = await response.text();
    floorCanvas.innerHTML = svgText;

    currentSvg = floorCanvas.querySelector("svg");

    // Guardamos el viewBox original
    originalViewBox = currentSvg.getAttribute("viewBox");

    setupSvgInteractions(currentSvg);

    await loadDeskStatus(floor);
}

function resetZoom() {
    if (currentFloor !== 0) return;
    if (!currentSvg || !originalViewBox) return;
    currentSvg.setAttribute("viewBox", originalViewBox);
}
function updateZoomUI() {
    document.getElementById("resetZoom").style.display =
        currentFloor === 0 ? "inline-block" : "none";
}

const deskModalEl = document.getElementById("deskModal");
const assignDeskModalEl = document.getElementById("assignDeskModal");
const pauseDeskModalEl = document.getElementById("pauseDeskModal");

const deskModalInstance = deskModalEl ? new bootstrap.Modal(deskModalEl) : null;

function openModalAfterHidden(fromEl, toEl) {
  const showTarget = () => {
    if (toEl) {
      bootstrap.Modal.getOrCreateInstance(toEl).show();
    }
  };

  if (!fromEl) {
    showTarget();
    return;
  }

  const fromInstance = bootstrap.Modal.getInstance(fromEl);
  if (fromInstance && fromEl.classList.contains("show")) {
    fromEl.addEventListener("hidden.bs.modal", showTarget, { once: true });
    fromInstance.hide();
    return;
  }

  showTarget();
}


function openDeskModal(svgDeskId, shouldShow = true) {
  selectedDeskSvgId = svgDeskId;

  const el = document.getElementById(svgDeskId);
  selectedDeskId = el.dataset.deskId;

  document.getElementById("deskCode").textContent = svgDeskId.replace("desk_", "");
  document.getElementById("deskFloor").textContent = el.dataset.floor ?? "—";
  document.getElementById("roomUabCode").textContent = el.dataset.roomUabCode ?? "—";
  const reservableBtn = document.getElementById("reservableDeskBtn");
  const freezeBtn = document.getElementById("freezeDeskBtn");
  const pauseBtn = document.getElementById("pauseDeskBtn");
  const assignBtn = document.getElementById("assignPersonBtn");


  const conflictsBox = document.getElementById("deskConflicts");
  const desk = deskDataById.get(String(selectedDeskId));

  if (desk.is_hot === true) {
    reservableBtn.disabled = true;
    freezeBtn.disabled = false;
  } else {
    reservableBtn.disabled = false;
    freezeBtn.disabled = true;
  }
  const hasAssignments =
    desk &&
    Array.isArray(desk._conflictsInRange) &&
    desk._conflictsInRange.some(c => c.kind === "assigned");

  if (hasAssignments) {
    pauseBtn.disabled = false;
    assignBtn.disabled = true;
  } else {
    pauseBtn.disabled = true;
    assignBtn.disabled = false;
  }

  if (!filterStart || !filterEnd) {
    conflictsBox.textContent = "Set a date range to see occupancy details.";
  } else if (!desk || !(desk._conflictsInRange || []).length) {
    conflictsBox.textContent = "No occupancy in the selected period.";
    } else {
    const items = (desk._conflictsInRange || [])
      .sort((a, b) => a._clipped[0] - b._clipped[0])
      .map(c => {
        const guestName    = c.guest_name ?? "";
        const guestHost    = c.guest_host ?? "";
        const guestProgram = c.guest_program ?? "";
        const guestRole    = c.guest_role ?? "";

        const [cs, ce] = c._clipped;
        const isAssigned = c.kind === "assigned";
        const isReleased = c.kind === "released";

        const label = isAssigned ? "Assigned" : isReleased ? "Released" : "Reserved";

        const badge = isAssigned
          ? `<span class="badge bg-info text-dark me-2">Assigned</span>`
          : isReleased
            ? `<span class="badge bg-success me-2">Released</span>`
            : `<span class="badge bg-danger me-2">Reserved</span>`;

        const isGuest =
          isAssigned &&
          !c.user_id &&
          (guestName || guestHost || guestProgram || guestRole);

        const who = isGuest
          ? `${guestName || "Guest"}${guestRole ? " (" + guestRole + ")" : ""}`
          : (c.person_name || c.worker || c.person || "—");

        const rawStart = c.start_date || c.startDate || "—";
        let rawEnd   = (c.end_date || c.endDate || null);

        const isInfinite = !rawEnd || rawEnd === "2099-12-31";
        if (rawEnd === "2099-12-31") rawEnd = null;

        const rangeText = isInfinite
          ? `${rawStart} → ∞`
          : `${rawStart} → ${rawEnd}`;

        const occId = c.id;

        const canEdit = isAssigned || isReleased;

        const contractParts = [
          c.contract_vinculation_type,
          c.contract_type,
          c.contract_position
        ].filter(Boolean);
        const contractStart = c.contract_start_date || "";
        const contractEnd = c.contract_end_date || "";
        const contractRange = contractStart
          ? `${contractStart} → ${contractEnd || "∞"}`
          : "";
        const contractInfo = isAssigned && c.contract_id ? `
          <div class="mt-3 p-2 border rounded bg-white">
            <div class="fw-semibold small">Related contract</div>
            <div class="small">${contractParts.join(" · ") || `Contract #${c.contract_id}`}</div>
            ${contractRange ? `<div class="text-muted small">${contractRange}</div>` : ""}
          </div>
        ` : "";

        const guestInfo = isGuest ? `
          <div class="text-muted small mt-1">
            Host: ${guestHost || "—"} · Program: ${guestProgram || "—"} · Role: ${guestRole || "—"}
          </div>
        ` : "";

        const guestEditControls = isGuest ? `
        <div class="w-100 mt-3">
          <div class="row g-2">
            <div class="col-md-6">
              <label class="form-label form-label-sm mb-0">Guest name</label>
              <input
                type="text"
                class="form-control form-control-sm occ-guest-name"
                value="${guestName}"
              />
            </div>
            <div class="col-md-6">
              <label class="form-label form-label-sm mb-0">Host</label>
              <input
                type="text"
                class="form-control form-control-sm occ-guest-host"
                value="${guestHost}"
              />
            </div>
            <div class="col-md-6">
              <label class="form-label form-label-sm mb-0">Program</label>
              <input
                type="text"
                class="form-control form-control-sm occ-guest-program"
                value="${guestProgram}"
              />
            </div>
            <div class="col-md-6">
              <label class="form-label form-label-sm mb-0">Role</label>
              <select
                class="form-select form-select-sm occ-guest-role"
              >
                <option value="">Select role…</option>
                <option value="speaker"   ${guestRole === "speaker"   ? "selected" : ""}>Speaker</option>
                <option value="lecturer" ${guestRole === "lecturer" ? "selected" : ""}>Lecturer</option>
                <option value="visitor"   ${guestRole === "visitor"   ? "selected" : ""}>Visitor</option>
              </select>
            </div>
          </div>
        </div>
      ` : "";

        const editControls = canEdit ? `
          <div class="mt-2 occ-edit-area d-none">
            <div class="d-flex flex-wrap align-items-center gap-2">
              <input
                type="date"
                class="form-control form-control-sm occ-start-input"
                value="${rawStart !== "—" ? rawStart : ""}"
                data-occ-id="${occId}"
                data-occ-kind="${c.kind}"
              />
              <span>→</span>
              <input
                type="date"
                class="form-control form-control-sm occ-end-input"
                value="${rawEnd || ""}"
                placeholder="∞"
                data-occ-id="${occId}"
                data-occ-kind="${c.kind}"
              />

              <button
                type="button"
                class="btn btn-sm btn-outline-primary occ-save-btn"
                data-occ-id="${occId}"
                data-occ-kind="${c.kind}"
              >
                Save
              </button>

              <button
                type="button"
                class="btn btn-sm btn-outline-danger occ-delete-btn"
                data-occ-id="${occId}"
                data-occ-kind="${c.kind}"
              >
                Delete
              </button>
            </div>
            ${contractInfo}
            ${guestEditControls}
          </div>
        ` : "";

        const editButton = canEdit ? `
          <button
            type="button"
            class="btn btn-sm btn-outline-secondary occ-toggle-edit"
            data-occ-id="${occId}"
            data-occ-kind="${c.kind}"
          >
            Edit
          </button>
        ` : "";

        return `
        <div class="mb-2 border rounded p-2 occ-item" data-occ-id="${occId}" data-occ-kind="${c.kind}">
          <div class="d-flex justify-content-between align-items-start gap-2">
            <div>
              ${badge}
              <strong>${who}</strong>
              <div class="text-muted">
                ${label}: ${rangeText}
              </div>
              ${guestInfo}
            </div>
            ${editButton}
          </div>
          ${editControls}
        </div>
      `;

      })
      .join("");

    conflictsBox.innerHTML = items;
  }

  if (shouldShow && deskModalInstance) {
    deskModalInstance.show();
  }
}


// Funcions pel càlcul d'estat de les taules

function toDayNum(yyyy_mm_dd) {
  const [y, m, d] = yyyy_mm_dd.split("-").map(Number);
  return Math.floor(Date.UTC(y, m - 1, d) / 86400000);
}

function clip(s, e, rs, re) {
  const cs = Math.max(s, rs);
  const ce = Math.min(e, re);
  return cs <= ce ? [cs, ce] : null;
}

function merge(intervals) {
  if (!intervals.length) return [];
  intervals.sort((a,b)=>a[0]-b[0]);
  const out = [intervals[0]];
  for (let i=1;i<intervals.length;i++){
    const [s,e]=intervals[i];
    const last=out[out.length-1];
    if (s <= last[1] + 1) last[1] = Math.max(last[1], e);
    else out.push([s,e]);
  }
  return out;
}

function coveredDays(merged) {
  return merged.reduce((sum,[s,e])=>sum+(e-s+1),0);
}

function computeStatus(desk, rangeStart, rangeEnd) {
  const isHotFalse = (desk.is_hot === false || desk.is_hot === "false" || desk.isHot === false);

  const rs = toDayNum(rangeStart);
  const re = toDayNum(rangeEnd);
  const rangeLen = (re - rs + 1);

  const occIntervals = [];
  const releaseIntervals = [];
  const conflicts = [];

  const pushOcc = (kind, occ, targetIntervals) => {
    const s = toDayNum(occ.start_date || occ.startDate);
    const endStr = occ.end_date || occ.endDate;
    const e = endStr ? toDayNum(endStr) : re;
    const clipped = clip(s, e, rs, re);
    if (!clipped) return;
    targetIntervals.push(clipped);
    conflicts.push({ kind, ...occ, _clipped: clipped });
  };

  (desk.assignments || []).forEach(a => pushOcc("assigned", a, occIntervals));
  (desk.reservations || []).forEach(r => pushOcc("reserved", r, occIntervals));

  (desk.releases || []).forEach(x => pushOcc("released", x, releaseIntervals));

  const occMerged = merge(occIntervals);
  const relMerged = merge(releaseIntervals);

  const effectiveMerged = subtractIntervals(occMerged, relMerged);
  const cov = coveredDays(effectiveMerged);

  desk._conflictsInRange = conflicts;

  if (cov === 0) return isHotFalse ? "disabled" : "available";

  if (cov === rangeLen) {
    const assignedCoversAll = (desk.assignments || []).some(a => {
      const s = toDayNum(a.start_date || a.startDate);
      const endStr = a.end_date || a.endDate;
      const e = endStr ? toDayNum(endStr) : Infinity;

      // assigned cubre todo, PERO si hay releases que cortan, ya no debería ser full.
      // Así que chequeamos con effectiveMerged:
      return effectiveMerged.length === 1 && effectiveMerged[0][0] === rs && effectiveMerged[0][1] === re && s <= rs && e >= re;
    });

    if (assignedCoversAll) return "assigned";
    return "reserved";
  }

  return "partial";
}


function subtractIntervals(occupiedMerged, releaseMerged) {
  // occupiedMerged y releaseMerged deben venir MERGEADOS
  const out = [];
  let j = 0;

  for (const [os, oe] of occupiedMerged) {
    let curS = os;
    let curE = oe;

    while (j < releaseMerged.length && releaseMerged[j][1] < curS) j++;

    let k = j;
    while (k < releaseMerged.length && releaseMerged[k][0] <= curE) {
      const [rs, re] = releaseMerged[k];

      if (rs > curS) out.push([curS, Math.min(curE, rs - 1)]);
      curS = Math.max(curS, re + 1);
      if (curS > curE) break;

      k++;
    }
    if (curS <= curE) out.push([curS, curE]);
  }
  return merge(out);
}



function setupSvgInteractions(svg) {

  // Oficinas
  svg.querySelectorAll("[id^='office_']").forEach(el => {
    el.classList.add("office");
    el.addEventListener("click", () => zoomToElement(el));
  });

  // Mesas
  svg.querySelectorAll("[id^='desk_']").forEach(el => {
    el.classList.add("desk", "available");
    el.addEventListener("click", e => {
      e.stopPropagation();
      openDeskModal(el.id);
    });
  });
}


document.getElementById("resetZoom").addEventListener("click", resetZoom);
document.querySelectorAll(".floor-selector button").forEach(btn => {
    btn.addEventListener("click", async () => {

        document.querySelectorAll(".floor-selector button")
        .forEach(b => b.classList.remove("active"));

        btn.classList.add("active");

        const floor = parseInt(btn.dataset.floor, 10);
        currentFloor = floor;

        await loadFloor(floor);
        updateZoomUI();

    });
});
document.getElementById("assignPersonBtn").addEventListener("click", () => {
  openModalAfterHidden(deskModalEl, assignDeskModalEl);
});

function dateOnly(value) {
  return value ? String(value).slice(0, 10) : "";
}

function setAssignmentDatesLocked(locked) {
  const startInput = document.getElementById("assignStartDate");
  const endInput = document.getElementById("assignEndDate");
  startInput.disabled = locked;
  endInput.disabled = locked;
}

function resetContractSelection() {
  const contractBox = document.getElementById("contractDetails");
  const contractSelect = document.getElementById("contractSelect");

  if (contractBox) contractBox.classList.add("d-none");
  if (contractSelect) {
    contractSelect.innerHTML = `<option value="">No related contract</option>`;
  }

  setAssignmentDatesLocked(false);
}

function contractLabel(contract) {
  const start = dateOnly(contract.start_date) || "No start";
  const end = dateOnly(contract.end_date) || "Indefinite";
  const parts = [
    contract.vinculation_type,
    contract.type,
    contract.position
  ].filter(Boolean);

  return `${parts.join(" · ") || "Contract"} (${start} → ${end})`;
}

async function loadPersonContracts(personId) {
  const contractBox = document.getElementById("contractDetails");
  const contractSelect = document.getElementById("contractSelect");

  resetContractSelection();
  if (!personId || !contractBox || !contractSelect) return;

  const res = await fetch(`/module/proxy/module2/api?action=personContracts&people_id=${encodeURIComponent(personId)}`);
  if (!res.ok) return;

  const contracts = await res.json();
  if (!Array.isArray(contracts) || !contracts.length) return;

  contracts.forEach(contract => {
    const option = document.createElement("option");
    option.value = contract.id;
    option.textContent = contractLabel(contract);
    option.dataset.startDate = dateOnly(contract.start_date);
    option.dataset.endDate = dateOnly(contract.end_date);
    contractSelect.appendChild(option);
  });

  contractBox.classList.remove("d-none");
}

document.getElementById("contractSelect").addEventListener("change", e => {
  const selected = e.target.selectedOptions[0];
  const hasContract = Boolean(e.target.value && selected);

  setAssignmentDatesLocked(hasContract);
  if (!hasContract) return;

  document.getElementById("assignStartDate").value = selected.dataset.startDate || "";
  document.getElementById("assignEndDate").value = selected.dataset.endDate || "";
});

document.getElementById("personSearch").addEventListener("input", async e => {
  const query = e.target.value.trim();
  const resultsBox = document.getElementById("personResults");
  resultsBox.innerHTML = "";
  delete e.target.dataset.personId;
  resetContractSelection();

  if (query.length < 2) return;

  const res = await fetch(`/module/proxy/module2/api?action=people`);
  if (!res.ok) return;

  const people = await res.json();
  const q = normalizeText(query);

  const filtered = people.filter(p => {
    const fullName = normalizeText(`${p.name} ${p.surname}`);
    return fullName.includes(q);
  });
  


  filtered.forEach(p => {
    const item = document.createElement("button");
    item.className = "list-group-item list-group-item-action";
    item.textContent = `${p.name} ${p.surname}`;
    item.onclick = () => {
      document.getElementById("personSearch").value =
        `${p.name} ${p.surname}`;
      document.getElementById("personSearch").dataset.personId = p.id;
      resultsBox.innerHTML = "";
      loadPersonContracts(p.id);
    };
    resultsBox.appendChild(item);
  });
});
document.getElementById("isGuest").addEventListener("change", e => {
  const input = document.getElementById("personSearch");
  const guestBox = document.getElementById("guestDetails");

  if (e.target.checked) {
    // Desactivar búsqueda de persona
    input.disabled = true;
    input.value = "Guest";
    delete input.dataset.personId;
    resetContractSelection();

    // Mostrar datos de guest
    guestBox.classList.remove("d-none");
  } else {
    // Reactivar búsqueda normal
    input.disabled = false;
    input.value = "";
    delete input.dataset.personId;
    resetContractSelection();

    // Ocultar y limpiar datos guest
    guestBox.classList.add("d-none");
    document.getElementById("guestName").value = "";
    document.getElementById("guestHost").value = "";
    document.getElementById("guestProgram").value = "";
    document.getElementById("guestRole").value = "";
  }
});

document.getElementById("confirmAssignBtn").addEventListener("click", async () => {

  const isGuest = document.getElementById("isGuest").checked;

  // Datos de guest (si aplica)
  let guestName = null;
  let guestHost = null;
  let guestProgram = null;
  let guestRole = null;

  if (isGuest) {
    guestName = document.getElementById("guestName").value.trim();
    guestHost = document.getElementById("guestHost").value.trim();
    guestProgram = document.getElementById("guestProgram").value.trim();
    guestRole = document.getElementById("guestRole").value;

    if (!guestName) {
      alert("Please enter the guest name.");
      return;
    }
    if (!guestRole) {
      alert("Please select the guest role.");
      return;
    }
  }


  const payload = {
    desk_id: Number(selectedDeskId),
    person_id: isGuest
      ? null
      : (Number(document.getElementById("personSearch").dataset.personId) || null),
    contract_id: isGuest
      ? null
      : (Number(document.getElementById("contractSelect").value) || null),
    is_guest: document.getElementById("isGuest").checked,
    start_date: document.getElementById("assignStartDate").value,
    end_date: document.getElementById("assignEndDate").value || null,
    guest_name: guestName,
    guest_host: guestHost,
    guest_program: guestProgram,
    guest_role: guestRole
  };

  const res = await fetch(`/module/proxy/module2/api?action=assignDesk`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });

  if (!res.ok) {
    alert("Failed to assign desk");
    return;
  }

  bootstrap.Modal.getInstance(
    document.getElementById("assignDeskModal")
  ).hide();

  location.reload();
  //await loadDeskStatus(currentFloor);
});

document.getElementById("reservableDeskBtn").addEventListener("click", async () => {

  const payload = {
    desk_id: Number(selectedDeskId),
  };

  const res = await fetch(`/module/proxy/module2/api?action=reservableDesk`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });

  if (!res.ok) {
    alert("Failed to change desk state");
    return;
  }

  bootstrap.Modal.getInstance(document.getElementById("deskModal")).hide();

  await loadDeskStatus(currentFloor);
});

document.getElementById("freezeDeskBtn").addEventListener("click", async () => {

  const payload = {
    desk_id: Number(selectedDeskId),
  };

  const res = await fetch(`/module/proxy/module2/api?action=freezeDesk`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });

  if (!res.ok) {
    alert("Failed to change desk state");
    return;
  }

  bootstrap.Modal.getInstance(document.getElementById("deskModal")).hide();

  await loadDeskStatus(currentFloor);
});


document.getElementById("applyFilter").addEventListener("click", async () => {
  const s = document.getElementById("filterStart").value;
  const eRaw = document.getElementById("filterEnd").value;

  if (!s) {
    alert("Please set a start date.");
    return;
  }

  // si está vacío, lo tratamos como indefinido
  const e = eRaw ? eRaw : null;

  if (e && parseDayNum(e) < parseDayNum(s)) {
    alert("End date must be after start date.");
    return;
  }

  filterStart = s;
  filterEnd = e; // <-- null si indefinido

  await loadDeskStatus(currentFloor);
});


document.getElementById("clearFilter").addEventListener("click", async () => {
  document.getElementById("filterStart").value = "";
  document.getElementById("filterEnd").value = "";
  filterStart = null;
  filterEnd = null;
  await loadDeskStatus(currentFloor);
});

document.getElementById("pauseDeskBtn").addEventListener("click", () => {
  // prefill con el filtro actual si existe
  const rs = filterStart || todayStr();
  const re = (filterEnd || filterStart || todayStr());

  document.getElementById("releaseStartDate").value = rs;
  document.getElementById("releaseEndDate").value = re;

  openModalAfterHidden(deskModalEl, pauseDeskModalEl);
});

document.getElementById("confirmReleaseBtn").addEventListener("click", async () => {
  const s = document.getElementById("releaseStartDate").value;
  const e = document.getElementById("releaseEndDate").value;

  if (!s || !e) { alert("Please set both dates."); return; }
  if (parseDayNum(e) < parseDayNum(s)) { alert("End date must be after start date."); return; }

  const payload = {
    desk_id: Number(selectedDeskId),
    start_date: s,
    end_date: e
  };

  const res = await fetch(`/module/proxy/module2/api?action=releaseDesk`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });

  if (!res.ok) {
    const t = await res.text();
    alert("Failed to release desk: " + t);
    return;
  }

  bootstrap.Modal.getInstance(document.getElementById("pauseDeskModal")).hide();
  await loadDeskStatus(currentFloor);
});


function zoomToElement(el) {
    if (currentFloor !== 0) return;
    if (!currentSvg || !originalViewBox) return;

    const padding = 100;
    const box = el.getBBox();

    const x = box.x - padding;
    const y = box.y - padding;
    const w = box.width + padding * 2;
    const h = box.height + padding * 2;

    currentSvg.setAttribute("viewBox", `${x} ${y} ${w} ${h}`);
}


function todayStr() {
  const d = new Date();
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const da = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${da}`;
}

(async () => {
  const t = todayStr();
  document.getElementById("filterStart").value = t;
  document.getElementById("filterEnd").value = t;
  filterStart = t;
  filterEnd = t;

  currentFloor = 0;
  await loadFloor(0);
  updateZoomUI();
})();

const deskConflictsEl = document.getElementById("deskConflicts");
if (!deskConflictsEl) {
  console.warn("DEBUG: #deskConflicts not found in DOM!");
} else {
  console.log("DEBUG: #deskConflicts listener attached.");

  deskConflictsEl.addEventListener("click", async (e) => {
    console.log("DEBUG: click inside #deskConflicts, target =", e.target);

    const toggleBtn = e.target.closest(".occ-toggle-edit");
    const saveBtn = e.target.closest(".occ-save-btn");
    const deleteBtn = e.target.closest(".occ-delete-btn");

    if (toggleBtn) {
      console.log("DEBUG: toggleBtn clicked:", toggleBtn);

      const wrapper = toggleBtn.closest(".occ-item");

      if (!wrapper) {
        console.warn("DEBUG: wrapper [data-occ-id] NOT found for toggleBtn");
        return;
      }

      const editArea = wrapper.querySelector(".occ-edit-area");
      if (!editArea) {
        console.warn("DEBUG: .occ-edit-area NOT found inside wrapper");
        return;
      }

      const isHidden = editArea.classList.contains("d-none");
      console.log("DEBUG: before toggle, isHidden =", isHidden);

      editArea.classList.toggle("d-none", !isHidden);
      toggleBtn.textContent = isHidden ? "Cancel" : "Edit";

      console.log("DEBUG: after toggle, isHidden now =", editArea.classList.contains("d-none"));
      return;
    }

    if (saveBtn) {
      console.log("DEBUG: saveBtn clicked:", saveBtn);

      const occId = Number(saveBtn.dataset.occId);
      const kind = saveBtn.dataset.occKind;
      console.log("DEBUG: save occId =", occId, "kind =", kind);

      const wrapper = saveBtn.closest(".occ-item");

      if (!wrapper) {
        console.warn("DEBUG: wrapper [data-occ-id] NOT found for saveBtn");
        return;
      }

      const startInput = wrapper.querySelector(".occ-start-input");
      const endInput = wrapper.querySelector(".occ-end-input");

      console.log("DEBUG: startInput =", startInput, "endInput =", endInput);

      const newStart = startInput?.value;
      const newEnd = endInput?.value || null;

      console.log("DEBUG: newStart =", newStart, "newEnd =", newEnd);

      if (!newStart) {
        alert("Start date is required.");
        return;
      }
      if (newEnd && parseDayNum(newEnd) < parseDayNum(newStart)) {
        alert("End date must be after start date.");
        return;
      }

      const action ="updateAssignment"

      const payload = {
        id: Number(occId),
        start_date: newStart,
        end_date: newEnd
      };

      const guestNameInput = wrapper.querySelector(".occ-guest-name");
      const guestRoleInput = wrapper.querySelector(".occ-guest-role");
      const guestHostInput = wrapper.querySelector(".occ-guest-host");
      const guestProgramInput = wrapper.querySelector(".occ-guest-program");

      if (guestNameInput || guestRoleInput || guestHostInput || guestProgramInput) {
        const guestName = guestNameInput ? guestNameInput.value.trim() : "";
        const guestRole = guestRoleInput ? guestRoleInput.value : "";
        const guestHost = guestHostInput ? guestHostInput.value.trim() : "";
        const guestProgram = guestProgramInput ? guestProgramInput.value.trim() : "";

        payload.guest_name = guestName;
        payload.guest_role = guestRole;
        payload.guest_host = guestHost;
        payload.guest_program = guestProgram;

        console.log("DEBUG: guest payload fields:", {
          guest_name: guestName,
          guest_role: guestRole,
          guest_host: guestHost,
          guest_program: guestProgram
        });
      }

      console.log("DEBUG: sending update action =", action, "payload =", payload);

      const res = await fetch(`/module/proxy/module2/api?action=${action}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });

      console.log("DEBUG: update response status =", res.status);

      if (!res.ok) {
        const t = await res.text();
        console.error("DEBUG: update failed:", t);
        alert("Failed to update period: " + t);
        return;
      }

      await loadDeskStatus(currentFloor);
      openDeskModal(selectedDeskSvgId, false);
      return;
    }

    if (deleteBtn) {
      console.log("DEBUG: deleteBtn clicked:", deleteBtn);

      const occId = Number(deleteBtn.dataset.occId);
      const kind = deleteBtn.dataset.occKind;
      console.log("DEBUG: delete occId =", occId, "kind =", kind);

      if (!confirm("Do you really want to delete this period?")) return;

      const action =
        kind === "assigned"
          ? "deleteAssignment"
          : "deleteRelease";

      const payload = {
        id: Number(occId)
      };

      console.log("DEBUG: sending delete action =", action, "payload =", payload);

      const res = await fetch(`/module/proxy/module2/api?action=${action}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });

      console.log("DEBUG: delete response status =", res.status);

      if (!res.ok) {
        const t = await res.text();
        console.error("DEBUG: delete failed:", t);
        alert("Failed to delete period: " + t);
        return;
      }

      await loadDeskStatus(currentFloor);
      openDeskModal(selectedDeskSvgId, false);
      return;
    }
  });
}

// Cuando se cierre el modal de asignar persona, reseteamos todo
if (assignDeskModalEl) {
  assignDeskModalEl.addEventListener("hidden.bs.modal", () => {
    // Reset campos de persona
    const personInput = document.getElementById("personSearch");
    if (personInput) {
      personInput.disabled = false;
      personInput.value = "";
      delete personInput.dataset.personId;
    }
    resetContractSelection();

    // Reset checkbox guest
    const isGuestChk = document.getElementById("isGuest");
    const guestBox   = document.getElementById("guestDetails");
    if (isGuestChk) isGuestChk.checked = false;
    if (guestBox) guestBox.classList.add("d-none");

    // Reset campos guest
    const guestName    = document.getElementById("guestName");
    const guestHost    = document.getElementById("guestHost");
    const guestProgram = document.getElementById("guestProgram");
    const guestRole    = document.getElementById("guestRole");

    if (guestName)    guestName.value    = "";
    if (guestHost)    guestHost.value    = "";
    if (guestProgram) guestProgram.value = "";
    if (guestRole)    guestRole.value    = "";

    // Reset fechas de asignación
    const assignStart = document.getElementById("assignStartDate");
    const assignEnd   = document.getElementById("assignEndDate");
    if (assignStart) assignStart.value = "";
    if (assignEnd)   assignEnd.value   = "";
  });
}

// Cuando se cierre el modal de pausa, limpiamos fechas
if (pauseDeskModalEl) {
  pauseDeskModalEl.addEventListener("hidden.bs.modal", () => {
    const relStart = document.getElementById("releaseStartDate");
    const relEnd   = document.getElementById("releaseEndDate");
    if (relStart) relStart.value = "";
    if (relEnd)   relEnd.value   = "";
  });
}

if (deskModalEl) {
  deskModalEl.addEventListener("hidden.bs.modal", () => {
    // limpiar conflictos y selección
    const conflictsBox = document.getElementById("deskConflicts");
    if (conflictsBox) conflictsBox.innerHTML = "";
  });
}
