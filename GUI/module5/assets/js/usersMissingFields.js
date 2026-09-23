//////////////////////////////////////////////////////////////////////////////////////////////////
//                               UNEIX WORKERS MISSING FIELDS                                   //
//////////////////////////////////////////////////////////////////////////////////////////////////

import { formSources, userFieldSections } from './fieldConfig.js';
import { formOptions, fetchOptions } from './formOptions.js';
import { SPANISH_EQUIVALENCES} from './exportUneixData.js';
import { exportUneixCSV } from './exportUneixData.js';
import { getFieldConfig } from './fieldConfig.js';

// =========================
// CONFIGURACIÓN DE GERENTE
// =========================
const MANAGER_STORAGE_KEY = "currentManagerId";
let managerId = localStorage.getItem(MANAGER_STORAGE_KEY)
  ? Number(localStorage.getItem(MANAGER_STORAGE_KEY))
  : 98; // valor por defecto (Gemma)

const DIRECTOR_STORAGE_KEY = "currentDirectorId";
let directorId = localStorage.getItem(DIRECTOR_STORAGE_KEY)
  ? Number(localStorage.getItem(DIRECTOR_STORAGE_KEY))
  : 158; // valor por defecto (Carme)

function setManagerId(newId) {
  managerId = newId;
  localStorage.setItem(MANAGER_STORAGE_KEY, newId);
}

function setDirectorId(newId) {
  directorId = newId;
  localStorage.setItem(DIRECTOR_STORAGE_KEY, newId);
}

// =========================
// MODAL PERSONALIZADO YES / NO
// =========================
function showYesNoModal(message) {
  return new Promise(resolve => {
    // Si ya existe un modal previo, lo eliminamos
    const existing = document.getElementById("yesNoModal");
    if (existing) existing.remove();

    const modal = document.createElement("div");
    modal.id = "yesNoModal";
    modal.className =
      "position-fixed top-0 start-0 w-100 h-100 bg-dark bg-opacity-50 d-flex justify-content-center align-items-center";
    modal.style.zIndex = 2000;

    modal.innerHTML = `
      <div class="bg-white rounded shadow p-4" style="max-width: 400px;">
        <p class="mb-4">${message}</p>
        <div class="d-flex justify-content-end gap-2">
          <button id="yesBtn" class="btn btn-primary btn-sm">Yes</button>
          <button id="noBtn" class="btn btn-secondary btn-sm">No</button>
        </div>
      </div>
    `;

    document.body.appendChild(modal);

    const yesBtn = modal.querySelector("#yesBtn");
    const noBtn = modal.querySelector("#noBtn");

    yesBtn.addEventListener("click", () => {
      modal.remove();
      resolve(true);
    });

    noBtn.addEventListener("click", () => {
      modal.remove();
      resolve(false);
    });
  });
}

