/* Code for the All Requests window, shows a historic of all requests and allows excel export */

import {
    STATUS_ORDER,
    escapeHtml,
    formatDateTime,
    getStatusLabel,
    getStatusBadgeClass,
    extractRequestData
} from "./categoryOptions.js";

import {
    openViewModal,
    normalizeServerPaths
} from "./budgeting.js";


// ---------------- DOM ELEMENTS ----------------

const allRequestsBody = document.getElementById("allRequestsBody");

const allRequestsProjectHeader = document.getElementById("allRequestsProjectHeader");
const allRequestsWorkerHeader = document.getElementById("allRequestsWorkerHeader");
const allRequestsStatusHeader = document.getElementById("allRequestsStatusHeader");
const allRequestsLastUpdateHeader = document.getElementById("allRequestsLastUpdateHeader");

const allProjectSortIndicator = document.getElementById("all-project-sort-indicator");
const allWorkerSortIndicator = document.getElementById("all-worker-sort-indicator");
const allStatusSortIndicator = document.getElementById("all-status-sort-indicator");
const allLastUpdateSortIndicator = document.getElementById("all-last-update-sort-indicator");

const allRequestsInternalIdSearch = document.getElementById("allRequestsInternalIdSearch");
const clearAllRequestsSearchBtn = document.getElementById("clearAllRequestsSearchBtn");

const filterCreationFrom = document.getElementById("filterCreationFrom");
const filterCreationTo = document.getElementById("filterCreationTo");
const filterCategories = document.getElementById("filterCategories");
const filterTravelAreas = document.getElementById("filterTravelAreas");
const filterProjects = document.getElementById("filterProjects");
const filterResponsibles = document.getElementById("filterResponsibles");

const resetAllRequestsFiltersBtn = document.getElementById("resetAllRequestsFiltersBtn");
const exportFilteredRequestsBtn = document.getElementById("exportFilteredRequestsBtn");
const exportAllRequestsBtn = document.getElementById("exportAllRequestsBtn");


// ---------------- GLOBAL STATE ----------------

let ALL_REQUESTS_CACHE = {};
let originalAllRows = [];
let ALL_PROJECTS_FOR_ALL_REQUESTS = [];

let allProjectSortState = "default";
let allWorkerSortState = "default";
let allStatusSortState = "default";
let allLastUpdateSortState = "default";

const allRequestsTomSelects = {};

// ---------------- USER ROLE ----------------

let ALL_REQUESTS_USER_PERMISSIONS = {};
let CAN_SEE_LOG_CERTIFICATES = false;

async function loadAllRequestsUserPermissions() {
    try {
        const res = await fetch("/module/proxy/module8/api?action=getBudgetPermissions");

        if (!res.ok) {
            throw new Error("Error loading permissions");
        }

        const data = await res.json();

        ALL_REQUESTS_USER_PERMISSIONS = data || {};
        CAN_SEE_LOG_CERTIFICATES = Boolean(data?.isGerencia);

        console.log(
            "All requests permissions:",
            ALL_REQUESTS_USER_PERMISSIONS,
            "Can see log certificates:",
            CAN_SEE_LOG_CERTIFICATES
        );

    } catch (error) {
        console.error("Error loading all requests permissions:", error);

        ALL_REQUESTS_USER_PERMISSIONS = {};
        CAN_SEE_LOG_CERTIFICATES = false;
    }
}
// ---------------- DATA NORMALIZATION ----------------

function getAllRequestProjectById(id) {
    return ALL_PROJECTS_FOR_ALL_REQUESTS.find(
        p => String(p.id) === String(id)
    );
}

function getProjectLabelForAllRequests(request) {
    const projectIds = Array.isArray(request.projectIds) ? request.projectIds : [];
    if (!projectIds.length) return "-";

    return projectIds
        .map(projectId => {
            const project = getAllRequestProjectById(projectId);
            return project ? `(${project.id}) ${project.short_name}` : String(projectId);
        })
        .join(", ");
}

