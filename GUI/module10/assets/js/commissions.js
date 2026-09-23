import {
  getSession,
  getMyCommissions,
  getProjectCommissions,
  getStakeholderCommissions,
  getCommissionDetail,
  approveCommission,
  saveCommissionReviewFields,
  rejectCommission,
  approveStakeholder,
  getDataExportOptions,
  downloadDataExport,
  finalPDFUrl,
  previewCommissionPDFUrl,
  addCenterExpense,
  deleteCenterExpense,
  excludeExpense,
  getPricingTables,
  saveExpensePricingReview,
  saveMileageReview,
  saveExpenseEdit,
  exportCommissions
} from "./travelApi.js";

let session = null;
let myCommissions = [];
let projectCommissions = [];
let stakeholderCommissions = [];
let pricingTables = [];
let selectedCommissionId = null;
let currentReviewReadOnly = false;
let reviewHeaderSaveTimer = null;
let stakeholderCommissionsLoaded = false;
let dataExportOptionsLoaded = false;

export async function initCommissionsWorkspace() {
  await loadSession();
  bindFilters();
  bindProjectActions();
  bindCenterExpenseForm();
  bindDataExportPanel();

  await refreshAllCommissions();

  window.addEventListener("commission:created", async () => {
    await refreshAllCommissions();
  });
}

async function loadSession() {
  try {
    session = await getSession();
    const projectTab = document.getElementById("project-review-tab");
    projectTab?.closest("li")?.classList.toggle("d-none", !session?.can_review);
    document.getElementById("pendingReviewSummaryCard")?.classList.toggle("d-none", !session?.can_review);
    document.getElementById("readyToExportSummaryCard")?.classList.toggle("d-none", !session?.can_review);
    document.getElementById("serviceSummaryGrid")?.classList.toggle("service-summary-grid--single", !session?.can_review);
  } catch (error) {
    console.error(error);
    document.getElementById("project-review-tab")?.closest("li")?.classList.add("d-none");
    document.getElementById("pendingReviewSummaryCard")?.classList.add("d-none");
    document.getElementById("readyToExportSummaryCard")?.classList.add("d-none");
    document.getElementById("serviceSummaryGrid")?.classList.add("service-summary-grid--single");
  }
}

async function refreshAllCommissions() {
  await loadMyCommissions();
  updateStakeholderTabVisibility();

  if (session?.can_review) {
    await loadProjectCommissions();
    updateStakeholderTabVisibility();
  }

  if (stakeholderCommissionsLoaded || hasFinalReviewCommissions()) {
    await loadStakeholderCommissions();
  }

  updateSummaryCards();
}

async function loadMyCommissions() {
  try {
    myCommissions = normalizeRows(await getMyCommissions());
    renderMyCommissions();
    renderLatestCommission();
  } catch (error) {
    console.error(error);
    renderTableError("previousCommissionsBody", 7, "Could not load your commissions.");
  }
}

async function loadProjectCommissions() {
  try {
    projectCommissions = normalizeRows(await getProjectCommissions());
    renderProjectCommissions();
  } catch (error) {
    console.error(error);
    renderTableError("projectCommissionsBody", 8, "Could not load project review commissions.");
  }
}

async function loadStakeholderCommissions() {
  try {
    stakeholderCommissionsLoaded = true;
    stakeholderCommissions = normalizeRows(await getStakeholderCommissions());
    renderStakeholderCommissions();
    updateStakeholderTabVisibility();
  } catch (error) {
    console.error(error);
    stakeholderCommissions = [];
    renderTableError("stakeholderCommissionsBody", 7, "Could not load final approvals.");
    updateStakeholderTabVisibility();
  }
}

function bindFilters() {
  ["myCommissionSearchInput", "myCommissionStatusFilter"].forEach((id) => {
    document.getElementById(id)?.addEventListener("input", renderMyCommissions);
    document.getElementById(id)?.addEventListener("change", renderMyCommissions);
  });

  ["projectSearchInput", "projectStatusFilter"].forEach((id) => {
    document.getElementById(id)?.addEventListener("input", renderProjectCommissions);
    document.getElementById(id)?.addEventListener("change", renderProjectCommissions);
  });

  ["stakeholderSearchInput", "stakeholderStatusFilter"].forEach((id) => {
    document.getElementById(id)?.addEventListener("input", renderStakeholderCommissions);
    document.getElementById(id)?.addEventListener("change", renderStakeholderCommissions);
  });
}

function bindProjectActions() {
  document.getElementById("stakeholder-approval-tab")?.addEventListener("shown.bs.tab", async () => {
    await loadStakeholderCommissions();
  });

  document.getElementById("projectCommissionsBody")?.addEventListener("click", async (event) => {
    const reviewBtn = event.target.closest("[data-review-commission]");
    if (reviewBtn) {
      await openReviewModal(reviewBtn.dataset.reviewCommission);
      return;
    }

    const approveBtn = event.target.closest("[data-approve-commission]");
    if (approveBtn) {
      await withButtonLock(approveBtn, () => changeCommissionStatus(approveBtn.dataset.approveCommission, "approve"), "Working...");
      return;
    }

    const rejectBtn = event.target.closest("[data-reject-commission]");
    if (rejectBtn) {
      await withButtonLock(rejectBtn, () => changeCommissionStatus(rejectBtn.dataset.rejectCommission, "reject"), "Working...");
      return;
    }
  });

  document.getElementById("stakeholderCommissionsBody")?.addEventListener("click", async (event) => {
    const approveBtn = event.target.closest("[data-approve-stakeholder]");
    if (approveBtn) {
      await withButtonLock(approveBtn, () => approveStakeholderCommission(approveBtn.dataset.approveStakeholder), "Working...");
    }
  });

  document.getElementById("projectCommissionExpensesBody")?.addEventListener("click", async (event) => {
    if (currentReviewReadOnly) return;

    const saveExpenseEditBtn = event.target.closest("[data-save-expense-edit]");
    if (saveExpenseEditBtn) {
      await withButtonLock(saveExpenseEditBtn, () => saveExpenseEditFromRow(saveExpenseEditBtn.closest("tr")), "Saving...");
      return;
    }

    const deleteCenterExpenseBtn = event.target.closest("[data-delete-center-expense]");
    if (deleteCenterExpenseBtn) {
      await withButtonLock(deleteCenterExpenseBtn, () => deleteCenterExpenseFromRow(deleteCenterExpenseBtn.closest("tr")), "Deleting...");
      return;
    }

    const deleteExpenseBtn = event.target.closest("[data-delete-expense]");
    if (deleteExpenseBtn) {
      await withButtonLock(deleteExpenseBtn, () => excludeExpenseFromRow(deleteExpenseBtn.closest("tr")), "Deleting...");
    }
  });

  document.getElementById("previousCommissionsBody")?.addEventListener("click", async (event) => {
    const viewBtn = event.target.closest("[data-view-commission]");
    if (viewBtn) await openLatestCommissionModal(viewBtn.dataset.viewCommission);
  });

  document.getElementById("approveCommissionBtn")?.addEventListener("click", async (event) => {
    if (selectedCommissionId) await withButtonLock(event.currentTarget, () => changeCommissionStatus(selectedCommissionId, "approve"), "Approving...");
  });

  document.getElementById("rejectCommissionBtn")?.addEventListener("click", async (event) => {
    if (selectedCommissionId) await withButtonLock(event.currentTarget, () => changeCommissionStatus(selectedCommissionId, "reject"), "Rejecting...");
  });

  document.getElementById("previewCommissionPdfBtn")?.addEventListener("click", () => {
    if (!selectedCommissionId) return;
    window.open(previewCommissionPDFUrl(selectedCommissionId, getSelectedReviewPeopleCategory(), getSelectedReviewRelatedTask()), "_blank", "noopener");
  });
  document.getElementById("reviewResearcherCategory")?.addEventListener("change", () => {
    saveReviewHeaderFields();
  });
  document.getElementById("reviewRelatedTask")?.addEventListener("input", () => {
    scheduleReviewHeaderSave();
  });
  document.getElementById("reviewRelatedTask")?.addEventListener("blur", () => {
    saveReviewHeaderFields();
  });

  document.getElementById("exportCommissionsBtn")?.addEventListener("click", async (event) => {
    await withButtonLock(event.currentTarget, () => exportProjectCommissions(), "Exporting...");
  });

  document.getElementById("exportSelectedCommissionsBtn")?.addEventListener("click", async (event) => {
    await withButtonLock(event.currentTarget, () => exportProjectCommissions(getSelectedProjectCommissionIds()), "Exporting...");
  });

  document.getElementById("selectAllProjectCommissions")?.addEventListener("change", (event) => {
    document.querySelectorAll(".project-commission-checkbox").forEach((checkbox) => {
      checkbox.checked = event.target.checked;
    });
  });

  document.getElementById("projectCommissionExpensesBody")?.addEventListener("change", (event) => {
    if (currentReviewReadOnly) return;

    const row = event.target.closest("tr");
    if (!row) return;
    if (event.target.closest(".pricing-review-control")) updatePricingReviewRow(row);
    if (event.target.closest(".mileage-distance-input, .mileage-rate-input")) updateMileageReviewRow(row);
  });
}