// =========================
// MODAL PARA SELECCIONAR NUEVO GERENTE
// =========================
function showManagerSelectModal(workers) {
  return new Promise(resolve => {
    // Eliminar si ya existe
    const existing = document.getElementById("managerSelectModal");
    if (existing) existing.remove();

    // Crear modal
    const modal = document.createElement("div");
    modal.id = "managerSelectModal";
    modal.className =
      "position-fixed top-0 start-0 w-100 h-100 bg-dark bg-opacity-50 d-flex justify-content-center align-items-center";
    modal.style.zIndex = 2001;

    modal.innerHTML = `
      <div class="bg-white rounded shadow p-4" style="max-width: 450px; width: 100%;">
        <h5 class="mb-3">Select the new manager</h5>
        <input list="managerList" id="managerInput" class="form-control mb-3" placeholder="Start typing a name...">
        <datalist id="managerList">
          ${workers
            .map(
              w => `<option data-id="${w.people_id}" value="${w.people_name} ${w.surname || ""}"></option>`
            )
            .join("")}
        </datalist>
        <div class="d-flex justify-content-end gap-2">
          <button id="cancelManager" class="btn btn-secondary btn-sm">Cancel</button>
          <button id="confirmManager" class="btn btn-primary btn-sm">Confirm</button>
        </div>
      </div>
    `;

    document.body.appendChild(modal);

    const input = modal.querySelector("#managerInput");
    const confirmBtn = modal.querySelector("#confirmManager");
    const cancelBtn = modal.querySelector("#cancelManager");
    const datalist = modal.querySelector("#managerList");

    confirmBtn.addEventListener("click", () => {
      const opt = Array.from(datalist.options).find(o => o.value === input.value);
      if (opt && opt.dataset.id) {
        modal.remove();
        resolve(Number(opt.dataset.id));
      } else {
        alert("Please select a valid name from the list.");
      }
    });

    cancelBtn.addEventListener("click", () => {
      modal.remove();
      resolve(null);
    });
  });
}


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
      if (year < 2025 || year > currentYear) {
        alert(`Year must be between 2025 and ${currentYear}.`);
        return;
      }

      modal.remove();
      resolve(year);
    });
  });
}




