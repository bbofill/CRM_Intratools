//////////////////////////////////////////////////////////////////////////////////////////////////
//               FORM MODAL FORMATS: sections, field data types, orders...                      //
//////////////////////////////////////////////////////////////////////////////////////////////////


import { formSources, userFieldSections } from './fieldConfig.js';
import { formOptions, fetchOptions } from './formOptions.js';
import { setupFieldDependencies, updateFieldDependencies } from "./fieldDependencies.js";
import { setupPhotoUpload } from "./photoUpload.js";


let currentEditingId = null;
let phoneIti = {};
let shouldReloadOnClose = false;


export function openUneixFormModal(existingData = {}) {
  const modalTitle = document.getElementById("addUserModalLabel");
  const isEditing = !!currentEditingId;

  if (modalTitle) {
    modalTitle.textContent = isEditing ? "Modify User" : "Add User";
  }
  // Forzar valores por defecto en nuevo usuario
  if (!currentEditingId) {  // solo si es alta nueva
    existingData.active = 1;
    existingData.agreesToUneix = 1;
  }
  const tabList = document.getElementById("formWizardTabs");
  const tabContent = document.getElementById("formWizardContent");
  tabList.innerHTML = "";
  tabContent.innerHTML = "";

  // Formats all sections except the ones that belong inside other sections
  const sectionNames = Object.keys(userFieldSections).filter(s => s !== "Residence" && s !== "Nationality" && s !== "Responsible" && s !== "Supervisor");

  sectionNames.forEach((section, index) => {
    const tabId = `tab-${section}`;
    const isActive = index === 0 ? "active" : "";

    tabList.innerHTML += `
      <li class="nav-item" role="presentation">
        <button class="nav-link ${isActive}" data-bs-toggle="tab" data-bs-target="#${tabId}" type="button">${section}</button>
      </li>`;

    const tabPane = document.createElement("div");
    tabPane.className = `tab-pane fade ${isActive ? "show active" : ""}`;
    tabPane.id = tabId;

    const formGroup = document.createElement("div");
    formGroup.className = "row g-3";
    tabPane.appendChild(formGroup);
    tabContent.appendChild(tabPane);

    // Profile picture
    if (section === 'General') {
      formGroup.appendChild(createProfilePictureField(existingData));
    }

    // Formats sections with multiple entries
    const repeaterOnlySections = ["Contract", "Education", "Phd", "Training", "Groups"];

    const isRepeaterOnly = repeaterOnlySections.includes(section);

    userFieldSections[section].forEach(field => {
      if (field.type === "repeater") {
        renderRepeaterSection(section, field, formGroup, existingData);
        return;
      }

      // Repeater sections are formated latter
      if (isRepeaterOnly) return;

      if (section === "Phd" && field.key === "responsible") return;

      const div = document.createElement("div");
      div.className = field.type === "boolean" ? "col-12" : "col-md-6";
      div.innerHTML = renderField(field, existingData[field.key] || "", section);
      formGroup.appendChild(div);
    });


  });

  // Navigation between sections
  initTabNavigation(tabList);

  fetchOptions().then(() => {
    bootstrap.Modal.getOrCreateInstance(document.getElementById("addUserModal")).show();
  });

  setupFieldDependencies();
  updateFieldDependencies();
  setupPhotoUpload();

}

// ----------------- FIELD RENDER -----------------

function getFieldOptions(field, currentValue) {
  const source = formSources[field.key];
  const sources = Array.isArray(source) ? source : [source];
  let options = sources.flatMap(src => formOptions[src] || []);

  // Caso especial: fundings/subvencions
  if (sources.includes("subvencions")) {
    const currentCode = String(currentValue || "").trim();

    options = options.filter(opt =>
      opt.funding_active || String(opt.code) === currentCode
    );

    // Marcar visualmente los inactivos
    options = options.map(opt => ({
      ...opt,
      name:
        !opt.funding_active && String(opt.code) === currentCode
          ? `${opt.name} [inactive]`
          : opt.name
    }));
  }

  return options;
}

