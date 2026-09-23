// --- departmentAdmin.js ---
// Department administrator -> gestión de tickets de su(s) departamento(s)
import { formatDateTime } from "./myTickets.js";

let deptTickets = [];
let deptWorkers = [];
let workersDB = [];

export function initDepartmentAdmin() {
  document.getElementById("openReassignPage")
    .addEventListener("click", () => {
      loadReassignPage();
    });

  loadDepartmentTickets();
}

// Cargar tickets del departamento(s) del manager
async function loadDepartmentTickets() {


  const tbody = document.getElementById("deptAdminTicketsBody");
  tbody.innerHTML = `<tr><td colspan="9" class="text-center text-muted">Loading tickets...</td></tr>`;

  try {
    const res = await fetch("/module/proxy/module6/api?action=getDepartmentTicketsAndWorkers");

    if (!res.ok) throw new Error(`HTTP ${res.status}`);

    const data = await res.json();


    deptTickets = data.tickets || [];
    workersDB = data.workers || [];

    renderDepartmentTickets();
  } catch (err) {
    console.error("Error loading tickets:", err);
    tbody.innerHTML = `<tr><td colspan="9" class="text-center text-danger">Error loading tickets</td></tr>`;
  }
}

// Renderizar tabla de tickets
function renderDepartmentTickets() {
  const tbody = document.getElementById("deptAdminTicketsBody");
  tbody.innerHTML = "";

  if (!deptTickets.length) {
    tbody.innerHTML = `<tr><td colspan="9" class="text-center text-muted">No unassigned tickets found.</td></tr>`;
    return;
  }

  deptTickets.forEach(t => {
    const tr = document.createElement("tr");
    const formattedDate = formatDateTime(t.creation_date);
    
    // Opciones de urgencia
    const urgencyOptions = ["Low", "Medium", "High"]
      .map(u => `<option value="${u}" ${t.urgency === u ? "selected" : ""}>${u}</option>`)
      .join("");

    // Opciones de trabajadores del mismo departamento
    const deptWorkers = workersDB.filter(w => w.department_name === t.department);
    const workerOptions = [`<option value="">Unassigned</option>`]
      .concat(
        deptWorkers.map(w =>
          `<option value="${w.worker_id}" ${t.assigned_to == w.worker_id ? "selected" : ""}>${w.worker_name}</option>`
        )
      )
      .join("");
    let descriptionHTML = t.description || "";
    if (t.file_path) {
      const encodedPath = `/module/proxy/module6/Uploads?file=${encodeURIComponent(t.file_path)}`;
      descriptionHTML += ` <a href="${encodedPath}" target="_blank" class="text-primary ms-2">View attachment</a>`;
    }
    tr.innerHTML = `
      <td>${t.id}</td>
      <td>${t.department}</td>
      <td>${t.issue}</td>
      <td>${descriptionHTML}</td>
      <td>${t.user_name || "Unknown"}</td>
      <td>${formattedDate}</td>
      <td>
        <select class="form-select form-select-sm urgency-select" data-id="${t.id}">
          ${urgencyOptions}
        </select>
      </td>
      <td>
        <select class="form-select form-select-sm worker-select" data-id="${t.id}">
          ${workerOptions}
        </select>
      </td>
      <td>
        <button class="btn btn-sm btn-danger assign-btn" data-id="${t.id}">Assign</button>
      </td>
    `;
    tbody.appendChild(tr);
  });

  attachDeptAdminListeners();
}

// Escuchas
function attachDeptAdminListeners() {
  document.querySelectorAll(".assign-btn").forEach(btn => {
    btn.addEventListener("click", async () => {
      const id = btn.dataset.id;
      const urgency = document.querySelector(`.urgency-select[data-id="${id}"]`).value;
      const assignedTo = document.querySelector(`.worker-select[data-id="${id}"]`).value;

      if (!assignedTo) {
        alert("Please select a worker to assign this ticket.");
        return;
      }

      try {
        const res = await fetch("/module/proxy/module6/api?action=assignDepartmentTicket", {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            ticket_id: id,
            urgency,
            assigned_to: assignedTo
          })
        });
        const data = await res.json();
        if (data.status === "ok") {
          alert("Ticket assigned successfully!");
          loadDepartmentTickets(); // recargar
        } else {
          alert(`Error: ${data.message}`);
        }
      } catch (err) {
        console.error("Error assigning ticket:", err);
      }
    });
  });
}


async function loadReassignPage() {

  const tbody = document.getElementById("reassignTicketsBody");
  tbody.innerHTML = `<tr><td colspan="9" class="text-center text-muted">Loading tickets...</td></tr>`;

  try {
    const res = await fetch("/module/proxy/module6/api?action=getDepartmentTicketsAndWorkersAssigned");

    if (!res.ok) throw new Error(`HTTP ${res.status}`);

    const data = await res.json();


    deptTickets = data.tickets || [];
    workersDB = data.workers || [];

    renderDepartmentTickets();
  } catch (err) {
    console.error("Error loading tickets:", err);
    tbody.innerHTML = `<tr><td colspan="9" class="text-center text-danger">Error loading tickets</td></tr>`;
  }
}