function bindDataExportPanel() {
  document.getElementById("serviceCommissionDataExportPanel")?.addEventListener("toggle", async (event) => {
    if (event.target.open && !dataExportOptionsLoaded) {
      await loadDataExportOptions();
    }
  });

  document.getElementById("selectAllDataExportProjects")?.addEventListener("click", () => {
    selectAllOptions(document.getElementById("dataExportProjects"));
  });
  document.getElementById("clearDataExportProjects")?.addEventListener("click", () => {
    clearAllOptions(document.getElementById("dataExportProjects"));
  });
  document.getElementById("selectAllDataExportPeople")?.addEventListener("click", () => {
    selectAllOptions(document.getElementById("dataExportPeople"));
  });
  document.getElementById("clearDataExportPeople")?.addEventListener("click", () => {
    clearAllOptions(document.getElementById("dataExportPeople"));
  });
  document.getElementById("downloadDataExportBtn")?.addEventListener("click", async (event) => {
    await withButtonLock(event.currentTarget, async () => {
      if (!dataExportOptionsLoaded) await loadDataExportOptions();
      await downloadDataExport(buildDataExportPayload());
    }, "Downloading...");
  });
}

async function loadDataExportOptions() {
  const hint = document.getElementById("dataExportHint");
  if (hint) hint.textContent = "Loading projects and people...";
  try {
    const options = await getDataExportOptions();
    renderDataExportOptions("dataExportProjects", options.projects || []);
    renderDataExportOptions("dataExportPeople", options.people || []);
    selectAllOptions(document.getElementById("dataExportProjects"));
    selectAllOptions(document.getElementById("dataExportPeople"));
    dataExportOptionsLoaded = true;
    if (hint) hint.textContent = "Select filters and document types to download an ordered ZIP.";
  } catch (error) {
    console.error(error);
    if (hint) hint.textContent = "Could not load data exploitation options.";
  }
}

function renderDataExportOptions(selectId, options) {
  const list = document.getElementById(selectId);
  if (!list) return;
  list.innerHTML = options.map((option) => `
    <label class="data-export-option">
      <input class="form-check-input" type="checkbox" value="${escapeAttribute(option.id)}" checked data-export-option>
      <span>${escapeHtml(option.name || option.id)}</span>
    </label>
  `).join("");
}

function selectAllOptions(list) {
  if (!list) return;
  list.querySelectorAll("[data-export-option]").forEach((option) => {
    option.checked = true;
  });
}

function clearAllOptions(list) {
  if (!list) return;
  list.querySelectorAll("[data-export-option]").forEach((option) => {
    option.checked = false;
  });
}

function selectedOptionValues(selectId) {
  const list = document.getElementById(selectId);
  if (!list) return [];
  return [...list.querySelectorAll("[data-export-option]:checked")].map((option) => option.value);
}

function buildDataExportPayload() {
  const documents = [...document.querySelectorAll(".data-export-document:checked")].map((checkbox) => checkbox.value);
  return {
    date_from: document.getElementById("dataExportDateFrom")?.value || "",
    date_to: document.getElementById("dataExportDateTo")?.value || "",
    projects: selectedOptionValues("dataExportProjects"),
    people: selectedOptionValues("dataExportPeople").map((value) => Number(value)).filter((value) => Number.isFinite(value) && value > 0),
    documents
  };
}

async function exportProjectCommissions(ids = []) {
  try {
    await exportCommissions(ids);
    await refreshAllCommissions();
  } catch (error) {
    console.error(error);
    alert(error?.message || "Could not export commissions.");
  }
}

function bindCenterExpenseForm() {
  const form = document.getElementById("centerExpenseForm");
  if (!form) return;

  form.addEventListener("submit", async (event) => {
    event.preventDefault();

    const submitButton = event.submitter || form.querySelector("[type='submit']");
    const commissionId = Number(document.getElementById("centerExpenseCommissionId")?.value || selectedCommissionId || 0);
    const amount = Number(String(form.amount?.value || "").replace(",", "."));
    const payload = {
      commission_id: commissionId,
      type: form.expense_type?.value || "other",
      amount,
      description: form.description?.value || ""
    };

    if (!payload.commission_id || !payload.type || payload.amount <= 0) {
      alert("Select a commission, expense type and valid amount.");
      return;
    }

    await withButtonLock(submitButton, async () => {
      const result = await addCenterExpense(payload);
      applyCommissionStatus(payload.commission_id, result?.status || "under_review");
      form.reset();
      bootstrap.Modal.getInstance(document.getElementById("centerExpenseModal"))?.hide();
      await refreshAllCommissions();
      await openReviewModal(payload.commission_id);
    }, "Saving...").catch((error) => {
      console.error(error);
      alert("Could not add center expense.");
    });
  });
}

function renderMyCommissions() {
  const tbody = document.getElementById("previousCommissionsBody");
  if (!tbody) return;

  const rows = filterRows(myCommissions, "myCommissionSearchInput", "myCommissionStatusFilter");

  if (!rows.length) {
    tbody.innerHTML = emptyRow(7, "No previous service commissions found.");
    return;
  }

  tbody.innerHTML = rows.map((row) => `
    <tr>
      <td>${escapeHtml(row.code || `SC-${row.id}`)}</td>
      <td>${escapeHtml(row.travel || "-")}</td>
      <td>${escapeHtml(row.destination || "-")}</td>
      <td>${formatDate(row.submitted)}</td>
      <td>${formatMyCommissionAmount(row)}</td>
      <td>${statusBadge(row.status)}</td>
      <td class="text-end">
        <button type="button" class="btn btn-sm btn-outline-secondary" data-view-commission="${row.id}">View</button>
      </td>
    </tr>
  `).join("");
}

function renderProjectCommissions() {
  const tbody = document.getElementById("projectCommissionsBody");
  if (!tbody) return;

  const rows = filterRows(projectCommissions, "projectSearchInput", "projectStatusFilter");

  if (!rows.length) {
    tbody.innerHTML = emptyRow(8, "No commissions pending project-team review.");
    return;
  }

  tbody.innerHTML = rows.map((row) => `
    <tr>
      <td><input class="form-check-input project-commission-checkbox" type="checkbox" value="${row.id}" aria-label="Select commission ${row.id}"></td>
      <td>${escapeHtml(row.code || `SC-${row.id}`)}</td>
      <td>${escapeHtml(row.applicant || "-")}</td>
      <td>${escapeHtml(row.project || "-")}</td>
      <td>${escapeHtml(row.destination || "-")}</td>
      <td>${formatProjectReviewTotal(row)}</td>
      <td>${statusBadge(row.status)}</td>
      <td class="text-end">
        ${renderProjectCommissionActions(row)}
      </td>
    </tr>
  `).join("");

  updateProjectKpis(rows);
}

function renderStakeholderCommissions() {
  const tbody = document.getElementById("stakeholderCommissionsBody");
  if (!tbody) return;

  const rows = filterRows(stakeholderCommissions.map((row) => ({ ...row, status: row.approval_status })), "stakeholderSearchInput", "stakeholderStatusFilter");
  if (!rows.length) {
    tbody.innerHTML = emptyRow(7, "No final approvals assigned to you.");
    return;
  }

  tbody.innerHTML = rows.map((row) => `
    <tr>
      <td>${escapeHtml(row.code || `SC-${row.service_id}`)}</td>
      <td>${escapeHtml(row.applicant || "-")}</td>
      <td>${escapeHtml(row.project || "-")}</td>
      <td>${escapeHtml(readableApproverType(row.approver_type))}</td>
      <td>${statusBadge(row.approval_status)}</td>
      <td>${row.approved_at ? formatDate(row.approved_at) : "-"}</td>
      <td class="text-end">
        <div class="btn-group btn-group-sm" role="group">
          <a class="btn btn-outline-secondary" href="${finalPDFUrl(row.service_id)}" target="_blank" rel="noopener">PDF</a>
          ${row.approval_status === "approved" ? "" : `<button type="button" class="btn btn-outline-success" data-approve-stakeholder="${row.approval_id}">Approve</button>`}
        </div>
      </td>
    </tr>
  `).join("");
}

