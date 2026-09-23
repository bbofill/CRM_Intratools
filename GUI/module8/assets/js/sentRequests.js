/* Code for the Sent Requests window, shows all requests sent by the user.
Allows users to view and manage their sent requests by adding invoice documents
to registration requests or cancelling requests. */

import {
  STATUS_ORDER,
  CATEGORY_ID_TO_KEY,
  escapeHtml,
  formatDateTime,
  getStatusLabel,
  getStatusBadgeClass,
  getCategoryLabel,
  extractRequestData
} from "./categoryOptions.js";

import {
  normalizeCategoryPartToFormData,
  getProjectLabelFromRequest,
  CURRENT_EDIT_REQUEST_ID,
  CURRENT_MODAL_MODE,
  getEditSelectedCategories,
  editModal,
  editAccordion,
  getStoredFiles,
  EDIT_FILES_STORE
} from "./budgeting.js";


// ---------------- DOM ELEMENTS ----------------

const sentRequestsProjectHeader = document.getElementById("sentRequestsProjectHeader");
const sentRequestsStatusHeader = document.getElementById("sentRequestsStatusHeader");
const sentRequestsLastUpdateHeader = document.getElementById("sentRequestsLastUpdateHeader");

const sentProjectSortIndicator = document.getElementById("sent-project-sort-indicator");
const sentStatusSortIndicator = document.getElementById("sent-status-sort-indicator");
const sentLastUpdateSortIndicator = document.getElementById("sent-last-update-sort-indicator");

const sentRequestsBody = document.getElementById("sentRequestsBody");


// ---------------- GLOBAL STATE ----------------

let originalSentRows = [];

let sentProjectSortState = "default";
let sentStatusSortState = "default";
let sentLastUpdateSortState = "default";

export let REQUESTS_CACHE = {};


// ---------------- LOAD SENT REQUESTS ----------------

export async function loadSentRequests() {
  try {
    const response = await fetch("/module/proxy/module8/api?action=getSentRequests");
    const data = await response.json();

    console.log("Sent requests loaded from backend:", data);

    const rawRequests = Array.isArray(data.requests)
      ? data.requests
      : Array.isArray(data)
        ? data
        : [];

    const normalizedRequests = rawRequests.flatMap(normalizeRequestRowsFromBackend);

    REQUESTS_CACHE = {};
    normalizedRequests.forEach((req) => {
      REQUESTS_CACHE[req.id] = req;
    });

    if (!normalizedRequests.length) {
      sentRequestsBody.innerHTML = `
        <tr>
          <td colspan="6" class="text-center text-muted py-4">
            No sent requests found
          </td>
        </tr>
      `;

      refreshOriginalSentRows();
      return;
    }

    sentRequestsBody.innerHTML = normalizedRequests
      .map(buildSentRequestRow)
      .join("");

    refreshOriginalSentRows();

    sentProjectSortState = "default";
    sentStatusSortState = "default";
    sentLastUpdateSortState = "default";

    ensureSentStatusLegend();
    resetSentSortIndicators();
  } catch (error) {
    console.error("Error loading sent requests:", error);

    sentRequestsBody.innerHTML = `
      <tr>
        <td colspan="6" class="text-center text-danger py-4">
          Error loading sent requests
        </td>
      </tr>
    `;

    refreshOriginalSentRows();
  }
}


// ---------------- TABLE RENDERING ----------------

