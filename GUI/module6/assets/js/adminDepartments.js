// --- departmentAdmin.js ---
// Módulo para gestionar departamentos y trabajadores 

// Variables globales
let departments = [];
let departmentWorkers = [];
let workersDB = [];

// --- Obtener nombre de trabajador por ID ---
function getWorkerName(worker_id) {
  const worker = workersDB.find(w => String(w.id) === String(worker_id));
  return worker ? worker.name : "Unknown";
}

function showWorkerSelectModal(availableWorkers) {
    return new Promise((resolve) => {
      // Si existe, eliminar
      const existing = document.getElementById("workerSelectModal");
      if (existing) existing.remove();
  
      // Crear modal
      const modal = document.createElement("div");
      modal.id = "workerSelectModal";
      modal.className =
        "position-fixed top-0 start-0 w-100 h-100 bg-dark bg-opacity-50 d-flex justify-content-center align-items-center";
      modal.style.zIndex = 2001;
  
      modal.innerHTML = `
        <div class="bg-white rounded shadow p-4" style="max-width: 450px; width: 100%;">
          <h5 class="mb-3 text-danger">Add worker to department</h5>
          <input list="workerList" id="workerInput" class="form-control mb-3" placeholder="Start typing a name...">
          <datalist id="workerList">
            ${availableWorkers
              .map(
                (w) =>
                  `<option data-id="${w.id}" value="${w.name}"></option>`
              )
              .join("")}
          </datalist>
          <div class="form-check mb-3">
            <input type="checkbox" id="isManagerCheckbox" class="form-check-input">
            <label for="isManagerCheckbox" class="form-check-label">Set as manager</label>
          </div>
          <div class="d-flex justify-content-end gap-2">
            <button id="cancelWorker" class="btn btn-secondary btn-sm">Cancel</button>
            <button id="confirmWorker" class="btn btn-danger btn-sm">Add</button>
          </div>
        </div>
      `;
  
      document.body.appendChild(modal);
  
      const input = modal.querySelector("#workerInput");
      const confirmBtn = modal.querySelector("#confirmWorker");
      const cancelBtn = modal.querySelector("#cancelWorker");
      const datalist = modal.querySelector("#workerList");
      const managerCheckbox = modal.querySelector("#isManagerCheckbox");
  
      confirmBtn.addEventListener("click", () => {
        const opt = Array.from(datalist.options).find(
          (o) => o.value === input.value
        );
        if (opt && opt.dataset.id) {
          const workerId = Number(opt.dataset.id);
          const isManager = managerCheckbox.checked ? 1 : 0;
          modal.remove();
          resolve({ workerId, isManager });
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

// --- Cargar datos del backend ---
async function loadDepartmentsData() {
  const tbody = document.getElementById("sectionsTableBody");
  if (tbody) tbody.innerHTML = `<tr><td colspan="4" class="text-center text-muted">Loading departments...</td></tr>`;

  try {
    const [deptRes, workersRes, relRes] = await Promise.all([
      fetch("/module/proxy/module6/api?action=getDepartments"),
      fetch("/module/proxy/module6/api?action=getAllWorkers"),
      fetch("/module/proxy/module6/api?action=getDepartmentWorkers"),
    ]);

    if (!deptRes.ok || !workersRes.ok || !relRes.ok) {
      throw new Error(`One of the API calls failed (${deptRes.status}, ${workersRes.status}, ${relRes.status})`);
    }

    departments = await deptRes.json();
    workersDB = await workersRes.json();
    departmentWorkers = await relRes.json();


    renderDepartmentsTable();
  } catch (err) {
    console.error("Error loading department data:", err);
    if (tbody)
      tbody.innerHTML = `<tr><td colspan="4" class="text-center text-danger">Error loading data</td></tr>`;
  }
}

// --- Renderizar tabla ---
function renderDepartmentsTable() {
  const tbody = document.getElementById("sectionsTableBody");
  if (!tbody) return;
  tbody.innerHTML = "";

  if (!departments.length) {
    tbody.innerHTML = `<tr><td colspan="4" class="text-center text-muted">No departments found.</td></tr>`;
    return;
  }

  departments.forEach((dept) => {
    const deptWorkers = departmentWorkers.filter(
      (dw) => dw.department_name === dept.name
    );

    const workerChips = deptWorkers
      .map((dw) => {
        const name = getWorkerName(dw.worker_id);
        const isManager = dw.manager === 1 || dw.manager === "1";
        const badgeClass = isManager ? "bg-danger" : "bg-secondary";
        return `
          <div class="d-inline-flex align-items-center mb-1 me-1 border rounded px-2 py-1 ${badgeClass} text-white">
            ${name}
            ${
              isManager
                ? `<small class="ms-1">(Manager)</small>`
                : `<small class="ms-1 text-white-50">(Worker)</small>`
            }
            <div class="ms-2 btn-group btn-group-sm">
              <button class="btn btn-light toggle-manager-btn" data-dept="${dept.name}" data-id="${dw.worker_id}" title="Toggle manager">
                👤
              </button>
              <button class="btn btn-outline-light remove-worker-btn" data-dept="${dept.name}" data-id="${dw.worker_id}" title="Remove worker">
                ✖
              </button>
            </div>
          </div>
        `;
      })
      .join("");

    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td><strong>${dept.name}</strong></td>
      <td>${workerChips || "<span class='text-muted'>No workers</span>"}</td>
      <td>
        <button class="btn btn-outline-danger btn-sm add-worker-btn" data-name="${dept.name}">+ Add Worker</button>
      </td>
      <td class="text-center download-actions">
        <div class="btn-group" role="group">
          <button 
            class="btn btn-light btn-sm border download-dept-btn d-flex align-items-center" 
            data-name="${dept.name}"
            style="gap: 6px; padding: 4px 10px;"
          >
            <i class="bi bi-file-earmark-arrow-down text-success"></i>
            Tickets
          </button>

          <button 
            class="btn btn-light btn-sm border downloadhistoric-dept-btn d-flex align-items-center" 
            data-name="${dept.name}"
            style="gap: 6px; padding: 4px 10px;"
          >
            <i class="bi bi-clock-history text-info"></i>
            History
          </button>
        </div>
      </td>

      <td>
        <button class="btn btn-outline-secondary btn-sm remove-dept-btn" data-name="${dept.name}">Remove Dept</button>
      </td>

    `;
    tbody.appendChild(tr);
  });

  attachDepartmentListeners();
}

// --- Escuchas de botones ---
function attachDepartmentListeners() {
  // Eliminar departamento
  document.querySelectorAll(".remove-dept-btn").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const name = btn.dataset.name;
      if (!confirm(`Are you sure you want to delete department "${name}"?`)) return;

      try {
        const res = await fetch(
          `/module/proxy/module6/api?action=deleteDepartment&name=${encodeURIComponent(name)}`,
          { method: "DELETE" }
        );
        const data = await res.json();
        if (data.status === "ok") {
          departments = departments.filter((d) => d.name !== name);
          departmentWorkers = departmentWorkers.filter(
            (dw) => dw.department_name !== name
          );
          renderDepartmentsTable();
        } else {
          alert(`Error deleting department: ${data.message}`);
        }
      } catch (err) {
        console.error("Error deleting department:", err);
      }
    });
  });

  // Descargar tickets
  document.querySelectorAll(".download-dept-btn").forEach(btn => {
    btn.addEventListener("click", () => {
      const dept = btn.dataset.name;
      window.location.href =
        `/module/proxy/module6/api?action=downloadDeptTickets&dept=${encodeURIComponent(dept)}`;
    });
  });

  // Descargar histórico
  document.querySelectorAll(".downloadhistoric-dept-btn").forEach(btn => {
    btn.addEventListener("click", () => {
      const dept = btn.dataset.name;
      window.location.href =
        `/module/proxy/module6/api?action=downloadDeptHistory&dept=${encodeURIComponent(dept)}`;
    });
  });

  // Eliminar trabajador
  document.querySelectorAll(".remove-worker-btn").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const deptName = btn.dataset.dept;
      const workerId = parseInt(btn.dataset.id);
      if (!confirm(`Remove ${getWorkerName(workerId)} from ${deptName}?`))
        return;

      try {
        const res = await fetch(
          `/module/proxy/module6/api?action=removeWorkerFromDepartment`,
          {
            method: "DELETE",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ department_name: deptName, worker_id: workerId }),
          }
        );
        const data = await res.json();
        if (data.status === "ok") {
          departmentWorkers = departmentWorkers.filter(
            (dw) =>
              !(
                dw.department_name === deptName && dw.worker_id == workerId
              )
          );
          renderDepartmentsTable();
        } else {
          alert(`Error removing worker: ${data.message}`);
        }
      } catch (err) {
        console.error("Error removing worker:", err);
      }
    });
  });

  // Añadir trabajador
 document.querySelectorAll(".add-worker-btn").forEach((btn) => {
  btn.addEventListener("click", async () => {
    const deptName = btn.dataset.name;
    const assigned = departmentWorkers
      .filter((dw) => dw.department_name === deptName)
      .map((dw) => parseInt(dw.worker_id));
    const available = workersDB.filter(
      (w) => !assigned.includes(parseInt(w.id))
    );

    if (!available.length) {
      alert("All workers are already assigned to this department.");
      return;
    }

    // Mostrar modal con los trabajadores disponibles
    const result = await showWorkerSelectModal(available);
    if (!result) return; // cancelado
    const { workerId, isManager } = result;

    try {
      const res = await fetch(
        `/module/proxy/module6/api?action=addWorkerToDepartment`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            department_name: deptName,
            worker_id: workerId,
            manager: isManager,
          }),
        }
      );
      const data = await res.json();
      if (data.status === "ok") {
        departmentWorkers.push({
          department_name: deptName,
          worker_id: workerId,
          manager: isManager,
        });
        renderDepartmentsTable();
      } else {
        alert(`Error adding worker: ${data.message}`);
      }
    } catch (err) {
      console.error("Error adding worker:", err);
    }
  });
});
// ------ hacer manager ------------
  document.querySelectorAll(".toggle-manager-btn").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const deptName = btn.dataset.dept;
      const workerId = parseInt(btn.dataset.id);
      const worker = departmentWorkers.find(
        (dw) =>
          dw.department_name === deptName &&
          String(dw.worker_id) === String(workerId)
      );
      if (!worker) return;

      const newManager = worker.manager === 1 ? 0 : 1;

      try {
        const res = await fetch(
          `/module/proxy/module6/api?action=updateWorkerManagerStatus`,
          {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              department_name: deptName,
              worker_id: workerId,
              manager: newManager,
            }),
          }
        );
        const data = await res.json();
        if (data.status === "ok") {
          worker.manager = newManager;
          renderDepartmentsTable();
        } else {
          alert(`Error updating manager flag: ${data.message}`);
        }
      } catch (err) {
        console.error("Error updating manager flag:", err);
      }
    });
  });
}

// --- Añadir departamento ---
function setupAddDepartmentForm() {
  const form = document.getElementById("addSectionForm");
  if (!form) return;

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const name = document.getElementById("newSectionName").value.trim();
    if (!name) return;

    if (departments.some((d) => d.name.toLowerCase() === name.toLowerCase())) {
      alert("That department already exists.");
      return;
    }

    try {
      const res = await fetch(`/module/proxy/module6/api?action=createDepartment`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name }),
      });
      const data = await res.json();
      if (data.status === "ok") {
        departments.push({ name });
        renderDepartmentsTable();
        form.reset();
      } else {
        alert(`Error creating department: ${data.message}`);
      }
    } catch (err) {
      console.error("Error creating department:", err);
    }
  });
}


// --- Inicialización ---
document.addEventListener("DOMContentLoaded", () => {

  const adminLink = document.querySelector('[data-section="admin"]');
  if (adminLink) {
    adminLink.addEventListener("click", async () => {
      await loadDepartmentsData();
    });
  }

  setupAddDepartmentForm();
});