function updateStakeholderTabVisibility() {
  const tabItem = document.getElementById("stakeholder-approval-tab")?.closest("li");
  if (!tabItem) return;

  const shouldShow = stakeholderCommissions.length > 0 || hasFinalReviewCommissions() || Boolean(session?.can_review);
  tabItem.classList.toggle("d-none", !shouldShow);
}

function hasFinalReviewCommissions() {
  return [...myCommissions, ...projectCommissions].some((row) => row.status === "final_review");
}

async function approveStakeholderCommission(approvalId) {
  try {
    await approveStakeholder(approvalId, "Confirmed from CRMIntratools");
    await refreshAllCommissions();
  } catch (error) {
    console.error(error);
    alert("Could not approve final validation.");
  }
}

function readableApproverType(value) {
  const map = {
    applicant: "Applicant",
    project_responsible: "Project responsible",
    management: "Management"
  };
  return map[value] || value || "-";
}

function renderProjectCommissionActions(row) {
  const finalStatus = isFinalCommissionStatus(row.status);

  return `
    <div class="btn-group btn-group-sm" role="group">
      <button type="button" class="btn btn-outline-secondary" data-review-commission="${row.id}">Review</button>
      ${finalStatus ? "" : `
        <button type="button" class="btn btn-outline-success" data-approve-commission="${row.id}">Approve</button>
        <button type="button" class="btn btn-outline-danger" data-reject-commission="${row.id}">Reject</button>
      `}
    </div>
  `;
}

function formatProjectReviewTotal(row) {
  const userTotal = Number(row.user_total || 0);
  const centerTotal = Number(row.center_total || 0);
  return formatMoney(userTotal + centerTotal);
}

function renderLatestCommission() {
  const latest = myCommissions[0];
  const title = document.getElementById("latestCommissionTravelName");
  const message = document.getElementById("latestCommissionMessage");
  const badge = document.getElementById("latestCommissionStatus");
  const summary = document.getElementById("latestCommissionSummary");
  const submitted = document.getElementById("latestCommissionSubmitted");
  const total = document.getElementById("latestCommissionTotal");
  const viewBtn = document.getElementById("viewFullStatusBtn");

  if (!latest) {
    if (title) title.textContent = "No commissions submitted yet.";
    if (message) message.textContent = "Your latest service commission will appear here once submitted.";
    badge?.classList.add("d-none");
    summary?.classList.add("d-none");
    viewBtn?.classList.add("d-none");
    return;
  }

  if (title) title.textContent = latest.destination || latest.travel || latest.code || `Commission ${latest.id}`;
  if (message) message.textContent = `${latest.code || `SC-${latest.id}`} · ${formatDate(latest.submitted)}`;
  if (badge) {
    badge.textContent = readableStatus(latest.status);
    badge.classList.remove("d-none");
  }
  if (summary) summary.classList.remove("d-none");
  if (submitted) submitted.textContent = formatDate(latest.submitted);
  if (total) total.textContent = formatMyCommissionAmount(latest);
  if (viewBtn) {
    viewBtn.classList.remove("d-none");
    viewBtn.onclick = () => openLatestCommissionModal(latest.id);
  }
}

async function openLatestCommissionModal(id) {
  const commission = myCommissions.find((row) => Number(row.id) === Number(id)) || {};
  const subtitle = document.getElementById("latestCommissionModalSubtitle");
  const body = document.getElementById("latestCommissionModalBody");

  if (subtitle) subtitle.textContent = commission.code || `SC-${id}`;
  if (body) body.innerHTML = `<p class="text-muted mb-0">Loading commission status...</p>`;

  const modalEl = document.getElementById("latestCommissionModal");
  if (modalEl && window.bootstrap) bootstrap.Modal.getOrCreateInstance(modalEl).show();

  try {
    const expenses = normalizeRows(await getCommissionDetail(id));
    const detail = expenses.find((expense) => expense.applicant || expense.project || expense.destination) || {};
    if (body) body.innerHTML = renderLatestCommissionModalBody(commission, detail, expenses);
  } catch (error) {
    console.error(error);
    if (body) body.innerHTML = `<p class="text-danger mb-0">Could not load this commission.</p>`;
  }
}

function renderLatestCommissionModalBody(commission, detail, expenses) {
  const total = Number(commission.user_total || commission.total || 0) + Number(commission.center_total || 0);
  const expenseRows = expenses.length
    ? expenses.map((expense) => `
      <tr>
        <td>${escapeHtml(formatExpenseType(expense.type))}</td>
        <td>${escapeHtml(serviceCommissionModalDescription(expense))}</td>
        <td class="text-end">${formatReviewExpenseAmount(expense)}</td>
      </tr>
    `).join("")
    : `<tr><td colspan="3" class="text-center text-muted py-3">No expenses loaded.</td></tr>`;

  return `
    <div class="review-summary-grid mb-4">
      <div>
        <span class="summary-label">Status</span>
        <strong>${escapeHtml(readableStatus(commission.status))}</strong>
      </div>
      <div>
        <span class="summary-label">Submitted</span>
        <strong>${escapeHtml(formatDate(commission.submitted))}</strong>
      </div>
      <div>
        <span class="summary-label">Project</span>
        <strong>${escapeHtml(detail.project || commission.project || "-")}</strong>
      </div>
      <div>
        <span class="summary-label">Destination</span>
        <strong>${escapeHtml(detail.destination || commission.destination || commission.travel || "-")}</strong>
      </div>
      <div>
        <span class="summary-label">Total</span>
        <strong>${formatMoney(total)}</strong>
      </div>
    </div>

    <div class="table-responsive">
      <table class="table table-sm align-middle service-table mb-0">
        <thead class="table-light">
          <tr>
            <th>Expense</th>
            <th>Description</th>
            <th class="text-end">Amount</th>
          </tr>
        </thead>
        <tbody>${expenseRows}</tbody>
      </table>
    </div>
  `;
}

function serviceCommissionModalDescription(expense) {
  return expense.reviewed_description || expense.description || expense.related_task || expense.location_label || "-";
}

async function openReviewModal(id, readOnly = false) {
  selectedCommissionId = Number(id);
  const allRows = [...myCommissions, ...projectCommissions];
  const commission = allRows.find((row) => Number(row.id) === Number(id)) || {};
  const isFinalStatus = isFinalCommissionStatus(commission.status);
  currentReviewReadOnly = readOnly || !session?.can_review || isFinalStatus;

  document.getElementById("centerExpenseCommissionId").value = selectedCommissionId;
  setText("projectCommissionReviewSubtitle", commission.code || `SC-${id}`);
  setText("reviewApplicantName", commission.applicant || session?.username || "-");
  setReviewCategorySelect("", currentReviewReadOnly);
  setReviewRelatedTask("", currentReviewReadOnly);
  setText("reviewResearcherCategoryHint", "");
  setText("reviewProjectName", commission.project || "-");
  setText("reviewDestination", commission.destination || commission.travel || "-");
  setText("reviewTotalAmount", formatMoney(Number(commission.user_total || commission.total || 0) + Number(commission.center_total || 0)));
  setReviewModalMode(currentReviewReadOnly);

  const approveBtn = document.getElementById("approveCommissionBtn");
  const rejectBtn = document.getElementById("rejectCommissionBtn");
  const centerBtn = document.getElementById("addCenterExpenseBtn");
  [approveBtn, rejectBtn].forEach((button) => setElementHidden(button, currentReviewReadOnly || isFinalStatus));
  setElementHidden(centerBtn, true);

  const tbody = document.getElementById("projectCommissionExpensesBody");
  if (tbody) tbody.innerHTML = emptyRow(currentReviewReadOnly ? 5 : 6, "Loading expenses...");

  try {
    await loadPricingTables();
    const expenses = normalizeRows(await getCommissionDetail(id));
    setElementHidden(centerBtn, true);
    updateApproveButtonState(expenses);
    updateReviewSummaryFromDetail(commission, expenses);
    renderManualTravelSummary(commission, expenses);
    renderReviewExpenses(expenses, currentReviewReadOnly);
  } catch (error) {
    console.error(error);
    renderTableError("projectCommissionExpensesBody", currentReviewReadOnly ? 5 : 6, "Could not load expenses.");
  }

  const modalEl = document.getElementById("projectCommissionReviewModal");
  if (modalEl && window.bootstrap) bootstrap.Modal.getOrCreateInstance(modalEl).show();
}

