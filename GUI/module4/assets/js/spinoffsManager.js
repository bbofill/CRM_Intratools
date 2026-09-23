//////////////////////////////////////////////////////////////////////////////////////////////////
//                  Spinoffs hub: formats spinoffs edition and visualization                    //
//////////////////////////////////////////////////////////////////////////////////////////////////

// assets/js/spinoffManager.js
document.addEventListener('DOMContentLoaded', async () => {
    const container = document.getElementById("spinoff-list");
    let currentEditingId = null;
  
    // Fetch spinoffs
    const res = await fetch("/module/proxy/module4/api?action=allSpinoffs");
    const spinoffs = await res.json();
  
    // Render cards
    const htmlBlocks = spinoffs.map(spinoff => `
      <div class="col">
        <div class="card h-100">
          <div class="card-body">
            <h5 class="mb-0">${spinoff.nom} (${spinoff.cif})</h5>
            <small class="text-muted">NIF: ${spinoff.nif} / ${spinoff.nif_ampliat}</small><br>
            <small class="text-muted">Conacit: ${spinoff.codi_conacit}</small>
            <div class="mt-2 d-flex gap-2 flex-wrap">
              <button class="btn btn-sm btn-danger btn-edit-spinoff" data-id="${spinoff.id}">
                Modify Spin Off data
              </button>
              <button class="btn btn-sm btn-outline-secondary btn-delete-spinoff" data-id="${spinoff.id}">
                ❌ Delete
              </button>
            </div>
          </div>
        </div>
      </div>`
    );
    container.innerHTML = htmlBlocks.join('');
  
    // Add spinoff
    document.getElementById('openAddSpinoffModalBtn').addEventListener('click', () => {
      currentEditingId = null;
      const modal = new bootstrap.Modal(document.getElementById('spinoffFormModal'));
  
      document.getElementById('formFieldsContainer').innerHTML = renderSpinoffForm();
      modal.show();
    });
  
    // Edit spinoff
    container.querySelectorAll('.btn-edit-spinoff').forEach(btn => {
      btn.addEventListener('click', async (e) => {
        const spinoffId = e.target.dataset.id;
        currentEditingId = spinoffId;
        const spinoff = spinoffs.find(s => s.id == spinoffId);
  
        const modal = new bootstrap.Modal(document.getElementById('spinoffFormModal'));
        document.getElementById('formFieldsContainer').innerHTML = renderSpinoffForm(spinoff);
        modal.show();
      });
    });
  
    // -------------------------------
    // Submit handler (add/edit save)
    // -------------------------------
    document.getElementById("spinoffForm").addEventListener("submit", async (e) => {
      e.preventDefault();
  
      const form = e.target;
      const payload = {
        id: currentEditingId,
        cif: form.querySelector("#spinoffCif").value.trim(),
        nom: form.querySelector("#spinoffNom").value.trim(),
        nif: form.querySelector("#spinoffNif").value.trim(),
        nif_ampliat: form.querySelector("#spinoffNifAmpliat").value.trim(),
        codi_conacit: form.querySelector("#spinoffConacit").value.trim(),
        data_creacio: form.querySelector("#spinoffDataCreacio").value,
        data_cessio: form.querySelector("#spinoffDataCessio").value,
        perc_participacio: form.querySelector("#spinoffPerc").value,
        data_extincio: form.querySelector("#spinoffDataExtincio").value,
        area_cnae: form.querySelector("#spinoffArea").value.trim(),
        ext_o_fin: form.querySelector("#spinoffExtFin").value.trim()
      };
  
      try {
        const res = await fetch("/module/proxy/module4/api?action=saveSpinoff", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload)
        });
        const result = await res.json();
        console.log("Saved:", result);
  
        location.reload();
      } catch (err) {
        console.error("Error saving spinoff:", err);
        alert("Error saving spinoff");
      }
    });
  
    // -------------------------------
    // Delete handler 
    // -------------------------------
    container.querySelectorAll('.btn-delete-spinoff').forEach(btn => {
      btn.addEventListener('click', async (e) => {
        const spinoffId = e.target.dataset.id;
  
        if (!confirm("Are you sure you want to delete this Spin Off?")) {
          return;
        }
  
        try {
          const res = await fetch("/module/proxy/module4/api?action=deleteSpinoff", {
            method: "POST", 
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ id: spinoffId })
          });
  
          if (!res.ok) {
            throw new Error("Failed to delete spinoff");
          }
  
          const result = await res.json();
          console.log("Deleted:", result);
  
          location.reload();
        } catch (err) {
          console.error("Error deleting spinoff:", err);
          alert("Error deleting spinoff");
        }
      });
    });
  
  });
  
  // Helper to render the form
  function renderSpinoffForm(spinoff = {}) {
    return `
      <div class="col-md-6">
        <label for="spinoffCif" class="form-label">CIF</label>
        <input type="text" class="form-control" id="spinoffCif" value="${spinoff.cif ?? ''}">
      </div>
      <div class="col-md-6">
        <label for="spinoffNom" class="form-label">Name</label>
        <input type="text" class="form-control" id="spinoffNom" value="${spinoff.nom ?? ''}">
      </div>
      <div class="col-md-6">
        <label for="spinoffNif" class="form-label">NIF</label>
        <input type="text" class="form-control" id="spinoffNif" value="${spinoff.nif ?? ''}">
      </div>
      <div class="col-md-6">
        <label for="spinoffNifAmpliat" class="form-label">NIF extended</label>
        <input type="text" class="form-control" id="spinoffNifAmpliat" value="${spinoff.nif_ampliat ?? ''}">
      </div>
      <div class="col-md-6">
        <label for="spinoffConacit" class="form-label">Conacit code</label>
        <input type="text" class="form-control" id="spinoffConacit" value="${spinoff.codi_conacit ?? ''}">
      </div>
      <div class="col-md-6">
        <label for="spinoffDataCreacio" class="form-label">Creation date</label>
        <input type="date" class="form-control" id="spinoffDataCreacio" value="${spinoff.data_creacio ?? ''}">
      </div>
      <div class="col-md-6">
        <label for="spinoffDataCessio" class="form-label">Cession date</label>
        <input type="date" class="form-control" id="spinoffDataCessio" value="${spinoff.data_cessio ?? ''}">
      </div>
      <div class="col-md-6">
        <label for="spinoffPerc" class="form-label">% Participation</label>
        <input type="number" class="form-control" id="spinoffPerc" value="${spinoff.perc_participacio ?? ''}">
      </div>
      <div class="col-md-6">
        <label for="spinoffDataExtincio" class="form-label">Extinction date</label>
        <input type="date" class="form-control" id="spinoffDataExtincio" value="${spinoff.data_extincio ?? ''}">
      </div>
      <div class="col-md-6">
        <label for="spinoffArea" class="form-label">CNAE Area</label>
        <input type="text" class="form-control" id="spinoffArea" value="${spinoff.area_cnae ?? ''}">
      </div>
      <div class="col-md-6">
        <label for="spinoffExtFin" class="form-label">Extinció o finalització de la relació</label>
        <input type="text" class="form-control" id="spinoffExtFin" value="${spinoff.ext_o_fin ?? ''}">
      </div>
    `;
  }
  