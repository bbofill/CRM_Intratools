//////////////////////////////////////////////////////////////////////////////////////////////////
//                                      UNEIX GROUP TABLE CSV                                   //
//////////////////////////////////////////////////////////////////////////////////////////////////
window.addEventListener("load", async () => {
    const container = document.getElementById("groups-container");
    const exportBtn = document.getElementById("exportCsvBtn");
  
    try {
      //  Obtener grupos incompletos (para mostrar y editar)
      const resMissing = await fetch("/module/proxy/module5/api?action=missingMandatoryFieldsGroups");
      let missingGroups = await resMissing.json();
  
      // Garantizar que sea un array
      if (!Array.isArray(missingGroups)) {
        missingGroups = [];
      }
  
  
      if (missingGroups.length === 0) {
        container.innerHTML = `
          <div class="col">
            <div class="alert alert-success text-center">
              ✅ All research groups have complete mandatory fields.
            </div>
          </div>`;
      } else {
        const cards = missingGroups.map((g, i) => {
          const missingList = Array.isArray(g.missing_fields)
            ? g.missing_fields.map(f => `<li>${f}</li>`).join("")
            : "<li>No missing fields</li>";
  
          return `
            <div class="col-md-6">
              <div class="card shadow-sm mb-3">
                <div class="card-body">
                  <h5 class="card-title">${g.name || "—"}</h5>
                  <p class="mb-1"><strong>Research code:</strong> ${g.research_code || "—"}</p>
                  <p class="mb-2"><strong>Intern code:</strong> ${g.intern_code || "—"}</p>
                  <ul class="small text-danger mb-3">${missingList}</ul>
                </div>
              </div>
            </div>`;
        });
  
        container.innerHTML = cards.join("");
  
        // Vincular eventos de edición
        document.querySelectorAll(".edit-btn").forEach(btn => {
          btn.addEventListener("click", e => {
            const index = e.target.dataset.index;
            openEditModal(missingGroups[index]);
          });
        });
      }
  
      // Obtener todos los grupos (para exportar los completos)
      const resAll = await fetch("/module/proxy/module5/api?action=getAllGroups");
      let allGroups = await resAll.json();
  
      if (!Array.isArray(allGroups)) {
        console.warn("Backend devolvió formato inesperado. Forzando allGroups = []");
        allGroups = [];
      }
  
      // Filtrar los grupos completos (no presentes en missingGroups)
      const incompleteCodes = new Set((missingGroups || []).map(g => g.intern_code));
      const completeGroups = allGroups.filter(g => !incompleteCodes.has(g.intern_code));
    
      // Habilitar el botón de export solo si hay grupos completos
      exportBtn.disabled = completeGroups.length === 0;
  
      // Asociar evento de exportación
      exportBtn.addEventListener("click", async () => {
        const currentYear = new Date().getFullYear();
        const defaultYear = currentYear - 1;

        const selectedYear = await showYearSelectModal(defaultYear);
        if (!selectedYear) return;

        const resAllYear = await fetch(`/module/proxy/module5/api?action=getAllGroups&year=${selectedYear}`);
        let allGroupsYear = await resAllYear.json();

        if (!Array.isArray(allGroupsYear)) {
          console.warn("Backend devolvió formato inesperado. Forzando allGroups = []");
          allGroupsYear = [];
        }

        // Reutilizamos missingGroups YA cargado (sin año)
        const incompleteCodes = new Set((missingGroups || []).map(g => g.intern_code));
        const completeGroupsYear = allGroupsYear.filter(g => !incompleteCodes.has(g.intern_code));

        if (completeGroupsYear.length === 0) {
          alert("No complete groups to export for that year!");
          return;
        }

        exportGroupsToCSV(completeGroupsYear, selectedYear);
      });

  
    } catch (err) {
      console.error("Error fetching data:", err);
      container.innerHTML = `
        <div class="alert alert-danger text-center">
          ❌ Error loading groups. Check console for details.
        </div>`;
    }
  });
  
  
// =========================
// MODAL PARA SELECCIONAR AÑO DE EXPORTACIÓN
// =========================
function showYearSelectModal(defaultYear) {
  return new Promise(resolve => {
    const existing = document.getElementById("yearSelectModal");
    if (existing) existing.remove();

    const modal = document.createElement("div");
    modal.id = "yearSelectModal";
    modal.className =
      "position-fixed top-0 start-0 w-100 h-100 bg-dark bg-opacity-50 d-flex justify-content-center align-items-center";
    modal.style.zIndex = 2002;

    const currentYear = new Date().getFullYear();

    modal.innerHTML = `
      <div class="bg-white rounded shadow p-4" style="max-width: 400px; width: 100%;">
        <h5 class="mb-3">Select the reference year</h5>
        <p class="small text-muted mb-2">
          Data will be exported relative to this year.
        </p>
        <input
          id="yearInput"
          type="number"
          class="form-control mb-3"
          min="2000"
          max="${currentYear}"
          value="${defaultYear}"
        >
        <div class="d-flex justify-content-end gap-2">
          <button id="cancelYear" class="btn btn-secondary btn-sm">Cancel</button>
          <button id="confirmYear" class="btn btn-primary btn-sm">Confirm</button>
        </div>
      </div>
    `;

    document.body.appendChild(modal);

    const yearInput = modal.querySelector("#yearInput");
    const cancelBtn = modal.querySelector("#cancelYear");
    const confirmBtn = modal.querySelector("#confirmYear");

    cancelBtn.addEventListener("click", () => {
      modal.remove();
      resolve(null); 
    });

    confirmBtn.addEventListener("click", () => {
      const year = Number(yearInput.value);
      const currentYear = new Date().getFullYear();

      if (!year || Number.isNaN(year)) {
        alert("Please enter a valid year.");
        return;
      }
      if (year < 2000 || year > currentYear) {
        alert(`Year must be between 2000 and ${currentYear}.`);
        return;
      }

      modal.remove();
      resolve(year);
    });
  });
}

  // =============================================================
  // EXPORTAR GRUPOS A CSV (solo completos, separado por |)
  // =============================================================
  function exportGroupsToCSV(groups, year) {
    if (!groups || groups.length === 0) {
      alert("No complete groups to export.");
      return;
    }

    const shortYear = String(year).slice(-2);

    const fixedCode = "0000001672";
  
    const lines = groups.map(g => {
      const researchCode = g.research_code || "—";
      const name = g.name ? g.name.replace(/\|/g, " ") : "—";
      const internCode = g.intern_code || "—";
      let startDate = "";
        if (g.start_date) {
        const cleanDate = g.start_date.split("T")[0]; 
        startDate = cleanDate.replace(/-/g, "");    
        }
  
      return `${year}|${fixedCode}|${researchCode}|${name}|${startDate}|${internCode}`;
    });
  
    const content = lines.join("\n");
    const blob = new Blob([content], { type: "text/plain;charset=utf-8" });
    const fileName = `RM${shortYear}48.csv`;
    const a = document.createElement("a");
    const timestamp = new Date().toISOString().slice(0, 10).replace(/-/g, "");
    a.href = URL.createObjectURL(blob);
    a.download = fileName;
    a.click();
    URL.revokeObjectURL(a.href);
  
    alert(`✅ Exported ${groups.length} complete groups successfully!`);
  }
  