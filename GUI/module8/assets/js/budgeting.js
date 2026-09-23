import {
  SECTION_BUILDERS,
  CATEGORY_ID_TO_KEY,
  escapeHtml,
  formatDateTime,
  extractRequestData,
  buildTravelIdentityBlock
} from "./categoryOptions.js";

import {
  loadSentRequests,
  REQUESTS_CACHE,
  getCurrentModalRequest
} from "./sentRequests.js";


// ---------------- DOM ELEMENTS ----------------

const accordion = document.getElementById("details-accordion");
const submitBtn = document.getElementById("submit-btn");
const resetBtn = document.getElementById("reset-btn");

const viewRejectionBox = document.getElementById("view-rejection-box");
const viewRejectionText = document.getElementById("view-rejection-text");

const checks = Array.from(document.querySelectorAll(".category-check"));
const equipmentCheck = document.getElementById("cat-equipment");

const editModalEl = document.getElementById("editRequestModal");
export const editModal = new bootstrap.Modal(editModalEl);
export const editAccordion = document.getElementById("edit-details-accordion");

const editChecks = Array.from(document.querySelectorAll(".edit-category-check"));
export const editEquipmentCheck = document.getElementById("edit-cat-equipment");

export const saveEditBtn = document.getElementById("save-edit-btn");
const approveManageBtn = document.getElementById("approve-manage-btn");
const rejectManageBtn = document.getElementById("reject-manage-btn");

const managementRejectBox = document.getElementById("management-reject-box");
const managementRejectComment = document.getElementById("management-reject-comment");

const editRequestModalLabel = document.getElementById("editRequestModalLabel");
const viewRequestsBody = document.getElementById("viewRequestsBody");


// ---------------- GLOBAL STATE ----------------

const FILES_STORE = new Map();
export const EDIT_FILES_STORE = new Map();

const FORM_STATE = new Map();
const EDIT_FORM_STATE = new Map();

let PROJECTS_CACHE = [];
let ALL_PROJECTS_CACHE = [];
let MANAGEMENT_REQUESTS_CACHE = {};
let BUDGET_PERMISSIONS_CACHE = null;

export let CURRENT_EDIT_REQUEST_ID = null;
export let CURRENT_MODAL_MODE = "edit"; // "edit" | "manage" | "view"

function createEmptyTravelContactInfo() {
  return {
    loaded: false,
    hasPhone: false,
    phone: "",
    phoneSource: "",
    hasValidPassport: false,
    hasPassportData: false,
    hasValidDocument: false,
    passport: null
  };
}

const TRAVEL_CONTACT_INFO_BY_SCOPE = {
  main: createEmptyTravelContactInfo(),
  edit: createEmptyTravelContactInfo()
};


// ---------------- CONSTANTS ----------------

const REJECTED_STATUSES = new Set([
  "rejected",
  "rejected-ip",
  "rejected-projects",
  "rejected-it",
  "rejected-accounting"
]);

const OPTIONAL_FIELD_NAMES = new Set([
  "travel_observations",
  "travel_preferences_files",
  "registration_observations",
  "registration_invoice",
  "accommodation_observations",
]);

const TEXT_FIELD_LIMITS = {
  travel_from_where: 150,
  travel_to_where: 150,
  institution: 150,
  travel_purpose: 300,
  travel_observations: 1500,

  registration_event: 300,
  registration_observations: 1500,

  accommodation_where: 200,
  accommodation_purpose: 300,
  accommodation_observations: 1500,

  equipment_other_text: 200,
  equipment_description: 1500,

  other_description: 1500,

  management_reject_comment: 1500
};


// ---------------- CATEGORY SELECTION ----------------

function getSelectedCategories() {
  return checks.filter(c => c.checked).map(c => c.value);
}

export function getEditSelectedCategories() {
  return editChecks.filter(c => c.checked).map(c => c.value);
}

function hasTravelSelected(selectedCategories = []) {
  return selectedCategories.includes("travel");
}


// ---------------- CATEGORY LOCKING ----------------

function enforceEquipmentLocking() {
  const selected = getSelectedCategories();
  const anyNonEquipment = selected.some(v => v !== "equipment");

  if (anyNonEquipment) {
    equipmentCheck.checked = false;
    equipmentCheck.disabled = true;
  } else {
    equipmentCheck.disabled = false;
  }
}

function enforceEditEquipmentLocking() {
  const selected = getEditSelectedCategories();
  const anyNonEquipment = selected.some(v => v !== "equipment");

  if (anyNonEquipment) {
    editEquipmentCheck.checked = false;
    editEquipmentCheck.disabled = true;
  } else {
    editEquipmentCheck.disabled = false;
  }
}

function enforceOtherLockingIfEquipmentSelected() {
  const selected = getSelectedCategories();
  const hasEquipment = selected.includes("equipment");

  checks.forEach(c => {
    if (c.value === "equipment") return;

    if (hasEquipment) {
      c.checked = false;
      c.disabled = true;
    } else {
      c.disabled = false;
    }
  });
}

function enforceEditOtherLockingIfEquipmentSelected() {
  const selected = getEditSelectedCategories();
  const hasEquipment = selected.includes("equipment");

  editChecks.forEach(c => {
    if (c.value === "equipment") return;

    if (hasEquipment) {
      c.checked = false;
      c.disabled = true;
    } else {
      c.disabled = false;
    }
  });
}


// ---------------- SECTION RENDERING ----------------

function renderSections(selected) {
  accordion.innerHTML = "";

  selected.forEach((key) => {
    const builder = SECTION_BUILDERS[key];
    if (builder) {
      accordion.insertAdjacentHTML("beforeend", builder("main", "details-accordion"));
    }
  });

  if (selected.includes("travel")) {
    const travelBody = document.getElementById("main-sec-travel-body");

    if (travelBody && !travelBody.querySelector('[data-travel-identity-block="main"]')) {
      travelBody.insertAdjacentHTML("beforeend", buildTravelIdentityBlock("main"));
      setupTravelIdentityBehavior(accordion, "main");
      setupTravelLuggageLogic(accordion);
    }
  }

  // Reaplicamos comportamientos después de pintar el HTML dinámico
  setupProcurementWarnings(accordion);
  setupRegistrationBehavior();
  setupEquipmentOtherBehavior();
  populateProjectSelects(accordion, PROJECTS_CACHE);
  setupProjectIPDependency(accordion, FORM_STATE, "main");
  setupFileInputs(accordion, FILES_STORE, "main");
  setupDateConstraints(accordion);

  bindFormStateTracking(accordion, FORM_STATE, "main");
  restoreFormState(accordion, FORM_STATE, "main");

  applyTextFieldLimits(accordion);
  applyDynamicRequired(accordion);

  if (selected.length) {
    const last = selected[selected.length - 1];
    const collapse = document.getElementById(`main-sec-${last}-body`);
    if (collapse) new bootstrap.Collapse(collapse, { toggle: true });
  }
}

function renderEditSections(selected) {
  editAccordion.innerHTML = "";

  selected.forEach((key) => {
    const builder = SECTION_BUILDERS[key];

    if (builder) {
      let html = builder("edit", "edit-details-accordion");

      html = html
        .replace(/id="reg-yes"/g, 'id="edit-reg-yes"')
        .replace(/id="reg-no"/g, 'id="edit-reg-no"')
        .replace(/for="reg-yes"/g, 'for="edit-reg-yes"')
        .replace(/for="reg-no"/g, 'for="edit-reg-no"')
        .replace(/id="pay-yes"/g, 'id="edit-pay-yes"')
        .replace(/id="pay-no"/g, 'id="edit-pay-no"')
        .replace(/for="pay-yes"/g, 'for="edit-pay-yes"')
        .replace(/for="pay-no"/g, 'for="edit-pay-no"')
        .replace(/id="invoice-wrapper"/g, 'id="edit-invoice-wrapper"')
        .replace(/id="registration-invoice-wrapper"/g, 'id="edit-registration-invoice-wrapper"')
        .replace(/id="registration-warning"/g, 'id="edit-registration-warning"')
        .replace(/id="payment-warning"/g, 'id="edit-payment-warning"')
        .replace(/id="equipment-category"/g, 'id="edit-equipment-category"')
        .replace(/id="equipment-other-wrapper"/g, 'id="edit-equipment-other-wrapper"');

      editAccordion.insertAdjacentHTML("beforeend", html);
    }
  });

  if (selected.includes("travel")) {
    const travelBody = document.getElementById("edit-sec-travel-body");

    if (
      canShowTravelIdentityBlock() &&
      travelBody &&
      !travelBody.querySelector('[data-travel-identity-block="edit"]')
    ) {
      travelBody.insertAdjacentHTML("beforeend", buildTravelIdentityBlock("edit"));
      setupTravelIdentityBehavior(editAccordion, "edit");
    }

    setupTravelLuggageLogic(editAccordion);
  }

  setupProcurementWarnings(editAccordion);
  setupEditRegistrationBehavior();
  setupEditEquipmentOtherBehavior();

  const projectsForEditModal =
    CURRENT_MODAL_MODE === "manage" || CURRENT_MODAL_MODE === "view"
      ? ALL_PROJECTS_CACHE
      : PROJECTS_CACHE;

  populateProjectSelects(editAccordion, projectsForEditModal);
  setupProjectIPDependency(editAccordion, EDIT_FORM_STATE, "edit");
  setupFileInputs(editAccordion, EDIT_FILES_STORE, "edit");
  setupDateConstraints(editAccordion);

  bindFormStateTracking(editAccordion, EDIT_FORM_STATE, "edit");
  restoreFormState(editAccordion, EDIT_FORM_STATE, "edit");

  if (selected.length) {
    const first = selected[0];
    const collapse = document.getElementById(`edit-sec-${first}-body`);
    if (collapse) new bootstrap.Collapse(collapse, { toggle: true });
  }

  applyTextFieldLimits(editAccordion);
  applyDynamicRequired(editAccordion);
}