function normalizeAllRequestsFromBackend(raw) {
    const source = extractRequestData(raw);
    const parts = Array.isArray(source.parts) ? source.parts : [];

    const projectIds = [...new Set(
        parts.map(part => part.project_id).filter(Boolean)
    )];

    const responsibleIds = [...new Set(
        parts
            .map(part => part.ip_id)
            .filter(Boolean)
            .map(String)
    )];

    const categories = [...new Set(
        parts
            .map(part => {
                const id = Number(part.category_id);

                if (id === 1) return "travel";
                if (id === 2) return "registration";
                if (id === 3) return "accommodation";
                if (id === 4) return "equipment";
                if (id === 5) return "other";

                return null;
            })
            .filter(Boolean)
    )];

    const data = {};

    // Pasamos las partes al formato que necesita el modal de detalle
    parts.forEach(part => {
        const categoryId = Number(part.category_id);

        if (categoryId === 1) {
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
        }

        if (categoryId === 2) {
            data.registration_project = part.project_id ?? "";
            data.registration_ip = part.ip_id ?? "";
            data.registration_event = part.purpose ?? "";
            data.registration_from_day = part.fromDay ?? "";
            data.registration_until_day = part.untilDay ?? "";
            data.registration_type = String(part.registration_type ?? "");
            data.registration_payment =
                String(part.registration_type ?? "") === "I" ? "invoice" : "payment";
            data.registration_invoice = String(part.registration_invoice ?? "");
            data.registration_registered =
                String(part.registration_invoice ?? "") === "1" ? "yes" : "no";
            data.registration_observations = part.observations ?? "";
            data.registration_file_path = normalizeServerPaths(part.file_paths);
        }

        if (categoryId === 3) {
            data.accommodation_project = part.project_id ?? "";
            data.accommodation_ip = part.ip_id ?? "";
            data.accommodation_purpose = part.purpose ?? "";
            data.accommodation_from_day = part.fromDay ?? "";
            data.accommodation_until_day = part.untilDay ?? "";
            data.accommodation_where = part.wherePlace ?? "";
            data.accommodation_observations = part.observations ?? "";
        }

        if (categoryId === 4) {
            data.equipment_project = part.project_id ?? "";
            data.equipment_ip = part.ip_id ?? "";
            data.equipment_category = part.equipment_category ?? "";
            data.equipment_other_text = part.purpose ?? "";
            data.equipment_description = part.observations ?? "";
            data.equipment_price_range =
                part.other_price != null ? String(part.other_price) : "";
        }

        if (categoryId === 5) {
            data.other_project = part.project_id ?? "";
            data.other_ip = part.ip_id ?? "";
            data.other_description = part.observations ?? "";
            data.other_price_range =
                part.other_price != null ? String(part.other_price) : "";
        }
    });

    const projectLabel = projectIds.length ? projectIds.join(", ") : "-";
    const workerName = source.name ?? "";
    const workerSurname = source.surname ?? "";
    const canceled = source.canceled === 1;
    const cancelationMotive = source.cancelation_motive ?? "";

    console.log("Cancelation status:", canceled, "Motive:", cancelationMotive);

    const updatedAt =
        source.acc_or_it_response_date ||
        source.projects_response_date ||
        source.creation_date ||
        "";

    const status = resolveAllRequestStatus(source, parts);

    const attendanceCertificateUploaded =
    Number(source.attendance_certificate_uploaded || 0) === 1;

    const attendanceCertificatePath =
        source.attendance_certificate_path || "";

    const attendanceCertificateUploadedAt =
        source.attendance_certificate_uploaded_at || "";
    const attendanceCertificateViewed =
        Number(source.attendance_certificate_viewed || 0) === 1;
    const canViewAttendanceCertificate =
        Boolean(source.can_view_attendance_certificate);
    const canTrackAttendanceCertificate =
        Boolean(source.can_track_attendance_certificate);

    const request = {
        id: String(source.combined_id),
        id_intern: source.id_intern ?? "",
        combinedId: source.combined_id,
        responsibleIds,
        rawParts: parts,
        projectIds,
        workerName,
        canceled,
        cancelationMotive,
        workerSurname,
        requestPreview: buildAllRequestPreview(parts),
        status,
        updatedAt,
        createdAt: source.creation_date ?? "",
        categories,
        data,
        rejectionComment: source.denied_comment ?? "",
        attendanceCertificateUploaded,
        attendanceCertificatePath,
        attendanceCertificateUploadedAt,
        attendanceCertificateViewed,
        canViewAttendanceCertificate,
        canTrackAttendanceCertificate
    };

    request.projectLabel = getProjectLabelForAllRequests(request);

    return [request];
}

function buildAllRequestPreview(parts = []) {
    if (!parts.length) return "-";

    return parts
        .map(part => {
            const categoryId = Number(part.category_id);

            if (categoryId === 1) return `Travel: ${part.purpose || "-"}`;
            if (categoryId === 2) return `Registration: ${part.purpose || "-"}`;
            if (categoryId === 3) return `Accommodation: ${part.purpose || "-"}`;
            if (categoryId === 4) return `Equipment: ${part.equipment_category || part.purpose || "-"}`;
            if (categoryId === 5) return "Other";

            return null;
        })
        .filter(Boolean)
        .join(" · ");
}

function resolveAllRequestStatus(source = {}, parts = []) {
    const hasRejectedComment = String(source.denied_comment ?? "").trim() !== "";

    const hasProjectsResponseDate =
        String(source.projects_response_date ?? "").trim() !== "";

    const hasFinalResponseDate =
        String(source.acc_or_it_response_date ?? "").trim() !== "";

    const hasAnyIpResponseDate = parts.some(part =>
        String(part.ip_response_date ?? "").trim() !== ""
    );

    const hasEquipment = parts.some(part => Number(part.category_id) === 4);

    // ---------------- REJECTED STATUSES ----------------
    if (hasRejectedComment) {
        if (hasFinalResponseDate) {
            return hasEquipment ? "rejected-it" : "rejected-accounting";
        }

        if (hasProjectsResponseDate) {
            return "rejected-projects";
        }

        if (hasAnyIpResponseDate) {
            return "rejected-ip";
        }

        return "rejected";
    }

    // ---------------- APPROVED / PENDING STATUSES ----------------
    const ipResponses = parts
        .map(part => Number(part.ip_response))
        .filter(value => !Number.isNaN(value));

    const allIpApproved =
        ipResponses.length > 0 &&
        ipResponses.length === parts.length &&
        ipResponses.every(value => value === 1);

    if (!parts.length || !allIpApproved) {
        return "pending";
    }

    if (Number(source.projects_response) !== 1) {
        return "approved-ip";
    }

    if (Number(source.acc_or_it_response) !== 1) {
        return "approved-projects";
    }

    return hasEquipment ? "approved-it" : "approved-accounting";
}


