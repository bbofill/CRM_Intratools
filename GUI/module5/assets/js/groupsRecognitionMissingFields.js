//////////////////////////////////////////////////////////////////////////////////////////////////
//                             UNEIX GROUP RECOGNITION TABLE CSV                                //
//////////////////////////////////////////////////////////////////////////////////////////////////
window.addEventListener("load", async () => {
  const container = document.getElementById("groups-container");
  const exportBtn = document.getElementById("exportCsvBtn");

  try {
    // Obtener grupos con campos faltantes
    const resMissing = await fetch("/module/proxy/module5/api?action=missingMandatoryFieldsGroupRecognition");
    let missingGroups = await resMissing.json();

    console.log(missingGroups)
    // Asegurar que sea un array
    if (!Array.isArray(missingGroups)) {
      console.warn("Backend devolvió formato inesperado. Forzando missingGroups = []");
      missingGroups = [];
    }

    if (missingGroups.length === 0) {
      container.innerHTML = `
        <div class="col">
          <div class="alert alert-success text-center">
            ✅ All group recognitions have complete mandatory fields.
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
                <h5 class="card-title">Group: ${g.codi_grupRecerca || "—"}</h5>
                <p class="mb-1"><strong>Recognition code:</strong> ${g.codi_reconeixement || "—"}</p>
                <p class="mb-2"><strong>Date of obtention:</strong> ${g.data_obtencio ? g.data_obtencio.split("T")[0] : "—"}</p>
                <ul class="small text-danger mb-3">${missingList}</ul>
              </div>
            </div>
          </div>`;
      });
      container.innerHTML = cards.join("");
    
    }

    // Obtener todos los reconocimientos (para exportar los completos)
    const resAll = await fetch("/module/proxy/module5/api?action=getAllGroupsRecognition");
    const allGroups = await resAll.json();

    // Filtrar los que no están incompletos
    const incompleteCodes = new Set((missingGroups || []).map(g => g.codi_grupRecerca));
    const completeGroups = Array.isArray(allGroups)
      ? allGroups.filter(g => !incompleteCodes.has(g.codi_grupRecerca))
      : [];

    // Activar el botón solo si hay grupos completos
    exportBtn.disabled = completeGroups.length === 0;

    // Exportación a CSV
    exportBtn.addEventListener("click", async () => {
      const currentYear = new Date().getFullYear();
      const defaultYear = currentYear - 1;

      const selectedYear = await showYearSelectModal(defaultYear);
      if (!selectedYear) return;

      const resAllYear = await fetch(`/module/proxy/module5/api?action=getAllGroupsRecognition&year=${selectedYear}`);
      let allGroupsYear = await resAllYear.json();

      if (!Array.isArray(allGroupsYear)) {
        console.warn("Backend devolvió formato inesperado. Forzando allGroups = []");
        allGroupsYear = [];
      }

      exportGroupsToCSV(allGroupsYear, missingGroups, selectedYear);
});



  } catch (err) {
    console.error("Error fetching data:", err);
    container.innerHTML = `
      <div class="alert alert-danger text-center">
        ❌ Error loading recognition data. Check console for details.
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
// EXPORTAR CSV (solo registros completos)
// =============================================================
function exportGroupsToCSV(groups, missingGroups, year) {
  if (!groups || groups.length === 0) {
    alert("No recognition data to export.");
    return;
  }

  // Códigos de los grupos incompletos
  const incompleteCodes = new Set((missingGroups || []).map(g => g.codi_grupRecerca));

  // Filtro: solo los grupos con todos los campos completos
  const completeGroups = groups.filter(g => {
    const hasAllFields =
      g.codi_grupRecerca &&
      g.codi_reconeixement &&
      g.data_obtencio &&
      !incompleteCodes.has(g.codi_grupRecerca);
    return hasAllFields;
  });

  if (completeGroups.length === 0) {
    alert("No complete recognition data to export.");
    return;
  }

  const shortYear = String(year).slice(-2);


  const lines = completeGroups.map(g => {
    const code = g.codi_grupRecerca || "";
    const recognition = g.codi_reconeixement || "";
    let date = "";
    if (g.data_obtencio) {
      const cleanDate = g.data_obtencio.split("T")[0];
      date = cleanDate.replace(/-/g, ""); // YYYYMMDD
    }
    return `${year}|0000001672|${code}|${recognition}|${date}`;
  });

  const content = lines.join("\n");
  const blob = new Blob([content], { type: "text/plain;charset=utf-8" });

  const fileName = `RM${shortYear}50.csv`;
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = fileName;
  a.click();
  URL.revokeObjectURL(a.href);

  alert(`✅ Exported ${completeGroups.length} complete recognition records successfully!`);
}