function buildSentRequestRow(request) {
  const projectLabel = getProjectLabelFromRequest(request);
  const requestPreview = buildRequestPreview(request);
  const lastUpdate = request.updatedAt || request.createdAt || "";
  const statusLabel = getStatusLabel(request.status);
  const statusBadgeClass = getStatusBadgeClass(request.status);
  const rejectionComment = String(request.rejectionComment || "").trim();

  const canModifyRejected =
    request.status === "rejected" ||
    request.status === "rejected-ip" ||
    request.status === "rejected-projects" ||
    request.status === "rejected-it" ||
    request.status === "rejected-accounting";

  const canModifyPendingInvoice = Boolean(request.canUploadPendingInvoice);

  const cannotCancelFinalApproved =
    request.status === "approved-accounting" ||
    request.status === "approved-it";

  const canCancel = !cannotCancelFinalApproved;

  return `
    <tr
      data-project="${escapeHtml(projectLabel)}"
      data-status="${escapeHtml(request.status || "")}"
      data-last-update="${escapeHtml(lastUpdate)}"
    >
      <td>${escapeHtml(request.id_intern)}</td>
      <td>${escapeHtml(projectLabel)}</td>
      <td>${escapeHtml(requestPreview)}</td>
      <td>
        <div class="d-flex flex-column gap-2">
          <div>
            <span class="badge ${statusBadgeClass}">
              ${escapeHtml(statusLabel)}
            </span>
            ${
              request.hasInvoicePending
                ? `<span class="badge bg-secondary ms-1">Pending invoice</span>`
                : ""
            }
          </div>

          ${
            rejectionComment
              ? `
                <div class="alert alert-danger py-2 px-3 mb-0 small">
                  <strong>Reason:</strong> ${escapeHtml(rejectionComment)}
                </div>
              `
              : ""
          }
        </div>
      </td>
      <td>${escapeHtml(formatDateTime(lastUpdate))}</td>
      <td class="text-end">
        <div class="d-inline-flex gap-2">
          ${
            !canModifyRejected && !canModifyPendingInvoice
              ? `
                <button
                  type="button"
                  class="btn btn-sm btn-outline-primary view-request-btn"
                  data-request-id="${escapeHtml(String(request.id))}"
                >
                  View
                </button>
              `
              : ""
          }

          ${
            canModifyRejected || canModifyPendingInvoice
              ? `
                <button
                  type="button"
                  class="btn btn-sm btn-primary modify-request-btn"
                  data-request-id="${escapeHtml(String(request.id))}"
                >
                  Modify
                </button>
              `
              : ""
          }

          ${
            canCancel
              ? `
                <button
                  type="button"
                  class="btn btn-sm btn-outline-danger cancel-request-btn"
                  data-request-id="${escapeHtml(String(request.id))}"
                >
                  Cancel
                </button>
              `
              : ""
          }
        </div>
      </td>
    </tr>
  `;
}

function buildRequestPreview(request = {}) {
  const parts = Array.isArray(request.rawParts) ? request.rawParts : [];
  if (!parts.length) return "-";

  return parts
    .map((part) => {
      const categoryKey = CATEGORY_ID_TO_KEY[Number(part.category_id)];
      if (!categoryKey) return null;

      const categoryLabel = getCategoryLabel(categoryKey);

      if (categoryKey === "other") {
        return categoryLabel;
      }

      const purpose = String(part.purpose ?? "").trim();

      if (categoryLabel === "Equipment" && !purpose) {
        return `${categoryLabel}: ${part.equipment_category}`;
      }

      if (!purpose) {
        return categoryLabel;
      }

      return `${categoryLabel}: ${purpose}`;
    })
    .filter(Boolean)
    .join(" · ");
}