function updateReviewSummaryFromDetail(commission, expenses) {
  const detail = expenses.find((expense) => expense.applicant || expense.project || expense.destination) || {};
  setText("reviewApplicantName", detail.applicant || commission.applicant || session?.username || "-");
  setText("reviewProjectName", detail.project || commission.project || "-");
  setText("reviewDestination", detail.destination || commission.destination || commission.travel || "-");
  updateResearcherCategoryReview(detail);
  updateRelatedTaskReview(detail);
}

function updateResearcherCategoryReview(detail) {
  const savedCategory = formatPeopleCategory(detail.people_category);
  const suggestedCategory = suggestGroupFromAcademicGrade(detail.academic_grade);
  const categoryValue = peopleCategoryValueFromGroup(savedCategory || suggestedCategory);
  const hintEl = document.getElementById("reviewResearcherCategoryHint");
  setReviewCategorySelect(categoryValue, currentReviewReadOnly);
  if (!hintEl) return;

  hintEl.classList.remove("text-danger");
  hintEl.classList.add("text-muted");
  if (!suggestedCategory) {
    hintEl.textContent = "";
    return;
  }

  if (savedCategory && savedCategory !== suggestedCategory) {
    hintEl.textContent = `Suggested ${suggestedCategory} from academic grade`;
    hintEl.classList.remove("text-muted");
    hintEl.classList.add("text-danger");
    return;
  }

  hintEl.textContent = `Suggested ${suggestedCategory} from academic grade`;
}

function setReviewCategorySelect(value, disabled) {
  const select = document.getElementById("reviewResearcherCategory");
  if (!select) return;
  select.value = value === "" || value === null || value === undefined ? "0" : String(value);
  select.disabled = Boolean(disabled);
}

function getSelectedReviewPeopleCategory() {
  const select = document.getElementById("reviewResearcherCategory");
  if (!select || select.disabled) return null;
  const value = Number(select.value);
  return Number.isInteger(value) && value >= 0 && value <= 2 ? value : null;
}

function updateRelatedTaskReview(detail) {
  const task = detail.export_related_task || detail.related_task || detail.manual_travel_purpose || "";
  setReviewRelatedTask(task, currentReviewReadOnly);
}

function setReviewRelatedTask(value, disabled) {
  const input = document.getElementById("reviewRelatedTask");
  if (!input) return;
  input.value = value || "";
  input.disabled = Boolean(disabled);
}

function getSelectedReviewRelatedTask() {
  const input = document.getElementById("reviewRelatedTask");
  if (!input || input.disabled) return null;
  return input.value.trim();
}

function scheduleReviewHeaderSave() {
  clearTimeout(reviewHeaderSaveTimer);
  reviewHeaderSaveTimer = setTimeout(() => {
    saveReviewHeaderFields();
  }, 500);
}

async function saveReviewHeaderFields() {
  if (!selectedCommissionId || currentReviewReadOnly) return;
  clearTimeout(reviewHeaderSaveTimer);

  try {
    const result = await saveCommissionReviewFields(
      selectedCommissionId,
      getSelectedReviewPeopleCategory(),
      getSelectedReviewRelatedTask()
    );
    applyCommissionStatus(selectedCommissionId, result?.status || "under_review");
    await refreshAllCommissions();
    renderProjectCommissions();
    updateSummaryCards();
  } catch (error) {
    console.error(error);
    alert(error?.message || "Could not save review fields.");
  }
}

function renderManualTravelSummary(commission, expenses) {
  const box = document.getElementById("manualTravelReviewSummary");
  if (!box) return;

  const detail = expenses.find((expense) => Number(expense.new_travel_id || 0) > 0) || {};
  if (!Number(detail.new_travel_id || commission.new_travel_id || 0)) {
    box.classList.add("d-none");
    box.innerHTML = "";
    return;
  }

  const attendanceFile = detail.attendance_certificate_file || commission.attendance_certificate_path || "";
  const attendanceLink = attendanceFile && session?.can_view_attendance_certificates
    ? `<a href="/module/proxy/module10/Uploads?file=${encodeURIComponent(attendanceFile)}" target="_blank" rel="noopener" class="btn btn-sm btn-outline-success">Attendance certificate</a>`
    : "";

  box.classList.remove("d-none");
  box.innerHTML = `
    <div>
      <span class="summary-label">Manual travel</span>
      <strong>${escapeHtml(detail.manual_travel_code || commission.code || "-")}</strong>
    </div>
    <div>
      <span class="summary-label">Route</span>
      <strong>${escapeHtml(detail.manual_travel_from || "-")} → ${escapeHtml(detail.manual_travel_to || "-")}</strong>
    </div>
    <div>
      <span class="summary-label">Dates</span>
      <strong>${formatDate(detail.manual_travel_start || detail.travel_start_date)} - ${formatDate(detail.manual_travel_end || detail.travel_end_date)}</strong>
    </div>
    <div>
      <span class="summary-label">Purpose</span>
      <strong>${escapeHtml(detail.manual_travel_purpose || "-")}</strong>
      ${attendanceLink ? `<div class="mt-2">${attendanceLink}</div>` : ""}
    </div>
  `;
}

function setReviewModalMode(readOnly) {
  setText("projectCommissionReviewTitle", readOnly ? "View commission" : "Review commission");
  setText(
    "projectCommissionExpensesHelp",
    readOnly
      ? "Review the information and documents submitted for this service commission."
      : "Approve or reject individual user expenses."
  );
  document.getElementById("projectCommissionActionsHeader")?.classList.toggle("d-none", readOnly);
  document.getElementById("projectCommissionReviewFooter")?.classList.toggle("view-only-footer", readOnly);
  document.querySelectorAll("#projectCommissionReviewFooter .review-action-btn").forEach((button) => {
    setElementHidden(button, readOnly);
  });
}

function renderReviewExpenses(expenses, readOnly = false) {
  const tbody = document.getElementById("projectCommissionExpensesBody");
  if (!tbody) return;

  if (!expenses.length) {
    tbody.innerHTML = emptyRow(readOnly ? 5 : 6, "No expenses loaded.");
    return;
  }

  tbody.innerHTML = expenses.map((expense, index) => `
    <tr class="${reviewExpenseRowClass(expense)}">
      <td>${escapeHtml(formatExpenseType(expense.type))}</td>
      <td>${escapeHtml(expense.purpose === "center" ? "Center" : "User")}</td>
      <td>${renderReviewExpenseDescriptionCell(expense)}</td>
      <td>${formatReviewExpenseAmount(expense)}</td>
      <td>${renderFileLinks(expense, { showAttendance: index === 0 })}</td>
      ${readOnly ? "" : `<td class="text-end">${renderExpenseEditControls(expense)}</td>`}
    </tr>
  `).join("");

  if (!readOnly) {
    tbody.querySelectorAll("tr").forEach(updatePricingReviewRow);
    tbody.querySelectorAll("tr").forEach(updateMileageReviewRow);
  }
}

function reviewExpenseRowClass(expense) {
  if (expenseHasReview(expense)) return "expense-reviewed-row";
  if (expense.purpose === "center") return "expense-pending-center-row";
  return "";
}

function expenseHasReview(expense) {
  return Boolean(
    expense.pricing_review_id ||
    (expenseUsesPricingReview(expense) && (
      expense.pricing_table_id ||
      expense.pricing_group_code ||
      expense.pricing_territory
    )) ||
    expense.reviewed_description ||
    Number(expense.researcher_advance_amount || 0) > 0
  );
}

function allExpensesReviewed(expenses) {
  return Array.isArray(expenses) && expenses.length > 0 && expenses.every(expenseHasReview);
}