function renderField(field, currentValue, section, index = null) {
  const source = formSources[field.key];

  const disabledAttr = field.readonly ? "disabled" : "";

  // Construir name y id dinámicos
  const baseName = index !== null
    ? `${section.toLowerCase()}[${index}][${field.key}]`
    : field.key;

  // --- BOOLEAN ---
  if (field.type === "boolean") {
    const checked = currentValue == 1 || currentValue === true ? "checked" : "";
    return `
      <div class="form-check mt-2">
        <input class="form-check-input" type="checkbox" name="${baseName}" id="${baseName}" ${checked}>
        <input type="hidden" name="${baseName}_hidden" value="0">
        <label class="form-check-label" for="${baseName}">${field.label}</label>
      </div>`;
  }

  // --- DATE ---
  if (field.type === "date") {
    const value = currentValue?.split?.("T")[0] || currentValue?.split?.(" ")[0] || currentValue;
    return `<label class="form-label">${field.label}</label>
            <input type="date" class="form-control" name="${baseName}" value="${value}">`;
  }

  // --- FILE ---
  if (field.type === "file") {
    const existingPath = currentValue || "";
    let preview = "";
    if (existingPath) {
      const fileUrl = `/module/proxy/module4/Uploads?file=${encodeURIComponent(existingPath)}`;
      preview = `
        <div class="mt-2 d-flex align-items-center gap-2 existing-file-preview">
          <i class="bi bi-file-earmark-pdf text-danger fs-4"></i>
          <a href="${fileUrl}" target="_blank">${existingPath.split("/").pop()}</a>
          <button type="button" class="btn btn-sm btn-outline-danger ms-auto clear-existing-file-btn">
            Remove
          </button>
        </div>`;
    }
  
    return `
      <div class="file-field-wrapper">
      <label class="form-label">${field.label}</label>
      <input type="file" class="form-control"
             name="${baseName}" 
             data-existing-file="${existingPath}"
             accept=".pdf">
      <div class="mt-2 d-none align-items-center gap-2 selected-file-preview">
        <i class="bi bi-file-earmark-arrow-up text-primary fs-4"></i>
        <span class="selected-file-name"></span>
        <button type="button" class="btn btn-sm btn-outline-secondary ms-auto clear-selected-file-btn">
          Clear
        </button>
      </div>
      <input type="hidden" name="${baseName}_existing" value="${existingPath}">
      ${preview}
      </div>`;
  }
  
  
  // --- PHONE ---
  if (["user_phone", "emergencyContact_phone"].includes(field.key)) {
    const telId = `${baseName}_${Date.now()}`;
    const safeValue = (currentValue || "").toString();
    setTimeout(() => initPhoneField(telId, safeValue), 0);
    return `
      <label class="form-label">${field.label}</label>
      <br>
      <input type="tel" class="form-control" id="${telId}" name="${baseName}" value="${safeValue.replace('+', '').replace(' ', '')}" />
    `;
  }

  // --- YEAR ---
  if (field.type === "year") {
    const value = currentValue || "";
    const currentYear = new Date().getFullYear();
    return `
      <label class="form-label">${field.label}</label>
      <input type="number" class="form-control" name="${baseName}" value="${value}" 
             min="1900" max="${currentYear + 10}" step="1" placeholder="YYYY">
    `;
  }

  // --- HOURS ---
  if (field.key === "totalDedication_hours") {
    const rawValue = currentValue ? (parseFloat(currentValue) / 100).toFixed(2) : "";
    const displayId = `${baseName}_display`;
    setTimeout(() => initTotalDedication(displayId, baseName, currentValue), 0);
    return `
      <label class="form-label">${field.label}</label>
      <input type="text" class="form-control" id="${displayId}" placeholder="e.g. 37.5 h" value="${rawValue}">
      <input type="hidden" name="${baseName}" id="${baseName}" value="${currentValue}">
      <div class="form-text">Enter hours per week (decimals allowed).</div>
    `;
  }

  // --- DATALIST / SELECT ---
  if (source) {
    const options = getFieldOptions(field, currentValue);
    const match = options.find(opt => String(opt.code) === String(currentValue));
    const displayValue = match ? match.name : "";
    //const codeValue = match ? match.code : currentValue;
    const codeValue = match ? match.code : (currentValue || "");
  
    // ⚡ Generar IDs únicos
    const uniqueId = `${field.key}_${Date.now()}_${Math.floor(Math.random() * 1000)}`;
    const listId = `list-${uniqueId}`;
  
    setTimeout(() => initDatalist(uniqueId, listId), 0);
  
    return `
      <label class="form-label">${field.label}</label>
      <input type="text" class="form-control" id="${uniqueId}_display" 
             value="${displayValue}" list="${listId}" autocomplete="off" ${disabledAttr}>
      <input type="hidden" name="${baseName}" id="${uniqueId}" value="${codeValue}">
      <datalist id="${listId}">
        ${options.map(opt => `<option data-code="${opt.code}" value="${opt.name}"></option>`).join("")}
      </datalist>
    `;
  }
  // --- ORCID ---
  if (field.key === "orcid") {
    const orcidId = `${baseName}_${Date.now()}`;

    // Formatear el valor actual si está sin guiones
    let formattedValue = (currentValue || "").replace(/[^0-9X]/gi, "");
    if (formattedValue.length === 16) {
      formattedValue = formattedValue.replace(/(.{4})(?=.)/g, "$1-");
    }

    setTimeout(() => initOrcidField(orcidId), 0);
    return `<label class="form-label">${field.label}</label>
            <input type="text" class="form-control" id="${orcidId}" name="${baseName}" 
                  value="${formattedValue}" placeholder="0000-0000-0000-0000"/>`;
  }

  // --- TEXT DEFAULT ---
  return `<label class="form-label">${field.label}</label>
          <input type="text" class="form-control" name="${baseName}" value="${currentValue}" />`;
}