window.addEventListener("load", async () => {

    await fetchOptions();
    const container = document.getElementById("workers-container");
  
    try {
      const res = await fetch("/module/proxy/module5/api?action=missingMandatoryFields");
      const data = await res.json();
      const isVisitor = (p) => p?.visitor === true || p?.general?.visitor === true;

      const workers = data.filter(p => !isVisitor(p));
      const visitors = data.filter(p => isVisitor(p));

      console.log("Fetched data:", data);
      let csvLines = [];
      const filename = `missingFields.csv`;
      const exportBtn = document.getElementById("exportCsvBtn");
      
      // Filtramos los trabajadores completos (sin missing_fields)
      const completeWorkers = data.filter(w => !w.missing_fields || w.missing_fields.length === 0);

      // Si hay trabajadores completos, habilitamos el botón
      if (completeWorkers.length > 0) {
        exportBtn.disabled = false;
      } else {
        exportBtn.disabled = true;
      }

      exportBtn.addEventListener("click", async () => { 
        const currentYear = new Date().getFullYear();
        const defaultYear = currentYear - 1;

        const selectedYear = await showYearSelectModal(defaultYear);
        if (!selectedYear) {
          // Usuario ha cancelado el diálogo de año
          return;
        }

        const resYear = await fetch(`/module/proxy/module5/api?action=missingMandatoryFields&year=${selectedYear}`);
        const dataForYear = await resYear.json();

        const completeWorkers = dataForYear.filter(w => !w.missing_fields || w.missing_fields.length === 0);
        if (completeWorkers.length === 0) {
          alert("No complete workers to export for that year!");
          return;
        }

      
        // Buscar gerente actual en TODOS los trabajadores (data)
        const currentManager = data.find(w => Number(w.people_id) === Number(managerId));
        const defaultManagerName = currentManager
          ? `${currentManager.people_name || ""} ${currentManager.surname || ""}`.trim()
          : `Actual manager (ID ${managerId})`;

      showYesNoModal(`Is ${defaultManagerName} still manager of the center?`).then(async stillManager => {
        if (!stillManager) {
          // Mostrar modal con lista autocompletable de personas
          const newManagerId = await showManagerSelectModal(data);
      
          if (newManagerId) {
            setManagerId(newManagerId);
            const newManager = data.find(p => Number(p.people_id) === newManagerId);
            alert(`New manager assigned: ${newManager.people_name} ${newManager.surname}`);
          } else {
            alert("Selection cancelled. Keeping previous manager.");
          }
        }

        // Buscar director actual en TODOS los trabajadores (data)
        const currentDirector = data.find(w => Number(w.people_id) === Number(directorId));
        const defaultDirectorName = currentDirector
          ? `${currentDirector.people_name || ""} ${currentDirector.surname || ""}`.trim()
          : `Actual director (ID ${directorId})`;

        await showYesNoModal(`Is ${defaultDirectorName} still director of the center?`).then(async stillDirector => {
          if (!stillDirector) {
            // Mostrar modal con lista autocompletable de personas
            const newDirectorId = await showManagerSelectModal(data);

            if (newDirectorId) {
              setDirectorId(newDirectorId);
              const newDirector = data.find(p => Number(p.people_id) === newDirectorId);
              alert(`New director assigned: ${newDirector.people_name} ${newDirector.surname}`);
            } else {
              alert("Selection cancelled. Keeping previous director.");
            }
          }

          // Exportar
          exportUneixCSV(completeWorkers, managerId, directorId, selectedYear);
        });
      });
    })
      if (!data.length) {
        document.getElementById("exportMissingFieldsBtn").disabled = true;
        container.innerHTML = `
          <div class="col">
            <div class="alert alert-success text-center">
              ✅ All active workers have complete mandatory fields.
            </div>
          </div>`;
        return;
      }
      const sortByName = (a, b) => {
        const surnameA = `${a.surname || ""} ${a.secondSurname || ""}`.trim().toLowerCase();
        const surnameB = `${b.surname || ""} ${b.secondSurname || ""}`.trim().toLowerCase();
        if (surnameA === surnameB) {
          const nameA = (a.people_name || "").toLowerCase();
          const nameB = (b.people_name || "").toLowerCase();
          return nameA.localeCompare(nameB);
        }
        return surnameA.localeCompare(surnameB);
      };

      workers.sort(sortByName);
      visitors.sort(sortByName);

      // Para pintar la lista: workers primero, luego visitors
      const sortedAll = [...workers, ...visitors];

      const incompleteWorkers = sortedAll.filter(p => p.missing_fields && p.missing_fields.length > 0);


      if (!incompleteWorkers.length) {
        document.getElementById("exportMissingFieldsBtn").disabled = true;
        container.innerHTML = `
          <div class="col">
            <div class="alert alert-success text-center">
              ✅ All active workers have complete mandatory fields.
            </div>
          </div>`;
        return;
      }

      const exportMissingBtn = document.getElementById("exportMissingFieldsBtn");
      exportMissingBtn.disabled = incompleteWorkers.length === 0;

      exportMissingBtn.addEventListener("click", () => {
        const rows = incompleteWorkers.map(w => {
          const fullName = `${w.people_name || "—"} ${w.surname || ""} ${w.secondSurname || ""}`.trim();

          const labels = (w.missing_fields || []).map(missingFieldLabel).join("; ");
          return [w.people_id, fullName, labels];
        });

        downloadCSV("missing_mandatory_fields.csv", [
          ["people_id", "full_name", "missing_fields"],
          ...rows
        ]);
      });


      const cards = incompleteWorkers.map((worker, i) => {
        const {
          people_id,
          web_user_id,
          people_name,
          surname,
          secondSurname,
          missing_fields = [],
          general = {}
        } = worker;
        
        const visitor = isVisitor(worker);

        const fullName = `${people_name || "—"} ${surname || ""} ${secondSurname || ""}`.trim();

        const imgSrc = general.picture_path
          ? `/module/proxy/module5/Uploads?file=${encodeURIComponent(general.picture_path)}`
          : "/module/proxy/module5/Uploads?file=/Uploads/00_Users/default_profile.png";
  
        const missingList = missing_fields.length
          ? `<ul class="text-danger small mb-2">
              ${missing_fields
                .map(f => {
                  const cfg = getFieldConfig(f);
                  let label = cfg ? cfg.label : f;
                  // Solo limpiar texto visual, no eliminar índice de `f`
                  label = label
                    .replace(/^people_grade\[\d+\]\./, "")
                    .replace(/^contract\[\d+\]\./, "")
                    .replace(/^people\./, "")
                    .replace(/^people_nationality\./, "");
                  return `<li data-field="${f}">${label}</li>`;
                })
                .join("")}
            </ul>`
          : `<p class="text-success small">No missing fields</p>`;

          const badge = visitor
          ? `<span class="badge bg-warning text-dark ms-2">Visitor</span>`
          : `<span class="badge bg-primary ms-2">Worker</span>`;

          const canEdit = visitor && missing_fields.length > 0;
          let visitorID = null;
          let visitorKeyType = null; // "WU" o "VI"

          if (visitor) {
            const wuid = Number(worker.web_user_id);
            if (!Number.isNaN(wuid) && wuid !== -1) {
              visitorID = wuid;
              visitorKeyType = "WU";
            } else {
              const vid = Number(worker.visit_id);
              if (!Number.isNaN(vid)) {
                visitorID = vid;
                visitorKeyType = "VI";
              }
            }
          }


          return `
          <div class="col">
            <div class="card h-100 shadow-sm">
              <div class="card-body">
                <div class="d-flex align-items-center mb-2">
                  <img src="${imgSrc}" class="rounded me-3" width="60" height="60" alt="Profile picture">
                  <div>
                    <h5 class="mb-1">${fullName} ${badge}</h5>
                  </div>
                  ${canEdit ? `
                    <button class="btn btn-sm btn-outline-warning ms-2 js-edit-visitor"
                      data-key-type="${visitorKeyType ?? ""}"
                      data-user-id="${visitorID ?? ""}">
                      Edit
                    </button>
                  ` : ""}

                </div>
                ${missingList}
              </div>
            </div>
          </div>`;
      });
      
      container.innerHTML = cards.join("");

      container.addEventListener("click", (e) => {
        const btn = e.target.closest(".js-edit-visitor");
        if (!btn) return;

        const raw = btn.getAttribute("data-user-id");
        const type = btn.getAttribute("data-key-type"); // "WU" o "VI"
        const id = Number(raw);

        if (!raw || Number.isNaN(id) || !type) {
          console.warn("Invalid visitor button data", { raw, type });
          return;
        }

        const person = incompleteWorkers.find(p => {
          if (type === "WU") return Number(p.web_user_id) === id;
          if (type === "VI") return Number(p.visit_id) === id;
          return false;
        });

        if (!person) {
          console.warn("Visitor not found", { type, id });
          return;
        }

        openVisitorModalSimple(person, { type, id });
      });



  
    } catch (err) {
      console.error("Error fetching data:", err);
      container.innerHTML = `
        <div class="col">
          <div class="alert alert-danger text-center">
            ❌ Error loading data. Check the console for details.
          </div>
        </div>`;
    }

  });
  
 

  function csvEscape(value) {
    const s = String(value ?? "");
    return `"${s.replace(/"/g, '""')}"`;
  }
  
  function downloadCSV(filename, rows) {
    const content = rows.map(r => r.map(csvEscape).join(",")).join("\n");
    const blob = new Blob([content], { type: "text/csv;charset=utf-8;" });
  
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = filename;
    link.click();
  }
  
  function missingFieldLabel(fieldKey) {
    const cfg = getFieldConfig(fieldKey);
    let label = cfg ? cfg.label : fieldKey;
  
    // Solo limpieza visual
    return label
      .replace(/^people_grade\[\d+\]\./, "")
      .replace(/^contract\[\d+\]\./, "")
      .replace(/^people\./, "")
      .replace(/^people_nationality\./, "");
  }


