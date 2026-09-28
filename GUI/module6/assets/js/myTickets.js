// --- myTickets.js ---
// Mostrar los tickets del usuario actual

  export function formatDateTime(dateString) {
    if (!dateString) return "—";

    // Convertir T a espacio para unificar formatos
    const normalized = dateString.replace("T", " ");

    // Separar fecha y hora
    const [datePart, timePart = ""] = normalized.split(" ");

    const [year, month, day] = datePart.split("-");
    if (!year || !month || !day) return dateString; // si no es formato esperado, devolver original

    // Tomar solo HH:MM (ignorar segundos y zona)
    let [hh = "00", mm = "00"] = timePart.split(":");

    return `${day}/${month}/${year} ${hh}:${mm}`;
  }

  function isWithin7Days(dateString) {
    const d = new Date(dateString);
    if (isNaN(d)) return false;
    const diff = Date.now() - d.getTime();
    return diff <= 7 * 24 * 60 * 60 * 1000;
  }
  
  let ALL_TICKETS = [];
  let STATUS_FILTER = "ALL";

  function getStatusKey(status) {
    if (status === "ClosedConfirmed") return "Closed";
    return status || "—";
  }

  function ensureStatusFilterOptions(tickets) {
    const sel = document.getElementById("myTicketsStatusFilter");
    if (!sel) return;

    const statuses = new Set();
    tickets.forEach(t => statuses.add(getStatusKey(t.status)));

    // Mantén "ALL" y regenera el resto
    const keepAll = sel.querySelector('option[value="ALL"]');
    sel.innerHTML = "";
    sel.appendChild(keepAll || new Option("All", "ALL"));

    Array.from(statuses)
      .filter(s => s && s !== "—")
      .sort((a, b) => a.localeCompare(b))
      .forEach(s => sel.appendChild(new Option(s, s)));

    // si el filtro actual ya no existe, vuelve a ALL
    const exists = Array.from(sel.options).some(o => o.value === STATUS_FILTER);
    if (!exists) STATUS_FILTER = "ALL";
    sel.value = STATUS_FILTER;

    // engancha listener 1 vez
    if (!sel.dataset.bound) {
      sel.addEventListener("change", () => {
        STATUS_FILTER = sel.value;
        renderMyTickets();
      });
      sel.dataset.bound = "1";
    }
  }

  function applyStatusFilter(tickets) {
    if (STATUS_FILTER === "ALL") return tickets;
    return tickets.filter(t => getStatusKey(t.status) === STATUS_FILTER);
  }

  function renderMyTickets() {
    const tbody = document.getElementById("myTicketsBody");
    if (!tbody) return;

    tbody.innerHTML = "";
    const filteredTickets = applyStatusFilter(ALL_TICKETS);

    if (!filteredTickets.length) {
      tbody.innerHTML = `<tr><td colspan="7" class="text-center text-muted">No tickets for this filter.</td></tr>`;
      return;
    }
    console.log("Rendering tickets with filter", STATUS_FILTER, filteredTickets);
    filteredTickets.forEach((t) => {
      const tr = document.createElement("tr");

      let desc = t.comment
        ? `${t.description} [Technician's comment]: ${t.comment}`
        : (t.description || "—");
      if (t.file_path) {
        const encodedPath = `/module/proxy/module6/Uploads?file=${encodeURIComponent(t.file_path)}`;
        desc += ` <a href="${encodedPath}" target="_blank" class="text-primary ms-2">View attachment</a>`;
      }

      tr.innerHTML = `
        <td>${t.id}</td>
        <td>${t.department || "—"}</td>
        <td>${t.issue || "—"}</td>
        <td>${desc}</td>
        <td>
          <span class="badge bg-${
            t.status === "Closed" || t.status === "ClosedConfirmed"
              ? "success"
              : t.status === "In progress"
              ? "warning text-dark"
              : t.status === "Assigned"
              ? "info text-dark"
              : "secondary"
          }">
            ${t.status === "ClosedConfirmed" ? "Closed" : t.status}
          </span>

          ${
            t.status === "Closed" && isWithin7Days(t.updated || t.last_update)
              ? `
                <div class="d-flex gap-2 mt-2 flex-wrap">
                  <button class="btn btn-xs btn-outline-success confirm-close-btn" data-id="${t.id}"
                    style="padding: 2px 6px; font-size: 0.70rem;">Confirm</button>
                  <button class="btn btn-xs btn-outline-primary reopen-btn" data-id="${t.id}"
                    style="padding: 2px 6px; font-size: 0.70rem;">Reopen</button>
                </div>
              `
              : t.status === "Waiting for user"
                ? `
                  <div class="d-flex gap-2 mt-2 flex-wrap">
                    <button class="btn btn-xs btn-outline-primary reply-btn" data-id="${t.id}"
                      style="padding: 2px 6px; font-size: 0.70rem;"
                      data-message="${t.comment || ""}">
                      Reply
                    </button>
                  </div>
                `
                : ""
          }
        </td>

        <td>${t.assigned_to || "Pending assignment"}</td>
        <td>${formatDateTime(t.updated || t.last_update)}</td>
      `;

      tbody.appendChild(tr);
    });

    bindMyTicketsRowActions();
  }