function renderRepeaterSection(section, field, formGroup, existingData) {
  const type = field.key.charAt(0).toUpperCase() + field.key.slice(1);

  // Responsible (repeater) is inside Phd (repeater)
  // so it can't be rendered independently
  if (section === "Phd" && type === "Responsible") {
    return; 
  }
  if (section === "Contract" && type === "Supervisor") {
    return; 
  }


  const containerId = `${field.key}-container`;
  if (document.getElementById(containerId)) return;

  const container = document.createElement("div");
  container.id = containerId;
  container.className = "col-12 mt-3";

  const header = document.createElement("div");
  header.className = "d-flex justify-content-between align-items-center mb-2";
  header.innerHTML = `<h6>${type}(s)</h6>
    <button type="button" class="btn btn-sm btn-outline-primary" id="add${type}Btn">+ Add ${field.key}</button>`;
  container.appendChild(header);

  formGroup.appendChild(container);

  const rawList = Array.isArray(existingData[field.key]) ? existingData[field.key] : [];
// Si no hay contratos, deja 1 bloque vacío para poder añadir
const dataList = rawList.length ? rawList.slice() : [{}];

if (type === "Contract" && rawList.length) {
  // Fecha de inicio robusta (por si alguna entrada usa otra clave o viene vacía)
  const getStartRaw = (c) =>
    c?.start_date ??
    c?.contract_start_date ??
    c?.startDate ??
    c?.start ??
    null;

  const parseDate = (val) => {
    if (!val) return null;
    // Asegura “YYYY-MM-DD” (si viene con hora la recorta)
    const iso = String(val).split("T")[0];
    const d = new Date(`${iso}T00:00:00Z`);
    return isNaN(d.getTime()) ? null : d;
  };

  const getStart = (c) => parseDate(getStartRaw(c));

  // Ordenar por start_date: antiguo → nuevo. Sin fecha → al final.
  dataList.sort((a, b) => {
    const da = getStart(a);
    const db = getStart(b);
    if (da && db) return da - db;
    if (da && !db) return -1;  // con fecha antes que sin fecha
    if (!da && db) return 1;
    return 0;
  });

  // Encontrar el más reciente por start_date
  let latestItem = null;
  let latestTime = -Infinity;
  dataList.forEach((c) => {
    const d = getStart(c);
    if (d && d.getTime() > latestTime) {
      latestTime = d.getTime();
      latestItem = c;
    }
  });

  // Render: más nuevo arriba, y resaltar SOLO el más reciente
  [...dataList].reverse().forEach((item) => {
    const isCurrent = latestItem ? item === latestItem : false;
    addDynamicFormBlock(type, item, null, null, isCurrent);
  });
} else {
  // Otras secciones o no hay contratos reales: render tal cual
  dataList.forEach(item => addDynamicFormBlock(type, item));
}


  setTimeout(() => {
    document.getElementById(`add${type}Btn`)?.addEventListener("click", () => addDynamicFormBlock(type));
  }, 0);
}

function addDynamicFormBlock(type, existing = {}, containerId = null, parentIndex = null, isLatest = false) {
  const section = userFieldSections[type];
  if (!section) return;

  const container = containerId
    ? containerId instanceof HTMLElement
      ? containerId
      : document.getElementById(containerId)
    : document.getElementById(`${type.toLowerCase()}-container`);

  if (!container) {
    console.error(`Container for ${type} not found`);
    return;
  }

  const index = container.querySelectorAll(`.${type.toLowerCase()}-block`).length;

  const wrapper = document.createElement("div");
  wrapper.className = `${type.toLowerCase()}-block border rounded p-3 mb-3 bg-light position-relative`;

  if (type === "Contract" && isLatest) {
    wrapper.classList.add("bg-highlight", "border-primary");
  } else {
    wrapper.classList.add("bg-light");
  }

  if (type === "Contract" && existing.contract_id) {
    const contractIdInput = document.createElement("input");
    contractIdInput.type = "hidden";
    contractIdInput.name = `contract[${index}][contract_id]`;
    contractIdInput.value = existing.contract_id;
    wrapper.appendChild(contractIdInput);
  }

  
  const deleteBtn = document.createElement("button");
  deleteBtn.type = "button";
  deleteBtn.className = "btn-close position-absolute top-0 end-0 m-2";
  deleteBtn.onclick = () => wrapper.remove();
  wrapper.appendChild(deleteBtn);

  const row = document.createElement("div");
  row.className = "row g-3";

  section.forEach(field => {
    if (field.key === type.toLowerCase()) return;
    if (type === "Phd" && field.key === "responsible") return; 
    if (type === "Contract" && field.key === "supervisor") return; 

    const div = document.createElement("div");
    div.className = "col-md-6";
    div.innerHTML = renderField(field, existing[field.key] || "", type, index);
    row.appendChild(div);
  });

  wrapper.appendChild(row);
  container.appendChild(wrapper);

  if (type === "Phd") {
    initResponsibleSubsection(wrapper, existing, index);
  }

  if (type === "Contract") {
    initSupervisorSubsection(wrapper, existing, index);
  }

  updateFieldDependencies?.();
}


