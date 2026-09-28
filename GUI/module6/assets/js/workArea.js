// --- workArea.js ---
// Vista para técnicos: gestionar tickets asignados a ti.

import { formatDateTime } from "./myTickets.js";

// Estados disponibles
const ticketStatuses = ["Open", "Assigned", "In progress", "Waiting for user", "Closed"];

const urgencyPriority = {
  "High": 3,
  "Medium": 2,
  "Low": 1
};

// Cargar tickets asignados
export async function loadWorkTickets() {
    const tbody = document.getElementById("workTicketsBody");
    tbody.innerHTML = `<tr><td colspan="9" class="text-center text-muted">Loading assigned tickets...</td></tr>`;
  
    try {
      const res = await fetch("/module/proxy/module6/api?action=getAssignedTickets");
      if (!res.ok) throw new Error("Server error");
      const tickets = await res.json();

      if (!tickets.length) {
        tbody.innerHTML = `<tr><td colspan="9" class="text-center text-muted">No tickets assigned to you.</td></tr>`;
        return;
      }

      // Ordenar por prioridad
      tickets.sort((a, b) => {
        const urgencyA = urgencyPriority[a.urgency] || 0;
        const urgencyB = urgencyPriority[b.urgency] || 0;
  
        if (urgencyA !== urgencyB) {
          return urgencyB - urgencyA; // primero las más altas
        }
  
        // si tienen misma prioridad, ordenar por fecha (más reciente primero)
        const dateA = new Date(a.created_at);
        const dateB = new Date(b.created_at);
        return dateB - dateA;
      });
  
      tbody.innerHTML = "";
      tickets.forEach((t) => {
        const tr = document.createElement("tr");
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
          <td>${t.submitted_by}</td>
          <td>${formatDateTime(t.creation_date)}</td>
          <td>${t.urgency || "—"}</td>
          <td>
            <select class="form-select form-select-sm status-select" data-id="${t.id}">
              ${ticketStatuses
                .map((s) => `<option value="${s}" ${t.status === s ? "selected" : ""}>${s}</option>`)
                .join("")}
            </select>
          </td>
          <td><button class="btn btn-outline-danger btn-sm save-status" data-id="${t.id}">Update</button></td>
        `;
        tbody.appendChild(tr);
      });
  
      attachStatusListeners();
    } catch (err) {
      console.error("❌ Error loading tickets:", err);
      tbody.innerHTML = `<tr><td colspan="8" class="text-center text-danger">Error loading tickets</td></tr>`;
    }
  }
  
  // Actualizar estado del ticket
  function attachStatusListeners() {
    document.querySelectorAll(".save-status").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const id = btn.dataset.id;
        const newStatus = document.querySelector(`.status-select[data-id="${id}"]`).value;
        const message = prompt("Optional message for the user:", "") || "";
  
        try {
          const res = await fetch("/module/proxy/module6/api?action=updateTicketStatus", {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ ticket_id: id, status: newStatus, message }),
          });
          const data = await res.json();
          if (data.status === "ok") {
            alert(`✅ Ticket ${id} updated to "${newStatus}"`);
            loadWorkTickets();
          } else {
            alert(`Error: ${data.message}`);
          }
        } catch (err) {
          console.error("Error updating status:", err);
          alert("Server error updating ticket");
        }
      });
    });
  }
  