// ---------------- STATUS LEGEND ----------------
function ensureSentStatusLegend() {
  const tbody = document.getElementById("sentRequestsBody");
  if (!tbody) return;

  const table = tbody.closest("table");
  if (!table) return;

  const existingLegend = document.getElementById("sent-status-legend");
  if (existingLegend) return;

  const legend = document.createElement("div");
  legend.id = "sent-status-legend";
  legend.className = "card shadow-sm mb-3 border-0";

  legend.innerHTML = `
    <div class="card-body">
      <div class="d-flex justify-content-between align-items-center gap-2">
        <div>
          <h6 class="mb-1 fw-bold">Request status guide</h6>
          <p class="mb-0 text-muted small">
            This guide explains the approval flow for your submitted requests.
          </p>
        </div>

        <button
          class="btn btn-sm btn-outline-secondary"
          type="button"
          data-bs-toggle="collapse"
          data-bs-target="#sent-status-legend-content"
          aria-expanded="false"
          aria-controls="sent-status-legend-content"
        >
          View guide
        </button>
      </div>

      <div class="collapse mt-3" id="sent-status-legend-content">
        <div class="row g-3">
          <div class="col-12 col-md-6">
            <span class="badge bg-warning text-dark mb-2">Pending management</span>
            <p class="small text-muted mb-0">
              The project responsible still needs to review and approve the request.
            </p>
          </div>

          <div class="col-12 col-md-6">
            <span class="badge bg-info text-dark mb-2">Approved by Responsible</span>
            <p class="small text-muted mb-0">
              The request has been approved by the project responsible. It will now be reviewed by the Projects team.
            </p>
          </div>

          <div class="col-12 col-md-6">
            <span class="badge bg-primary mb-2">Approved by Projects</span>
            <p class="small text-muted mb-0">
              The Projects team has checked whether the project has enough funding for the request.
              The request will then continue to Accounting, or to IT if it is an equipment request.
            </p>
          </div>

          <div class="col-12 col-md-6">
            <span class="badge bg-success mb-2">Approved by Accounting</span>
            <span class="badge bg-success mb-2">Approved by IT</span>
            <p class="small text-muted mb-0">
              The request has reached the final processing step. Accounting processes non-equipment requests,
              while IT processes equipment requests.
            </p>
          </div>

          <div class="col-12">
            <span class="badge bg-danger mb-2">Rejected</span>
            <p class="small text-muted mb-0">
              The request was rejected by the corresponding team: the project responsible, Projects, Accounting, or IT.
            </p>
          </div>
        </div>
      </div>
    </div>
  `;

  table.parentNode.insertBefore(legend, table);
}

// ---------------- BACKEND DATA NORMALIZATION ----------------

function normalizeRequestRowsFromBackend(raw) {
  const source = extractRequestData(raw);
  const rawIpGroups = Array.isArray(source.parts) ? source.parts : [];

  const parts = rawIpGroups.flatMap(group =>
    Array.isArray(group.parts) ? group.parts : []
  );

  if (!parts.length) {
    return [{
      id: String(source.combined_id),
      combinedId: source.combined_id,
      id_intern: source.id_intern || "",
      ipId: "",
      projectIds: [],
      categories: [],
      data: {},
      rawParts: [],
      status: "pending",
      hasInvoicePending: false,
      canUploadPendingInvoice: false,
      rejectionComment: String(source.denied_comment ?? "").trim(),
      createdAt: source.creation_date ?? "",
      updatedAt:
        source.acc_or_it_response_date ||
        source.projects_response_date ||
        source.creation_date ||
        ""
    }];
  }

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

  const hasInvoicePending = hasInvoicePendingBadge({ rawParts: parts });

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
    combinedId: source.combined_id,
    id_intern: source.id_intern || "",
    ipId: "",
    projectIds,
    categories,
    data,
    rawParts: parts,
    status: resolveRequestStatusForRequest(source, parts),
    hasInvoicePending,
    canUploadPendingInvoice: hasInvoicePending,
    rejectionComment: String(source.denied_comment ?? "").trim(),
    createdAt: source.creation_date ?? "",
    updatedAt
  }];
}

function normalizeIpGroup(group = {}) {
  const ipId = group.ip_id ?? "";
  const parts = Array.isArray(group.parts) ? group.parts : [];

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
  const repeatedCategories = new Set();

  parts.forEach(part => {
    const categoryKey = CATEGORY_ID_TO_KEY[Number(part.category_id)];
    if (!categoryKey) return;

    if (data[`${categoryKey}_project`]) {
      repeatedCategories.add(categoryKey);
    }

    Object.assign(data, normalizeCategoryPartToFormData(part, categoryKey));
  });

  return {
    ipId,
    projectIds,
    categories,
    data,
    rawParts: parts,
    repeatedCategories: [...repeatedCategories]
  };
}


// ---------------- STATUS LOGIC ----------------

