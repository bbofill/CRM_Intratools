//////////////////////////////////////////////////////////////////////////////////////////////////
//                   Groups hub: formats groups edition and visualization                       //
//////////////////////////////////////////////////////////////////////////////////////////////////

import { formOptions, fetchOptions } from './formOptions.js';


let currentGroupFilter = null;

document.addEventListener('DOMContentLoaded', async () => {
  const container = document.getElementById("group-list");
  if (!container) return;

  fetchOptions().then(() => {

  

  document.getElementById("AllGroupsBtn").addEventListener("click", (e) => {
    e.preventDefault();
    currentGroupFilter = null;
    loadGroups();
  });

  document.getElementById("ProjectGroupsBtn").addEventListener("click", (e) => {
    e.preventDefault();
    currentGroupFilter = "project";
    loadGroups();
  });

  document.getElementById("SgrGroupsBtn").addEventListener("click", (e) => {
    e.preventDefault();
    currentGroupFilter = "sgr";
    loadGroups();
  });

  document.getElementById("NormalGroupsBtn").addEventListener("click", (e) => {
    e.preventDefault();
    currentGroupFilter = "group";
    loadGroups();
  });
  document.getElementById("UnitGroupsBtn").addEventListener("click", (e) => {
    e.preventDefault();
    currentGroupFilter = "unit";
    loadGroups();
  });
});

  const loadGroups = async () => {
    const res = await fetch("/module/proxy/module4/api?action=allGroups");
    const raw = await res.json();

    // De-duplicar por research_code por si el LEFT JOIN trae varias filas
    const byResearchCode = new Map();
    for (const g of raw) if (!byResearchCode.has(g.research_code)) byResearchCode.set(g.research_code, g);
    const groups = [...byResearchCode.values()];

    // Clasificación según outdated/project
    const isOut = g => String(g.outdated) === '1';
    const isProj = g => String(g.project) === '1';
    const isUnit = g => String(g.category) === 'unit';

    const activeNoProj = groups.filter(g => !isOut(g) && !isProj(g) && !isUnit(g));
    const activeProj   = groups.filter(g => !isOut(g) && isProj(g));
    const activeUnit   = groups.filter(g => !isOut(g) && isUnit(g));
    const outdated     = groups.filter(g => isOut(g));

    // Orden: 1) activos sin project, 2) activos con project, 3) outdated
    const sortedGroups = [...activeNoProj, ...activeProj, ...activeUnit, ...outdated];

    let filteredGroups = sortedGroups;

    if (currentGroupFilter === "project") {
      filteredGroups = sortedGroups.filter(g => String(g.project) === "1" && String(g.outdated) !== "1");
    } else if (currentGroupFilter === "sgr") {
      filteredGroups = sortedGroups.filter(g => String(g.sgr) === "1" && String(g.outdated) !== "1");
    } else if (currentGroupFilter === "group") {
      filteredGroups = sortedGroups.filter(g => String(g.project) !== "1" && String(g.sgr) !== "1" && String(g.outdated) !== "1");
    } else if (currentGroupFilter === "unit") {
      filteredGroups = sortedGroups.filter(g => String(g.category) === "unit" && String(g.outdated) !== "1");
    }

    // Render
    const html = filteredGroups.map(group => {
      const { intern_code, research_code, name } = group;
      const outdatedFlag = isOut(group);
      const projectFlag  = isProj(group);
      const sgrFlag = String(group.sgr) === '1';
      const unitFlag = String(group.category) === 'unit';


      const cardClass = outdatedFlag ? "bg-light text-muted opacity-50" : "";

      const badges = [
        outdatedFlag ? '<span class="badge bg-secondary ms-2">Inactive</span>' : '',
        (!outdatedFlag && projectFlag) ? '<span class="badge bg-info ms-2">Project</span>' : '',
        (!outdatedFlag && sgrFlag) ? '<span class="badge bg-success ms-2">SGR</span>' : '',
        (!outdatedFlag && unitFlag) ? '<span class="badge bg-warning ms-2">Unit</span>' : '',
        (!outdatedFlag && !projectFlag && !sgrFlag && !unitFlag) ? '<span class="badge bg-primary ms-2">Group</span>' : ''
      ].join('');

      return `
        <div class="col">
          <div class="card h-100 ${cardClass}">
            <div class="card-body d-flex align-items-center">
              <div>
                <h5 class="mb-0">
                  ${name ?? ''}, ${intern_code ?? ""} (${research_code ?? ""})
                  ${badges}
                </h5>
                <div class="mt-2 d-flex gap-2 flex-wrap">
                  <button class="btn btn-sm btn-danger"
                          data-role="edit-data"
                          data-research-code="${research_code}">
                    Modify group data
                  </button>
                  <button class="btn btn-sm btn-outline-danger"
                          data-role="edit-members"
                          data-intern-code="${intern_code}">
                    Modify group members
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>`;
    }).join('');

    container.innerHTML = html;

    return byResearchCode;
  };

  // Inicial carga
  let byResearchCode = await loadGroups();

  // Add group
  const addBtn = document.getElementById('openAddGroupModalBtn');
  if (addBtn) {
    addBtn.addEventListener('click', () => {
      const modalEl = document.getElementById('groupFormModal');
      if (!modalEl || !window.bootstrap?.Modal) return;

      modalEl.dataset.mode = "add"; // marcar modo
      modalEl.dataset.researchCode = "";

      const modal = new bootstrap.Modal(modalEl);
      const recognitionOptions = (formOptions.recognition ?? [])
        .map(opt => `<option value="${opt.code}">${opt.name}</option>`)
        .join('');
      
      const knowledgeAreas = (formOptions.knowledgeArea ?? [])
        .map(opt => `<option value="${opt.code}">${opt.name}</option>`)
        .join('');
      
      const characterOptions = (formOptions.character ?? [])
        .map(opt => `<option value="${opt.code}">${opt.name}</option>`)
        .join('');

      const typologyOptions = (formOptions.typology ?? [])
        .map(opt => `<option value="${opt.code}">${opt.name}</option>`)
        .join('');

      document.getElementById('formFieldsContainer').innerHTML = `
        <div class="col-md-6">
          <label for="groupName" class="form-label">Group Name</label>
          <input type="text" class="form-control" id="groupName" value="">
        </div>
        <div class="col-md-6">
          <label for="knowledgeArea" class="form-label">Knowledge Area</label>
          <select class="form-select" id="knowledgeArea">
            <option value="">-- Select --</option>
            ${knowledgeAreas}
          </select>
        </div>
        <div class="col-md-6">
          <label for="formerName" class="form-label">Former name</label>
          <input type="text" class="form-control" id="formerName" value="">
        </div>
        <div class="col-md-6">
          <label for="researchCode" class="form-label">Research Code</label>
          <input type="text" class="form-control" id="researchCode" value="">
        </div>
        <div class="col-md-6">
          <label for="internCode" class="form-label">Internal Code</label>
          <input type="text" class="form-control" id="internCode" value="">
        </div>
        <div class="col-md-6">
          <label for="startDate" class="form-label">Start date</label>
          <input type="date" class="form-control" id="startDate" value="">
        </div>
        <div class="col-md-6">
          <label for="endDate" class="form-label">End date</label>
          <input type="date" class="form-control" id="endDate" value="">
        </div>
        <div class="col-md-6">
          <label for="recognitionCode" class="form-label">Recognition type</label>
          <select class="form-select" id="recognitionCode">
            <option value="">-- Select --</option>
            ${recognitionOptions}
          </select>
        </div>
        <div class="col-md-6">
          <label for="recognitionDate" class="form-label">Recognition obtaining date</label>
          <input type="date" class="form-control" id="recognitionDate" value="">
        </div>
        <div class="col-12 d-flex gap-4 align-items-center mt-2">
          <div class="form-check">
            <input class="form-check-input" type="checkbox" id="outdated">
            <label class="form-check-label" for="outdated">Outdated</label>
          </div>
          <div class="form-check">
            <input class="form-check-input" type="checkbox" id="project">
            <label class="form-check-label" for="project">Project</label>
          </div>
          <div class="form-check">
            <input class="form-check-input" type="checkbox" id="sgr">
            <label class="form-check-label" for="sgr">SGR</label>
          </div>
          <div class="form-check">
            <input class="form-check-input" type="checkbox" id="unit">
            <label class="form-check-label" for="unit">Unit</label>
          </div>
  </div>
          <div class="col-md-6">
            <label for="cif" class="form-label">Unit CIF</label>
            <input type="text" class="form-control" id="cif" value="">
          </div>

          <div class="col-md-6">
            <label for="character" class="form-label">Unit Character</label>
            <select class="form-select" id="character">
              <option value="">-- Select --</option>
              ${characterOptions}
            </select>
          </div>

          <div class="col-md-6">
            <label for="typology" class="form-label">Unit Typology</label>
            <select class="form-select" id="typology">
              <option value="">-- Select --</option>
              ${typologyOptions}
            </select>
          </div>
      
      `;

      setupUnitRecognitionLogic();
      setupUnitInfoLogic();
      modal.show();
    });
  }

  // Event delegation para los botones de cada tarjeta
  container.addEventListener('click', (e) => {
    const btn = e.target.closest('button[data-role]');
    if (!btn) return;

    const role = btn.dataset.role;

    if (role === 'edit-data') {
      const researchCode = btn.dataset.researchCode;
      const group = byResearchCode.get(researchCode);
      if (!group) return;

      const modalEl = document.getElementById('groupFormModal');
      if (!modalEl || !window.bootstrap?.Modal) return;

      modalEl.dataset.mode = "edit"; // marcar modo
      modalEl.dataset.researchCode = researchCode;

      const modal = new bootstrap.Modal(modalEl);

      const recognitionOptions = (formOptions.recognition ?? [])
        .map(opt => {
          const selected = opt.code === group.codi_reconeixement ? 'selected' : '';
          return `<option value="${opt.code}" ${selected}>${opt.name}</option>`;
        }).join('');

        const characterOptions = formOptions.character.map(opt => {
          const selected =
            String(opt.code).trim() === String(group.character).trim()
              ? 'selected'
              : '';
          return `<option value="${opt.code}" ${selected}>${opt.name}</option>`;
        }).join('');

        const typologyOptions = formOptions.typology.map(opt => {
          const selected =
            String(opt.code).trim() === String(group.typology).trim()
              ? 'selected'
              : '';
          return `<option value="${opt.code}" ${selected}>${opt.name}</option>`;
        }).join('');

        const knowledgeAreas = formOptions.knowledgeArea.map(opt => {
          const selected =
            String(opt.code).trim() === String(group.type).trim()
              ? 'selected'
              : '';
          return `<option value="${opt.code}" ${selected}>${opt.name}</option>`;
        }).join('');
        
      const outdatedChecked = (group.outdated === 1 || group.outdated === '1' || group.outdated === true) ? 'checked' : '';
      const projectChecked  = (group.project  === 1 || group.project  === '1' || group.project  === true) ? 'checked' : '';
      const sgrChecked = (group.sgr === 1 || group.sgr === '1' || group.sgr === true) ? 'checked' : '';
      const unitChecked = (group.category === 'unit') ? 'checked' : '';


      document.getElementById('formFieldsContainer').innerHTML = `
        <div class="col-md-6">
          <label for="groupName" class="form-label">Group Name</label>
          <input type="text" class="form-control" id="groupName" value="${group.name ?? ''}">
        </div>
        <div class="col-md-6">
          <label for="knowledgeArea" class="form-label">Knowledge Area</label>
          <select class="form-select" id="knowledgeArea">
            ${knowledgeAreas}
          </select>
        </div>
        <div class="col-md-6">
          <label for="formerName" class="form-label">Former name</label>
          <input type="text" class="form-control" id="formerName" value="${group.former_name ?? ''}">
        </div>
        <div class="col-md-6">
          <label for="researchCode" class="form-label">Research Code</label>
          <input type="text" class="form-control" id="researchCode" value="${group.research_code ?? ''}" disabled>
        </div>
        <div class="col-md-6">
          <label for="internCode" class="form-label">Internal Code</label>
          <input type="text" class="form-control" id="internCode" value="${group.intern_code ?? ''}" disabled>
        </div>
        <div class="col-md-6">
          <label for="startDate" class="form-label">Start date</label>
          <input type="date" class="form-control" id="startDate" value="${(group.start_date ?? '').slice(0, 10)}">
        </div>
        <div class="col-md-6">
          <label for="endDate" class="form-label">End date</label>
          <input type="date" class="form-control" id="endDate" value="${(group.end_date ?? '').slice(0, 10)}">
        </div>
        <div class="col-md-6">
          <label for="recognitionCode" class="form-label">Recognition type</label>
          <select class="form-select" id="recognitionCode">
            ${recognitionOptions}
          </select>
        </div>
        <div class="col-md-6">
          <label for="recognitionDate" class="form-label">Recognition obtaining date</label>
          <input type="date" class="form-control" id="recognitionDate" value="${(group.data_obtencio ?? '').slice(0, 10)}">
        </div>
        <div class="col-12 d-flex gap-4 align-items-center mt-2">
          <div class="form-check">
            <input class="form-check-input" type="checkbox" id="outdated" ${outdatedChecked}>
            <label class="form-check-label" for="outdated">Outdated</label>
          </div>
          <div class="form-check">
            <input class="form-check-input" type="checkbox" id="project" ${projectChecked}>
            <label class="form-check-label" for="project">Project</label>
          </div>
          <div class="form-check">
          <input class="form-check-input" type="checkbox" id="sgr" ${sgrChecked}>
          <label class="form-check-label" for="sgr">SGR</label>
        </div>
            <div class="form-check">
            <input class="form-check-input" type="checkbox" id="unit" ${unitChecked}>
            <label class="form-check-label" for="unit">Unit</label>
          </div>
        </div>

        <div class="col-md-6">
          <label for="cif" class="form-label">Unit CIF</label>
          <input type="text" class="form-control" id="cif" value="${group.cif ?? ''}">
        </div>

        <div class="col-md-6">
          <label for="character" class="form-label">Unit Character</label>
          <select class="form-select" id="character">
          <option value="">-- Select --</option>
            ${characterOptions}
          </select>
        </div>

        <div class="col-md-6">
          <label for="typology" class="form-label">Unit Typology</label>
          <select class="form-select" id="typology">
          <option value="">-- Select --</option>
            ${typologyOptions}
          </select>
        </div>
      `;

      setupUnitRecognitionLogic();
      setupUnitInfoLogic();
      modal.show();
    }

    if (role === 'edit-members') {
      const internCode = btn.dataset.internCode;
      if (!internCode) return;
      window.location.href = `groupMembers.html?code=${encodeURIComponent(internCode)}`;
    }
  });

  // --- Guardar grupo (submit del modal) ---
  const form = document.getElementById("groupForm");
  form.addEventListener("submit", async (e) => {
    e.preventDefault(); // evitar reload

    const modalEl = document.getElementById('groupFormModal');
    if (!modalEl) return;

    const mode = modalEl.dataset.mode;
    const researchCode = modalEl.dataset.researchCode || document.getElementById("researchCode").value;

    const groupName = document.getElementById("groupName").value.trim();
    const internCode = document.getElementById("internCode").value.trim();

    // Validaciones obligatorias
    if (!groupName || !internCode || !researchCode.trim()) {
      alert("Group Name, Internal Code and Research Code are required.");
      return;
    }

    const payload = {
      research_code: researchCode,
      intern_code: internCode,
      name: groupName,
      former_name: document.getElementById("formerName").value,
      start_date: document.getElementById("startDate").value,
      end_date: document.getElementById("endDate").value,
      codi_reconeixement: document.getElementById("recognitionCode").value,
      data_obtencio: document.getElementById("recognitionDate").value,
      outdated: document.getElementById("outdated").checked ? 1 : 0,
      project: document.getElementById("project").checked ? 1 : 0,
      sgr: document.getElementById("sgr").checked ? 1 : 0,
      category: document.getElementById("unit").checked ? 'unit' : '',
      type: document.getElementById("knowledgeArea").value,
      cif: document.getElementById("cif").value,
      character: document.getElementById("character").value,
      typology: document.getElementById("typology").value,
    };

    const res = await fetch("/module/proxy/module4/api?action=saveGroup", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    });

    if (!res.ok) {
      alert("Error saving group");
      return;
    }

    bootstrap.Modal.getInstance(modalEl).hide();
    byResearchCode = await loadGroups();
  });

});