function updateApproveButtonState(expenses) {
  const approveBtn = document.getElementById("approveCommissionBtn");
  if (!approveBtn || currentReviewReadOnly) return;

  const ready = allExpensesReviewed(expenses);
  approveBtn.disabled = !ready;
  approveBtn.title = ready ? "" : "All expenses must be reviewed before approving.";
}

async function loadPricingTables() {
  if (pricingTables.length) return;
  const data = await getPricingTables();
  pricingTables = Array.isArray(data?.tables) ? data.tables : [];
}

function renderPricingReviewControls(expense) {
  if (String(expense.type || "") === "mileage" && expense.purpose !== "center") {
    return renderMileageReviewControls(expense);
  }

  if (!expenseUsesPricingReview(expense)) {
    return "-";
  }

  const tableType = expense.type === "per_diem" ? "per_diem" : "accommodation";
  const tables = pricingTables.filter((table) => table.table_type === tableType);
  if (!tables.length) {
    return `<span class="text-muted small">No ${escapeHtml(tableType)} tables</span>`;
  }

  const suggestedGroup = normalizeGroup(expense.pricing_group_code) || suggestGroupFromAcademicGrade(expense.academic_grade);
  const selectedTable = selectPricingTable(expense, tables);
  const selectedTerritory = expense.pricing_territory || firstTerritoryForGroup(selectedTable, suggestedGroup);
  const calculatedAmount = Number(expense.calculated_amount || 0);
  const accommodationNights = expense.type === "accommodation" ? getAccommodationNights(expense) : 0;
  const userAmount = Number(expense.amount || 0);

  return `
    <div class="pricing-review-panel text-start" data-expense-id="${escapeHtml(expense.id)}" data-expense-type="${escapeHtml(expense.type)}" data-breakdown="${escapeAttribute(expense.per_diem_breakdown || "")}" data-user-amount="${escapeAttribute(userAmount)}" data-nights="${escapeAttribute(accommodationNights)}">
      <label class="form-label small mb-1">Pricing table</label>
      <select class="form-select form-select-sm pricing-review-control pricing-table-select">
        ${tables.map((table) => `
          <option value="${escapeAttribute(table.id)}" ${Number(table.id) === Number(selectedTable?.id) ? "selected" : ""}>
            ${escapeHtml(table.title || `Table ${table.id}`)} · ${escapeHtml(table.category || "-")}
          </option>
        `).join("")}
      </select>
      <div class="row g-2 mt-1">
        <div class="col-6">
          <label class="form-label small mb-1">Group</label>
          <select class="form-select form-select-sm pricing-review-control pricing-group-select">
            ${["G1", "G2", "G3"].map((group) => `<option value="${group}" ${group === suggestedGroup ? "selected" : ""}>${group}</option>`).join("")}
          </select>
        </div>
        <div class="col-6">
          <label class="form-label small mb-1">Zone</label>
          <select class="form-select form-select-sm pricing-review-control pricing-territory-select" data-selected-territory="${escapeAttribute(selectedTerritory)}"></select>
        </div>
      </div>
      ${expense.type === "accommodation" ? `<div class="small text-muted mt-2">Accommodation nights: ${escapeHtml(formatNumber(accommodationNights))}. Submitted amount: ${escapeHtml(formatMoney(userAmount))}.</div>` : ""}
      <div class="small text-muted mt-2 pricing-rate-summary"></div>
      <input type="hidden" class="pricing-calculated-amount" value="${escapeAttribute(calculatedAmount)}">
    </div>
  `;
}

function renderExpenseEditControls(expense) {
  const expenseID = String(expense.id || "");
  const collapseID = `expenseEdit_${expenseID}`;
  const pricingControls = renderPricingReviewControls(expense);

  return `
    <button class="btn btn-sm btn-outline-secondary" type="button" data-bs-toggle="collapse" data-bs-target="#${escapeAttribute(collapseID)}" aria-expanded="false" aria-controls="${escapeAttribute(collapseID)}">
      Edit expense
    </button>
    <div class="collapse mt-2" id="${escapeAttribute(collapseID)}">
      <div class="expense-edit-panel text-start" data-expense-id="${escapeHtml(expenseID)}">
        ${renderExpenseAdminEditFields(expense)}
        ${pricingControls && pricingControls !== "-" ? `<div class="border-top mt-3 pt-3">${pricingControls}</div>` : ""}
        <button type="button" class="btn btn-sm btn-outline-secondary mt-2 w-100" data-save-expense-edit>Save expense</button>
        <button type="button" class="btn btn-sm btn-outline-danger mt-2 w-100" data-delete-expense>Delete expense</button>
      </div>
    </div>
  `;
}

function renderMileageReviewControls(expense) {
  const distance = getMileageDistance(expense);
  const rate = Number(expense.mileage_rate || 0.30);
  const calculatedAmount = Number(expense.amount || distance * rate);

  return `
    <div class="mileage-review-panel text-start" data-expense-id="${escapeHtml(expense.id)}">
      <div class="row g-2">
        <div class="col-6">
          <label class="form-label small mb-1">Distance (km)</label>
          <input type="number" min="0" step="0.01" class="form-control form-control-sm mileage-distance-input" value="${escapeAttribute(distance.toFixed(2))}">
        </div>
        <div class="col-6">
          <label class="form-label small mb-1">EUR/km</label>
          <input type="number" min="0" step="0.01" class="form-control form-control-sm mileage-rate-input" value="${escapeAttribute(rate.toFixed(2))}">
        </div>
      </div>
      <div class="small text-muted mt-2 mileage-rate-summary"></div>
      <input type="hidden" class="mileage-calculated-amount" value="${escapeAttribute(calculatedAmount.toFixed(2))}">
    </div>
  `;
}

function renderPricingReviewSummary(expense) {
  if (String(expense.type || "") === "mileage") return "";
  if (!expenseUsesPricingReview(expense)) return "";
  if (!expense.pricing_table_id && !expense.pricing_group_code && !expense.pricing_territory) return "";
  return `
    <div class="small text-muted mt-1">
      Pricing: ${escapeHtml(expense.pricing_group_code || "-")} · ${escapeHtml(expense.pricing_territory || "-")}
      ${Number(expense.calculated_amount || 0) > 0 ? ` · ${formatMoney(expense.calculated_amount)}` : ""}
    </div>
  `;
}

function renderMileageReviewSummary(expense) {
  if (expense.purpose === "center") return "";
  if (String(expense.type || "") !== "mileage") return "";

  const distance = getMileageDistance(expense);
  const rate = Number(expense.mileage_rate || 0);
  const calculated = Number(expense.calculated_amount || 0);
  if (!distance || !rate || calculated <= 0) {
    return `<div class="small text-muted mt-1">Mileage: ${formatNumber(distance)} km · Pending rate review</div>`;
  }

  return `<div class="small text-muted mt-1">Mileage: ${formatNumber(distance)} km · ${formatMoney(rate)}/km · ${formatMoney(calculated)}</div>`;
}

function renderExpenseAdvanceSummary(expense) {
  const advance = Number(expense.researcher_advance_amount || 0);
  if (advance <= 0) return "";

  return `<div class="small text-muted mt-1">Advanced payment ${formatMoney(advance)}</div>`;
}

function renderExpenseAdminEditFields(expense) {
  const advance = Number(expense.researcher_advance_amount || 0);
  const amount = Number(expense.calculated_amount || expense.amount || 0);
  return `
    <label class="form-label small mb-1">Description</label>
    <input type="text" class="form-control form-control-sm expense-description-input" value="${escapeAttribute(reviewExpenseDescription(expense))}">
    ${expenseCanEditTransportMethod(expense) ? `<div class="mt-2">
      <label class="form-label small mb-1">Means of transport</label>
      <select class="form-select form-select-sm expense-transport-method-input">
        ${renderTransportMethodOptions(expense.transport_method || "")}
      </select>
    </div>` : ""}
    ${expenseUsesDirectAmount(expense) ? `<div class="mt-2">
      <label class="form-label small mb-1">Amount (EUR)</label>
      <input type="number" min="0" step="0.01" class="form-control form-control-sm expense-amount-input" value="${escapeAttribute(amount.toFixed(2))}">
    </div>` : ""}
    ${expense.purpose === "center" ? "" : `<div class="mt-2">
      <label class="form-label small mb-1">Advanced payment</label>
      <input type="number" min="0" step="0.01" class="form-control form-control-sm expense-advance-input" value="${escapeAttribute(advance.toFixed(2))}">
    </div>`}
  `;
}