const VISITOR_FIELDS = [

  { key: "people_name", label: "Name", type: "text"  },
  { key: "surname", label: "Surname" },
  { key: "secondSurname", label: "Second surname", type: "text"  },
  { key: "general.nif", label: "Nif", type: "text"  },
  { key: "general.status", label: "Status", type: "select", optionsKey: "status" },
  { key: "general.birth_date", label: "Birth date", type: "date" },
  { key: "nationalities[0].nationality_code", label: "Nationality", type: "select", optionsKey: "nationalities" },
  { key: "general.birth_country", label: "Birth country", type: "select", optionsKey: "countries" },
  { key: "general.birth_province", label: "Birth province", type: "select", optionsKey: "provinces" },
  { key: "general.birth_city", label: "Birth city", type: "select", optionsKey: "cities" },
  { key: "general.first_incorporation_year", label: "First registered", type: "date" },
  { key: "general.gender", label: "Gender", type: "select", optionsKey: "gender" },
  { key: "general.orcid", label: "ORCID", type: "text" },
  { key: "grades[0].graduation_university", label: "Grade university", type: "select", optionsKey: "universities" },
  { key: "grades[0].graduation_country", label: "Grade country", type: "select", optionsKey: "countries" },
  { key: "grades[0].graduation_year", label: "Grade year", type: "number" },
  { key: "grades[1].graduation_university", label: "Doctorate university", type: "select", optionsKey: "universities" },
  { key: "grades[1].graduation_country", label: "Doctorate country", type: "select", optionsKey: "countries" },
  { key: "grades[1].graduation_year", label: "Doctorate year", type: "number" },
  { key: "contracts[0].contracting_institution", label: "Institution", type: "select", optionsKey: "universities" },
  { key: "contracts[0].start_date", label: "Start date", type: "date" },
  { key: "contracts[0].end_date", label: "End date", type: "date" },
  { key: "web_user_id", label: "Web user id", readonly: true, type: "text" },
  { key: "visit_id", label: "Visit id", readonly: true, type: "text" },
];

