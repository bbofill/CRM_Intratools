// ---- conditionTables.js ----

import { openNewListModal, setupNewListForm } from './modalHandler.js';


let currentUser = null;
let currentRole = null;
let allLists = [];


async function loadUser() {
  const res = await fetch('/module/proxy/module7/api?action=checkUserInformation');
  const info = await res.json();
  currentUser = info.userID;
  currentRole = info.role;
  // console.log(currentUser, currentRole)
}


// Carga las listas existentes desde el backend
  export async function loadLists() {
    await loadUser();
    try {
      const res = await fetch('/module/proxy/module7/api?action=getLists');
      if (!res.ok) throw new Error('Failed to load lists');
      const json = await res.json();
      return json.data || [];
    } catch (err) {
      console.error('Error loading lists:', err);
      return [];
    }
  }
  
  // Renderiza la tabla principal
  export function renderListTable(data, currentUser, currentRole) {
    const tableBody = document.getElementById('listTableBody');
  
    if (!data || data.length === 0) {
      tableBody.innerHTML = `
        <tr>
          <td colspan="4" class="text-center text-muted py-4">
            No lists available.
          </td>
        </tr>`;
      return;
    }
  
    tableBody.innerHTML = data.map(list => `
      <tr class="${list.description ? "has-description" : ""}" data-description="${list.description || ""}">
        <td>${list.name}</td>
        <td>${list.created_by || '-'}</td>
        <td class="conditions-cell">
          ${list.description ? `<div class="row-description">${list.description}</div>` : ""}
          <code>${JSON.stringify(list.conditions)}</code>
        </td>
        <td class="text-center">
          <div class="btn-group" role="group">

            <!-- MODIFY -> Only owner -->
            ${list.userID === currentUser ? `
              <button class="btn btn-sm btn-outline-primary" data-id="${list.id}" data-action="modify">
                <i class="bi bi-pencil-square"></i> Modify
              </button>
            ` : `
              <button class="btn btn-sm btn-outline-secondary" disabled>
                <i class="bi bi-lock"></i> Modify
              </button>
            `}
            <button class="btn btn-sm btn-outline-success" data-id="${list.id}" data-action="export">
              <i class="bi bi-file-earmark-arrow-down me-1"></i>Export
            </button>
            <button class="btn btn-sm btn-outline-info" data-id="${list.id}" data-action="notif">
              <i class="bi bi-envelope me-1"></i>Notify
            </button>
            <button class="btn btn-sm btn-outline-warning" data-id="${list.id}" data-action="duplicate">
              <i class="bi bi-files me-1"></i>Duplicate
            </button>
          <!-- DELETE -> Owner, admin, manager -->
          ${(list.userID === currentUser || currentRole === "admin" || currentRole === "manager") ? `
            <button class="btn btn-sm btn-outline-danger" data-id="${list.id}" data-action="delete">
              <i class="bi bi-trash3"></i> Delete
            </button>
          ` : `
            <button class="btn btn-sm btn-outline-secondary" disabled>
              <i class="bi bi-lock-fill"></i> Delete
            </button>
          `}
          </div>
        </td>
      </tr>
    `).join('');
  }
  
  export function setupEventHandlers() {
    const btnNewList = document.getElementById('btn-new-list');
    const tableBody = document.getElementById('listTableBody');
    const searchInput = document.getElementById('searchInput');
  
    // Crear nueva lista
    btnNewList.addEventListener('click', () => {
        openNewListModal();
    });
  
    // Buscar al pulsar botón Search
    searchInput.addEventListener('keyup', () => {
      const query = searchInput.value.trim().toLowerCase();
      const filtered = allLists.filter(list => 
        list.name.toLowerCase().includes(query) ||
        list.created_by?.toLowerCase().includes(query)
      );
      renderListTable(filtered, currentUser, currentRole);
    });

    // Acciones sobre cada fila
    tableBody.addEventListener('click', async (e) => {
      const btn = e.target.closest('button');
      if (!btn) return;
  
      const id = btn.dataset.id;
      const action = btn.dataset.action;
  
      if (action === 'delete') {
        if (!confirm('Are you sure you want to delete this list?')) return;
        const res = await fetch(`/module/proxy/module7/api?action=deleteList&id=${id}`);
        if (res.ok) {
          alert('List deleted successfully.');
          const data = await loadLists();
          allLists = data;
          renderListTable(data, currentUser, currentRole);
        } else {
          alert('Failed to delete list.');
        }
  
      } else if (action === 'export') {
        try {
          const res = await fetch(`/module/proxy/module7/api?action=exportList&id=${id}`);
          if (!res.ok) throw new Error('Failed to export CSV');
      

        // Leer el nombre original enviado por backend
        const disposition = res.headers.get("Content-Disposition");
        let filename = `list_${id}.csv`; // fallback

        if (disposition && disposition.includes("filename=")) {
          filename = disposition.split("filename=")[1].replace(/"/g, "");
        }

        // Leer el contenido
        const blob = await res.blob();

        // Detectar CSV vacío con mensaje de aviso
        const text = await blob.text();
        if (text.includes("No users found")) {
          alert("No users match the selected conditions.");
          return;
        }

        // Crear descarga
        const url = window.URL.createObjectURL(new Blob([text], { type: "text/csv" }));
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        a.remove();
        window.URL.revokeObjectURL(url);

        console.log(`CSV downloaded successfully as ${filename}`);
      } catch (err) {
        console.error(err);
        alert("Error downloading CSV");
      }
  
      } else if (action === 'modify') {
        const btn = e.target.closest('button[data-id]');
        if (!btn) return;
      
        const id = btn.getAttribute('data-id');
        console.log("Modify clicked for ID:", id);
      
        const res = await fetch(`/module/proxy/module7/api?action=getList&id=${id}`);
        if (!res.ok) {
          alert('Error fetching list details');
          return;
        }
        const data = await res.json();
        openNewListModal(data);
      } else if (action === 'notif') {
        // Enviar notificación
        window.location.href = `./notify.html?id=${id}`;
      } else if (action === 'duplicate') {
        duplicateList(id)
      }
    });

    tableBody.addEventListener("mouseover", (e) => {
      const row = e.target.closest("tr.has-description");
      if (!row) return;

      const tooltip = row.querySelector(".row-description");
      tooltip.style.display = "block";
    });

    tableBody.addEventListener("mouseout", (e) => {
      const row = e.target.closest("tr.has-description");
      if (!row) return;

      const tooltip = row.querySelector(".row-description");
      tooltip.style.display = "none";
    });

  }
  async function loadGroups() {
    const groupSelect = document.getElementById("groupSelect");
    if (!groupSelect) return;
  
    try {
      const res = await fetch("/module/proxy/module7/api?action=getGroups");
      if (!res.ok) throw new Error("Failed to load groups");
      const json = await res.json();
  
      groupSelect.innerHTML = ""; 
  
      if (!json.data || json.data.length === 0) {
        groupSelect.innerHTML = `<option disabled>No groups found</option>`;
        return;
      }
  
      json.data.forEach((g) => {
        const option = document.createElement("option");
        option.value = g.code;
        option.textContent = g.name || g.code;
        groupSelect.appendChild(option);
      });
  
    } catch (err) {
      console.error("[loadGroups] Error loading groups:", err);
      groupSelect.innerHTML = `<option disabled>Error loading groups</option>`;
    }
  }
  async function loadFundings() {
    const fundingSelect = document.getElementById("fundingSelect");
    if (!fundingSelect) return;
  
    try {
      const res = await fetch("/module/proxy/module7/api?action=getFundings");
      if (!res.ok) throw new Error("Failed to load fundings");
      const json = await res.json();
  
      fundingSelect.innerHTML = ""; 
  
      if (!json.data || json.data.length === 0) {
        fundingSelect.innerHTML = `<option disabled>No fundings found</option>`;
        return;
      }
  
      json.data.forEach((g) => {
        const option = document.createElement("option");
        option.value = g.code;
        option.textContent = g.name || g.code;
        fundingSelect.appendChild(option);
      });
  
    } catch (err) {
      console.error("[loadFundings] Error loading fundings:", err);
      fundingSelect.innerHTML = `<option disabled>Error loading fundings</option>`;
    }
  }
  
  function duplicateList(id) {
    if (!confirm("Do you want to duplicate this list?")) return;

    fetch(`/module/proxy/module7/api?action=duplicateList&id=${id}`, {
      method: 'POST'
    })
    .then(res => res.json())
    .then(result => {
      alert(result.message || "Duplicated successfully!");
      return loadLists();
    })
    .then(updatedData => {
      allLists = updatedData;
      renderListTable(updatedData, currentUser, currentRole);
    })
    .catch(err => {
      console.error("Error duplicating list:", err);
    });
  }
  
  document.addEventListener('DOMContentLoaded', async () => {
    await loadGroups();
    await loadFundings();

    setupNewListForm();
    setupEventHandlers();
    allLists = await loadLists();
    renderListTable(allLists, currentUser, currentRole);
  });


