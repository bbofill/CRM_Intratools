//////////////////////////////////////////////////////////////////////////////////////////////////
//                               UNEIX GROUP MEMBERS TABLE CSV                                  //
//////////////////////////////////////////////////////////////////////////////////////////////////
import { getFieldConfig } from "./fieldConfig.js";

function getMissingFieldLabel(fieldKey) {
  // fieldKey llega tipo "people.nif", "people.surname", etc.
  const cfg = getFieldConfig(fieldKey);

  // Ajusta estas keys según tu usersMissingFields.js:
  // - label / title / description / displayName...
  const label =
    cfg?.label ||
    cfg?.title ||
    cfg?.description ||
    cfg?.displayName ||
    null;

  return label ? label : fieldKey;
}

function renderMissingFields(missingFields) {
  if (!Array.isArray(missingFields) || missingFields.length === 0) {
    return `<p class="text-success small mb-2">No missing fields 🎉</p>`;
  }

  const items = missingFields.map(fieldKey => {
    const cfg = getFieldConfig(fieldKey);

    const label = getMissingFieldLabel(fieldKey);
    const desc = cfg?.help || cfg?.hint || cfg?.tooltip || cfg?.descriptionLong || ""; 
    return `
      <li class="d-flex flex-column">
        <div class="d-flex align-items-center justify-content-between">
          <span>${label}</span>
        </div>
        ${desc ? `<div class="text-muted small ms-4">${desc}</div>` : ""}
      </li>
    `;
  }).join("");

  return `
    <div class="mt-2">
      <div class="small fw-semibold text-danger mb-1">Missing mandatory fields</div>
      <ul class="text-danger small mb-2 ps-3">${items}</ul>
    </div>
  `;
}


window.addEventListener("load", async () => {
  const container = document.getElementById("groups-container");
  const exportBtn = document.getElementById("exportCsvBtn");

  try {
    // Obtener miembros incompletos
    const resIncomplete = await fetch("/module/proxy/module5/api?action=missingMandatoryFieldsGroupMembers");
    let incompleteMembers = await resIncomplete.json();
    if (!Array.isArray(incompleteMembers)) incompleteMembers = [];

    // Obtener todos los miembros
    const resAll = await fetch("/module/proxy/module5/api?action=getAllGroupMembers");
    let allMembers = await resAll.json();
    if (!Array.isArray(allMembers)) allMembers = [];

    // Calcular los completos (todos - incompletos)
    const incompleteIds = new Set(incompleteMembers.map(m => m.people_id));
    const completeMembers = allMembers.filter(m => !incompleteIds.has(m.people_id));

    // Habilitar o deshabilitar el botón Export según haya completos
    exportBtn.disabled = completeMembers.length === 0;

    // 5Renderizar miembros incompletos
    if (incompleteMembers.length === 0) {
      container.innerHTML = `
        <div class="col">
          <div class="alert alert-success text-center">
            ✅ All group members have complete mandatory fields.
          </div>
        </div>`;
    } else {
      const cards = incompleteMembers.map((m, i) => {
        const fullName = `${m.people_name || "—"} ${m.surname || ""}`.trim();
        const researchCode = m.research_code || "—";
        const missingList = renderMissingFields(m.missing_fields);
        return `
          <div class="col-md-6">
            <div class="card h-100 shadow-sm">
              <div class="card-body">
                <h5 class="mb-1">${fullName}</h5>
                <p class="text-muted mb-1"><strong>Group:</strong> ${researchCode}</p>
                ${missingList}
              </div>
            </div>
          </div>`;
      });
      container.innerHTML = cards.join("");

      document.querySelectorAll(".edit-btn").forEach(btn => {
        btn.addEventListener("click", e => {
          const index = e.target.dataset.index;
          openEditModal(incompleteMembers[index]);
        });
      });
    }

    // Exportación CSV (solo los completos)
    exportBtn.addEventListener("click", async () => {
      const currentYear = new Date().getFullYear();
      const defaultYear = currentYear - 1;

      const selectedYear = await showYearSelectModal(defaultYear);
      if (!selectedYear) return;

      const resAllYear = await fetch(`/module/proxy/module5/api?action=getAllGroupMembers&year=${selectedYear}`);
      let allMembersYear = await resAllYear.json();
      if (!Array.isArray(allMembersYear)) allMembersYear = [];

      // Reutilizamos incompleteMembers YA cargado (sin año)
      const incompleteIds = new Set(incompleteMembers.map(m => m.people_id));
      const completeMembersYear = allMembersYear.filter(m => !incompleteIds.has(m.people_id));

      if (completeMembersYear.length === 0) {
        alert("No complete members to export for that year.");
        return;
      }

      exportGroupMembersToCSV(completeMembersYear, selectedYear);
    });

  } catch (err) {
    container.innerHTML = `
      <div class="alert alert-danger text-center">
        ❌ Error loading group members data. Check console for details.
      </div>`;
  }
});