function expenseCanEditTransportMethod(expense) {
  return expense.purpose === "center" && String(expense.type || "") === "budget_travel";
}

function renderTransportMethodOptions(selectedValue) {
  const options = [
    ["", "Select..."],
    ["plane", "Plane"],
    ["train", "Train"],
    ["bus", "Bus"],
    ["taxi", "Taxi"],
    ["metro", "Metro"],
    ["public_transport", "Public transport"],
    ["boat", "Boat"]
  ];
  const selected = String(selectedValue || "");
  return options.map(([value, label]) => `
    <option value="${escapeAttribute(value)}" ${value === selected ? "selected" : ""}>${escapeHtml(label)}</option>
  `).join("");
}

function expenseUsesDirectAmount(expense) {
  if (isBudgetCenterExpenseType(expense.type)) return false;
  if (expense.purpose === "center") return true;
  return !["per_diem", "accommodation", "mileage"].includes(String(expense.type || ""));
}

function expenseUsesPricingReview(expense) {
  if (expense.purpose === "center") return false;
  return ["per_diem", "accommodation"].includes(String(expense.type || ""));
}

function isBudgetCenterExpenseType(type) {
  return ["budget_travel", "budget_accommodation", "budget_registration"].includes(String(type || ""));
}

function renderReviewExpenseDescriptionCell(expense) {
  if (expense.purpose === "center") {
    return escapeHtml(reviewExpenseDescription(expense) || "-");
  }

  return `${escapeHtml(reviewExpenseDescription(expense) || "-")}${renderOriginalDescriptionSummary(expense)}${renderReviewDestination(expense)}${formatPerDiemRange(expense)}${renderPricingReviewSummary(expense)}${renderMileageReviewSummary(expense)}${renderExpenseAdvanceSummary(expense)}`;
}

function reviewExpenseDescription(expense) {
  return expense.reviewed_description || expense.description || "";
}

function renderOriginalDescriptionSummary(expense) {
  if (!expense.reviewed_description || expense.reviewed_description === expense.description) return "";
  return `<div class="small text-muted mt-1">Original: ${escapeHtml(expense.description || "-")}</div>`;
}

function renderReviewDestination(expense) {
  if (!["accommodation", "per_diem"].includes(String(expense.type || "")) || !expense.destination) return "";

  return `<div class="small text-muted mt-1">Destination: ${escapeHtml(expense.destination)}</div>`;
}

function formatReviewExpenseAmount(expense) {
  if (isBudgetCenterExpenseType(expense.type)) {
    return `<span class="text-muted">-</span>`;
  }

  if (expense.purpose === "center") {
    const amount = Number(expense.calculated_amount || expense.amount || 0);
    return `${formatMoney(amount)}<div class="small text-muted">Reviewed</div>`;
  }

  const calculated = Number(expense.calculated_amount || 0);
  if (["per_diem", "accommodation"].includes(String(expense.type || "")) && calculated > 0) {
    return `${formatMoney(calculated)}<div class="small text-muted">Reviewed</div>`;
  }

  if (String(expense.type || "") === "mileage") {
    const amount = Number(expense.calculated_amount || 0);
    if (amount > 0) {
      return `${formatMoney(amount)}<div class="small text-muted">${formatNumber(getMileageDistance(expense))} km reviewed</div>`;
    }

    return `${formatNumber(getMileageDistance(expense))} km<div class="small text-muted">Pending rate review</div>`;
  }

  if (expenseUsesDirectAmount(expense)) {
    const calculated = Number(expense.calculated_amount || 0);
    if (calculated > 0) {
      return `${formatMoney(calculated)}<div class="small text-muted">Reviewed</div>`;
    }
  }

  return formatMoney(expense.amount);
}

function updatePricingReviewRow(row) {
  const panel = row.querySelector(".pricing-review-panel");
  if (!panel) return;

  const table = pricingTables.find((item) => Number(item.id) === Number(panel.querySelector(".pricing-table-select")?.value));
  const group = panel.querySelector(".pricing-group-select")?.value || "G1";
  const territorySelect = panel.querySelector(".pricing-territory-select");
  const preferredTerritory = territorySelect?.value || territorySelect?.dataset.selectedTerritory || "";
  const territories = uniqueTerritories(table);

  if (territorySelect) {
    territorySelect.innerHTML = territories.map((territory) => `
      <option value="${escapeAttribute(territory)}" ${territory === preferredTerritory ? "selected" : ""}>${escapeHtml(territory)}</option>
    `).join("");
    if (preferredTerritory && !territories.includes(preferredTerritory)) {
      territorySelect.insertAdjacentHTML("afterbegin", `<option value="${escapeAttribute(preferredTerritory)}" selected>${escapeHtml(preferredTerritory)}</option>`);
    }
  }

  const territory = territorySelect?.value || "";
  const amount = calculateReviewedAmount(panel.dataset.expenseType, table, group, territory, panel.dataset.breakdown, panel);
  const amountInput = panel.querySelector(".pricing-calculated-amount");
  const summary = panel.querySelector(".pricing-rate-summary");
  if (amountInput) amountInput.value = amount.toFixed(2);
  if (summary) summary.textContent = describePricingRate(panel.dataset.expenseType, table, group, territory, panel.dataset.breakdown, amount, panel);
}

async function savePricingReviewFromRow(row) {
  try {
    const result = await savePricingReviewOnly(row);
    if (!result) return;
    applyCommissionStatus(selectedCommissionId, result?.status || "under_review");
    renderProjectCommissions();
    updateSummaryCards();
    await refreshAllCommissions();
    await openReviewModal(selectedCommissionId);
  } catch (error) {
    console.error(error);
    alert("Could not save pricing review.");
  }
}

async function savePricingReviewOnly(row) {
  const panel = row?.querySelector(".pricing-review-panel");
  if (!panel) return null;

  updatePricingReviewRow(row);

  return saveExpensePricingReview({
    expense_id: Number(panel.dataset.expenseId || 0),
    pricing_table_id: Number(panel.querySelector(".pricing-table-select")?.value || 0),
    group_code: panel.querySelector(".pricing-group-select")?.value || "G1",
    territory: panel.querySelector(".pricing-territory-select")?.value || "",
    calculated_amount: Number(panel.querySelector(".pricing-calculated-amount")?.value || 0)
  });
}

function updateMileageReviewRow(row) {
  const panel = row.querySelector(".mileage-review-panel");
  if (!panel) return;

  const distance = Number(panel.querySelector(".mileage-distance-input")?.value || 0);
  const rate = Number(panel.querySelector(".mileage-rate-input")?.value || 0);
  const amount = distance * rate;
  const amountInput = panel.querySelector(".mileage-calculated-amount");
  const summary = panel.querySelector(".mileage-rate-summary");

  if (amountInput) amountInput.value = amount.toFixed(2);
  if (summary) summary.textContent = `${formatNumber(distance)} km x ${formatMoney(rate)}/km = ${formatMoney(amount)}.`;
}

async function saveMileageReviewFromRow(row) {
  try {
    const result = await saveMileageReviewOnly(row);
    if (!result) return;
    applyCommissionStatus(selectedCommissionId, result?.status || "under_review");
    renderProjectCommissions();
    updateSummaryCards();
    await refreshAllCommissions();
    await openReviewModal(selectedCommissionId);
  } catch (error) {
    console.error(error);
    alert("Could not save mileage review.");
  }
}

async function saveMileageReviewOnly(row) {
  const panel = row?.querySelector(".mileage-review-panel");
  if (!panel) return null;

  updateMileageReviewRow(row);

  return saveMileageReview({
    expense_id: Number(panel.dataset.expenseId || 0),
    distance_km: Number(panel.querySelector(".mileage-distance-input")?.value || 0),
    mileage_rate: Number(panel.querySelector(".mileage-rate-input")?.value || 0),
    calculated_amount: Number(panel.querySelector(".mileage-calculated-amount")?.value || 0)
  });
}