function setupUnitRecognitionLogic() {
  const unitCheckbox = document.getElementById('unit');
  const recognitionCode = document.getElementById('recognitionCode');
  const recognitionDate = document.getElementById('recognitionDate');

  if (!unitCheckbox || !recognitionCode || !recognitionDate) return;

  const applyState = () => {
    if (unitCheckbox.checked) {
      recognitionCode.value = '';
      recognitionDate.value = '';
      recognitionCode.disabled = true;
      recognitionDate.disabled = true;
    } else {
      recognitionCode.disabled = false;
      recognitionDate.disabled = false;
    }
  };

  applyState();

  unitCheckbox.addEventListener('change', applyState);
}

function setupUnitInfoLogic() {
  const unitCheckbox = document.getElementById('unit');
  const cif = document.getElementById('cif');
  const character = document.getElementById('character');
  const typology = document.getElementById('typology');

  if (!unitCheckbox || !cif || !character || !typology) return;

  const applyState = () => {
    if (unitCheckbox.checked) {
      cif.disabled = false;
      character.disabled = false;
      typology.disabled = false;
    } else {
      cif.value = '';
      character.value = '';
      typology.value = '';
      cif.disabled = true;
      character.disabled = true;
      typology.disabled = true;
    }
  };

  applyState();

  unitCheckbox.addEventListener('change', applyState);
}