// ----------------- RESPONSIBLE -----------------
function initResponsibleSubsection(wrapper, existing, index) {
  let responsibleContainer = wrapper.querySelector(".responsibles-container");
  if (!responsibleContainer) {
    responsibleContainer = document.createElement("div");
    responsibleContainer.className = "responsibles-container mt-3";
    responsibleContainer.dataset.index = index;
    const header = document.createElement("div");
    header.className = "d-flex justify-content-between align-items-center mb-2";
    header.innerHTML = `<h6>Responsible(s)</h6>
      <button type="button" class="btn btn-sm btn-outline-primary">+ Add responsible</button>`;
    const addBtn = header.querySelector("button");
    addBtn.addEventListener("click", () => addDynamicFormBlock("Responsible", {}, responsibleContainer, index));
    responsibleContainer.appendChild(header);
    wrapper.appendChild(responsibleContainer);
  }

  const responsibles = existing.responsible || [{}];
  responsibles.forEach(r => addDynamicFormBlock("Responsible", r, responsibleContainer, index));
}

// ----------------- Supervisor -----------------
function initSupervisorSubsection(wrapper, existing, index) {
  let supervisorContainer = wrapper.querySelector(".supervisors-container");
  if (!supervisorContainer) {
    supervisorContainer = document.createElement("div");
    supervisorContainer.className = "supervisors-container mt-3";
    supervisorContainer.dataset.index = index;
    const header = document.createElement("div");
    header.className = "d-flex justify-content-between align-items-center mb-2";
    header.innerHTML = `<h6>Supervisor(s)</h6>
      <button type="button" class="btn btn-sm btn-outline-primary">+ Add supervisor</button>`;
    const addBtn = header.querySelector("button");
    addBtn.addEventListener("click", () => addDynamicFormBlock("Supervisor", {}, supervisorContainer, index));
    supervisorContainer.appendChild(header);
    wrapper.appendChild(supervisorContainer);
  }

  const supervisors = existing.supervisor || [{}];
  supervisors.forEach(s => addDynamicFormBlock("Supervisor", s, supervisorContainer, index));
}

// ----------------- HELPERS -----------------
function createProfilePictureField(existingData) {
  const container = document.createElement("div");
  container.className = "mb-4";
  const imgSrc = existingData.picture_path
    ? `/module/proxy/module4/Uploads?file=${encodeURIComponent(existingData.picture_path)}`
    : `/module/proxy/module4/Uploads?file=${encodeURIComponent("Uploads/00_Users/default_profile.png")}`;

  container.innerHTML = `
    <label class="form-label">Profile picture</label>
    <input type="file" class="form-control mb-2" id="upload-photo" name="profile_picture" accept="image/*">
    <div class="text-center mt-2">
      <img id="profile-img" src="${imgSrc}"
        class="rounded-circle" style="width: 150px; height: 150px; object-fit: cover;">
    </div>
  `;
  return container;
}

function initTabNavigation(tabList) {
  const nextBtn = document.getElementById("nextTab");
  const prevBtn = document.getElementById("prevTab");

  const tabs = Array.from(tabList.querySelectorAll("button"));

  function getActiveTabIndex() {
    return tabs.findIndex(btn => btn.classList.contains("active"));
  }

  function isTabEnabled(btn) {
    return !btn.disabled && !btn.classList.contains("d-none") && btn.offsetParent !== null;
  }

  nextBtn.onclick = () => {
    let i = getActiveTabIndex();
    while (i < tabs.length - 1) {
      i++;
      if (isTabEnabled(tabs[i])) {
        tabs[i].click();
        break;
      }
    }
  };

  prevBtn.onclick = () => {
    let i = getActiveTabIndex();
    while (i > 0) {
      i--;
      if (isTabEnabled(tabs[i])) {
        tabs[i].click();
        break;
      }
    }
  };
}


function initPhoneField(telId, currentValue) {
  const input = document.getElementById(telId);
  if (!input || phoneIti[telId]) return;
  phoneIti[telId] = window.intlTelInput(input, {
    initialCountry: "es",
    nationalMode: true,         
    separateDialCode: true,
    utilsScript: "/assets/libs/intl-tel-input/build/js/utils.js",
  });
  if (currentValue) {
    phoneIti[telId].setNumber(currentValue);
  }
}