function bindMyTicketsRowActions() {
  document.querySelectorAll(".confirm-close-btn").forEach(btn => {
    btn.onclick = async () => {
      const id = btn.dataset.id;
      if (!confirm("Are you sure you want to confirm this ticket is definitely closed?")) return;
      const res = await fetch("/module/proxy/module6/api?action=confirmCloseTicket", {
        method: "PUT",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify({ ticket_id: id })
      });
      const data = await res.json();
      if (data.status === "ok") {
        alert("Ticket successfully confirmed as closed.");
        loadMyTickets();
      }
    };
  });

  document.querySelectorAll(".reopen-btn").forEach(btn => {
    btn.onclick = async () => {
      const id = btn.dataset.id;
      const comment = prompt("Please describe why you want to reopen the ticket:");
      if (!comment) return;
      const res = await fetch("/module/proxy/module6/api?action=reopenTicket", {
        method: "PUT",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify({ ticket_id: id, comment })
      });
      const data = await res.json();
      if (data.status === "ok") {
        alert("Ticket reopened!");
        loadMyTickets();
      }
    };
  });

  document.querySelectorAll(".reply-btn").forEach(btn => {
    btn.onclick = async () => {
      const id = btn.dataset.id;
      const technicianMessage = btn.dataset.message || "";
      const comment = prompt("Please write your reply to the technician:");
      if (!comment) return;
      const res = await fetch("/module/proxy/module6/api?action=replyTicket", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ticket_id: id, message: technicianMessage, comment })
      });

      try {
        const data = await res.json();
        if (data.status === "ok") {
          alert("Reply sent and ticket moved back to In progress.");
          loadMyTickets();
        } else {
          alert("There was a problem sending your reply.");
        }
      } catch (e) {
        alert("Unexpected error sending reply.");
      }
    };
  });
}

// Cargar tickets del usuario actual
export async function loadMyTickets() {
    const tbody = document.getElementById("myTicketsBody");
    if (!tbody) return;
  
    tbody.innerHTML = `<tr><td colspan="7" class="text-center text-muted">Loading tickets...</td></tr>`;
  
    try {
      const res = await fetch("/module/proxy/module6/api?action=getMyTickets");
      if (!res.ok) throw new Error("Server error");
  
      const tickets = await res.json();

      ALL_TICKETS = tickets;

      console.log("Loaded tickets:", ALL_TICKETS);
      if (Array.isArray(ALL_TICKETS)) {
        ALL_TICKETS.sort((a, b) => new Date(b.updated) - new Date(a.updated));
        ensureStatusFilterOptions(ALL_TICKETS);
      }

      renderMyTickets();
      return;

    } catch (err) {
      console.error("Error loading tickets:", err);
      tbody.innerHTML = `<tr><td colspan="7" class="text-center text-danger">Error loading tickets</td></tr>`;
    }
  }
  
  // Escucha del enlace lateral para mostrar la sección
  document.addEventListener("click", (e) => {
    const link = e.target.closest('.nav-link[data-section="myTickets"]');
    if (!link) return;
    e.preventDefault();
  
    if (window.showSection) window.showSection("myTickets");
    loadMyTickets();
  });
  