async function saveExpenseEditFromRow(row) {
  const panel = row?.querySelector(".expense-edit-panel");
  if (!panel) return;

  const payload = {
    expense_id: Number(panel.dataset.expenseId || 0),
    description: panel.querySelector(".expense-description-input")?.value || "",
    transport_method: panel.querySelector(".expense-transport-method-input")?.value || "",
    amount: Number(panel.querySelector(".expense-amount-input")?.value || 0),
    researcher_advance_amount: Number(panel.querySelector(".expense-advance-input")?.value || 0)
  };

  try {
    await savePricingReviewOnly(row);
    await saveMileageReviewOnly(row);
    const result = await saveExpenseEdit(payload);
    applyCommissionStatus(selectedCommissionId, result?.status || "under_review");
    renderProjectCommissions();
    updateSummaryCards();
    await refreshAllCommissions();
    await openReviewModal(selectedCommissionId);
  } catch (error) {
    console.error(error);
    alert("Could not save expense edit.");
  }
}

async function deleteCenterExpenseFromRow(row) {
  const panel = row?.querySelector(".expense-edit-panel");
  const expenseId = Number(panel?.dataset.expenseId || 0);
  if (!expenseId) return;
  if (!confirm("Delete this center expense?")) return;

  try {
    const result = await deleteCenterExpense(expenseId);
    applyCommissionStatus(selectedCommissionId, result?.status || "under_review");
    renderProjectCommissions();
    updateSummaryCards();
    await refreshAllCommissions();
    await openReviewModal(selectedCommissionId);
  } catch (error) {
    console.error(error);
    alert("Could not delete center expense.");
  }
}

async function excludeExpenseFromRow(row) {
  const panel = row?.querySelector(".expense-edit-panel");
  const expenseId = Number(panel?.dataset.expenseId || 0);
  if (!expenseId) return;
  const visibleExpenseRows = document.querySelectorAll("#projectCommissionExpensesBody tr .expense-edit-panel").length;
  if (visibleExpenseRows <= 1) {
    alert("This is the only expense in the commission. Reject the commission instead.");
    return;
  }
  if (!confirm("Delete this expense from the commission?")) return;

  try {
    const result = await excludeExpense(expenseId);
    applyCommissionStatus(selectedCommissionId, result?.status || "under_review");
    renderProjectCommissions();
    updateSummaryCards();
    await refreshAllCommissions();
    await openReviewModal(selectedCommissionId);
  } catch (error) {
    console.error(error);
    alert("Could not delete expense.");
  }
}

async function changeCommissionStatus(id, action) {
  try {
    if (action === "approve") {
      const rows = normalizeRows(await getCommissionDetail(id));
      if (!allExpensesReviewed(rows)) {
        alert("All expenses must be reviewed before approving the commission.");
        if (Number(selectedCommissionId) === Number(id)) {
          updateApproveButtonState(rows);
          renderReviewExpenses(rows, currentReviewReadOnly);
        }
        return;
      }
    }

    const confirmed = confirm(action === "approve" ? "Approve this commission?" : "Reject this commission?");
    if (!confirmed) return;

    if (action === "approve") {
      const peopleCategory = Number(selectedCommissionId) === Number(id) ? getSelectedReviewPeopleCategory() : null;
      const relatedTask = Number(selectedCommissionId) === Number(id) ? getSelectedReviewRelatedTask() : null;
      await approveCommission(id, peopleCategory, relatedTask);
    } else await rejectCommission(id);

    await refreshAllCommissions();
    bootstrap.Modal.getInstance(document.getElementById("projectCommissionReviewModal"))?.hide();
  } catch (error) {
    console.error(error);
    alert(error?.message || "Could not update commission status.");
  }
}

function updateSummaryCards() {
  const pending = projectCommissions.filter((row) => ["submitted", "under_review", "final_review", ""].includes(String(row.status || ""))).length;
  const ready = projectCommissions.filter((row) => row.status === "approved").length;
  setText("pendingReviewCount", pending);
  setText("readyToExportCount", ready);
  updateProjectKpis(projectCommissions);
}

function applyCommissionStatus(id, status) {
  if (!id || !status) return;

  projectCommissions = projectCommissions.map((row) => (
    Number(row.id) === Number(id) ? { ...row, status } : row
  ));

  myCommissions = myCommissions.map((row) => (
    Number(row.id) === Number(id) ? { ...row, status } : row
  ));

  if (Number(selectedCommissionId) === Number(id)) {
    if (isFinalCommissionStatus(status)) {
      setElementHidden(document.getElementById("approveCommissionBtn"), true);
      setElementHidden(document.getElementById("rejectCommissionBtn"), true);
    }
  }
}

function isFinalCommissionStatus(status) {
  return ["final_review", "approved", "rejected", "exported"].includes(String(status || "").toLowerCase());
}

function updateProjectKpis(rows) {
  setText("projectPendingCount", rows.filter((row) => ["submitted", "under_review", "", "final_review"].includes(String(row.status || ""))).length);
  setText("projectApprovedCount", rows.filter((row) => row.status === "approved").length);
  setText("projectRejectedCount", rows.filter((row) => row.status === "rejected").length);
  setText("projectExportedCount", rows.filter((row) => row.status === "exported").length);
}

function filterRows(rows, searchInputId, statusFilterId) {
  const search = document.getElementById(searchInputId)?.value?.trim().toLowerCase() || "";
  const status = document.getElementById(statusFilterId)?.value || "";

  return rows.filter((row) => {
    const matchesStatus = !status || row.status === status;
    const haystack = Object.values(row).join(" ").toLowerCase();
    return matchesStatus && (!search || haystack.includes(search));
  });
}

function getSelectedProjectCommissionIds() {
  return [...document.querySelectorAll(".project-commission-checkbox:checked")].map((checkbox) => checkbox.value);
}

function renderFileLinks(expense, options = {}) {
  const files = [
    ...buildTypedFileLinks(expense.receipts, "Receipt"),
    ...buildTypedFileLinks(expense.payment_proofs, "Payment proof"),
    ...buildTypedFileLinks(expense.mileage_proofs, "Mileage proof"),
    ...buildTypedFileLinks(expense.travel_files, "Travel document")
  ];

  if (!files.length) return "-";

  return files.map((file) => {
    const encoded = encodeURIComponent(file.path);
    return `<a href="/module/proxy/module10/Uploads?file=${encoded}" target="_blank" rel="noopener">${escapeHtml(file.label)}</a>`;
  }).join(" · ");
}

function buildTypedFileLinks(value, label) {
  const files = String(value || "")
    .split(",")
    .map((file) => file.trim())
    .filter(Boolean);

  return files.map((path, index) => ({
    path,
    label: files.length > 1 ? `${label} ${index + 1}` : label
  }));
}

function getMileageDistance(expense) {
  const distance = Number(expense.distance_km || 0);
  if (distance > 0) return distance;

  return Number(expense.amount || 0);
}

function getAccommodationNights(expense) {
  const start = parseDateOnly(expense.travel_start_date);
  const end = parseDateOnly(expense.travel_end_date);
  if (!start || !end || end <= start) return 0;

  return Math.max(0, Math.round((end - start) / 86400000));
}

function selectPricingTable(expense, tables) {
  const saved = tables.find((table) => Number(table.id) === Number(expense.pricing_table_id));
  if (saved) return saved;

  const projectType = String(expense.project_type || "").trim().toLowerCase();
  const matchingCategory = tables.find((table) => String(table.category || "").trim().toLowerCase() === projectType);
  return matchingCategory || tables[0];
}

function suggestGroupFromAcademicGrade(value) {
  const grade = String(value ?? "").trim().toUpperCase();
  if (["1", "2", "6", "7"].includes(grade)) return "G2";
  if (["3", "4", "5", "G"].includes(grade)) return "G3";
  return "G1";
}

function formatPeopleCategory(value) {
  if (value === null || value === undefined || value === "") return "";
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) {
    return normalizeGroup(value);
  }
  return normalizeGroup(`G${numeric + 1}`);
}

function peopleCategoryValueFromGroup(value) {
  const group = normalizeGroup(value);
  if (!group) return "";
  return Number(group.slice(1)) - 1;
}

function normalizeGroup(value) {
  const group = String(value || "").trim().toUpperCase();
  return ["G1", "G2", "G3"].includes(group) ? group : "";
}

function firstTerritoryForGroup(table, group) {
  const row = tableRows(table).find((item) => item.group_code === group);
  return row?.territory || tableRows(table)[0]?.territory || "";
}

function uniqueTerritories(table) {
  return [...new Set(tableRows(table).map((row) => row.territory).filter(Boolean))];
}