function initOrcidField(orcidId) {
  const input = document.getElementById(orcidId);
  if (!input) return;

  input.addEventListener("input", function () {
    let value = this.value.replace(/[^0-9X]/gi, "");
    if (value.length > 16) value = value.substring(0, 16);
    this.value = value.replace(/(.{4})(?=.)/g, "$1-");
  });
}

function initTotalDedication(displayId, hiddenId, currentValue) {
  const display = document.getElementById(displayId);
  const hidden = document.getElementById(hiddenId);
  if (display && hidden) {
    display.addEventListener("input", () => {
      const raw = display.value.replace(/[^\d.,]/g, "").replace(",", ".");
      const floatVal = parseFloat(raw);
      hidden.value = !isNaN(floatVal) ? Math.round(floatVal * 100) : "";
    });
  }
}
function initDatalist(uniqueId, listId) {
  const display = document.getElementById(`${uniqueId}_display`);
  const hidden = document.getElementById(uniqueId);
  const datalist = document.getElementById(listId);

  if (!display || !hidden || !datalist) {
    console.warn("initDatalist: elementos no encontrados", { uniqueId, listId });
    return;
  }

  function syncHiddenAndValidity() {
    const entered = normalizeText(display.value);

    const allOptions = [...datalist.querySelectorAll("option")];
    const match = allOptions.find(opt => normalizeText(opt.value) === entered);

    if (match) {
      hidden.value = match.dataset.code || match.value;
      display.classList.remove("is-invalid");
    } else {
      hidden.value = "";
      if (display.value.trim() !== "") {
        display.classList.add("is-invalid");
      } else {
        display.classList.remove("is-invalid");
      }
    }

    updateFieldDependencies?.();
  }

  display.addEventListener("change", syncHiddenAndValidity);
  display.addEventListener("blur", syncHiddenAndValidity);
  display.addEventListener("input", syncHiddenAndValidity);

  // Asegura que, si ya tiene texto al cargar, se sincroniza
  setTimeout(syncHiddenAndValidity, 100);
}

function normalizeText(str) {
  return str
    .normalize("NFD")             // separa letras y tildes
    .replace(/\p{Diacritic}/gu, "")  // quita diacríticos
    .toLowerCase()
    .trim();
}

function validateDatalistFields(formEl) {
  const invalid = [];

  formEl.querySelectorAll("input[list]").forEach(display => {
    // Ignorar campos deshabilitados o readonly
    if (display.disabled || display.readOnly || display.classList.contains("disabled")) return;

    const hidden = display.parentElement.querySelector("input[type='hidden']");
    if (!hidden) return;

    const hasText = display.value.trim() !== "";
    const hasCode = hidden.value && hidden.value.trim() !== "";

    const isInvalid = hasText && !hasCode;

    if (isInvalid) {
      display.classList.add("is-invalid");
      if (!display.nextElementSibling || !display.nextElementSibling.classList.contains("invalid-feedback")) {
        const fb = document.createElement("div");
        fb.className = "invalid-feedback";
        fb.textContent = "Select a correct value from the list.";
        display.insertAdjacentElement("afterend", fb);
      }
      invalid.push(display);
    } else {
      display.classList.remove("is-invalid");
      const fb = display.nextElementSibling;
      if (fb && fb.classList.contains("invalid-feedback")) fb.remove();
    }
  });

  return invalid;
}