function resolveRequestStatusForRequest(source = {}, parts = []) {
  if (!parts.length) {
    return "pending";
  }

  const ipResponses = parts
    .map(part => Number(part.ip_response))
    .filter(v => !Number.isNaN(v));

  const hasRejectedComment = String(source.denied_comment ?? "").trim() !== "";

  const someIpAnswered = ipResponses.some(v => v === 1);

  const allIpsAnswered =
    ipResponses.length > 0 &&
    ipResponses.length === parts.length &&
    ipResponses.every(v => v === 1);

  // Si algún IP ha respondido y Projects aún no, el rechazo viene del IP
  if (someIpAnswered && hasRejectedComment && Number(source.projects_response) === 0) {
    return "rejected-ip";
  }

  if (!allIpsAnswered) {
    return "pending";
  }

  if (String(source.denied_comment ?? "").trim()) {
    if (Number(source.projects_response) === 1 && Number(source.acc_or_it_response) === 0) {
      return "rejected-projects";
    }

    if (Number(source.projects_response) === 1 && Number(source.acc_or_it_response) === 1) {
      const hasEquipment = parts.some(
        part => CATEGORY_ID_TO_KEY[Number(part.category_id)] === "equipment"
      );

      return hasEquipment ? "rejected-it" : "rejected-accounting";
    }

    return "rejected";
  }

  if (Number(source.projects_response) === 0) {
    return "approved-ip";
  }

  if (Number(source.projects_response) === 1 && Number(source.acc_or_it_response) === 0) {
    return "approved-projects";
  }

  if (Number(source.projects_response) === 1 && Number(source.acc_or_it_response) === 1) {
    const hasEquipment = parts.some(
      part => CATEGORY_ID_TO_KEY[Number(part.category_id)] === "equipment"
    );

    return hasEquipment ? "approved-it" : "approved-accounting";
  }

  return "approved-ip";
}

function hasInvoicePendingBadge(group = {}) {
  const parts = Array.isArray(group.rawParts) ? group.rawParts : [];

  return parts.some(part => {
    const categoryKey = CATEGORY_ID_TO_KEY[Number(part.category_id)];
    if (categoryKey !== "registration") return false;

    const registrationType = String(part.registration_type ?? "");
    const registrationInvoice = String(part.registration_invoice ?? "");

    const paymentIsPending = registrationType !== "I";
    const invoiceMissing = registrationInvoice !== "1";

    return paymentIsPending || invoiceMissing;
  });
}


// ---------------- CURRENT MODAL REQUEST ----------------

export function getCurrentModalRequest() {
  if (CURRENT_MODAL_MODE === "manage") {
    return MANAGEMENT_REQUESTS_CACHE[CURRENT_EDIT_REQUEST_ID] || null;
  }

  if (CURRENT_MODAL_MODE === "view") {
    return REQUESTS_CACHE[CURRENT_EDIT_REQUEST_ID] || null;
  }

  return REQUESTS_CACHE[CURRENT_EDIT_REQUEST_ID] || null;
}


// ---------------- CANCEL LOGIC ----------------

function cancelRequest(requestId) {
  if (!requestId) return;

  const confirmed = confirm("Are you sure you want to cancel this request?");
  if (!confirmed) return;

  const motive = prompt(
    "Please provide a reason for cancellation. If it is health related, please make sure to also inform HR:"
  );

  if (motive === null) {
    alert("Cancellation aborted.");
    return;
  }

  const trimmedMotive = motive.trim();

  if (!trimmedMotive) {
    alert("Cancellation reason is required.");
    return;
  }

  fetch(`/module/proxy/module8/api?action=cancelRequest`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      motive: trimmedMotive,
      idCombined: requestId
    })
  })
    .then(async res => {
      if (!res.ok) {
        throw new Error("Request failed");
      }

      alert("Request cancelled successfully");
      await loadSentRequests();
    })
    .catch(error => {
      console.error("Error cancelling request:", error);
      alert("Error cancelling request");
    });
}


// ---------------- EDITING HELPERS ----------------

function normalizeComparableValue(value) {
  const str = String(value ?? "").trim();

  if (!str) return "";

  // Normalizamos fechas ISO completas a YYYY-MM-DD para comparar bien
  if (/^\d{4}-\d{2}-\d{2}T/.test(str)) {
    return str.slice(0, 10);
  }

  return str;
}