function getByPath(obj, path) {
  const parts = String(path).split(".");
  let cur = obj;

  for (const part of parts) {
    if (cur == null) return "";

    const m = part.match(/^(\w+)(?:\[(\d+)\])?$/); // grades[0]
    if (!m) return "";

    const key = m[1];
    const idx = m[2] != null ? Number(m[2]) : null;

    cur = cur[key];
    if (idx != null) {
      if (!Array.isArray(cur)) return "";
      cur = cur[idx];
    }
  }

  if (cur == null) return "";
  return String(cur);
}

function normalizeDateToInput(v) {
  const s = String(v ?? "").trim();
  if (!s) return "";
  // "YYYY-MM-DD..." -> recorta
  if (/^\d{4}-\d{2}-\d{2}/.test(s)) return s.slice(0, 10);
  // "YYYY" -> (opción A) convertir a YYYY-01-01 para que el date lo acepte
  if (/^\d{4}$/.test(s)) return `${s}-01-01`;
  return "";
}

// formOptions puede traer items como {code,name} o {value,label} etc.
function getOptionsByKey(optionsKey) {
  const list = formOptions?.[optionsKey];
  return Array.isArray(list) ? list : [];
}

function optionValue(opt) {
  return opt.code ?? opt.value ?? opt.id ?? opt.key ?? "";
}
function optionLabel(opt) {
  return opt.name ?? opt.label ?? opt.text ?? String(optionValue(opt));
}

function normalizeOrcidRaw(v) {
  // deja solo dígitos y X (último carácter puede ser X)
  return String(v ?? "")
    .trim()
    .toUpperCase()
    .replace(/[^0-9X]/g, "")
    .slice(0, 16);
}

function formatOrcid(v) {
  const raw = normalizeOrcidRaw(v);
  if (!raw) return "";
  // agrupa cada 4: 0000-0000-0000-0000
  return raw.replace(/(.{4})(?=.)/g, "$1-");
}