// ----------------- SUBMIT -----------------
document.getElementById("addUneixForm").addEventListener("submit", function (e) {
  e.preventDefault();

  // ----------- VALIDACIÓN OBLIGATORIA ------------
    const invalids = validateDatalistFields(this);
  if (invalids.length) {
    const first = invalids[0];
    first.scrollIntoView({ behavior: "smooth", block: "center" });
    first.focus();
    alert("There are fields with invalid values. Please select an option from the list.");
    return; // Cancela el submit
  }

  const nameInput = this.querySelector("input[name='people_name']");
  const surnameInput = this.querySelector("input[name='surname']");

  const name = nameInput ? nameInput.value.trim() : "";
  const surname = surnameInput ? surnameInput.value.trim() : "";

  if (!name || !surname) {
    alert("At least name and surname must be filled.");
    return;  // cancela el submit
  }
  // ----------- VALIDACIÓN EDUCATION ------------
  const educationBlocks = Array.from(document.querySelectorAll("#education-container .education-block"))
    .filter(block => block.offsetParent !== null && !block.classList.contains("template"));

  if (educationBlocks.length > 0) {
    let invalidEducation = false;

    educationBlocks.forEach(block => {
      const typeOfDegree = block.querySelector("input[name*='[grade_master_doctorate]'], select[name*='[grade_master_doctorate]']");
      const area = block.querySelector("input[name*='[grade_code]'], select[name*='[grade_code]']");

      const typeOfDegreeVal = typeOfDegree?.value?.trim() || "";
      const areaVal = area?.value?.trim() || "";

      if (!typeOfDegreeVal || !areaVal) {
        invalidEducation = true;

        // marcar los campos vacíos en rojo
        [typeOfDegree, area].forEach(el => {
          if (el && !el.value.trim()) {
            el.classList.add("is-invalid");
            if (!el.nextElementSibling || !el.nextElementSibling.classList.contains("invalid-feedback")) {
              const fb = document.createElement("div");
              fb.className = "invalid-feedback";
              fb.textContent = "This field is required.";
              el.insertAdjacentElement("afterend", fb);
            }
          }
        });
      }
    });

    if (invalidEducation) {
      alert("Please fill in both 'Type of degree' and 'Area' in all Education blocks.");
      return; 
    }
  }

  // ----------- VALIDACIÓN PhD ------------
  const phdBlocks = Array.from(document.querySelectorAll("#phd-container .phd-block"))
    .filter(block => block.offsetParent !== null && !block.classList.contains("template"));

  if (phdBlocks.length > 0) {
    let invalidPhd = false;

    phdBlocks.forEach(block => {
      const program = block.querySelector("input[name*='[phd_program]'], select[name*='[phd_program]']");
      const programVal = program?.value?.trim() || "";

      if (!programVal) {
        invalidPhd = true;

        // marcar campo como inválido
        program.classList.add("is-invalid");
        if (!program.nextElementSibling || !program.nextElementSibling.classList.contains("invalid-feedback")) {
          const fb = document.createElement("div");
          fb.className = "invalid-feedback";
          fb.textContent = "PhD program is required.";
          program.insertAdjacentElement("afterend", fb);
        }
      }
    });

    if (invalidPhd) {
      alert("Please fill in 'PhD Program' in all PhD blocks.");
      return; 
    }
  }

  // Validar que solo contengan letras, espacios o guiones
  const invalidCharsRegex = /[^a-zA-ZÀ-ÿ\s'-]/;

  if (invalidCharsRegex.test(name) || invalidCharsRegex.test(surname)) {
    alert("Name and surname cannot contain special characters or numbers.");
    return;  // cancela el submit
  }
  
  // ----------- GENERAL ------------
  const general = {};
  this.querySelectorAll("input[name]:not([type='file']):not([type='checkbox']), select[name], textarea[name]").forEach(input => {
    // Si hay un input hidden asociado al datalist, usar ese valor
    const hidden = document.getElementById(input.name);
    if (hidden && hidden.type === "hidden" && document.getElementById(`${input.name}_display`)) {
      general[input.name] = hidden.value || "";  // usa el code
    } else if (input.classList.contains("iti__tel-input")) {
      const iti = phoneIti[input.id];
      const { dialCode } = iti.getSelectedCountryData(); // ej: 34
      const prefix = `+${dialCode}`;
      general[input.name] = `${prefix} ${input.value}`;

    } else if (input.name.includes("orcid")) {
      general[input.name] = input.value.replace(/-/g, "");
    } else {
      general[input.name] = input.value;
    }
  });

  this.querySelectorAll("input[type='checkbox'][name]").forEach(cb => {
    general[cb.name] = cb.checked ? 1 : 0;
  });

  // ----------- REPETERS ------------
  const sections = ["residence", "nationality", "contract", "education", "phd", "training", "groups"];
  const formData = { general };

  sections.forEach(type => {
    formData[type] = [];
    document.querySelectorAll(`#${type}-container .${type}-block`).forEach(block => {
      const blockData = {};
      block.querySelectorAll("input[name], select[name], textarea[name]").forEach(input => {
        const keyMatch = input.name.match(/\[([^\]]+)\]$/);
        if (keyMatch) {
          const key = keyMatch[1];
          if (input.type === "checkbox") {         
            blockData[key] = input.checked ? 1 : 0;
            return;
          }
          /*
          const hidden = document.getElementById(input.name);
          if (hidden && hidden.type === "hidden" && document.getElementById(`${input.name}_display`)) {
            blockData[key] = hidden.value || "";
          } else {
            blockData[key] = input.value;
          }*/
            /*
            const hidden = block.querySelector(`input[type="hidden"][name="${input.name}"]`);
            let valueToSend = input.value;
            
            // Si existe un hidden asociado, buscamos su display por su ID SEGURO
            if (hidden) {
                const hiddenId = hidden.id;
            
                // IDs válidos solo tienen letras/números/-/_
                if (/^[A-Za-z0-9._-]+$/.test(hiddenId)) {
                    const display = block.querySelector(`#${hiddenId}_display`);
                    if (display) {
                        // Es un datalist → usar ALWAYS el hidden
                        valueToSend = hidden.value || "";
                    }
                }
            }
            
            blockData[key] = valueToSend;*/
            if (input.tagName === "INPUT" && input.list) {
              const hidden = block.querySelector(`input[type="hidden"][name="${input.name}"]`);
          
              if (hidden) {
                  blockData[key] = hidden.value || "";
                  return;
              }
          }
          
          // Otros tipos de input: usar su valor normal
          blockData[key] = input.value;
        }
      });
      if (type === "phd") {
        blockData.responsibles = [];
        block.querySelectorAll(".responsibles-container .responsible-block").forEach(respBlock => {
          const respData = {};
          respBlock.querySelectorAll("input[name], select[name], textarea[name]").forEach(input => {
            const keyMatch = input.name.match(/\[([^\]]+)\]$/);
            if (keyMatch) {
              const key = keyMatch[1];
              respData[key] = input.value;
            }
          });
          blockData.responsibles.push(respData);
        });
      }
      if (type === "contract") {
        blockData.supervisors = [];
        block.querySelectorAll(".supervisors-container .supervisor-block").forEach(respBlock => {
          const respData = {};
          respBlock.querySelectorAll("input[name], select[name], textarea[name]").forEach(input => {
            const keyMatch = input.name.match(/\[([^\]]+)\]$/);
            if (keyMatch) {
              const key = keyMatch[1];
              respData[key] = input.value;
            }
          });
          blockData.supervisors.push(respData);
        });
      }
      formData[type].push(blockData);
    });
  });