function isComparableField(el) {
  if (!el) return false;
  if (!el.name) return false;
  if (el.type === "file") return false;
  if (el.disabled) return false;
  if (el.type === "hidden") return false;
  if (el.offsetParent === null) return false;

  return true;
}

function getRelevantFormFieldNames(root, isPendingInvoiceOnly = false) {
  const names = new Set();

  root.querySelectorAll("input, select, textarea").forEach((el) => {
    if (!isComparableField(el)) return;

    if (isPendingInvoiceOnly) {
      const isAllowedField =
        el.name === "registration_registered" ||
        el.name === "registration_payment";

      if (!isAllowedField) return;
    }

    names.add(el.name);
  });

  return [...names];
}

function buildCurrentComparablePayload(root, isPendingInvoiceOnly = false) {
  const data = {};

  root.querySelectorAll("input, select, textarea").forEach((el) => {
    if (!isComparableField(el)) return;

    if (isPendingInvoiceOnly) {
      const isAllowedField =
        el.name === "registration_registered" ||
        el.name === "registration_payment";

      if (!isAllowedField) return;
    }

    if (el.type === "radio") {
      if (el.checked) {
        data[el.name] = el.value;
      } else if (!(el.name in data)) {
        data[el.name] = "";
      }

      return;
    }

    if (el.type === "checkbox") {
      data[el.name] = el.checked ? "true" : "false";
      return;
    }

    data[el.name] = el.value ?? "";
  });

  return data;
}

function buildOriginalComparablePayload(currentRequest, fieldNames = []) {
  const originalData = currentRequest?.data || {};
  const data = {};

  fieldNames.forEach((name) => {
    data[name] = originalData[name] ?? "";
  });

  return data;
}

function hasComparableChanges(currentRequest, root, isPendingInvoiceOnly = false) {
  if (!currentRequest) return false;

  const originalCategories = Array.isArray(currentRequest.categories)
    ? currentRequest.categories.map(String).sort()
    : [];

  const currentCategories = getEditSelectedCategories().map(String).sort();

  if (JSON.stringify(originalCategories) !== JSON.stringify(currentCategories)) {
    return true;
  }

  const fieldNames = getRelevantFormFieldNames(root, isPendingInvoiceOnly);
  const originalComparable = buildOriginalComparablePayload(currentRequest, fieldNames);
  const currentComparable = buildCurrentComparablePayload(root, isPendingInvoiceOnly);

  console.log("Original comparable:", originalComparable);
  console.log("Current comparable:", currentComparable);

  return fieldNames.some((name) => {
    return normalizeComparableValue(originalComparable[name]) !==
      normalizeComparableValue(currentComparable[name]);
  });
}

function isPendingInvoiceEdit(currentRequest) {
  return (
    currentRequest.canUploadPendingInvoice &&
    currentRequest.status !== "rejected" &&
    currentRequest.status !== "rejected-ip" &&
    currentRequest.status !== "rejected-projects" &&
    currentRequest.status !== "rejected-it" &&
    currentRequest.status !== "rejected-accounting"
  );
}

function buildEditPayload(isPendingInvoiceOnly = false) {
  const selected = getEditSelectedCategories();

  const payload = {
    categories: selected
  };

  editAccordion.querySelectorAll("input, select, textarea").forEach((el) => {
    if (!el.name) return;

    if (isPendingInvoiceOnly) {
      const isAllowedField =
        (el.type === "file" && el.name === "registration_invoice") ||
        el.name === "registration_registered" ||
        el.name === "registration_payment";

      if (!isAllowedField) return;
    } else {
      if (el.type !== "file" && !isComparableField(el)) return;
    }

    if (el.type === "file") {
      payload[el.name] = getStoredFiles(EDIT_FILES_STORE, "edit", el.name).map(f => f.name);
    } else if (el.type === "radio") {
      if (el.checked) payload[el.name] = el.value;
    } else {
      payload[el.name] = el.value;
    }
  });

  return payload;
}