function getBudgetCategoryLabel(categoryId) {
    const id = Number(categoryId);

    if (id === 1) return "Travel";
    if (id === 2) return "Registration";
    if (id === 3) return "Accommodation";
    if (id === 4) return "Equipment";
    if (id === 5) return "Other";

    return `Category ${categoryId}`;
}

function getBudgetPartLabel(part) {
    const categoryLabel = getBudgetCategoryLabel(part.category_id);
    const partId = part.id || part.part_id || "-";

    return `${categoryLabel} — Request #${partId}`;
}


// ---------------- TABLE RENDER ----------------

function buildAllRequestRow(request) {
    // console.log("Building row for request:", request);

    const workerFullName = `${request.workerName} ${request.workerSurname}`.trim() || "-";
    const statusLabel = getStatusLabel(request.status);
    const statusBadgeClass = getStatusBadgeClass(request.status);
    const project = request.projectLabel || "-";
    const attendanceCertificateUrl = request.attendanceCertificatePath
        ? `/module/proxy/module8/Uploads?file=${encodeURIComponent(request.attendanceCertificatePath)}`
        : "";
    const highlightAttendanceCertificate =
        request.canTrackAttendanceCertificate &&
        request.attendanceCertificateUploaded &&
        !request.attendanceCertificateViewed;

    return `
        <tr
            class="${highlightAttendanceCertificate ? "attendance-certificate-unopened" : ""}"
            data-id-intern="${escapeHtml(request.id_intern || "")}"
            data-project="${escapeHtml(project)}"
            data-worker-name="${escapeHtml(request.workerName)}"
            data-worker-surname="${escapeHtml(request.workerSurname)}"
            data-status="${escapeHtml(request.canceled ? 'canceled' : request.status)}"
            data-last-update="${escapeHtml(request.updatedAt || "")}"
        >
            <td class="request-id-cell">${formatRequestIdCell(request.id_intern)}</td>
            <td>${escapeHtml(project)}</td>
            <td>${escapeHtml(workerFullName)}</td>
            <td>${escapeHtml(request.requestPreview)}</td>
            <td>
                ${request.canceled ? `
                    <div>
                        <span class="badge bg-danger ms-1">Canceled</span>
                        <div class="small text-muted mt-1">${escapeHtml(request.cancelationMotive)}</div>
                    </div>
                ` : `
                    <span class="badge ${statusBadgeClass}">
                        ${escapeHtml(statusLabel)}
                    </span>
                `}
            </td>
            <td>${escapeHtml(formatDateTime(request.updatedAt))}</td>
            <td class="text-end">
                <div class="d-flex justify-content-end gap-2 flex-wrap">
                    <button
                        type="button"
                        class="btn btn-sm btn-outline-primary all-view-request-btn"
                        data-request-id="${escapeHtml(String(request.id))}"
                    >
                        View
                    </button>

                    ${request.attendanceCertificateUploaded && attendanceCertificateUrl && request.canViewAttendanceCertificate ? `
                        <div class="d-flex flex-column align-items-center">
                            <a
                                href="${escapeHtml(attendanceCertificateUrl)}"
                                target="_blank"
                                rel="noopener noreferrer"
                                class="btn btn-sm btn-outline-success d-inline-flex align-items-center justify-content-center text-center"
                                style="min-width: 88px;"
                                data-attendance-certificate-link="1"
                                data-request-id="${escapeHtml(String(request.id))}"
                                title="View attendance certificate"
                            >
                                Attendance Certificate
                            </a>
                            ${request.attendanceCertificateUploadedAt ? `
                                <div class="small text-muted mt-1 text-center">
                                    ${escapeHtml(formatDateTime(request.attendanceCertificateUploadedAt))}
                                </div>
                            ` : ""}
                        </div>
                    ` : ""}

                    ${CAN_SEE_LOG_CERTIFICATES ? `
                        <div class="d-flex flex-column align-items-center">
                            <button
                                type="button"
                                class="btn btn-sm btn-outline-dark all-log-certificates-btn d-inline-flex align-items-center justify-content-center text-center"
                                style="min-width: 88px;"
                                data-request-id="${escapeHtml(String(request.id))}"
                                title="Generate log certificates"
                            >
                                Log Certificates
                            </button>
                        </div>
                    ` : ""}
                </div>
            </td>
        </tr>
    `;
}

