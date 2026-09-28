//////////////////////////////////////////////////////////////////////////////////////////////////
//                   Projects hub: formats projects edition and visualization                   //
//////////////////////////////////////////////////////////////////////////////////////////////////

import { fetchOptions } from './formOptions.js';

const managementUnits = [
  { code: "TRANSFERÈNCIA", name: "TRANSFERÈNCIA" },
  { code: "RECERCA", name: "RECERCA" },
  { code: "FORMACIÓ", name: "FORMACIÓ" }
];

document.addEventListener('DOMContentLoaded', async () => {
  const container = document.getElementById("group-list");
  if (!container) return;

  const searchInput = document.getElementById("projectSearchInput");
  const statusFilter = document.getElementById("projectStatusFilter");
  const clearFiltersBtn = document.getElementById("clearProjectFiltersBtn");
  const resultSummary = document.getElementById("projectResultSummary");
  let projects = [];

  await fetchOptions();
  let projectTypes = await loadProjectTypes();

  const escapeHtml = (value) => String(value ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");

  const normalizeDate = (value) => {
    const raw = String(value ?? "").trim();
    if (!raw) return null;

    const date = new Date(raw.slice(0, 10));
    return Number.isNaN(date.getTime()) ? null : date;
  };

  const todayOnly = () => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    return today;
  };

  const formatDate = (value) => {
    const raw = String(value ?? "").slice(0, 10);
    if (!raw) return "No date";

    const [year, month, day] = raw.split("-");
    return year && month && day ? `${day}/${month}/${year}` : raw;
  };

  const getProjectStatus = (project) => {
    const start = normalizeDate(project.start_date);
    const end = normalizeDate(project.end_date);
    const today = todayOnly();

    if (!start && !end) {
      return {
        key: "no-dates",
        label: "No dates",
        badgeClass: "bg-secondary",
        cardClass: "project-card-no-dates"
      };
    }

    if (end && end < today) {
      return {
        key: "ended",
        label: "Ended",
        badgeClass: "bg-secondary",
        cardClass: "project-card-ended"
      };
    }

    if (start && start > today) {
      return {
        key: "upcoming",
        label: "Upcoming",
        badgeClass: "bg-info text-dark",
        cardClass: "project-card-upcoming"
      };
    }

    return {
      key: "active",
      label: "Active",
      badgeClass: "bg-success",
      cardClass: "project-card-active"
    };
  };

  const getSearchText = (project) => [
    project.id,
    project.name,
    project.short_name,
    project.number,
    project.type,
    project.management_unit,
    project.start_date,
    project.end_date
  ].map(value => String(value ?? "").toLowerCase()).join(" ");

  const renderProjects = () => {
    const query = searchInput?.value.trim().toLowerCase() ?? "";
    const selectedStatus = statusFilter?.value ?? "all";

    const filteredProjects = projects.filter(project => {
      const status = getProjectStatus(project);
      const matchesStatus = selectedStatus === "all" || status.key === selectedStatus;
      const matchesSearch = !query || getSearchText(project).includes(query);
      return matchesStatus && matchesSearch;
    });

    if (resultSummary) {
      resultSummary.textContent = `${filteredProjects.length} of ${projects.length} projects shown`;
    }

    if (!filteredProjects.length) {
      container.innerHTML = `
        <div class="col-12">
          <div class="alert alert-warning mb-0">No projects match the current search.</div>
        </div>`;
      return;
    }

    const html = filteredProjects.map(project => {
      const { id, name, short_name, number, type, management_unit } = project;
      const status = getProjectStatus(project);
      const title = short_name || name || '';
      const projectNumber = number ? ` · ${number}` : "";
      const fullName = name && short_name && name !== short_name ? name : "";

      return `
        <div class="col">
          <div class="card h-100 shadow-sm project-card ${status.cardClass}">
            <div class="card-body">
              <div class="d-flex justify-content-between align-items-start gap-2">
                <div class="min-w-0">
                  <h5 class="mb-1 project-title">
                    ${escapeHtml(title)}${escapeHtml(projectNumber)} (${escapeHtml(id || "")})
                  </h5>
                  ${fullName ? `<div class="text-muted small project-full-name">${escapeHtml(fullName)}</div>` : ""}
                </div>
                <span class="badge ${status.badgeClass}">${status.label}</span>
              </div>
              <div class="project-meta mt-3">
                <span>${escapeHtml(type || "No type")}</span>
                <span>${escapeHtml(management_unit || "No unit")}</span>
                <span>${formatDate(project.start_date)} - ${formatDate(project.end_date)}</span>
              </div>
              <div class="mt-3 d-flex gap-2 flex-wrap">
                <button class="btn btn-sm btn-danger"
                        data-role="edit-data"
                        data-project-id="${escapeHtml(id)}">
                  Modify project data
                </button>
                <button class="btn btn-sm btn-outline-danger"
                        data-role="edit-members"
                        data-project-id="${escapeHtml(id)}">
                  Modify project members
                </button>
              </div>
            </div>
          </div>
        </div>`;
    }).join('');

    container.innerHTML = html;
  };

  const loadProjects = async () => {
    const res = await fetch("/module/proxy/module4/api?action=allProjects");
    const raw = await res.json();

    const byId = new Map();
    for (const p of raw) {
      if (!byId.has(String(p.id))) byId.set(String(p.id), p);
    }

    projects = [...byId.values()].sort((a, b) => {
      const aStatus = getProjectStatus(a);
      const bStatus = getProjectStatus(b);
      const statusOrder = { active: 0, upcoming: 1, "no-dates": 2, ended: 3 };
      const statusDiff = statusOrder[aStatus.key] - statusOrder[bStatus.key];
      if (statusDiff !== 0) return statusDiff;
      return String(a.short_name || a.name || a.id || "").localeCompare(String(b.short_name || b.name || b.id || ""));
    });
    renderProjects();
    return byId;
  };

  let byId = await loadProjects();

  searchInput?.addEventListener("input", renderProjects);
  statusFilter?.addEventListener("change", renderProjects);
  clearFiltersBtn?.addEventListener("click", () => {
    if (searchInput) searchInput.value = "";
    if (statusFilter) statusFilter.value = "all";
    renderProjects();
  });

  async function loadProjectTypes() {
    try {
      const res = await fetch("/module/proxy/module4/api?action=projectTypes");
      if (!res.ok) return [];
      const values = await res.json();
      return Array.isArray(values) ? values.filter(Boolean).sort((a, b) => String(a).localeCompare(String(b))) : [];
    } catch (error) {
      console.error(error);
      return [];
    }
  }

  const renderProjectForm = (project = null, mode = "add") => {
    const modalEl = document.getElementById('groupFormModal');
    const formContainer = document.getElementById('formFieldsContainer');
    if (!modalEl || !formContainer) return;

    modalEl.dataset.mode = mode;
    modalEl.dataset.projectId = project?.id ?? "";

  const managementUnitOptions = managementUnits.map(opt => {
    const selected = String(opt.code) === String(project?.management_unit ?? "") ? "selected" : "";
    return `<option value="${opt.code}" ${selected}>${opt.name}</option>`;
  }).join('');

  const fonsRomanentsValue = Number(project?.fons_romanents ?? 0);
  const projectTypeOptions = projectTypes.map((type) => `<option value="${type}"></option>`).join('');

    formContainer.innerHTML = `
      <div class="col-md-6">
        <label for="projectId" class="form-label">ID</label>
        <input type="text" class="form-control" id="projectId"
               value="${project?.id ?? ''}" ${mode === "edit" ? "disabled" : ""}>
      </div>

      <div class="col-md-6">
        <label for="projectName" class="form-label">Project Name</label>
        <input type="text" class="form-control" id="projectName" value="${project?.name ?? ''}">
      </div>

      <div class="col-md-6">
        <label for="projectShortName" class="form-label">Short Name</label>
        <input type="text" class="form-control" id="projectShortName" value="${project?.short_name ?? ''}">
      </div>

      <div class="col-md-6">
        <label for="projectNumber" class="form-label">Project number</label>
        <input type="text" class="form-control" id="projectNumber" value="${project?.number ?? ''}">
      </div>

      <div class="col-md-6">
        <label for="projectType" class="form-label">Project type</label>
        <input type="text" class="form-control" id="projectType" list="projectTypeOptions" value="${project?.type ?? ''}" placeholder="FPI, FI, AGAUR...">
        <datalist id="projectTypeOptions">${projectTypeOptions}</datalist>
      </div>

      <div class="col-md-6">
        <label for="managementUnit" class="form-label">Management Unit</label>
        <select class="form-select" id="managementUnit">
          <option value="">-- Select --</option>
          ${managementUnitOptions}
        </select>
      </div>

      <div class="col-md-6">
        <label for="startDate" class="form-label">Start date</label>
        <input type="date" class="form-control" id="startDate" value="${(project?.start_date ?? '').slice(0, 10)}">
      </div>

      <div class="col-md-6">
        <label for="endDate" class="form-label">End date</label>
        <input type="date" class="form-control" id="endDate" value="${(project?.end_date ?? '').slice(0, 10)}">
      </div>

      <div class="col-md-6">
        <label for="fons_romanents" class="form-label">
          Does this project have remaining funds?
        </label>
        <select id="fons_romanents" name="fons_romanents" class="form-select">
          <option value="0" ${fonsRomanentsValue === 0 ? "selected" : ""}>No</option>
          <option value="1" ${fonsRomanentsValue === 1 ? "selected" : ""}>Yes</option>
        </select>
      </div>
    `;
  };

  const addBtn = document.getElementById('openAddGroupModalBtn');
  if (addBtn) {
    addBtn.addEventListener('click', () => {
      const modalEl = document.getElementById('groupFormModal');
      if (!modalEl || !window.bootstrap?.Modal) return;

      renderProjectForm(null, "add");
      const modal = new bootstrap.Modal(modalEl);
      modal.show();
    });
  }

  container.addEventListener('click', (e) => {
    const btn = e.target.closest('button[data-role]');
    if (!btn) return;

    const role = btn.dataset.role;

    if (role === 'edit-data') {
      const projectId = btn.dataset.projectId;
      const project = byId.get(String(projectId));
      if (!project) return;

      const modalEl = document.getElementById('groupFormModal');
      if (!modalEl || !window.bootstrap?.Modal) return;

      renderProjectForm(project, "edit");
      const modal = new bootstrap.Modal(modalEl);
      modal.show();
    }

    if (role === 'edit-members') {
      const projectId = btn.dataset.projectId;
      if (!projectId) return;

      window.location.href = `projectMembers.html?id=${encodeURIComponent(projectId)}`;
    }
  });

  const form = document.getElementById("groupForm");
  if (form) {
    form.addEventListener("submit", async (e) => {
      e.preventDefault();

      const modalEl = document.getElementById('groupFormModal');
      if (!modalEl) return;

      const mode = modalEl.dataset.mode;
      const existingProjectId = modalEl.dataset.projectId;

      const payload = {
        id: mode === "edit"
          ? existingProjectId
          : document.getElementById("projectId").value.trim(),
        name: document.getElementById("projectName").value.trim(),
        short_name: document.getElementById("projectShortName").value.trim(),
        number: document.getElementById("projectNumber").value.trim(),
        type: document.getElementById("projectType").value.trim(),
        management_unit: document.getElementById("managementUnit").value,
        start_date: document.getElementById("startDate").value,
        end_date: document.getElementById("endDate").value,
        fons_romanents: document.getElementById("fons_romanents").value,
      };

      if (!payload.id || !payload.name) {
        alert("Project ID and Project Name are required.");
        return;
      }

      const res = await fetch("/module/proxy/module4/api?action=saveProject", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });

      if (!res.ok) {
        const msg = await res.text();
        console.error(msg);
        alert("Error saving project");
        return;
      }

      bootstrap.Modal.getInstance(modalEl)?.hide();
      projectTypes = await loadProjectTypes();
      byId = await loadProjects();
    });
  }
});