// ---------------- SAVE EDIT ----------------

function saveEditRequest() {
  if (!CURRENT_EDIT_REQUEST_ID) return;

  const currentRequest = getCurrentModalRequest();
  if (!currentRequest) return;

  const isPendingInvoiceOnly = isPendingInvoiceEdit(currentRequest);
  const payload = buildEditPayload(isPendingInvoiceOnly);

  console.log("Edited request payload:", payload, isPendingInvoiceOnly);

  if (isPendingInvoiceOnly) {
    savePendingInvoice(currentRequest, payload);
    return;
  }

  saveFullRequestEdit(currentRequest, payload);
}

function savePendingInvoice(currentRequest, payload) {
  const formData = new FormData();
  const hasDataChanges = hasComparableChanges(currentRequest, editAccordion, true);
  const files = getStoredFiles(EDIT_FILES_STORE, "edit", "registration_invoice");

  const hasNewInvoiceFiles = files.length > 0;

  if (!hasDataChanges && !hasNewInvoiceFiles) {
    alert("No changes detected. Please, upload an invoice.");
    return;
  }

  const hasInvoice = payload.registration_registered || "";
  const registrationType = payload.registration_payment || "";

  formData.append("idCombined", CURRENT_EDIT_REQUEST_ID);
  formData.append("has_invoice", hasInvoice);
  formData.append("registration_type", registrationType || "");

  if (registrationType !== "invoice" || files.length === 0) {
    alert("You must upload the invoice.");
    return;
  }

  files.forEach(file => {
    formData.append("registration_invoice", file);
  });

  console.log("FormData entries:");

  for (const pair of formData.entries()) {
    console.log(pair[0] + ": " + pair[1]);
  }

  fetch(`/module/proxy/module8/api?action=addInvoice`, {
    method: "POST",
    body: formData
  })
    .then(async res => {
      if (!res.ok) {
        throw new Error("Request failed");
      }

      editModal.hide();
      alert("Invoice uploaded successfully");
      window.location.reload();
    })
    .catch(error => {
      console.error("Error uploading invoice:", error);
      alert("Error uploading invoice");
    });
}