function formatRequestIdCell(value) {
    const id = String(value || "").trim();
    if (!id) return "-";

    const breakIndex = id.lastIndexOf("-");
    if (breakIndex > 0 && breakIndex < id.length - 1) {
        return `
            <span class="request-id-wrap">
                <span>${escapeHtml(id.slice(0, breakIndex))}</span>
                <span>${escapeHtml(id.slice(breakIndex))}</span>
            </span>
        `;
    }

    return `<span class="request-id-wrap">${escapeHtml(id)}</span>`;
}


// ---------------- FILTER OPTIONS ----------------

function populateProjectFilter() {
    if (!filterProjects) return;

    filterProjects.innerHTML = "";

    ALL_PROJECTS_FOR_ALL_REQUESTS.forEach(project => {
        const option = document.createElement("option");
        option.value = String(project.id);
        option.textContent = `(${project.id}) ${project.short_name}`;
        filterProjects.appendChild(option);
    });
}

function getResponsibleLabelById(id) {
    const idStr = String(id);

    for (const request of Object.values(ALL_REQUESTS_CACHE)) {
        const parts = Array.isArray(request.rawParts) ? request.rawParts : [];

        for (const part of parts) {
            if (String(part.ip_id || "") !== idStr) continue;

            const fullName = [
                part.ip_name,
                part.ip_surname
            ]
                .filter(Boolean)
                .join(" ")
                .trim();

            if (fullName) {
                return fullName;
            }
        }
    }

    return idStr;
}

function populateResponsibleFilter() {
    if (!filterResponsibles) return;

    const responsiblesMap = new Map();

    Object.values(ALL_REQUESTS_CACHE).forEach(request => {
        const parts = Array.isArray(request.rawParts) ? request.rawParts : [];

        parts.forEach(part => {
            const id = String(part.ip_id || "").trim();
            if (!id) return;

            const fullName = [
                part.ip_name,
                part.ip_surname
            ]
                .filter(Boolean)
                .join(" ")
                .trim();

            responsiblesMap.set(id, fullName || id);
        });
    });

    filterResponsibles.innerHTML = "";

    Array.from(responsiblesMap.entries())
        .sort((a, b) => a[1].localeCompare(b[1]))
        .forEach(([id, label]) => {
            const option = document.createElement("option");
            option.value = id;          // esto es lo que se envía al backend
            option.textContent = label; // esto es lo que ve el usuario
            filterResponsibles.appendChild(option);
        });
}

function initTomSelectOnce(element, key, placeholder) {
    if (!element || typeof TomSelect !== "function") return;

    if (allRequestsTomSelects[key]) {
        allRequestsTomSelects[key].destroy();
        delete allRequestsTomSelects[key];
    }

    element.classList.remove("form-select");

    allRequestsTomSelects[key] = new TomSelect(element, {
        plugins: ["remove_button"],
        maxItems: null,
        persist: false,
        create: false,
        placeholder
    });
}

function initAllRequestsTomSelects() {
    initTomSelectOnce(filterCategories, "categories", "Select categories...");
    initTomSelectOnce(filterTravelAreas, "travelAreas", "Select travel areas...");
    initTomSelectOnce(filterProjects, "projects", "Select projects...");
    initTomSelectOnce(filterResponsibles, "responsibles", "Select responsibles...");
}

function getMultiSelectValues(select) {
    if (!select) return [];

    if (select.tomselect) {
        return select.tomselect.getValue();
    }

    return Array.from(select.selectedOptions).map(option => option.value);
}

function updateTravelAreaFilterAvailability() {
    const selectedCategories = getMultiSelectValues(filterCategories);

    const shouldEnableTravelArea =
        selectedCategories.length === 0 ||
        selectedCategories.includes("travel");

    if (filterTravelAreas?.tomselect) {
        if (shouldEnableTravelArea) {
            filterTravelAreas.tomselect.enable();
        } else {
            filterTravelAreas.tomselect.clear();
            filterTravelAreas.tomselect.disable();
        }
    } else if (filterTravelAreas) {
        filterTravelAreas.disabled = !shouldEnableTravelArea;

        if (!shouldEnableTravelArea) {
            Array.from(filterTravelAreas.options).forEach(option => {
                option.selected = false;
            });
        }
    }
}