function openVisitorModalSimple(person, keyInfo) {
  // eliminar modal previo
  const prev = document.getElementById("visitorMissingModal");
  if (prev) prev.remove();

  const idLabel = keyInfo?.type === "VI" ? "Visit" : "User";
  const idValue = keyInfo?.id ?? person.web_user_id ?? person.visit_id ?? "—";

  const userId = person.web_user_id;
  const fullName = `${person.people_name || "—"} ${person.surname || ""} ${person.secondSurname || ""}`.trim();
  const missing = Array.isArray(person.missing_fields) ? person.missing_fields : [];

  const modal = document.createElement("div");
  modal.id = "visitorMissingModal";
  modal.className = "position-fixed top-0 start-0 w-100 h-100 bg-dark bg-opacity-50 d-flex justify-content-center align-items-center";
  modal.style.zIndex = 99999;

  const inputsHtml = VISITOR_FIELDS.map((f) => {
    const path = f.key; // aquí f.key es path (general.xxx, grades[0].xxx...)
    const rawVal = getByPath(person, path);
    const ro = f.readonly ? "disabled" : "";
    const type = f.type || "text";

    // missing: por sufijo o por alias textual
    const isMissing = missing.some(m => {
      const ms = String(m).toLowerCase().trim();
      const ps = String(path).toLowerCase().trim();

      if (ms === ps) return true;
      if (ms === String(f.label || "").toLowerCase()) return true;

      const mSuffix = ms.includes(".") ? ms.split(".").pop() : ms;   // nif
      const pSuffix = ps.includes(".") ? ps.split(".").pop() : ps;   // nif
      return mSuffix === pSuffix;
    });

    const badge = isMissing ? `<span class="badge bg-danger ms-2">missing</span>` : "";

    // DATE
    if (type === "date") {
      const dateVal = normalizeDateToInput(rawVal);
      return `
        <div class="mb-2">
          <label class="form-label small mb-1">${escapeHTML(f.label || path)} ${badge}</label>
          <input class="form-control form-control-sm"
            name="${escapeAttr(path)}"
            type="date"
            value="${escapeAttr(dateVal)}"
            ${ro}
          />
        </div>
      `;
    }

    // SELECT
    if (type === "select") {
      const options = getOptionsByKey(f.optionsKey);
      let current = String(rawVal ?? "").trim();

      const useEquivalence =
        (f.optionsKey === "universities" || f.optionsKey === "institution"); // ajusta si tu key es otra

      const is98 = current === "98";

      const optsHtml = options.map(opt => {
        const v = String(optionValue(opt)).trim();
        const l = String(optionLabel(opt));

        let selected = "";

        if (v === current) {
          selected = "selected";
        } else if (useEquivalence && !is98) {
          const eq = String(SPANISH_EQUIVALENCES?.[v] ?? "").trim();
          if (eq && eq === current) selected = "selected";
        }

        return `<option value="${escapeAttr(v)}" ${selected}>${escapeHTML(l)}</option>`;
      }).join("");

      return `
        <div class="mb-2">
          <label class="form-label small mb-1">${escapeHTML(f.label || path)} ${badge}</label>
          <select class="form-select form-select-sm js-tomselect"
            name="${escapeAttr(path)}"
            data-options-key="${escapeAttr(f.optionsKey || "")}"
            ${ro}
          >
            <option value="">-- select --</option>
            ${optsHtml}
          </select>
        </div>
      `;
    }

    // NUMBER
    if (type === "number") {
      return `
        <div class="mb-2">
          <label class="form-label small mb-1">${escapeHTML(f.label || path)} ${badge}</label>
          <input class="form-control form-control-sm"
            name="${escapeAttr(path)}"
            type="number"
            value="${escapeAttr(rawVal)}"
            ${ro}
          />
        </div>
      `;
    }

    // TEXT (default)
    const displayVal =
      path === "general.orcid"
        ? formatOrcid(rawVal)
        : rawVal;

    return `
      <div class="mb-2">
        <label class="form-label small mb-1">${escapeHTML(f.label || path)} ${badge}</label>
        <input class="form-control form-control-sm"
          name="${escapeAttr(path)}"
          type="text"
          value="${escapeAttr(displayVal)}"
          ${ro}
          ${path === "general.orcid" ? 'inputmode="numeric" placeholder="0000-0000-0000-0000"' : ""}
        />
      </div>
    `;
  }).join("");



  modal.innerHTML = `
    <div class="bg-white rounded shadow p-4" style="max-width: 700px; width: 95%; max-height: 90vh; overflow:auto;">
      <div class="d-flex justify-content-between align-items-start mb-3">
        <div>
          <h5 class="mb-1">Edit visitor missing fields</h5>
          <div class="small text-muted">${escapeHTML(idLabel)} #${escapeHTML(idValue)} — ${escapeHTML(fullName)}</div>
        </div>
        <button type="button" class="btn btn-sm btn-outline-secondary" id="closeVisitorModal">Close</button>
      </div>


      <form id="visitorMissingForm">
        <div class="fw-semibold mb-2">Fill fields</div>
        ${inputsHtml}

        <div class="d-flex justify-content-end gap-2 mt-3">
          <button type="button" class="btn btn-sm btn-outline-secondary" id="cancelVisitorModal">Cancel</button>
          <button type="submit" class="btn btn-sm btn-warning" disabled>Save</button>
        </div>

        <div id="visitorMissingMsg" class="small mt-3 alert alert-info py-2 mb-0">
          Saving changes for visitors is not implemented yet. Please update visitor data from the intranet.
        </div>
      </form>

      <div id="visitorMissingMsg" class="small mt-2"></div>
    </div>
  `;

  document.body.appendChild(modal);

  // Activar TomSelect si existe en la página
  if (window.TomSelect) {
    modal.querySelectorAll("select.js-tomselect:not([disabled])").forEach(sel => {
      if (sel.tomselect) return; // por si reabres
      new TomSelect(sel, {
        create: false,
        allowEmptyOption: true,
        sortField: { field: "text", direction: "asc" },
        maxOptions: 1000,
      });
    });
  }


  
  const closeWithCleanup = () => {
    document.removeEventListener("keydown", onKeyDown);
    modal.remove();
  };
  modal.querySelector("#closeVisitorModal").addEventListener("click", closeWithCleanup);
  modal.querySelector("#cancelVisitorModal").addEventListener("click", closeWithCleanup);

  modal.addEventListener("mousedown", (e) => {
    if (e.target === modal) closeWithCleanup();
  });

  const onKeyDown = (e) => {
    if (e.key === "Escape") closeWithCleanup();
  };
  document.addEventListener("keydown", onKeyDown);

  modal.querySelector("#visitorMissingForm").addEventListener("submit", (e) => {
    e.preventDefault();

    const msg = modal.querySelector("#visitorMissingMsg");
    msg.className = "small mt-3 alert alert-info py-2 mb-0";
    msg.textContent =
      "Saving changes for visitors is not implemented yet. Please update visitor data from the intranet.";
  });


}

// Helpers mínimos para evitar XSS/HTML roto
function escapeHTML(s) {
  return String(s ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}
function escapeAttr(s) {
  return escapeHTML(s).replaceAll("`", "&#096;");
}

function buildRawFormPayload(formEl, keyInfo) {
  const fd = new FormData(formEl);

  const fields = {};
  for (const [k, v] of fd.entries()) {
    fields[k] = String(v ?? ""); 
  }

  return {
    key: {
      type: keyInfo?.type,
      id: String(keyInfo?.id ?? ""),
    },
    fields
  };
}

async function saveVisitorRawToBackend(payload) {
  const url = "/module/proxy/module5/api?action=updateVisitor";

  console.log(payload)
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  let json = null;
  try { json = await res.json(); } catch (_) {}

  if (!res.ok) {
    const msg = (json && (json.error || json.message)) || `HTTP ${res.status}`;
    throw new Error(msg);
  }
  return json;
}