// ------------- ENVIAR --------------
const formElement = document.getElementById("addUneixForm");
const fileInput = formElement.querySelector("#upload-photo");
const file = fileInput?.files[0];

const formPayload = new FormData();

// Agregar el archivo de imagen si existe
if (file) {
  formPayload.append("profile_picture", file);
}

// Preparar estructura JSON para datos (sin archivos)
const cleanFormData = { general, contract: [], training: [] };

// Recorrer secciones
sections.forEach(type => {
  const blocks = [];
  document.querySelectorAll(`#${type}-container .${type}-block`).forEach((block, blockIndex) => {
    const blockData = {};
    block.querySelectorAll("input[name], select[name], textarea[name]").forEach(input => {
      const keyMatch = input.name.match(/\[([^\]]+)\]$/);
      if (keyMatch) {
        const key = keyMatch[1];
        if (input.type === "file" && input.files.length > 0) {
          const file = input.files[0];
          const fileKey = `${type}_${blockIndex}_${type}_file`;
          formPayload.append(fileKey, file);
          blockData[`${type}_file_path`] = file.name;
        } else if (input.type === "file" && input.files.length === 0) {
          const hiddenInput = block.querySelector(`input[name="${input.name}_existing"]`);
          if (hiddenInput && hiddenInput.value) {
            blockData[`${type}_file_path`] = hiddenInput.value;
          }
        } else if (input.type === "checkbox") {
          blockData[key] = input.checked ? 1 : 0;
        } else {
          blockData[key] = input.value;
        }
      }
    });
    // Recolectar responsables si es un bloque de phd
    if (type === "phd") {
      blockData.responsibles = [];
      block.querySelectorAll(".responsibles-container .responsible-block").forEach(respBlock => {
        const respData = {};
        respBlock.querySelectorAll("input[name], select[name], textarea[name]").forEach(input => {
          const keyMatch = input.name.match(/\[([^\]]+)\]$/);
          if (keyMatch) {
            const key = keyMatch[1];
            respData[key] = input.value;
          }
        });
        blockData.responsibles.push(respData);
      });
    }
    if (type === "contract") {
      blockData.supervisors = [];
      block.querySelectorAll(".supervisors-container .supervisor-block").forEach(respBlock => {
        const respData = {};
        respBlock.querySelectorAll("input[name], select[name], textarea[name]").forEach(input => {
          const keyMatch = input.name.match(/\[([^\]]+)\]$/);
          if (keyMatch) {
            const key = keyMatch[1];
            respData[key] = input.value;
          }
        });
        blockData.supervisors.push(respData);
      });
    }
    blocks.push(blockData);
  });
  cleanFormData[type] = blocks;
});


// Finalmente, añadir el JSON
formPayload.append("payload", JSON.stringify({
  id: currentEditingId || null,
  formData: cleanFormData
}));

console.log(cleanFormData)