// ---------------- LOG CERTIFICATES ----------------
function ensureLogCertificatesModal() {
    let modal = document.getElementById("logCertificatesModal");

    if (modal) return modal;

    const wrapper = document.createElement("div");

    wrapper.innerHTML = `
        <div
            class="modal fade"
            id="logCertificatesModal"
            tabindex="-1"
            aria-labelledby="logCertificatesModalLabel"
            aria-hidden="true"
        >
            <div class="modal-dialog modal-xl modal-dialog-scrollable">
                <div class="modal-content">
                    <div class="modal-header">
                        <h5 class="modal-title" id="logCertificatesModalLabel">
                            Log Certificates
                        </h5>
                        <button
                            type="button"
                            class="btn-close"
                            data-bs-dismiss="modal"
                            aria-label="Close"
                        ></button>
                    </div>

                    <div class="modal-body">
                        <div id="logCertificatesStatus" class="mb-3">
                            <div class="d-flex align-items-center gap-2 text-muted">
                                <div class="spinner-border spinner-border-sm" role="status"></div>
                                <span>Verifying log chains...</span>
                            </div>
                        </div>

                        <div class="table-responsive">
                            <table class="table table-sm align-middle mb-0">
                                <thead>
                                    <tr>
                                        <th>Part</th>
                                        <th>Status</th>
                                        <th class="text-end">Certificate</th>
                                    </tr>
                                </thead>
                                <tbody id="logCertificatesBody">
                                    <tr>
                                        <td colspan="3" class="text-muted">
                                            Preparing verification...
                                        </td>
                                    </tr>
                                </tbody>
                            </table>
                        </div>
                    </div>

                    <div class="modal-footer">
                        <button
                            type="button"
                            class="btn btn-secondary"
                            data-bs-dismiss="modal"
                        >
                            Close
                        </button>
                    </div>
                </div>
            </div>
        </div>
    `;

    document.body.appendChild(wrapper.firstElementChild);

    return document.getElementById("logCertificatesModal");
}

async function verifyBudgetLogChain(combinedId, partId) {
    const params = new URLSearchParams({
        action: "verifyBudgetAuditChain",
        idCombined: String(combinedId),
        partId: String(partId)
    });

    const response = await fetch(`/module/proxy/module8/api?${params.toString()}`);

    if (!response.ok) {
        const text = await response.text();
        throw new Error(text || "Error verifying log chain");
    }

    return response.json();
}

function buildLogCertificateDownloadUrl(combinedId, partId) {
    const params = new URLSearchParams({
        action: "exportBudgetAuditPDF",
        idCombined: String(combinedId),
        partId: String(partId)
    });

    return `/module/proxy/module8/api?${params.toString()}`;
}

function renderLogCertificatePendingRows(request) {
    const tbody = document.getElementById("logCertificatesBody");
    if (!tbody) return;

    const parts = Array.isArray(request.rawParts) ? request.rawParts : [];

    if (!parts.length) {
        tbody.innerHTML = `
            <tr>
                <td colspan="3" class="text-muted">
                    This request has no parts.
                </td>
            </tr>
        `;
        return;
    }

    tbody.innerHTML = parts.map(part => {
        const partId = part.id || part.part_id;

        return `
            <tr data-log-part-id="${escapeHtml(String(partId))}">
                <td>${escapeHtml(getBudgetPartLabel(part))}</td>
                <td>
                    <span class="text-muted">
                        <span class="spinner-border spinner-border-sm me-1" role="status"></span>
                        Verifying...
                    </span>
                </td>
                <td class="text-end">
                    <button class="btn btn-sm btn-outline-secondary" disabled>
                        Download
                    </button>
                </td>
            </tr>
        `;
    }).join("");
}

function updateLogCertificateRow(partId, html) {
    const row = document.querySelector(
        `#logCertificatesBody tr[data-log-part-id="${CSS.escape(String(partId))}"]`
    );

    if (!row) return;

    row.innerHTML = html;
}

async function openLogCertificatesModal(request) {
    const modalElement = ensureLogCertificatesModal();
    const modal = bootstrap.Modal.getOrCreateInstance(modalElement);

    const statusBox = document.getElementById("logCertificatesStatus");
    const title = document.getElementById("logCertificatesModalLabel");

    if (title) {
        title.textContent = `Log Certificates — ${request.id_intern || request.id}`;
    }

    if (statusBox) {
        statusBox.innerHTML = `
            <div class="d-flex align-items-center gap-2 text-muted">
                <div class="spinner-border spinner-border-sm" role="status"></div>
                <span>Verifying log chains...</span>
            </div>
        `;
    }

    renderLogCertificatePendingRows(request);

    modal.show();

    const parts = Array.isArray(request.rawParts) ? request.rawParts : [];
    let validCount = 0;
    let errorCount = 0;

    for (const part of parts) {
        const partId = part.id || part.part_id;
        const combinedId = request.combinedId || request.id;

        if (!partId) {
            errorCount++;

            updateLogCertificateRow(partId, `
                <td>${escapeHtml(getBudgetPartLabel(part))}</td>
                <td>
                    <span class="badge bg-danger">Error</span>
                    <div class="small text-muted mt-1">Missing part ID</div>
                </td>
                <td class="text-end">
                    <button class="btn btn-sm btn-outline-secondary" disabled>
                        Download
                    </button>
                </td>
            `);

            continue;
        }

        try {
            const result = await verifyBudgetLogChain(combinedId, partId);

            if (!result.ok) {
                errorCount++;

                updateLogCertificateRow(partId, `
                    <td>${escapeHtml(getBudgetPartLabel(part))}</td>
                    <td>
                        <span class="badge bg-danger">Invalid chain</span>
                        <div class="small text-muted mt-1">
                            ${escapeHtml(result.error || "The log chain could not be verified")}
                        </div>
                    </td>
                    <td class="text-end">
                        <button class="btn btn-sm btn-outline-secondary" disabled>
                            Download
                        </button>
                    </td>
                `);

                continue;
            }

            validCount++;

            const downloadUrl = buildLogCertificateDownloadUrl(combinedId, partId);

            updateLogCertificateRow(partId, `
                <td>${escapeHtml(getBudgetPartLabel(part))}</td>
                <td>
                    <span class="badge bg-success">Verified</span>
                    ${result.lastHash ? `
                        <div class="small text-muted mt-1">
                            Last hash: ${escapeHtml(String(result.lastHash))}
                        </div>
                    ` : ""}
                </td>
                <td class="text-end">
                    <a
                        href="${escapeHtml(downloadUrl)}"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="btn btn-sm btn-outline-primary d-inline-flex align-items-center justify-content-center text-center"
                    >
                        Download PDF
                    </a>
                </td>
            `);

        } catch (error) {
            console.error(error);
            errorCount++;

            updateLogCertificateRow(partId, `
                <td>${escapeHtml(getBudgetPartLabel(part))}</td>
                <td>
                    <span class="badge bg-danger">Error</span>
                    <div class="small text-muted mt-1">
                        ${escapeHtml(error.message || "Verification failed")}
                    </div>
                </td>
                <td class="text-end">
                    <button class="btn btn-sm btn-outline-secondary" disabled>
                        Download
                    </button>
                </td>
            `);
        }
    }

    if (statusBox) {
        if (errorCount > 0) {
            statusBox.innerHTML = `
                <div class="alert alert-warning mb-0">
                    Verification finished. ${validCount} chain(s) verified, ${errorCount} with errors.
                </div>
            `;
        } else {
            statusBox.innerHTML = `
                <div class="alert alert-success mb-0">
                    All log chains have been verified successfully.
                </div>
            `;
        }
    }
}