function syncUI() {
  captureFormState(accordion, FORM_STATE, "main");

  enforceOtherLockingIfEquipmentSelected();
  enforceEquipmentLocking();

  const selected = getSelectedCategories();
  renderSections(selected);
}

function syncEditUI() {
  captureFormState(editAccordion, EDIT_FORM_STATE, "edit");

  enforceEditOtherLockingIfEquipmentSelected();
  enforceEditEquipmentLocking();

  renderEditSections(getEditSelectedCategories());

  applyTextFieldLimits(editAccordion);
  applyDynamicRequired(editAccordion);
}


// ---------------- REGISTRATION BEHAVIOUR ----------------

function setupRegistrationBehavior() {
  const yes = document.getElementById("reg-yes");
  const no = document.getElementById("reg-no");
  const invoice = document.getElementById("registration-invoice-wrapper");
  const warning = document.getElementById("registration-warning");

  if (!yes || !no || !invoice || !warning) return;

  function update() {
    const hasChoice = yes.checked || no.checked;

    if (!hasChoice) {
      invoice.classList.add("d-none");
      warning.classList.add("d-none");
      return;
    }

    const registered = yes.checked;

    invoice.classList.toggle("d-none", !registered);
    warning.classList.toggle("d-none", registered);

    applyDynamicRequired(accordion);
  }

  yes.addEventListener("change", update);
  no.addEventListener("change", update);

  update();
}

function setupEditRegistrationBehavior() {
  const yes = document.getElementById("edit-reg-yes");
  const no = document.getElementById("edit-reg-no");
  const invoice = document.getElementById("edit-registration-invoice-wrapper");
  const warning = document.getElementById("edit-registration-warning");

  if (!yes || !no || !invoice || !warning) return;

  function update() {
    const hasChoice = yes.checked || no.checked;

    if (!hasChoice) {
      invoice.classList.add("d-none");
      warning.classList.add("d-none");
      return;
    }

    const registered = yes.checked;

    invoice.classList.toggle("d-none", !registered);
    warning.classList.toggle("d-none", registered);

    applyDynamicRequired(editAccordion);
  }

  yes.addEventListener("change", update);
  no.addEventListener("change", update);

  update();
}


// ---------------- EQUIPMENT BEHAVIOUR ----------------

function setupEquipmentOtherBehavior() {
  const cat = document.getElementById("equipment-category");
  const otherWrap = document.getElementById("equipment-other-wrapper");

  if (!cat || !otherWrap) return;

  function update() {
    otherWrap.classList.toggle("d-none", cat.value !== "other");
    applyDynamicRequired(accordion);
  }

  cat.addEventListener("change", update);
  update();
}

function setupEditEquipmentOtherBehavior() {
  const cat = document.getElementById("edit-equipment-category");
  const otherWrap = document.getElementById("edit-equipment-other-wrapper");

  if (!cat || !otherWrap) return;

  function update() {
    otherWrap.classList.toggle("d-none", cat.value !== "other");
    applyDynamicRequired(editAccordion);
  }

  cat.addEventListener("change", update);
  update();
}


// ---------------- PROCUREMENT WARNINGS ----------------

export function setupProcurementWarnings(container = document) {
  const configs = [
    {
      selectName: "equipment_price_range",
      warningSelector: '[data-procurement-warning="equipment"]',
    },
    {
      selectName: "other_price_range",
      warningSelector: '[data-procurement-warning="other"]',
    },
  ];

  configs.forEach(({ selectName, warningSelector }) => {
    const select = container.querySelector(`[name="${selectName}"]`);
    const warning = container.querySelector(warningSelector);

    if (!select || !warning) return;

    const updateWarning = () => {
      const value = Number(select.value);
      const showWarning = value >= 2;

      warning.classList.toggle("d-none", !showWarning);
    };

    select.addEventListener("change", updateWarning);
    updateWarning();
  });
}


// ---------------- PROJECT & IP BEHAVIOUR ----------------

function getProjectById(id, projects = PROJECTS_CACHE) {
  return projects.find(p => String(p.id) === String(id));
}

export function getProjectLabelFromRequest(request, projects = null) {
  const projectIds = Array.isArray(request.projectIds)
    ? request.projectIds
    : [];

  if (!projectIds.length) return "-";

  const sourceProjects =
    Array.isArray(projects) && projects.length
      ? projects
      : (Array.isArray(ALL_PROJECTS_CACHE) && ALL_PROJECTS_CACHE.length
          ? ALL_PROJECTS_CACHE
          : PROJECTS_CACHE);

  return projectIds
    .map(projectId => {
      const project = getProjectById(projectId, sourceProjects);
      if (!project) return String(projectId);
      return `(${project.id}) ${project.short_name}`;
    })
    .join(", ");
}

function populateProjectSelects(root = document, projects = PROJECTS_CACHE) {
  const selects = root.querySelectorAll("select[name$='_project']");

  selects.forEach(select => {
    if (!Array.isArray(projects) || projects.length === 0) return;

    const currentValue = select.value;
    const first = select.querySelector("option[value='']") || select.options[0];

    select.innerHTML = "";
    if (first) select.appendChild(first);

    projects.forEach(proj => {
      const option = document.createElement("option");
      option.value = proj.id;
      option.textContent = `(${proj.id}) ${proj.short_name}`;
      select.appendChild(option);
    });

    if (currentValue) {
      select.value = currentValue;
    }
  });
}

function setupProjectIPDependency(root = document, store = FORM_STATE, scope = "main") {
  const projectSelects = root.querySelectorAll("select[name$='_project']");

  projectSelects.forEach(projectSelect => {
    if (projectSelect.dataset.ipHooked === "1") return;
    projectSelect.dataset.ipHooked = "1";

    const section = projectSelect.name.replace("_project", "");
    const row = projectSelect.closest(".row");
    const ipWrapper = row ? row.querySelector(`[data-ip-wrapper='${section}']`) : null;
    const ipSelect = ipWrapper ? ipWrapper.querySelector(`select[name='${section}_ip']`) : null;

    if (!ipWrapper || !ipSelect) return;

    function update() {
      const proj = getProjectById(projectSelect.value);

      const storedIPValue = getStoredFieldValue(store, scope, ipSelect.name);
      const candidateIPValue =
        storedIPValue !== undefined
          ? String(storedIPValue ?? "")
          : String(ipSelect.value ?? "");

      ipSelect.innerHTML = `<option value="">Select Responsible...</option>`;
      ipWrapper.classList.add("d-none");

      const ips = proj?.IPs;

      // Si aún no hay proyecto seleccionado, no borres el valor guardado.
      // Esto pasa durante el repintado antes del restore.
      if (!projectSelect.value) {
        applyDynamicRequired(root);
        return;
      }

      if (!Array.isArray(ips) || ips.length === 0) {
        ipSelect.value = "";
        setStoredFieldValue(store, scope, ipSelect.name, "");
        ipSelect.dispatchEvent(new Event("change", { bubbles: true }));
        applyDynamicRequired(root);
        return;
      }

      ips.forEach(ip => {
        if (!ip?.id) return;

        const opt = document.createElement("option");
        opt.value = String(ip.id);
        opt.textContent = `${ip.name ?? ""} ${ip.surname ?? ""}`.trim();
        ipSelect.appendChild(opt);
      });

      const hasCandidateOption = Array.from(ipSelect.options).some(
        opt => String(opt.value) === String(candidateIPValue)
      );

      if (ips.length === 1) {
        ipSelect.value = String(ips[0].id);
        setStoredFieldValue(store, scope, ipSelect.name, ipSelect.value);
        ipSelect.dispatchEvent(new Event("change", { bubbles: true }));
        applyDynamicRequired(root);
        return;
      }

      ipWrapper.classList.remove("d-none");

      if (hasCandidateOption) {
        ipSelect.value = candidateIPValue;
      } else {
        ipSelect.value = "";
      }

      setStoredFieldValue(store, scope, ipSelect.name, ipSelect.value);
      ipSelect.dispatchEvent(new Event("change", { bubbles: true }));

      applyDynamicRequired(root);
    }

    projectSelect.addEventListener("change", update);
    update();
  });
}

// ---------------- DATES ----------------

function formatLocalDate(date = new Date()) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");

  return `${year}-${month}-${day}`;
}

export function setupDateConstraints(container = document) {
  const today = formatLocalDate();

  const travelFrom = container.querySelector('[name="travel_from_day"]');
  const travelUntil = container.querySelector('[name="travel_until_day"]');

  const registrationFrom = container.querySelector('[name="registration_from_day"]');
  const registrationUntil = container.querySelector('[name="registration_until_day"]');

  const accommodationFrom = container.querySelector('[name="accommodation_from_day"]');
  const accommodationUntil = container.querySelector('[name="accommodation_until_day"]');

  const docType = container.querySelector('[name="travel_passport_doc_type"]');
  const docExpiration = container.querySelector('[name="travel_passport_expiration"]');
  const docWarning = container.querySelector('[data-travel-doc-expiration-warning]');

  const syncRange = (fromInput, untilInput) => {
    if (!fromInput || !untilInput) return;

    fromInput.min = today;

    const updateUntilMin = () => {
      const minDate = fromInput.value || today;
      untilInput.min = minDate;

      if (untilInput.value && untilInput.value < minDate) {
        untilInput.value = "";
      }
    };

    updateUntilMin();
    fromInput.addEventListener("change", updateUntilMin);
  };

  syncRange(travelFrom, travelUntil);
  syncRange(accommodationFrom, accommodationUntil);
  syncRange(registrationFrom, registrationUntil);

  if (docExpiration) {
    const updateDocumentRules = () => {
      const tripStart = travelFrom?.value || "";
      const tripEnd = travelUntil?.value || "";
      const expiration = docExpiration.value || "";
      const selectedDocType = docType?.value || "";

      // Solo bloqueamos fechas anteriores a hoy
      docExpiration.min = today;

      if (expiration && expiration < today) {
        docExpiration.value = "";
      }

      const showRenewalWarning =
        !!expiration &&
        !!tripEnd &&
        expiration >= today &&
        expiration < tripEnd &&
        (!selectedDocType || selectedDocType === "dni");

      if (docWarning) {
        docWarning.classList.toggle("d-none", !showRenewalWarning);
      }
    };

    updateDocumentRules();

    travelFrom?.addEventListener("change", updateDocumentRules);
    travelUntil?.addEventListener("change", updateDocumentRules);
    docExpiration.addEventListener("change", updateDocumentRules);
    docType?.addEventListener("change", updateDocumentRules);
  }
}