fetch("/module/proxy/module4/api?action=saveUser", {
  method: "POST",
  body: formPayload
})
.then(res => res.json())
.then((data) => {
  console.log(data);
  currentEditingId = data.people_id; 
  shouldReloadOnClose = true;
  const savedToast = document.createElement("div");
  const deskWarnings = Array.isArray(data.desk_overlap_warnings) ? data.desk_overlap_warnings : [];
  savedToast.className = `alert ${deskWarnings.length ? "alert-warning" : "alert-success"} mt-2`;
  savedToast.textContent = deskWarnings.length
    ? `New data saved correctly, but there are desk overlaps: ${deskWarnings.map(w => w.message).join(" ")}`
    : "New data saved correctly. You may continue editing or close this window.";
  document.getElementById("addUneixForm").prepend(savedToast);
  setTimeout(() => savedToast.remove(), deskWarnings.length ? 9000 : 4000);
})
.catch(err => console.error("Error:", err));
});

// ----------------- EVENT LISTENERS -----------------
document.addEventListener("change", (e) => {
  const fileInput = e.target.closest(".file-field-wrapper input[type='file']");
  if (!fileInput) return;

  const wrapper = fileInput.closest(".file-field-wrapper");
  const selectedPreview = wrapper?.querySelector(".selected-file-preview");
  const selectedName = wrapper?.querySelector(".selected-file-name");
  const file = fileInput.files?.[0];

  if (!selectedPreview || !selectedName) return;

  if (file) {
    selectedName.textContent = file.name;
    selectedPreview.classList.remove("d-none");
    selectedPreview.classList.add("d-flex");
  } else {
    selectedName.textContent = "";
    selectedPreview.classList.add("d-none");
    selectedPreview.classList.remove("d-flex");
  }
});

document.addEventListener("click", async (e) => {
  const clearSelectedFileBtn = e.target.closest(".clear-selected-file-btn");
  if (clearSelectedFileBtn) {
    const wrapper = clearSelectedFileBtn.closest(".file-field-wrapper");
    const fileInput = wrapper?.querySelector("input[type='file']");
    const selectedPreview = wrapper?.querySelector(".selected-file-preview");
    const selectedName = wrapper?.querySelector(".selected-file-name");

    if (fileInput) fileInput.value = "";
    if (selectedName) selectedName.textContent = "";
    if (selectedPreview) {
      selectedPreview.classList.add("d-none");
      selectedPreview.classList.remove("d-flex");
    }
    return;
  }

  const clearFileBtn = e.target.closest(".clear-existing-file-btn");
  if (clearFileBtn) {
    const wrapper = clearFileBtn.closest(".file-field-wrapper");
    const hiddenInput = wrapper?.querySelector("input[type='hidden'][name$='_existing']");
    const fileInput = wrapper?.querySelector("input[type='file']");
    const preview = wrapper?.querySelector(".existing-file-preview");

    if (hiddenInput) hiddenInput.value = "";
    if (fileInput) {
      fileInput.value = "";
      fileInput.dataset.existingFile = "";
    }
    if (preview) {
      preview.classList.remove("align-items-center");
      preview.classList.add("alert", "alert-warning", "py-2", "mb-0");
      preview.innerHTML = "File will be removed when you save.";
    }
    return;
  }

  const btn = e.target.closest("button[data-id]");

  if (btn) {
    const userId = btn.dataset.id;
    currentEditingId = parseInt(userId);

    btn.disabled = true;
    const originalText = btn.innerHTML;
    btn.innerHTML = "Loading...";

    try {
      const res = await fetch(`/module/proxy/module4/api?action=allUneixUsers&id=${encodeURIComponent(userId)}`);

      if (!res.ok) {
        throw new Error(`Error loading user detail: ${res.status}`);
      }

      const data = await res.json();
      const userData = Array.isArray(data) ? data[0] : data;

      await fetchOptions();

      openUneixFormModal(userData || {});
      bootstrap.Modal.getOrCreateInstance(document.getElementById("addUserModal")).show();

    } catch (err) {
      console.error(err);
      alert("Could not load the selected user. Please try again.");
    } finally {
      btn.disabled = false;
      btn.innerHTML = originalText;
    }

    return;
  }

  if (e.target.id === "openAddUserModalBtn") {
    fetchOptions().then(() => {
      openUneixFormModal();
      bootstrap.Modal.getOrCreateInstance(document.getElementById("addUserModal")).show();
    });
  }
});

document.addEventListener("input", (e) => {
  if (e.target.name === "people_name" || e.target.name === "surname") {
    e.target.value = e.target.value.replace(/[^a-zA-ZÀ-ÿ\s'-]/g, "");
  }
});
// ----------------- RESET SAVED ID -----------------
document.getElementById("addUserModal").addEventListener("hidden.bs.modal", () => {
  currentEditingId = null;
  const form = document.getElementById("addUneixForm");
  if (form) {
    form.reset();
  }

  const tabList = document.getElementById("formWizardTabs");
  const tabContent = document.getElementById("formWizardContent");
  if (tabList) tabList.innerHTML = "";
  if (tabContent) tabContent.innerHTML = "";

  if (shouldReloadOnClose) {
    shouldReloadOnClose = false;
    location.reload();
  }
});