// ---------------- FILTER EVENTS ----------------

function applyInternalIdSearchToTable() {
    const query = String(allRequestsInternalIdSearch?.value || "")
        .trim()
        .toLowerCase();

    const rows = Array.from(allRequestsBody.querySelectorAll("tr"));

    rows.forEach(row => {
        const idIntern = String(row.dataset.idIntern || "").toLowerCase();

        const shouldShow =
            !query ||
            idIntern.includes(query);

        row.classList.toggle("d-none", !shouldShow);
    });
}

filterCategories?.addEventListener("change", updateTravelAreaFilterAvailability);
allRequestsInternalIdSearch?.addEventListener("input", applyInternalIdSearchToTable);

clearAllRequestsSearchBtn?.addEventListener("click", () => {
    if (allRequestsInternalIdSearch) {
        allRequestsInternalIdSearch.value = "";
    }

    applyInternalIdSearchToTable();
});

resetAllRequestsFiltersBtn?.addEventListener("click", () => {
    if (filterCreationFrom) filterCreationFrom.value = "";
    if (filterCreationTo) filterCreationTo.value = "";

    filterCategories?.tomselect?.clear();
    filterTravelAreas?.tomselect?.clear();
    filterProjects?.tomselect?.clear();
    filterResponsibles?.tomselect?.clear();

    if (!filterCategories?.tomselect) {
        [filterCategories, filterTravelAreas, filterProjects, filterResponsibles].forEach(select => {
            if (!select) return;

            Array.from(select.options).forEach(option => {
                option.selected = false;
            });
        });
    }

    updateTravelAreaFilterAvailability();
});


// ---------------- EXCEL EXPORT ----------------

function getAllRequestsExportFilters() {
    return {
        idIntern: String(allRequestsInternalIdSearch?.value || "").trim(),
        creationFrom: filterCreationFrom?.value || "",
        creationTo: filterCreationTo?.value || "",
        categories: getMultiSelectValues(filterCategories),
        travelAreas: getMultiSelectValues(filterTravelAreas),
        projects: getMultiSelectValues(filterProjects),
        responsibles: getMultiSelectValues(filterResponsibles),
    };
}

function buildFilteredExportQueryParams(filters) {
    const params = new URLSearchParams();

    if (filters.idIntern) {
        params.set("idIntern", filters.idIntern);
    }

    if (filters.creationFrom) {
        params.set("creationFrom", filters.creationFrom);
    }

    if (filters.creationTo) {
        params.set("creationTo", filters.creationTo);
    }

    filters.categories.forEach(category => {
        params.append("category", category);
    });

    filters.travelAreas.forEach(area => {
        params.append("travelArea", area);
    });

    filters.projects.forEach(projectId => {
        params.append("projectId", projectId);
    });

    filters.responsibles.forEach(responsibleId => {
        params.append("responsibleId", responsibleId);
    });

    return params;
}

exportFilteredRequestsBtn?.addEventListener("click", async () => {
    try {
        const filters = getAllRequestsExportFilters();
        const params = buildFilteredExportQueryParams(filters);

        const response = await fetch(
            `/module/proxy/module8/api?action=exportFilteredRequestsExcel&${params.toString()}`
        );

        if (!response.ok) {
            throw new Error("Error exporting filtered Excel");
        }

        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);

        const a = document.createElement("a");
        const dateTime = new Date().toISOString().replace(/[:.]/g, "");

        a.href = url;
        a.download = `filtered_requests_${dateTime}.xlsx`;
        document.body.appendChild(a);
        a.click();
        a.remove();

        window.URL.revokeObjectURL(url);
    } catch (error) {
        console.error(error);
        alert("Error exporting filtered Excel");
    }
});