// ---------------- TRAVEL PHONE / IDENTITY ----------------
async function loadBudgetPermissionsIfNeeded() {
  if (BUDGET_PERMISSIONS_CACHE) {
    return BUDGET_PERMISSIONS_CACHE;
  }

  try {
    const res = await fetch("/module/proxy/module8/api?action=getBudgetPermissions");

    if (!res.ok) {
      throw new Error(`Permissions request failed: ${res.status}`);
    }

    BUDGET_PERMISSIONS_CACHE = await res.json();
  } catch (error) {
    console.error("Error loading budgeting permissions:", error);

    BUDGET_PERMISSIONS_CACHE = {
      isAccounting: false
    };
  }

  return BUDGET_PERMISSIONS_CACHE;
}

function isAccountingUser() {
  return Boolean(BUDGET_PERMISSIONS_CACHE?.isAccounting);
}

function canShowTravelIdentityBlock() {
  const restrictedModal =
    CURRENT_MODAL_MODE === "manage" || CURRENT_MODAL_MODE === "view";

  return !restrictedModal || isAccountingUser();
}

function initPhoneIntlInput(input) {
  if (!input || typeof window.intlTelInput !== "function") return null;
  if (input._itiInstance) return input._itiInstance;

  const iti = window.intlTelInput(input, {
    initialCountry: "es",
    nationalMode: false,
    autoPlaceholder: "polite",
    preferredCountries: ["es", "fr", "de", "it", "pt", "gb"],
  });

  input._itiInstance = iti;

  return iti;
}

function isPhoneEffectivelyEmpty(rawValue = "", iti = null) {
  const value = String(rawValue || "").trim();
  if (!value) return true;

  const digits = value.replace(/\D/g, "");
  if (!digits) return true;

  const dialCode = iti?.getSelectedCountryData?.()?.dialCode || "";

  return Boolean(dialCode) && value.startsWith("+") && digits === dialCode;
}

function applyStoredPhoneToInput(input, rawPhone = "") {
  const iti = initPhoneIntlInput(input);
  const value = String(rawPhone || "").trim();

  if (!iti) {
    input.value = value;
    return { iti: null, hasRealPhone: Boolean(value) };
  }

  if (value) {
    iti.setNumber(value);
  } else {
    iti.setCountry("es");
    input.value = "";
  }

  const hasRealPhone = !isPhoneEffectivelyEmpty(value, iti);

  // Si solo hay prefijo, dejamos la bandera pero no lo mostramos como número real
  if (!hasRealPhone) {
    const iso2 = iti.getSelectedCountryData()?.iso2;
    if (iso2) iti.setCountry(iso2);
    input.value = "";
  }

  return { iti, hasRealPhone };
}

function getPhoneForSubmit(input) {
  if (!input) return "";

  const iti = input._itiInstance;
  const rawValue = String(input.value || "").trim();

  if (!rawValue) return "";
  if (!iti) return rawValue;

  const intlValue = String(iti.getNumber?.() || "").trim();
  if (intlValue) return intlValue;

  const selectedCountry = iti.getSelectedCountryData?.() || {};
  const dialCode = selectedCountry.dialCode ? `+${selectedCountry.dialCode}` : "";

  if (rawValue.startsWith("+")) return rawValue;
  if (dialCode) return `${dialCode} ${rawValue}`.trim();

  return rawValue;
}

function setFieldValidity(el, message = "") {
  if (!el) return;
  el.setCustomValidity(message);
  el.classList.toggle("is-invalid", Boolean(message));
}

function validatePhoneField(input, { required = true } = {}) {
  if (!input || input.disabled || !isElementVisible(input)) return true;

  const iti = input._itiInstance;
  const rawValue = String(input.value || "").trim();
  const digits = rawValue.replace(/\D/g, "");

  let message = "";

  if (isPhoneEffectivelyEmpty(rawValue, iti)) {
    message = required ? "Please enter a contact phone number." : "";
  } else if (!/^\+?[-()\s\d]{6,25}$/.test(rawValue)) {
    message = "Use only digits, spaces, parentheses, hyphens and an optional leading +.";
  } else if (digits.length < 6 || digits.length > 15) {
    message = "Phone numbers must contain between 6 and 15 digits.";
  }

  setFieldValidity(input, message);
  return !message;
}

function validateTravelIdentityFields(root = document, selected = []) {
  if (!selected.includes("travel")) return true;

  const phoneInput = root.querySelector('[name="travel_contact_phone"]');
  const phoneOk = validatePhoneField(phoneInput, { required: true });

  if (!phoneOk) {
    phoneInput?.reportValidity();
    phoneInput?.focus();
    return false;
  }

  return true;
}

function requiresPassportByArea(root = document) {
  const areaSelect = root.querySelector('[name="travel_area"]');
  return areaSelect?.value === "non_eu";
}

async function fetchTravelContactInfo(combinedId = "", scope = "main") {
  resetTravelContactInfo(scope);

  const url = combinedId
    ? `/module/proxy/module8/api?action=getTravelContactInfo&combinedId=${encodeURIComponent(combinedId)}`
    : "/module/proxy/module8/api?action=getTravelContactInfo";

  const res = await fetch(url, {
    cache: "no-store",
    headers: {
      "Cache-Control": "no-store"
    }
  });
  const data = await res.json();

  if (!res.ok || data.ok === false) {
    throw new Error(data.error || `Travel contact info request failed: ${res.status}`);
  }

  TRAVEL_CONTACT_INFO_BY_SCOPE[scope] = {
    loaded: true,
    hasPhone: Boolean(data.hasPhone),
    phone: data.phone || "",
    phoneSource: data.phoneSource || "",
    hasValidPassport: Boolean(data.hasValidPassport),
    hasPassportData: Boolean(data.hasPassportData),
    hasValidDocument: Boolean(data.hasValidDocument),
    passport: data.passport || null
  };

  return TRAVEL_CONTACT_INFO_BY_SCOPE[scope];
}

async function loadTravelContactInfoForModal(combinedId = "") {
  try {
    return await fetchTravelContactInfo(combinedId, "edit");
  } catch (error) {
    console.error("Error loading travel contact info:", error);
    resetTravelContactInfo("edit");
    return TRAVEL_CONTACT_INFO_BY_SCOPE.edit;
  }
}

function resetTravelContactInfo(scope = "main") {
  TRAVEL_CONTACT_INFO_BY_SCOPE[scope] = createEmptyTravelContactInfo();
}

function getTravelContactInfoForScope(scope = "main") {
  return TRAVEL_CONTACT_INFO_BY_SCOPE[scope] || createEmptyTravelContactInfo();
}

function prepareEmptyPassportForm() {
  document.querySelector('[name="travel_passport_doc_type"]').value = "passport";
  document.querySelector('[name="travel_passport_doc_type"]').disabled = true;
  document.querySelector('[name="travel_passport_doc_number"]').value = "";
  document.querySelector('[name="travel_passport_expiration"]').value = "";
}

function renderStoredIdentityInfo(data) {
  const box = document.querySelector("#stored-identity-box");
  if (!box) return;

  const identity = data.passport;

  if (!identity) {
    box.classList.add("d-none");
    box.innerHTML = "";
    return;
  }

  box.classList.remove("d-none");
  box.innerHTML = `
    <div><strong>Stored document:</strong></div>
    <div>Type: ${identity.doc_type || "-"}</div>
    <div>Number: ${identity.doc_number || "-"}</div>
    <div>Expiration: ${identity.expiration || "-"}</div>
  `;
}

function applyIdentityToForm(data, requiresPassport) {
  const typeInput = document.querySelector('[name="travel_passport_doc_type"]');
  const numInput = document.querySelector('[name="travel_passport_doc_number"]');
  const expInput = document.querySelector('[name="travel_passport_expiration"]');

  if (!typeInput || !numInput || !expInput) return;

  const identity = data.passport || null;

  if (requiresPassport) {
    if (data.hasValidPassport && identity?.doc_type === "passport") {
      typeInput.value = "passport";
      numInput.value = identity.doc_number || "";
      expInput.value = identity.expiration || "";
    } else {
      typeInput.value = "passport";
      numInput.value = "";
      expInput.value = "";
    }

    typeInput.disabled = true;
    return;
  }

  if (identity) {
    typeInput.value = identity.doc_type || "";
    numInput.value = identity.doc_number || "";
    expInput.value = identity.expiration || "";
  } else {
    typeInput.value = "";
    numInput.value = "";
    expInput.value = "";
  }

  typeInput.disabled = false;
}

function onIdentityConfirmationChange(value, data, requiresPassport) {
  const formBox = document.querySelector("#travel-passport-fields");

  if (value === "yes") {
    formBox.classList.add("d-none");

    if (requiresPassport && !data.hasValidPassport) {
      showIdentityMessage("A passport is required for this trip.");
      clearIdentityConfirmation();
      formBox.classList.remove("d-none");
    }

    return;
  }

  if (value === "no") {
    formBox.classList.remove("d-none");
    applyIdentityToForm(data, requiresPassport);
  }
}