function tableRows(table) {
  return Array.isArray(table?.rows) ? table.rows : [];
}

function matchingRows(table, group, territory) {
  return tableRows(table).filter((row) =>
    String(row.group_code || "") === group &&
    String(row.territory || "") === territory
  );
}

function calculateReviewedAmount(type, table, group, territory, breakdownJson, panel = null) {
  const rows = matchingRows(table, group, territory);
  if (!rows.length) return 0;

  if (type === "accommodation") {
    return calculateAccommodationPricing(rows, panel).amount;
  }

  return calculatePerDiemPricing(rows, breakdownJson).amount;
}

function describePricingRate(type, table, group, territory, breakdownJson, amount, panel = null) {
  const rows = matchingRows(table, group, territory);
  if (!table || !territory) return "Select table, group and zone.";
  if (!rows.length) return "No matching rate for this group and zone.";

  if (type === "accommodation") {
    return calculateAccommodationPricing(rows, panel).summary;
  }

  return calculatePerDiemPricing(rows, breakdownJson).summary;
}

function calculateAccommodationPricing(rows, panel) {
  const nightlyRate = Number(rows[0]?.amount || 0);
  const nights = Math.max(0, Number(panel?.dataset.nights || 0));
  const submittedAmount = Number(panel?.dataset.userAmount || 0);
  const tableMaximum = nightlyRate * nights;
  const amount = submittedAmount > 0 ? Math.min(submittedAmount, tableMaximum) : tableMaximum;

  return {
    amount,
    summary: `Calculated: ${formatMoney(amount)}. ${formatNumber(nights)} nights x ${formatMoney(nightlyRate)} = ${formatMoney(tableMaximum)} maximum. Submitted: ${formatMoney(submittedAmount)}.`
  };
}

function calculatePerDiemPricing(rows, breakdownJson) {
  const counts = parsePerDiemCounts(breakdownJson);
  const mealRows = rows.map((row) => ({
    ...row,
    meal_type: String(row.meal_type || "").toUpperCase(),
    amount: Number(row.amount || 0)
  }));
  const hasFullBoard = mealRows.some((row) => row.meal_type === "FULL_BOARD");
  const hasPositiveSplitMeals = mealRows.some((row) => ["A", "B", "C"].includes(row.meal_type) && row.amount > 0);

  if (hasFullBoard && !hasPositiveSplitMeals) {
    const fullBoardAmount = mealRows.find((row) => row.meal_type === "FULL_BOARD")?.amount || 0;
    const meals = Number(counts.B || 0);
    const dinners = Number(counts.C || 0);
    const fullBoards = Math.min(meals, dinners);
    const halfBoards = Math.abs(meals - dinners);
    const amount = (fullBoards * fullBoardAmount) + (halfBoards * fullBoardAmount / 2);

    return {
      amount,
      summary: `Calculated: ${formatMoney(amount)} (${fullBoards} full board, ${halfBoards} half board at ${formatMoney(fullBoardAmount / 2)}). Full board rate: ${formatMoney(fullBoardAmount)}. Breakfasts A ignored.`
    };
  }

  const amount = mealRows.reduce((total, row) => {
    return total + row.amount * Number(counts[row.meal_type] || 0);
  }, 0);
  const rates = mealRows
    .filter((row) => ["A", "B", "C"].includes(row.meal_type))
    .map((row) => `${row.meal_type}: ${formatMoney(row.amount)}`)
    .join(" · ");

  return {
    amount,
    summary: `Calculated: ${formatMoney(amount)} (${counts.A || 0}A, ${counts.B || 0}B, ${counts.C || 0}C). ${rates}`
  };
}

function parsePerDiemCounts(value) {
  try {
    const parsed = JSON.parse(decodeEscapedBreakdown(value) || "{}");
    return parsed.final || {};
  } catch {
    return {};
  }
}

function decodeEscapedBreakdown(value) {
  return String(value || "")
    .replaceAll("¤", "'")
    .replaceAll("¶", "\"")
    .replaceAll("§", ",");
}

function formatMyCommissionAmount(row) {
  if (String(row?.status || "").toLowerCase() === "submitted") {
    return "Pending review";
  }

  return formatMoney(row?.total);
}

function formatPerDiemRange(expense) {
  if (!expense.departure_date && !expense.return_date) return "";
  return `<div class="small text-muted">${escapeHtml(formatDateOnly(expense.departure_date))} ${escapeHtml(expense.departure_time || "")} → ${escapeHtml(formatDateOnly(expense.return_date))} ${escapeHtml(expense.return_time || "")}</div>`;
}

function formatExpenseType(type) {
  const map = {
    budget_travel: "Travel ticket",
    budget_accommodation: "Accommodation",
    budget_registration: "Registration",
    mileage: "Mileage",
    transport: "Transport",
    accommodation: "Accommodation",
    per_diem: "Per diem / Diets",
    tax: "Tourist tax",
    registration: "Registration",
    poster: "Poster printing",
    management_fee: "Management fee",
    internal_transport: "Internal transport",
    bank_fee: "Bank fee",
    other: "Other"
  };

  const key = String(type || "").trim();
  return map[key] || key.replace(/_/g, " ").replace(/\b\w/g, (letter) => letter.toUpperCase()) || "-";
}

function normalizeRows(value) {
  if (Array.isArray(value)) return value;
  if (Array.isArray(value?.data)) return value.data;
  if (Array.isArray(value?.rows)) return value.rows;
  return [];
}

function emptyRow(colspan, message) {
  return `<tr><td colspan="${colspan}" class="text-center text-muted py-4">${escapeHtml(message)}</td></tr>`;
}

function renderTableError(tbodyId, colspan, message) {
  const tbody = document.getElementById(tbodyId);
  if (tbody) tbody.innerHTML = emptyRow(colspan, message);
}

function statusBadge(status) {
  return `<span class="status-badge status-badge--${escapeHtml(status || "submitted")}">${escapeHtml(readableStatus(status))}</span>`;
}

function readableStatus(status) {
  const map = {
    submitted: "Submitted",
    under_review: "Under review",
    final_review: "Final approvals",
    pending: "Pending",
    approved: "Approved",
    rejected: "Rejected",
    exported: "Exported",
    center: "Center cost"
  };
  return map[status] || "Submitted";
}

function formatMoney(value) {
  const number = Number(value || 0);
  return new Intl.NumberFormat("en-GB", { style: "currency", currency: "EUR" }).format(number);
}

function formatNumber(value) {
  const number = Number(value || 0);
  return new Intl.NumberFormat("en-GB", { maximumFractionDigits: 2 }).format(number);
}

function formatDate(value) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString("en-GB");
}

function formatDateOnly(value) {
  if (!value) return "";
  const text = String(value);
  const isoDate = text.match(/^(\d{4}-\d{2}-\d{2})/);
  if (isoDate) {
    const [year, month, day] = isoDate[1].split("-");
    return `${day}/${month}/${year}`;
  }

  return formatDate(value);
}

function parseDateOnly(value) {
  if (!value) return null;
  const match = String(value).match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (!match) return null;
  return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
}

function setText(id, value) {
  const element = document.getElementById(id);
  if (element) element.textContent = value;
}

async function withButtonLock(button, action, busyText = "Working...") {
  if (!button) return action();
  if (button.dataset.busy === "1") return null;

  const wasDisabled = button.disabled;
  const previousHTML = button.innerHTML;
  const previousAriaBusy = button.getAttribute("aria-busy");

  button.dataset.busy = "1";
  button.disabled = true;
  button.setAttribute("aria-busy", "true");
  button.innerHTML = `<span class="spinner-border spinner-border-sm me-1" aria-hidden="true"></span>${escapeHtml(busyText)}`;

  try {
    return await action();
  } finally {
    delete button.dataset.busy;
    button.disabled = wasDisabled;
    if (previousAriaBusy === null) {
      button.removeAttribute("aria-busy");
    } else {
      button.setAttribute("aria-busy", previousAriaBusy);
    }
    button.innerHTML = previousHTML;
  }
}

function setElementHidden(element, hidden) {
  if (!element) return;
  element.classList.toggle("d-none", hidden);
  element.hidden = hidden;
  element.style.display = hidden ? "none" : "";
}

function escapeHtml(value) {
  return String(value ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

function escapeAttribute(value) {
  return escapeHtml(value).replace(/`/g, "&#096;");
}