exportAllRequestsBtn?.addEventListener("click", async () => {
    try {
        const response = await fetch("/module/proxy/module8/api?action=exportAllRequestsExcel");

        if (!response.ok) {
            throw new Error("Error exporting Excel");
        }

        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);

        const a = document.createElement("a");
        const dateTime = new Date().toISOString().replace(/[:.]/g, "");

        a.href = url;
        a.download = `all_requests_${dateTime}.xlsx`;
        document.body.appendChild(a);
        a.click();
        a.remove();

        window.URL.revokeObjectURL(url);
    } catch (error) {
        console.error(error);
        alert("Error exporting Excel");
    }
});


// ---------------- LOAD REQUESTS ----------------

async function loadAllRequests() {
    try {
        const [projectsResponse, requestsResponse] = await Promise.all([
            fetch("/module/proxy/module8/api?action=getAllProjects"),
            fetch("/module/proxy/module8/api?action=getAllRequests")
        ]);

        const projectsData = await projectsResponse.json();
        const requestsData = await requestsResponse.json();

        ALL_PROJECTS_FOR_ALL_REQUESTS = Array.isArray(projectsData.projects)
            ? projectsData.projects
            : Array.isArray(projectsData)
                ? projectsData
                : [];

        const rawRequests = Array.isArray(requestsData.requests)
            ? requestsData.requests
            : Array.isArray(requestsData)
                ? requestsData
                : [];

        const normalizedRequests = rawRequests.flatMap(normalizeAllRequestsFromBackend);

        ALL_REQUESTS_CACHE = {};
        normalizedRequests.forEach(req => {
            ALL_REQUESTS_CACHE[req.id] = req;
        });

        populateProjectFilter();
        populateResponsibleFilter();
        initAllRequestsTomSelects();

        if (!normalizedRequests.length) {
            allRequestsBody.innerHTML = `
                <tr>
                    <td colspan="7" class="text-center text-muted py-4">
                        No requests found
                    </td>
                </tr>
            `;

            refreshOriginalAllRows();
            return;
        }

        const sortedRequests = [...normalizedRequests].sort((a, b) => {
            const aDate = new Date(a.updatedAt || 0).getTime();
            const bDate = new Date(b.updatedAt || 0).getTime();

            return bDate - aDate; // newest first
        });

        allRequestsBody.innerHTML = sortedRequests
            .map(buildAllRequestRow)
            .join("");

        refreshOriginalAllRows();

        allProjectSortState = "default";
        allWorkerSortState = "default";
        allStatusSortState = "default";
        allLastUpdateSortState = "newest";

        resetAllSortIndicators();
        allLastUpdateSortIndicator.textContent = "↓";

    } catch (error) {
        console.error("Error loading all requests:", error);

        allRequestsBody.innerHTML = `
            <tr>
                <td colspan="7" class="text-center text-danger py-4">
                    Error loading all requests
                </td>
            </tr>
        `;

        refreshOriginalAllRows();
    }
}


// ---------------- VIEW MODAL ----------------

document.addEventListener("click", async e => {
    const certificateLink = e.target.closest("[data-attendance-certificate-link]");
    if (!certificateLink) return;

    e.preventDefault();

    const requestId = certificateLink.dataset.requestId;
    const request = ALL_REQUESTS_CACHE[requestId];
    if (!request) return;
    const certificateUrl = certificateLink.href;

    try {
        const response = await fetch("/module/proxy/module8/api?action=markAttendanceCertificateViewed", {
            method: "POST",
            headers: {
                "Accept": "application/json",
                "Content-Type": "application/json"
            },
            credentials: "include",
            body: JSON.stringify({ combinedId: Number(requestId) })
        });

        if (!response.ok) {
            throw new Error("Could not mark attendance certificate as viewed");
        }

        request.attendanceCertificateViewed = true;
        certificateLink.closest("tr")?.classList.remove("attendance-certificate-unopened");
        await loadAllRequests();
    } catch (error) {
        console.error(error);
    } finally {
        if (certificateUrl) {
            window.open(certificateUrl, "_blank", "noopener");
        }
    }
});

document.addEventListener("click", e => {
    const viewBtn = e.target.closest(".all-view-request-btn");
    if (!viewBtn) return;

    const requestId = viewBtn.dataset.requestId;
    const request = ALL_REQUESTS_CACHE[requestId];

    if (!request) {
        alert("Request not found");
        return;
    }

    openViewModal(request);
});

document.addEventListener("click", e => {
    const logBtn = e.target.closest(".all-log-certificates-btn");
    if (!logBtn) return;

    if (!CAN_SEE_LOG_CERTIFICATES) {
        alert("You do not have permission to access log certificates.");
        return;
    }

    const requestId = logBtn.dataset.requestId;
    const request = ALL_REQUESTS_CACHE[requestId];

    if (!request) {
        alert("Request not found");
        return;
    }

    openLogCertificatesModal(request);
});