export function setupTravelLuggageLogic(container = document) {
  const luggageTypeSelect = container.querySelector('[name="travel_luggage_type"]');
  const checkedKgWrapper = container.querySelector('[data-travel-checked-kg-wrapper]');
  const checkedKgSelect = container.querySelector('[name="travel_checked_kg"]');

  if (!luggageTypeSelect || !checkedKgWrapper || !checkedKgSelect) return;

  const updateLuggageUI = () => {
    const value = luggageTypeSelect.value;
    const needsCheckedKg = value === "checked" || value === "hand_checked";

    checkedKgWrapper.classList.toggle("d-none", !needsCheckedKg);

    if (!needsCheckedKg) {
      checkedKgSelect.value = "";
    }
  };

  luggageTypeSelect.addEventListener("change", updateLuggageUI);

  updateLuggageUI();
}

function setupTravelIdentityBehavior(root = document, scope = "main") {
  const phoneInput = root.querySelector('[name="travel_contact_phone"]');
  const phoneYes = root.querySelector('[name="travel_contact_phone_confirmed"][value="yes"]');
  const phoneNo = root.querySelector('[name="travel_contact_phone_confirmed"][value="no"]');

  const docType = root.querySelector('[name="travel_passport_doc_type"]');
  const docNumber = root.querySelector('[name="travel_passport_doc_number"]');
  const docExpiration = root.querySelector('[name="travel_passport_expiration"]');
  const passportYes = root.querySelector('[name="travel_passport_confirmed"][value="yes"]');
  const passportNo = root.querySelector('[name="travel_passport_confirmed"][value="no"]');

  const phoneSummary = root.querySelector(`[data-travel-phone-summary="${scope}"]`);
  const passportSummary = root.querySelector(`[data-travel-passport-summary="${scope}"]`);
  const phoneConfirmWrap = root.querySelector(`[data-phone-confirm-wrapper="${scope}"]`);
  const passportConfirmWrap = root.querySelector(`[data-passport-confirm-wrapper="${scope}"]`);

  if (!phoneInput || !docType || !docNumber || !docExpiration) return;

  const info = getTravelContactInfoForScope(scope);

  const { iti: phoneIti, hasRealPhone } = applyStoredPhoneToInput(phoneInput, info.phone || "");

  const refreshIdentityValidation = () => {
    validatePhoneField(phoneInput, { required: true });
  };

  phoneInput.addEventListener("blur", refreshIdentityValidation);
  phoneInput.addEventListener("input", refreshIdentityValidation);
  root.querySelector('[name="travel_until_day"]')?.addEventListener("change", refreshIdentityValidation);

  if (info.hasPhone && hasRealPhone) {
    phoneInput.disabled = true;

    if (phoneYes) phoneYes.checked = true;
    if (phoneNo) phoneNo.checked = false;

    if (phoneConfirmWrap) {
      phoneConfirmWrap.classList.remove("d-none");
    }
  } else {
    phoneInput.disabled = false;

    if (phoneYes) phoneYes.checked = false;
    if (phoneNo) phoneNo.checked = false;

    if (phoneSummary) {
      phoneSummary.innerHTML = `
        <div class="alert alert-warning mb-0">
          No phone found. Please provide one.
        </div>
      `;
    }

    if (phoneConfirmWrap) {
      phoneConfirmWrap.classList.add("d-none");
    }
  }

  const areaSelect = root.querySelector('[name="travel_area"]');

  function refreshTravelDocumentRules() {
    const outsideEU = requiresPassportByArea(root);

    const hasStoredPassportData = Boolean(info.hasPassportData || info.passport);
    const storedDocType = String(info.passport?.doc_type || "").toLowerCase();

    const hasValidPassport =
      Boolean(info.hasValidPassport) && storedDocType === "passport";

    if (outsideEU) {
      if (hasValidPassport) {
        docType.value = "passport";
        docNumber.value = info.passport?.doc_number ?? "";
        docExpiration.value = String(info.passport?.expiration ?? "").slice(0, 10);

        docType.disabled = true;
        docNumber.disabled = true;
        docExpiration.disabled = true;
      } else {
        docType.value = "passport";
        docNumber.value = "";
        docExpiration.value = "";

        docType.disabled = true;
        docNumber.disabled = false;
        docExpiration.disabled = false;
      }
    } else {
      if (hasStoredPassportData && info.passport) {
        docType.value = info.passport.doc_type ?? "";
        docNumber.value = info.passport.doc_number ?? "";
        docExpiration.value = String(info.passport.expiration ?? "").slice(0, 10);
      } else {
        docType.value = "";
        docNumber.value = "";
        docExpiration.value = "";
      }
    }

    if (outsideEU) {
      if (hasValidPassport) {
        docType.disabled = true;
        docNumber.disabled = true;
        docExpiration.disabled = true;

        if (passportYes) passportYes.checked = true;
        if (passportNo) passportNo.checked = false;

        if (passportSummary) {
          passportSummary.innerHTML = `
            <div class="alert alert-success mb-0">
              A valid passport was found for this traveller.
            </div>
          `;
        }

        if (passportConfirmWrap) {
          passportConfirmWrap.classList.remove("d-none");
        }
      } else {
        if (!docType.value || storedDocType !== "passport") {
          docType.value = "passport";
        }

        docType.disabled = true;
        docNumber.disabled = false;
        docExpiration.disabled = false;

        if (passportYes) passportYes.checked = false;
        if (passportNo) passportNo.checked = false;

        if (passportSummary) {
          passportSummary.innerHTML = `
            <div class="alert alert-warning mb-0">
              For trips outside the European Union, a valid passport is mandatory.
            </div>
          `;
        }

        if (passportConfirmWrap) {
          passportConfirmWrap.classList.add("d-none");
        }
      }
    } else {
      if (hasStoredPassportData && info.passport) {
        if (Boolean(info.hasValidDocument)) {
          docType.disabled = true;
          docNumber.disabled = true;
          docExpiration.disabled = true;

          if (passportYes) passportYes.checked = true;
          if (passportNo) passportNo.checked = false;

          if (passportConfirmWrap) {
            passportConfirmWrap.classList.remove("d-none");
          }

          if (passportSummary) {
            passportSummary.innerHTML = `
              <div class="alert alert-success mb-0">
                A valid identification document was found.
              </div>
            `;
          }
        } else {
          docType.disabled = false;
          docNumber.disabled = false;
          docExpiration.disabled = false;

          if (passportYes) passportYes.checked = false;
          if (passportNo) passportNo.checked = false;

          if (passportConfirmWrap) {
            passportConfirmWrap.classList.add("d-none");
          }

          if (passportSummary) {
            passportSummary.innerHTML = `
              <div class="alert alert-warning mb-0">
                The stored identification document is not valid anymore. Please update it.
              </div>
            `;
          }
        }
      } else {
        docType.disabled = false;
        docNumber.disabled = false;
        docExpiration.disabled = false;

        if (passportYes) passportYes.checked = false;
        if (passportNo) passportNo.checked = false;

        if (passportSummary) {
          passportSummary.innerHTML = `
            <div class="alert alert-warning mb-0">
              No valid identification document found. Please provide one.
            </div>
          `;
        }

        if (passportConfirmWrap) {
          passportConfirmWrap.classList.add("d-none");
        }
      }
    }

    updatePassportEditability();
    applyDynamicRequired(root);
  }

  function updatePhoneEditability() {
    const currentRawValue = phoneInput.value || "";
    const currentHasRealPhone = !isPhoneEffectivelyEmpty(currentRawValue, phoneIti);

    if (!currentHasRealPhone) {
      phoneInput.disabled = false;
      return;
    }

    if (phoneNo?.checked) {
      phoneInput.disabled = false;
      return;
    }

    if (phoneYes?.checked) {
      phoneInput.disabled = true;
    }
  }

  function updatePassportEditability() {
    const outsideEU = requiresPassportByArea(root);

    if (outsideEU) {
      docType.value = "passport";
      docType.disabled = true;

      if (passportNo?.checked) {
        docNumber.disabled = false;
        docExpiration.disabled = false;
        return;
      }

      if (passportYes?.checked) {
        docNumber.disabled = true;
        docExpiration.disabled = true;
        return;
      }

      docNumber.disabled = false;
      docExpiration.disabled = false;
      return;
    }

    if (!Boolean(info.hasValidDocument)) {
      docType.disabled = false;
      docNumber.disabled = false;
      docExpiration.disabled = false;
      return;
    }

    const editable = passportNo?.checked;

    docType.disabled = !editable;
    docNumber.disabled = !editable;
    docExpiration.disabled = !editable;
  }

  phoneYes?.addEventListener("change", updatePhoneEditability);
  phoneNo?.addEventListener("change", updatePhoneEditability);
  phoneInput.addEventListener("input", updatePhoneEditability);

  passportYes?.addEventListener("change", updatePassportEditability);
  passportNo?.addEventListener("change", updatePassportEditability);

  areaSelect?.addEventListener("change", refreshTravelDocumentRules);

  refreshTravelDocumentRules();
}


// ---------------- REQUIRED / VALIDATION ----------------

function isElementVisible(el) {
  if (!el) return false;
  if (el.disabled) return false;
  if (el.type === "hidden") return false;

  return el.offsetParent !== null;
}

function shouldBeRequired(el) {
  if (!el.name) return false;
  if (OPTIONAL_FIELD_NAMES.has(el.name)) return false;
  if (!isElementVisible(el)) return false;

  if (el.type === "file") {
    return el.name === "registration_invoice" && isElementVisible(el);
  }

  return true;
}

function applyDynamicRequired(root = document) {
  const fields = root.querySelectorAll("input, select, textarea");

  fields.forEach((el) => {
    el.required = false;

    if (el.type === "radio") return;

    if (shouldBeRequired(el)) {
      el.required = true;
    }
  });

  const radioGroups = new Map();

  root.querySelectorAll('input[type="radio"]').forEach((radio) => {
    if (!radio.name) return;
    if (!isElementVisible(radio)) return;
    if (radio.disabled) return;

    if (!radioGroups.has(radio.name)) {
      radioGroups.set(radio.name, []);
    }

    radioGroups.get(radio.name).push(radio);
  });

  radioGroups.forEach((group, name) => {
    if (OPTIONAL_FIELD_NAMES.has(name)) return;

    if (group.length > 0) {
      group.forEach(r => r.required = false);
      group[0].required = true;
    }
  });
}