// =========================
// MODAL PARA SELECCIONAR AÑO DE EXPORTACIÓN
// =========================
function showYearSelectModal(defaultYear) {
  return new Promise(resolve => {
    const existing = document.getElementById("yearSelectModal");
    if (existing) existing.remove();

    const modal = document.createElement("div");
    modal.id = "yearSelectModal";
    modal.className =
      "position-fixed top-0 start-0 w-100 h-100 bg-dark bg-opacity-50 d-flex justify-content-center align-items-center";
    modal.style.zIndex = 2002;

    const currentYear = new Date().getFullYear();

    modal.innerHTML = `
      <div class="bg-white rounded shadow p-4" style="max-width: 400px; width: 100%;">
        <h5 class="mb-3">Select the reference year</h5>
        <p class="small text-muted mb-2">
          Data will be exported relative to this year.
        </p>
        <input
          id="yearInput"
          type="number"
          class="form-control mb-3"
          min="2000"
          max="${currentYear}"
          value="${defaultYear}"
        >
        <div class="d-flex justify-content-end gap-2">
          <button id="cancelYear" class="btn btn-secondary btn-sm">Cancel</button>
          <button id="confirmYear" class="btn btn-primary btn-sm">Confirm</button>
        </div>
      </div>
    `;

    document.body.appendChild(modal);

    const yearInput = modal.querySelector("#yearInput");
    const cancelBtn = modal.querySelector("#cancelYear");
    const confirmBtn = modal.querySelector("#confirmYear");

    cancelBtn.addEventListener("click", () => {
      modal.remove();
      resolve(null); 
    });

    confirmBtn.addEventListener("click", () => {
      const year = Number(yearInput.value);
      const currentYear = new Date().getFullYear();

      if (!year || Number.isNaN(year)) {
        alert("Please enter a valid year.");
        return;
      }
      if (year < 2000 || year > currentYear) {
        alert(`Year must be between 2000 and ${currentYear}.`);
        return;
      }

      modal.remove();
      resolve(year);
    });
  });
}


// =============================================================
// EXPORTAR CSV — solo miembros completos
// =============================================================
function exportGroupMembersToCSV(members, year) {
  const shortYear = String(year).slice(-2);


  const lines = members.map(m => {
    const nif = m.nif || "";
    const nifExtended = m.nif_extended || m.nif;
    const code = m.research_code || "";
    const ipRaw = (m.ip ?? "").toString().trim();
    const ip = ipRaw === "1" ? "S" : "N";
    return `${year}|0000001672|${code}|${nifExtended}|${nif}|${ip}|P`;
  });

  const content = lines.join("\n");
  const blob = new Blob([content], { type: "text/plain;charset=utf-8" });

  const fileName = `RM${shortYear}52.csv`;
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = fileName;
  a.click();
  URL.revokeObjectURL(a.href);

  alert(`✅ Exported ${members.length} group member records successfully!`);
}