// ---------------- SORT FUNCTIONALITY ----------------

function resetAllSortIndicators() {
    allProjectSortIndicator.textContent = "↕";
    allWorkerSortIndicator.textContent = "↕";
    allStatusSortIndicator.textContent = "↕";
    allLastUpdateSortIndicator.textContent = "↕";
}

function restoreOriginalAllRows() {
    allRequestsBody.innerHTML = "";
    originalAllRows.forEach(row => allRequestsBody.appendChild(row.cloneNode(true)));
}

function refreshOriginalAllRows() {
    originalAllRows = Array.from(allRequestsBody.querySelectorAll("tr"))
        .map(row => row.cloneNode(true));
}

function sortAllRowsByProject() {
    const rows = Array.from(allRequestsBody.querySelectorAll("tr"));

    rows.sort((a, b) => {
        const aValue = (a.dataset.project || "").toLowerCase().trim();
        const bValue = (b.dataset.project || "").toLowerCase().trim();

        return aValue.localeCompare(bValue);
    });

    allRequestsBody.innerHTML = "";
    rows.forEach(row => allRequestsBody.appendChild(row));
}

function sortAllRowsByWorker() {
    const rows = Array.from(allRequestsBody.querySelectorAll("tr"));

    rows.sort((a, b) => {
        const aSurname = (a.dataset.workerSurname || "").toLowerCase().trim();
        const bSurname = (b.dataset.workerSurname || "").toLowerCase().trim();

        const surnameCompare = aSurname.localeCompare(bSurname);
        if (surnameCompare !== 0) return surnameCompare;

        const aName = (a.dataset.workerName || "").toLowerCase().trim();
        const bName = (b.dataset.workerName || "").toLowerCase().trim();

        return aName.localeCompare(bName);
    });

    allRequestsBody.innerHTML = "";
    rows.forEach(row => allRequestsBody.appendChild(row));
}

function sortAllRowsByStatus() {
    const rows = Array.from(allRequestsBody.querySelectorAll("tr"));

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

    allRequestsBody.innerHTML = "";
    rows.forEach(row => allRequestsBody.appendChild(row));
}

function sortAllRowsByLastUpdate(direction = "newest") {
    const rows = Array.from(allRequestsBody.querySelectorAll("tr"));

    rows.sort((a, b) => {
        const aDate = new Date(a.dataset.lastUpdate || 0).getTime();
        const bDate = new Date(b.dataset.lastUpdate || 0).getTime();

        if (direction === "oldest") return aDate - bDate;

        return bDate - aDate;
    });

    allRequestsBody.innerHTML = "";
    rows.forEach(row => allRequestsBody.appendChild(row));
}

allRequestsProjectHeader.addEventListener("click", () => {
    allWorkerSortState = "default";
    allStatusSortState = "default";
    allLastUpdateSortState = "default";

    if (allProjectSortState === "default") {
        allProjectSortState = "alphabetical";
        sortAllRowsByProject();
        resetAllSortIndicators();
        allProjectSortIndicator.textContent = "↓";
        return;
    }

    allProjectSortState = "default";
    restoreOriginalAllRows();
    resetAllSortIndicators();
});

allRequestsWorkerHeader.addEventListener("click", () => {
    allProjectSortState = "default";
    allStatusSortState = "default";
    allLastUpdateSortState = "default";

    if (allWorkerSortState === "default") {
        allWorkerSortState = "alphabetical";
        sortAllRowsByWorker();
        resetAllSortIndicators();
        allWorkerSortIndicator.textContent = "↓";
        return;
    }

    allWorkerSortState = "default";
    restoreOriginalAllRows();
    resetAllSortIndicators();
});

allRequestsStatusHeader.addEventListener("click", () => {
    allProjectSortState = "default";
    allWorkerSortState = "default";
    allLastUpdateSortState = "default";

    if (allStatusSortState === "default") {
        allStatusSortState = "status";
        sortAllRowsByStatus();
        resetAllSortIndicators();
        allStatusSortIndicator.textContent = "↓";
        return;
    }

    allStatusSortState = "default";
    restoreOriginalAllRows();
    resetAllSortIndicators();
});

allRequestsLastUpdateHeader.addEventListener("click", () => {
    allProjectSortState = "default";
    allWorkerSortState = "default";
    allStatusSortState = "default";

    if (allLastUpdateSortState === "default") {
        allLastUpdateSortState = "newest";
        sortAllRowsByLastUpdate("newest");
        resetAllSortIndicators();
        allLastUpdateSortIndicator.textContent = "↓";
        return;
    }

    if (allLastUpdateSortState === "newest") {
        allLastUpdateSortState = "oldest";
        sortAllRowsByLastUpdate("oldest");
        resetAllSortIndicators();
        allLastUpdateSortIndicator.textContent = "↑";
        return;
    }

    allLastUpdateSortState = "default";
    restoreOriginalAllRows();
    resetAllSortIndicators();
});


// ---------------- INIT ----------------

document.addEventListener("DOMContentLoaded", async() => {
    await loadAllRequestsUserPermissions();
    loadAllRequests();
});