function validateVisibleRequiredFields(root = document) {
  applyDynamicRequired(root);

  const fields = Array.from(root.querySelectorAll("input, select, textarea"))
    .filter((el) => isElementVisible(el) && !el.disabled);

  for (const el of fields) {
    if (!el.checkValidity()) {
      el.reportValidity();
      el.focus();
      return false;
    }
  }

  return true;
}

function getTextLength(value = "") {
  return String(value).length;
}

function ensureCounterElement(el) {
  const counterId = `${el.name || el.id}-counter`;

  let counter = el.parentElement?.querySelector(`[data-char-counter-for="${counterId}"]`);
  if (counter) return counter;

  counter = document.createElement("div");
  counter.className = "form-text text-end";
  counter.dataset.charCounterFor = counterId;

  if (el.parentElement) {
    el.parentElement.appendChild(counter);
  }

  return counter;
}

function updateCharCounter(el) {
  const limit = Number(el.maxLength);
  if (!limit || limit < 0) return;

  const counter = ensureCounterElement(el);
  const current = getTextLength(el.value);

  counter.textContent = `${current} / ${limit}`;

  if (current >= limit) {
    counter.classList.add("text-danger");
  } else {
    counter.classList.remove("text-danger");
  }
}

function applyTextFieldLimits(root = document) {
  const fields = root.querySelectorAll("input[type='text'], textarea");

  fields.forEach((el) => {
    const limit = TEXT_FIELD_LIMITS[el.name] ?? TEXT_FIELD_LIMITS[el.id];
    if (!limit) return;

    el.maxLength = limit;

    if (el.dataset.charLimitHooked === "1") {
      updateCharCounter(el);
      return;
    }

    el.dataset.charLimitHooked = "1";

    el.addEventListener("input", () => {
      if (el.value.length > limit) {
        el.value = el.value.slice(0, limit);
      }

      updateCharCounter(el);
    });

    updateCharCounter(el);
  });
}


// ---------------- FORM STATE ----------------

function getFormStateKey(scope, name) {
  return `${scope}::${name}`;
}

function setStoredFieldValue(store, scope, name, value) {
  store.set(getFormStateKey(scope, name), value);
}

function getStoredFieldValue(store, scope, name) {
  return store.get(getFormStateKey(scope, name));
}

function clearFormStateForScope(store, scope) {
  for (const key of Array.from(store.keys())) {
    if (key.startsWith(`${scope}::`)) {
      store.delete(key);
    }
  }
}

function captureFormState(root = document, store = FORM_STATE, scope = "main") {
  root.querySelectorAll("input, select, textarea").forEach((el) => {
    if (!el.name || el.type === "file") return;

    if (el.type === "radio") {
      if (el.checked) {
        setStoredFieldValue(store, scope, el.name, el.value);
      }

      return;
    }

    if (el.type === "checkbox") {
      setStoredFieldValue(store, scope, el.name, el.checked);
      return;
    }

    setStoredFieldValue(store, scope, el.name, el.value);
  });
}