function saveFullRequestEdit(currentRequest, payload) {
  const formData = new FormData();

  const filesRegistr = getStoredFiles(EDIT_FILES_STORE, "edit", "registration_invoice");
  const filesTravel = getStoredFiles(EDIT_FILES_STORE, "edit", "travel_preferences_files");

  const hasDataChanges = hasComparableChanges(currentRequest, editAccordion, false);
  const hasFileChanges = filesRegistr.length > 0 || filesTravel.length > 0;

  if (!hasDataChanges && !hasFileChanges) {
    alert("No changes detected.");
    return;
  }

  const fullPayload = {
    ...currentRequest.data,
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

  fetch(`/module/proxy/module8/api?action=modifyRequest`, {
    method: "POST",
    body: formData
  })
    .then(async res => {
      if (!res.ok) {
        throw new Error("Request failed");
      }

      if (CURRENT_MODAL_MODE === "manage") {
        MANAGEMENT_REQUESTS_CACHE[CURRENT_EDIT_REQUEST_ID] = {
          ...currentRequest,
          data: {
            ...currentRequest.data,
            ...payload
          }
        };
      } else {
        REQUESTS_CACHE[CURRENT_EDIT_REQUEST_ID] = {
          ...currentRequest,
          data: {
            ...currentRequest.data,
            ...payload
          }
        };
      }

      editModal.hide();
      alert("Request modified successfully");
      window.location.reload();
    })
    .catch(error => {
      console.error("Error modifying request:", error);
      alert("Error modifying request");
    });
}


// ---------------- SORTING ----------------

function sortSentRowsByProject() {
  const rows = Array.from(sentRequestsBody.querySelectorAll("tr"));

  rows.sort((a, b) => {
    const aProject = (a.dataset.project || "").toLowerCase().trim();
    const bProject = (b.dataset.project || "").toLowerCase().trim();

    return aProject.localeCompare(bProject);
  });

  sentRequestsBody.innerHTML = "";
  rows.forEach(row => sentRequestsBody.appendChild(row));
}

function sortSentRowsByStatus() {
  const rows = Array.from(sentRequestsBody.querySelectorAll("tr"));

  rows.sort((a, b) => {
    const aStatus = a.dataset.status || "";
    const bStatus = b.dataset.status || "";
    const aOrder = STATUS_ORDER[aStatus] ?? 999;
    const bOrder = STATUS_ORDER[bStatus] ?? 999;

    if (aOrder !== bOrder) return aOrder - bOrder;

    const aDate = new Date(a.dataset.lastUpdate || 0).getTime();
    const bDate = new Date(b.dataset.lastUpdate || 0).getTime();

    return bDate - aDate;
  });

  sentRequestsBody.innerHTML = "";
  rows.forEach(row => sentRequestsBody.appendChild(row));
}

function sortSentRowsByLastUpdate(direction = "newest") {
  const rows = Array.from(sentRequestsBody.querySelectorAll("tr"));

  rows.sort((a, b) => {
    const aDate = new Date(a.dataset.lastUpdate || 0).getTime();
    const bDate = new Date(b.dataset.lastUpdate || 0).getTime();

    if (direction === "oldest") {
      return aDate - bDate;
    }

    return bDate - aDate;
  });

  sentRequestsBody.innerHTML = "";
  rows.forEach(row => sentRequestsBody.appendChild(row));
}

function resetSentSortIndicators() {
  sentProjectSortIndicator.textContent = "↕";
  sentStatusSortIndicator.textContent = "↕";
  sentLastUpdateSortIndicator.textContent = "↕";
}

function restoreOriginalSentRows() {
  sentRequestsBody.innerHTML = "";
  originalSentRows.forEach(row => sentRequestsBody.appendChild(row.cloneNode(true)));
}

function refreshOriginalSentRows() {
  originalSentRows = Array.from(sentRequestsBody.querySelectorAll("tr"))
    .map(row => row.cloneNode(true));
}


// ---------------- EVENT LISTENERS ----------------

document.addEventListener("click", (e) => {
  const cancelBtn = e.target.closest(".cancel-request-btn");
  if (!cancelBtn) return;

  const requestId = cancelBtn.dataset.requestId;
  cancelRequest(requestId);
});

document.getElementById("save-edit-btn").addEventListener("click", saveEditRequest);

sentRequestsProjectHeader.addEventListener("click", () => {
  sentStatusSortState = "default";
  sentLastUpdateSortState = "default";

  if (sentProjectSortState === "default") {
    sentProjectSortState = "alphabetical";
    sortSentRowsByProject();
    resetSentSortIndicators();
    sentProjectSortIndicator.textContent = "↓";
    return;
  }

  sentProjectSortState = "default";
  restoreOriginalSentRows();
  resetSentSortIndicators();
});

sentRequestsStatusHeader.addEventListener("click", () => {
  sentProjectSortState = "default";
  sentLastUpdateSortState = "default";

  if (sentStatusSortState === "default") {
    sentStatusSortState = "status";
    sortSentRowsByStatus();
    resetSentSortIndicators();
    sentStatusSortIndicator.textContent = "↓";
    return;
  }

  sentStatusSortState = "default";
  restoreOriginalSentRows();
  resetSentSortIndicators();
});

sentRequestsLastUpdateHeader.addEventListener("click", () => {
  sentProjectSortState = "default";
  sentStatusSortState = "default";

  if (sentLastUpdateSortState === "default") {
    sentLastUpdateSortState = "newest";
    sortSentRowsByLastUpdate("newest");
    resetSentSortIndicators();
    sentLastUpdateSortIndicator.textContent = "↓";
    return;
  }

  if (sentLastUpdateSortState === "newest") {
    sentLastUpdateSortState = "oldest";
    sortSentRowsByLastUpdate("oldest");
    resetSentSortIndicators();
    sentLastUpdateSortIndicator.textContent = "↑";
    return;
  }

  sentLastUpdateSortState = "default";
  restoreOriginalSentRows();
  resetSentSortIndicators();
});