function restoreFormState(root = document, store = FORM_STATE, scope = "main") {
  root.querySelectorAll("input, select, textarea").forEach((el) => {
    if (!el.name || el.type === "file") return;

    const storedValue = getStoredFieldValue(store, scope, el.name);
    if (storedValue === undefined) return;

    if (el.type === "radio") {
      el.checked = el.value === storedValue;
      return;
    }

    if (el.type === "checkbox") {
      el.checked = Boolean(storedValue);
      return;
    }

    el.value = storedValue ?? "";
  });

  root.querySelectorAll("input, select, textarea").forEach((el) => {
    if (!el.name || el.type === "file") return;
    el.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

function bindFormStateTracking(root = document, store = FORM_STATE, scope = "main") {
  root.querySelectorAll("input, select, textarea").forEach((el) => {
    if (!el.name || el.type === "file") return;
    if (el.dataset.stateHooked === "1") return;

    el.dataset.stateHooked = "1";

    const eventName =
      el.type === "radio" || el.type === "checkbox" || el.tagName === "SELECT"
        ? "change"
        : "input";

    el.addEventListener(eventName, () => {
      if (el.type === "radio") {
        if (el.checked) {
          setStoredFieldValue(store, scope, el.name, el.value);
        }

        return;
      }

      if (el.type === "checkbox") {
        setStoredFieldValue(store, scope, el.name, el.checked);
        return;
      }

      setStoredFieldValue(store, scope, el.name, el.value);
    });
  });
}


// ---------------- FILE MANAGEMENT ----------------

function setupFileInputs(root = document, store = FILES_STORE, scope = "main") {
  const fileInputs = root.querySelectorAll('input[type="file"][data-max-files]');

  fileInputs.forEach((input) => {
    if (input.dataset.fileHooked === "1") {
      renderSelectedFilesList(root, store, scope, input.name);
      return;
    }

    input.dataset.fileHooked = "1";

    input.addEventListener("change", () => {
      const maxFiles = Number(input.dataset.maxFiles || 5);
      const currentFiles = getStoredFiles(store, scope, input.name);
      const selectedNow = input.files ? Array.from(input.files) : [];

      let updatedFiles = [...currentFiles];

      for (const file of selectedNow) {
        if (fileAlreadyExists(updatedFiles, file)) continue;

        if (updatedFiles.length >= maxFiles) {
          alert(`You can upload a maximum of ${maxFiles} files in this field.`);
          break;
        }

        updatedFiles.push(file);
      }

      setStoredFiles(store, scope, input.name, updatedFiles);
      renderSelectedFilesList(root, store, scope, input.name);

      input.value = "";
    });

    renderSelectedFilesList(root, store, scope, input.name);
  });
}

function getFileStoreKey(scope, inputName) {
  return `${scope}::${inputName}`;
}

export function getStoredFiles(store, scope, inputName) {
  const key = getFileStoreKey(scope, inputName);
  return store.get(key) || [];
}

function setStoredFiles(store, scope, inputName, files) {
  const key = getFileStoreKey(scope, inputName);
  store.set(key, files);
}

function fileAlreadyExists(existingFiles, newFile) {
  return existingFiles.some(
    (f) =>
      f.name === newFile.name &&
      f.size === newFile.size &&
      f.lastModified === newFile.lastModified &&
      f.type === newFile.type
  );
}

function renderSelectedFilesList(root, store, scope, inputName) {
  const container = root.querySelector(`[data-files-list="${inputName}"]`);
  if (!container) return;

  const files = getStoredFiles(store, scope, inputName);

  if (!files.length) {
    container.innerHTML = "";
    return;
  }

  container.innerHTML = files
    .map(
      (file, index) => `
        <div class="d-flex align-items-center justify-content-between border rounded px-3 py-2 mb-2 bg-light">
          <div class="me-3 text-truncate">
            <strong>${escapeHtml(file.name)}</strong>
            <small class="text-muted ms-2">(${formatFileSize(file.size)})</small>
          </div>
          <button
            type="button"
            class="btn btn-sm btn-outline-danger"
            data-remove-file="${inputName}"
            data-file-index="${index}"
            data-file-scope="${scope}"
          >
            Remove
          </button>
        </div>
      `
    )
    .join("");
}

function formatFileSize(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;

  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function extractFileName(path = "") {
  return String(path).split("/").pop() || path;
}

function buildServerFileUrl(path = "") {
  return `/module/proxy/module8/Uploads?file=${String(path).replace(/^\/+/, "%")}`;
}

export function normalizeServerPaths(value) {
  if (!value) return [];

  if (Array.isArray(value)) {
    return value.filter(Boolean);
  }

  return [String(value)];
}

function renderServerFiles(root = document, data = {}) {
  const sections = ["travel", "registration"];

  sections.forEach(section => {
    const container = root.querySelector(`[data-server-files="${section}"]`);
    if (!container) return;

    const rawPaths =
      data[`${section}_file_path`] ??
      (data[`${section}_file_path`] ? [data[`${section}_file_path`]] : []);

    const paths = Array.isArray(rawPaths) ? rawPaths.filter(Boolean) : [];

    if (!paths.length) {
      container.innerHTML = "";
      return;
    }

    container.innerHTML = paths.map(path => {
      const fileName = extractFileName(path);
      const href = buildServerFileUrl(path);

      return `
        <div class="d-flex align-items-center justify-content-between border rounded px-3 py-2 mb-2 bg-light">
          <div class="me-3 text-truncate">
            <strong>${escapeHtml(fileName)}</strong>
          </div>
          <a
            href="${escapeHtml(href)}"
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-sm btn-outline-primary"
          >
            View file
          </a>
        </div>
      `;
    }).join("");
  });
}


// ---------------- BACKEND DATA NORMALIZATION ----------------

export function normalizeCategoryPartToFormData(part, categoryKey) {
  const data = {};

  if (!categoryKey || !part) return data;

  // Campos comunes de todas las categorías
  data[`${categoryKey}_project`] = part.project_id ?? "";
  data[`${categoryKey}_ip`] = part.ip_id ?? "";
  data[`${categoryKey}_observations`] = part.observations ?? "";

  switch (categoryKey) {
    case "travel":
      data.travel_project = part.project_id ?? "";
      data.travel_ip = part.ip_id ?? "";
      data.travel_purpose = part.purpose ?? "";
      data.travel_from_day = part.fromDay ?? "";
      data.travel_until_day = part.untilDay ?? "";
      data.travel_from_where = part.travel_fromPlace ?? "";
      data.travel_to_where = part.wherePlace ?? "";
      data.institution = part.institution ?? "";
      data.travel_observations = part.observations ?? "";
      data.travel_luggage_type = part.luggage_type ?? "";
      data.travel_checked_kg = part.luggage_weight ?? "";
      data.travel_seat_preference = part.seat_preference ?? "";
      data.travel_time_preference = part.time_preference ?? "";
      data.travel_area = part.travel_area ?? "";
      data.travel_file_path = normalizeServerPaths(part.file_paths);
      break;

    case "registration":
      data.registration_project = part.project_id ?? "";
      data.registration_ip = part.ip_id ?? "";
      data.registration_event = part.purpose ?? "";
      data.registration_from_day = part.fromDay ?? "";
      data.registration_until_day = part.untilDay ?? "";
      data.registration_type = String(part.registration_type ?? "");
      data.registration_payment = String(part.registration_type ?? "") === "I" ? "invoice" : "payment";
      data.registration_invoice = String(part.registration_invoice ?? "");
      data.registration_registered = String(part.registration_invoice ?? "") === "1" ? "yes" : "no";
      data.registration_observations = part.observations ?? "";
      data.registration_file_path = normalizeServerPaths(part.file_paths);

      data.registration_pending_invoice =
        data.registration_payment === "payment" ||
        data.registration_registered === "no";
      break;

    case "accommodation":
      data.accommodation_project = part.project_id ?? "";
      data.accommodation_ip = part.ip_id ?? "";
      data.accommodation_purpose = part.purpose ?? "";
      data.accommodation_from_day = part.fromDay ?? "";
      data.accommodation_until_day = part.untilDay ?? "";
      data.accommodation_where = part.wherePlace ?? "";
      data.accommodation_observations = part.observations ?? "";
      break;

    case "equipment":
      data.equipment_project = part.project_id ?? "";
      data.equipment_ip = part.ip_id ?? "";
      data.equipment_category = part.equipment_category ?? "";
      data.equipment_other_text = part.purpose ?? "";
      data.equipment_description = part.observations ?? "";
      data.equipment_price_range = part.other_price != null
        ? String(part.other_price)
        : "";
      break;

    case "other":
      data.other_project = part.project_id ?? "";
      data.other_ip = part.ip_id ?? "";
      data.other_description = part.observations ?? "";
      data.other_price_range = part.other_price != null
        ? String(part.other_price)
        : "";
      break;
  }

  return data;
}

function normalizeManagementRequestsFromBackend(raw) {
  const source = extractRequestData(raw);
  const parts = Array.isArray(source.parts) ? source.parts : [];

  const categories = [...new Set(
    parts
      .map(part => CATEGORY_ID_TO_KEY[Number(part.category_id)])
      .filter(Boolean)
  )];

  const projectIds = [...new Set(
    parts
      .map(part => part.project_id)
      .filter(Boolean)
  )];

  const data = {};

  parts.forEach(part => {
    const categoryKey = CATEGORY_ID_TO_KEY[Number(part.category_id)];
    if (!categoryKey) return;

    Object.assign(data, normalizeCategoryPartToFormData(part, categoryKey));
  });

  const lastIpResponseDate = parts
    .map(p => p.ip_response_date)
    .filter(Boolean)
    .sort()
    .at(-1) || null;

  const updatedAt =
    source.acc_or_it_response_date ||
    source.projects_response_date ||
    lastIpResponseDate ||
    source.creation_date ||
    "";

  return [{
    id: String(source.combined_id),
    id_intern: source.id_intern || "",
    combinedId: source.combined_id,
    ipId: "",
    projectIds,
    categories,
    data,
    rawParts: parts,
    canUploadPendingInvoice: false,
    rejectionComment: source.denied_comment ?? "",
    createdAt: source.creation_date ?? "",
    updatedAt,
    worker: source.surname ?? "",
    workerName: source.name ?? ""
  }];
}


// ---------------- MANAGEMENT REQUESTS ----------------

async function loadManagementRequests() {
  try {
    const response = await fetch("/module/proxy/module8/api?action=getManagementRequests");
    const data = await response.json();

    console.log("Management requests loaded from backend:", data);

    const rawRequests = Array.isArray(data.requests)
      ? data.requests
      : Array.isArray(data)
        ? data
        : [];

    const normalizedRequests = rawRequests
      .flatMap(normalizeManagementRequestsFromBackend)
      .sort((a, b) => {
        const aTime = new Date(a.createdAt || 0).getTime();
        const bTime = new Date(b.createdAt || 0).getTime();

        return aTime - bTime;
      });

    MANAGEMENT_REQUESTS_CACHE = {};

    normalizedRequests.forEach((req) => {
      MANAGEMENT_REQUESTS_CACHE[req.id] = req;
    });

    if (!normalizedRequests.length) {
      viewRequestsBody.innerHTML = `
        <tr>
          <td colspan="5" class="text-center text-muted py-4">
            No management requests found
          </td>
        </tr>
      `;
      return;
    }

    viewRequestsBody.innerHTML = normalizedRequests
      .map(buildManagementRequestRow)
      .join("");

  } catch (error) {
    console.error("Error loading management requests:", error);

    viewRequestsBody.innerHTML = `
      <tr>
        <td colspan="5" class="text-center text-danger py-4">
          Error loading management requests
        </td>
      </tr>
    `;
  }
}

function buildManagementRequestRow(request) {
  // console.log("Building management request row for request:", request);

  const projectLabel = getProjectLabelFromRequest(request, ALL_PROJECTS_CACHE);
  const lastUpdate = request.updatedAt || "";

  return `
    <tr
      data-last-update="${escapeHtml(lastUpdate)}"
    >
      <td>${escapeHtml(request.id_intern || "-")}</td>
      <td>${escapeHtml(projectLabel)}</td>
      <td>${escapeHtml(request.workerName + " " + request.worker || "-")}</td>
      <td>${escapeHtml(formatDateTime(lastUpdate))}</td>
      <td class="text-end">
        <button
          type="button"
          class="btn btn-sm btn-outline-primary manage-request-btn"
          data-request-id="${escapeHtml(String(request.id))}"
        >
          Review
        </button>
      </td>
    </tr>
  `;
}


// ---------------- MODAL HELPERS ----------------

function isRejectedRequest(request) {
  return REJECTED_STATUSES.has(request?.status);
}

function shouldUnlockCategorySelectors(request) {
  return CURRENT_MODAL_MODE === "edit" && isRejectedRequest(request);
}

function applyCategorySelectorLock(request) {
  const unlockCategories = shouldUnlockCategorySelectors(request);

  if (!unlockCategories) {
    editChecks.forEach(c => {
      c.disabled = true;
    });

    return;
  }

  // Si está rechazada y el usuario la modifica, mantenemos las reglas normales
  enforceEditOtherLockingIfEquipmentSelected();
  enforceEditEquipmentLocking();
}

function buildModalTitle(baseTitle, request = null) {
  const idIntern = request.id_intern;

  if (!idIntern) return baseTitle;

  return `${baseTitle} - ${idIntern}`;
}

function updateApproveButtonLock() {
  const hasRejectionComment = managementRejectComment.value.trim().length > 0;

  approveManageBtn.disabled = hasRejectionComment;
  approveManageBtn.classList.toggle("disabled", hasRejectionComment);
  approveManageBtn.classList.toggle("btn-success", !hasRejectionComment);
  approveManageBtn.classList.toggle("btn-outline-secondary", hasRejectionComment);

  approveManageBtn.title = hasRejectionComment
    ? "Clear the rejection comment before approving this request."
    : "";
}

function renderViewRejectionComment(request) {
  const comment = request?.rejectionComment?.trim();

  if (CURRENT_MODAL_MODE === "view" && comment) {
    viewRejectionText.textContent = comment;
    viewRejectionBox.classList.remove("d-none");
    return;
  }

  viewRejectionText.textContent = "";
  viewRejectionBox.classList.add("d-none");
}

function renderManageRejectionComment(request) {
  if (CURRENT_MODAL_MODE !== "manage") {
    managementRejectComment.value = "";
    managementRejectBox.classList.add("d-none");
    updateApproveButtonLock();
    return;
  }

  const comment = request?.rejectionComment?.trim() || "";

  managementRejectBox.classList.remove("d-none");
  managementRejectComment.value = comment;
  updateApproveButtonLock();
}

function renderEditRejectionComment(request) {
  const comment = request?.rejectionComment?.trim();

  if (CURRENT_MODAL_MODE === "edit" && comment) {
    viewRejectionText.textContent = comment;
    viewRejectionBox.classList.remove("d-none");
    return;
  }

  viewRejectionText.textContent = "";
  viewRejectionBox.classList.add("d-none");
}

function setModalMode(mode, request = null) {
  CURRENT_MODAL_MODE = mode;

  viewRejectionText.textContent = "";
  viewRejectionBox.classList.add("d-none");

  if (mode === "manage") {
    editRequestModalLabel.textContent = buildModalTitle("Review request", request);
    saveEditBtn.classList.add("d-none");
    approveManageBtn.classList.remove("d-none");
    rejectManageBtn.classList.remove("d-none");
    managementRejectBox.classList.remove("d-none");
    managementRejectComment.value = "";
    updateApproveButtonLock();

    editChecks.forEach(c => c.disabled = true);
    return;
  }

  if (mode === "view") {
    editRequestModalLabel.textContent = buildModalTitle("View request", request);
    saveEditBtn.classList.add("d-none");
    approveManageBtn.classList.add("d-none");
    rejectManageBtn.classList.add("d-none");
    managementRejectBox.classList.add("d-none");
    managementRejectComment.value = "";
    updateApproveButtonLock();

    editChecks.forEach(c => c.disabled = true);
    return;
  }

  editRequestModalLabel.textContent = buildModalTitle("View request", request);
  saveEditBtn.classList.remove("d-none");
  approveManageBtn.classList.add("d-none");
  rejectManageBtn.classList.add("d-none");
  managementRejectBox.classList.add("d-none");
  managementRejectComment.value = "";
  updateApproveButtonLock();

  editChecks.forEach(c => c.disabled = false);
}

function setModalFieldsReadOnly(readOnly = false) {
  editAccordion.querySelectorAll("input, select, textarea").forEach(el => {
    if (el.type === "file") {
      el.disabled = readOnly;
      return;
    }

    if (el.name && el.name.endsWith("_ip")) {
      el.disabled = readOnly;
      return;
    }

    if (
      el.type === "radio" ||
      el.tagName === "SELECT" ||
      el.tagName === "TEXTAREA" ||
      el.tagName === "INPUT"
    ) {
      el.disabled = readOnly;
    }
  });
}

function resetEditModalBeforeOpen() {
  clearFormStateForScope(EDIT_FORM_STATE, "edit");
  EDIT_FILES_STORE.clear();
  resetTravelContactInfo("edit");

  editChecks.forEach(c => {
    c.checked = false;
  });

  editAccordion.innerHTML = "";
  managementRejectComment.value = "";
}

function openAllEditAccordionSections() {
  const collapses = editAccordion.querySelectorAll(".accordion-collapse");

  collapses.forEach(collapseEl => {
    const instance = bootstrap.Collapse.getOrCreateInstance(collapseEl, { toggle: false });
    instance.show();
  });
}


// ---------------- FILL EDIT MODAL ----------------

function normalizeValueForInput(el, value) {
  if (value == null) return "";

  if (el.type === "date") {
    return String(value).slice(0, 10);
  }

  if (el.type === "datetime-local") {
    return String(value).replace("Z", "").slice(0, 16);
  }

  return String(value);
}

function fillEditForm(data = {}) {
  Object.entries(data).forEach(([name, value]) => {
    if (name.endsWith("_ip")) return;

    const elements = editAccordion.querySelectorAll(`[name="${name}"]`);

    if (!elements.length) {
      // console.warn("Field not found in modal:", name, "value:", value);
      return;
    }

    elements.forEach(el => {
      if (el.type === "radio") {
        el.checked = String(el.value) === String(value);
        return;
      }

      if (el.type === "checkbox") {
        el.checked = value === true || value === "true" || value === 1 || value === "1";
        return;
      }

      if (el.type !== "file") {
        el.value = normalizeValueForInput(el, value);
      }
    });

    elements.forEach(el => {
      el.dispatchEvent(new Event("change", { bubbles: true }));
    });
  });

  editAccordion.querySelectorAll(`select[name$="_project"]`).forEach(projectSelect => {
    projectSelect.dispatchEvent(new Event("change", { bubbles: true }));
  });

  Object.entries(data).forEach(([name, value]) => {
    if (!name.endsWith("_ip")) return;

    const ipSelect = editAccordion.querySelector(`[name="${name}"]`);

    if (!ipSelect) {
      console.warn("IP field not found in modal:", name, "value:", value);
      return;
    }

    ipSelect.value = String(value ?? "");
    ipSelect.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

function setPendingInvoiceEditMode() {
  editChecks.forEach(c => {
    c.disabled = true;
  });

  const meetingWarning = document.getElementById("edit-registration-warning");
  const invoiceFieldWrapper = document.getElementById("edit-invoice-wrapper");

  if (meetingWarning && !meetingWarning.classList.contains("d-none")) {
    meetingWarning.classList.add("d-none");
  }

  if (invoiceFieldWrapper) {
    invoiceFieldWrapper.classList.add(
      "border",
      "border-2",
      "border-warning",
      "rounded",
      "p-3",
      "bg-warning-subtle"
    );
  }

  editAccordion.querySelectorAll("input, select, textarea").forEach(el => {
    const isRegistrationFile =
      el.type === "file" && el.name === "registration_invoice";

    const isRegistrationType =
      el.type === "radio" && el.name === "registration_payment";

    const hasInvoice =
      el.type === "radio" && el.name === "registration_registered";

    if (isRegistrationFile) {
      el.disabled = false;
      return;
    }

    if (isRegistrationType) {
      el.disabled = false;
      return;
    }

    if (hasInvoice) {
      el.disabled = false;
      return;
    }

    el.disabled = true;
  });
}


// ---------------- OPEN MODALS ----------------

export async function openViewModal(requestOrId) {
  const request =
    typeof requestOrId === "object" && requestOrId !== null
      ? requestOrId
      : REQUESTS_CACHE[requestOrId];

  if (!request) {
    alert("Request not found");
    return;
  }

  CURRENT_EDIT_REQUEST_ID = request.id;

  setModalMode("view", request);
  resetEditModalBeforeOpen();

  await loadBudgetPermissionsIfNeeded();

  if (canShowTravelIdentityBlock()) {
    await loadTravelContactInfoForModal(request.id);
  } else {
    resetTravelContactInfo("edit");
  }

  editChecks.forEach(c => {
    c.checked = false;
  });

  editChecks.forEach(c => {
    c.checked = request.categories.includes(c.value);
  });

  syncEditUI();
  applyCategorySelectorLock(request);

  fillEditForm(request.data);
  renderServerFiles(editAccordion, request.data);
  setModalFieldsReadOnly(true);
  renderViewRejectionComment(request);

  editModal.show();
}

async function openEditModal(requestId) {
  const request = REQUESTS_CACHE[requestId];

  if (!request) {
    alert("Request not found");
    return;
  }

  EDIT_FILES_STORE.clear();
  clearFormStateForScope(EDIT_FORM_STATE, "edit");

  CURRENT_EDIT_REQUEST_ID = requestId;

  setModalMode("edit", request);
  resetEditModalBeforeOpen();

  await loadTravelContactInfoForModal(request.id);

  editChecks.forEach(c => {
    c.checked = request.categories.includes(c.value);
    c.disabled = false;
  });

  syncEditUI();
  fillEditForm(request.data);

  if (
    request.canUploadPendingInvoice &&
    request.status !== "rejected" &&
    request.status !== "rejected-ip" &&
    request.status !== "rejected-projects" &&
    request.status !== "rejected-it" &&
    request.status !== "rejected-accounting"
  ) {
    setPendingInvoiceEditMode();
  } else {
    setModalFieldsReadOnly(false);
  }

  renderEditRejectionComment(request);
  editModal.show();
}

async function openManageModal(requestId) {
  const request = MANAGEMENT_REQUESTS_CACHE[requestId];

  if (!request) {
    alert("Request not found");
    return;
  }

  EDIT_FILES_STORE.clear();
  clearFormStateForScope(EDIT_FORM_STATE, "edit");

  CURRENT_EDIT_REQUEST_ID = requestId;

  setModalMode("manage", request);
  resetEditModalBeforeOpen();

  await loadBudgetPermissionsIfNeeded();

  if (canShowTravelIdentityBlock()) {
    await loadTravelContactInfoForModal(request.id);
  } else {
    resetTravelContactInfo("edit");
  }

  editChecks.forEach(c => {
    c.checked = request.categories.includes(c.value);
  });

  syncEditUI();
  applyCategorySelectorLock(request);
  fillEditForm(request.data);
  renderServerFiles(editAccordion, request.data);
  setModalFieldsReadOnly(false);
  renderManageRejectionComment(request);
  openAllEditAccordionSections();

  console.log("MANAGE request.data", request.data);
  console.log(
    "Modal names:",
    Array.from(editAccordion.querySelectorAll("[name]")).map(el => el.name)
  );

  editModal.show();
}


// ---------------- SUBMIT HELPERS ----------------

function getCategoryFromBackendError(message = "") {
  const match = String(message).match(/^([a-z_]+)\s*:/i);
  return match ? match[1].toLowerCase() : null;
}

function getAccordionItemByCategory(category, isEdit = false) {
  const prefix = isEdit ? "edit" : "main";
  return document.getElementById(`${prefix}-sec-${category}`);
}

function highlightAccordionTitle(category, isEdit = false) {
  const item = getAccordionItemByCategory(category, isEdit);
  if (!item) return;

  item.classList.add("section-invalid");

  const btn = item.querySelector(".accordion-button");

  if (btn) {
    btn.classList.add("section-invalid");
  }
}

function clearAccordionHighlights(root = document) {
  root.querySelectorAll(".accordion-item.section-invalid").forEach(item => {
    item.classList.remove("section-invalid");
  });

  root.querySelectorAll(".accordion-button.section-invalid").forEach(btn => {
    btn.classList.remove("section-invalid");
  });
}

async function extractBackendErrorMessage(response) {
  const contentType = response.headers.get("content-type") || "";

  if (contentType.includes("application/json")) {
    const data = await response.json();

    return (
      data?.error ||
      data?.message ||
      data?.detail ||
      "Unknown error"
    );
  }

  return await response.text();
}


// ---------------- SUBMIT NEW REQUEST ----------------

function submitNewRequest() {
  const selected = getSelectedCategories();

  if (!selected.length) {
    alert("Please select at least one category.");
    return;
  }

  if (!validateVisibleRequiredFields(accordion)) {
    return;
  }

  const areaSelect = accordion.querySelector('[name="travel_area"]');
  const docTypeSelect = accordion.querySelector('[name="travel_passport_doc_type"]');

  if (
    selected.includes("travel") &&
    areaSelect?.value === "non_eu" &&
    String(docTypeSelect?.value || "").toLowerCase() !== "passport"
  ) {
    alert("For trips outside the European Union, the identification document must be a passport.");
    docTypeSelect?.focus();
    return;
  }

  const formData = new FormData();

  formData.append("categories", JSON.stringify(selected));

  document
    .querySelectorAll("#details-accordion input, #details-accordion select, #details-accordion textarea")
    .forEach((el) => {
      if (!el.name) return;

      if (el.type === "file") {
        const files = getStoredFiles(FILES_STORE, "main", el.name);

        files.forEach((file) => {
          formData.append(el.name, file);
        });
      } else if (el.type === "radio") {
        if (el.checked) {
          formData.append(el.name, el.value);
        }
      } else if (el.type === "checkbox") {
        formData.append(el.name, el.checked ? "true" : "false");
      } else {
        formData.append(el.name, el.value);
      }
    });

  if (selected.includes("travel")) {
    const phoneInput = accordion.querySelector('[name="travel_contact_phone"]');

    if (phoneInput) {
      const fullPhone = getPhoneForSubmit(phoneInput);
      formData.set("travel_contact_phone", fullPhone || "");
    }

    if (!validateTravelIdentityFields(accordion, selected)) {
      return;
    }
  }

  fetch("/module/proxy/module8/api?action=submitRequest", {
    method: "POST",
    body: formData
  })
    .then(async (response) => {
      if (!response.ok) {
        const backendMessage = await extractBackendErrorMessage(response);
        const category = getCategoryFromBackendError(backendMessage);

        throw {
          backendMessage: backendMessage || "",
          userMessage: "There are missing fields in the form. Please review the highlighted sections.",
          category
        };
      }

      return response.json();
    })
    .then((data) => {
      console.log("Request submitted:", data);
      alert("Request submitted successfully");

      window.location.reload();

      checks.forEach(c => {
        c.checked = false;
        c.disabled = false;
      });

      FILES_STORE.clear();
      clearFormStateForScope(FORM_STATE, "main");

      accordion.innerHTML = "";
      enforceEquipmentLocking();
    })
    .catch((error) => {
      console.error("Error submitting request:", error);

      clearAccordionHighlights(accordion);

      const category =
        error?.category ||
        getCategoryFromBackendError(error?.backendMessage || error?.message || "");

      if (category) {
        highlightAccordionTitle(category, false);
      }

      const userMessage =
        error?.userMessage ||
        String(error?.message || "").trim() ||
        "Error submitting request";

      alert(userMessage);
    });
}


// ---------------- MANAGEMENT ACTIONS ----------------

function approveCurrentManagementRequest() {
  if (managementRejectComment.value.trim()) {
    alert("Clear the rejection comment before approving this request.");
    updateApproveButtonLock();
    return;
  }

  if (!CURRENT_EDIT_REQUEST_ID) return;

  const currentRequest =
    CURRENT_MODAL_MODE === "manage"
      ? MANAGEMENT_REQUESTS_CACHE[CURRENT_EDIT_REQUEST_ID]
      : REQUESTS_CACHE[CURRENT_EDIT_REQUEST_ID];

  if (!currentRequest) {
    console.error("Current request not found for approve:", {
      CURRENT_EDIT_REQUEST_ID,
      CURRENT_MODAL_MODE,
      inRequestsCache: REQUESTS_CACHE[CURRENT_EDIT_REQUEST_ID],
      inManagementCache: MANAGEMENT_REQUESTS_CACHE[CURRENT_EDIT_REQUEST_ID]
    });

    alert("Request not found.");
    return;
  }

  const selected = getEditSelectedCategories();

  const payload = {
    categories: selected
  };

  editAccordion.querySelectorAll("input, select, textarea").forEach((el) => {
    if (!el.name) return;

    if (el.type === "file") {
      payload[el.name] = getStoredFiles(EDIT_FILES_STORE, "edit", el.name).map(f => f.name);
    } else if (el.type === "radio") {
      if (el.checked) payload[el.name] = el.value;
    } else {
      payload[el.name] = el.value;
    }
  });

  console.log("Edited request payload:", payload);

  const formData = new FormData();

  const filesRegistr = getStoredFiles(EDIT_FILES_STORE, "edit", "registration_invoice");
  const filesTravel = getStoredFiles(EDIT_FILES_STORE, "edit", "travel_preferences_files");

  const fullPayload = {
    ...payload,
    categories: payload.categories || currentRequest.categories || []
  };

  formData.append("idCombined", CURRENT_EDIT_REQUEST_ID);
  formData.append("categories", JSON.stringify(fullPayload.categories || []));

  Object.entries(fullPayload).forEach(([key, value]) => {
    if (key === "categories") return;
    if (key === "registration_invoice") return;
    if (key === "travel_preferences_files") return;
    if (key.endsWith("_file_path")) return;
    if (key === "registration_pending_invoice") return;

    formData.append(key, value ?? "");
  });

  filesRegistr.forEach(file => {
    formData.append("registration_invoice", file);
  });

  filesTravel.forEach(file => {
    formData.append("travel_preferences_files", file);
  });

  console.log("FormData entries:");

  for (const pair of formData.entries()) {
    console.log(pair[0] + ": ", pair[1]);
  }

  fetch(`/module/proxy/module8/api?action=acceptRequest`, {
    method: "POST",
    body: formData
  })
    .then(async res => {
      if (!res.ok) {
        throw new Error("Request failed");
      }

      {
        REQUESTS_CACHE[CURRENT_EDIT_REQUEST_ID] = {
          ...currentRequest,
          data: {
            ...currentRequest.data,
            ...payload
          }
        };
      }

      editModal.hide();
      alert("Request approved successfully");

      window.location.reload();
    })
    .catch(error => {
      console.error("Error modifying request:", error);
      alert("Error modifying request");
    });
}

function rejectCurrentManagementRequest() {
  if (!CURRENT_EDIT_REQUEST_ID) return;

  const comment = managementRejectComment.value.trim();

  if (!comment) {
    alert("Please add a rejection comment.");
    return;
  }

  console.log("Rejected request:", CURRENT_EDIT_REQUEST_ID, "Comment:", comment);

  const response = fetch(`/module/proxy/module8/api?action=rejectRequest`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      motive: comment,
      idCombined: CURRENT_EDIT_REQUEST_ID
    })
  })
    .then(async res => {
      if (!res.ok) {
        throw new Error("Request failed");
      }

      return res.json();
    })
    .then(data => {
      console.log("Request cancelled:", data);
      alert("Request rejected");
      window.location.reload();
    })
    .catch(error => {
      console.error("Error cancelling request:", error);
      alert("Error rejecting request");
    });

  editModal.hide();
  alert("Request rejected");
  window.location.reload();
}


// ---------------- FIELD ERROR CLEANUP ----------------

function clearFieldError(el) {
  if (!(el instanceof HTMLElement)) return;
  if (!el.matches("input, textarea, select")) return;

  el.classList.remove("field-invalid");
  el.removeAttribute("aria-invalid");

  const parent = el.closest(".col, .mb-3, .form-group, .form-check, .w-100") || el.parentElement;
  const error = parent?.querySelector(".field-error-text");

  if (error) error.remove();
}


// ---------------- RESET ----------------

function resetMainForm() {
  checks.forEach(c => {
    c.checked = false;
    c.disabled = false;
  });

  FILES_STORE.clear();
  clearFormStateForScope(FORM_STATE, "main");

  accordion.innerHTML = "";
  enforceEquipmentLocking();
}


// ---------------- INIT DATA ----------------

async function initPageData() {
  try {
    const [userProjectsRes, allProjectsRes] = await Promise.all([
      fetch("/module/proxy/module8/api?action=getProjects"),
      fetch("/module/proxy/module8/api?action=getAllProjects")
    ]);

    const userProjectsData = await userProjectsRes.json();
    const allProjectsData = await allProjectsRes.json();

    PROJECTS_CACHE = Array.isArray(userProjectsData.projects)
      ? userProjectsData.projects
      : (Array.isArray(userProjectsData) ? userProjectsData : []);

    ALL_PROJECTS_CACHE = Array.isArray(allProjectsData.projects)
      ? allProjectsData.projects
      : (Array.isArray(allProjectsData) ? allProjectsData : []);

    populateProjectSelects(document, PROJECTS_CACHE);
    applyTextFieldLimits(document);

    await fetchTravelContactInfo();
    await loadSentRequests();
    await loadManagementRequests();
  } catch (e) {
    console.error("Error loading projects:", e);
  }
}


// ---------------- EVENT LISTENERS ----------------

managementRejectComment.addEventListener("input", updateApproveButtonLock);

document.addEventListener("DOMContentLoaded", initPageData);

checks.forEach(c => {
  c.addEventListener("change", syncUI);
});

editChecks.forEach(c => {
  c.addEventListener("change", syncEditUI);
});

resetBtn.addEventListener("click", resetMainForm);

submitBtn.addEventListener("click", submitNewRequest);

approveManageBtn.addEventListener("click", approveCurrentManagementRequest);

rejectManageBtn.addEventListener("click", rejectCurrentManagementRequest);

document.addEventListener("click", (e) => {
  const modifyBtn = e.target.closest(".modify-request-btn");

  if (modifyBtn) {
    const requestId = modifyBtn.dataset.requestId;
    openEditModal(requestId);
    return;
  }

  const manageBtn = e.target.closest(".manage-request-btn");

  if (manageBtn) {
    const requestId = manageBtn.dataset.requestId;
    openManageModal(requestId);
    return;
  }

  const viewBtn = e.target.closest(".view-request-btn");

  if (viewBtn) {
    const requestId = viewBtn.dataset.requestId;
    openViewModal(requestId);
    return;
  }

  const removeFileBtn = e.target.closest("[data-remove-file]");

  if (removeFileBtn) {
    const inputName = removeFileBtn.dataset.removeFile;
    const fileIndex = Number(removeFileBtn.dataset.fileIndex);
    const scope = removeFileBtn.dataset.fileScope;

    const isEditScope = scope === "edit";
    const store = isEditScope ? EDIT_FILES_STORE : FILES_STORE;
    const root = isEditScope ? editAccordion : accordion;

    const files = getStoredFiles(store, scope, inputName);

    files.splice(fileIndex, 1);
    setStoredFiles(store, scope, inputName, files);

    renderSelectedFilesList(root, store, scope, inputName);
  }
});

document.addEventListener("input", (e) => {
  clearFieldError(e.target);
});

document.addEventListener("change", (e) => {
  clearFieldError(e.target);
});


// ---------------- INITIAL UI STATE ----------------

syncUI();
