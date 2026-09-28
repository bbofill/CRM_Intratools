document.addEventListener('DOMContentLoaded', async () => {
  const container = document.getElementById("funding-list");
  const fundingForm = document.getElementById("fundingForm");
  const formFieldsContainer = document.getElementById("formFieldsContainer");
  const addFundingBtn = document.getElementById("openAddFundingModalBtn");
  const modalElement = document.getElementById("fundingFormModal");

  if (!container || !fundingForm || !formFieldsContainer || !addFundingBtn || !modalElement) {
    console.error("Some required DOM elements were not found.");
    return;
  }

  let currentEditingId = null;
  let fundings = [];

  const modal = new bootstrap.Modal(modalElement);

  function renderForm(funding = null) {
    const isEdit = !!funding;

    formFieldsContainer.innerHTML = `
      <div class="col-md-6">
        <label for="fundingName" class="form-label">Funding name</label>
        <input
          type="text"
          class="form-control"
          id="fundingName"
          name="fundingName"
          value="${funding?.name ?? ''}"
          required
        >
      </div>

      <div class="col-md-6">
        <label for="fundingCode" class="form-label">Code</label>
        <input
          type="text"
          class="form-control"
          id="fundingCode"
          name="fundingCode"
          value="${funding?.code ?? ''}"
          required
        >
      </div>

      <div class="col-12">
        <div class="form-check mt-2">
          <input
            type="checkbox"
            class="form-check-input"
            id="fundingActive"
            name="fundingActive"
            ${funding?.active ? 'checked' : ''}
            ${!isEdit ? 'checked' : ''}
          >
          <label for="fundingActive" class="form-check-label">Active</label>
        </div>
        <div class="form-text">Inactive fundings will not be available for new contracts but will remain associated with existing ones.</div>
      </div>
    `;

    document.getElementById('fundingFormModalLabel').textContent = isEdit
      ? 'Modify Funding'
      : 'Add Funding';
  }

  function renderCards() {
    const htmlBlocks = fundings.map(({ id, name, code, active }) => `
      <div class="col">
        <div class="card h-100 ${!active ? 'border-secondary' : ''}">
          <div class="card-body d-flex align-items-center">
            <div>
              <h5 class="mb-0">
                ${name} (${code})
                ${!active ? '<span class="badge bg-secondary ms-2">Inactive</span>' : ''}
              </h5>
              <div class="mt-2 d-flex gap-2 flex-wrap">
                <button class="btn btn-sm btn-danger btn-edit-funding" data-id="${id}">
                  Modify funding data
                </button>
                <button class="btn btn-sm btn-outline-secondary btn-delete-funding" data-id="${id}">
                  ❌ Delete
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    `);

    container.innerHTML = htmlBlocks.join('');
  }

  async function loadFundings() {
    try {
      const res = await fetch("/module/proxy/module4/api?action=allFundings");

      if (!res.ok) {
        throw new Error("Failed to fetch fundings");
      }

      fundings = await res.json();
      renderCards();
    } catch (err) {
      console.error("Error loading fundings:", err);
      container.innerHTML = `
        <div class="col-12">
          <div class="alert alert-danger">Error loading fundings.</div>
        </div>
      `;
    }
  }

  addFundingBtn.addEventListener('click', () => {
    currentEditingId = null;
    renderForm();
    modal.show();
  });

  container.addEventListener('click', async (e) => {
    const editBtn = e.target.closest('.btn-edit-funding');
    const deleteBtn = e.target.closest('.btn-delete-funding');
    const toggleBtn = e.target.closest('.btn-toggle-funding');

    if (editBtn) {
      const fundingId = editBtn.dataset.id;
      currentEditingId = fundingId;

      const funding = fundings.find(f => String(f.id) === String(fundingId));
      if (!funding) {
        alert("Funding not found.");
        return;
      }

      renderForm(funding);
      modal.show();
      return;
    }

    if (deleteBtn) {
      const fundingId = deleteBtn.dataset.id;

      if (!confirm("Are you sure you want to delete this funding? This action cannot be undone and will remove it from all associated contracts.")) {
        return;
      }

      try {
        const res = await fetch("/module/proxy/module4/api?action=deleteFunding", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ id: fundingId })
        });

        if (!res.ok) {
          throw new Error("Failed to delete funding");
        }

        const result = await res.json();
        console.log("Deleted:", result);

        await loadFundings();
      } catch (err) {
        console.error("Error deleting funding:", err);
        alert("Error deleting funding");
      }

      return;
    }

    if (toggleBtn) {
      const fundingId = toggleBtn.dataset.id;
      const funding = fundings.find(f => String(f.id) === String(fundingId));

      if (!funding) {
        alert("Funding not found.");
        return;
      }

      try {
        const res = await fetch("/module/proxy/module4/api?action=saveFunding", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            id: funding.id,
            name: funding.name,
            code: funding.code,
            active: !funding.active
          })
        });

        if (!res.ok) {
          throw new Error("Failed to update funding status");
        }

        const result = await res.json();
        console.log("Status updated:", result);

        await loadFundings();
      } catch (err) {
        console.error("Error updating funding status:", err);
        alert("Error updating funding status");
      }
    }
  });

  fundingForm.addEventListener("submit", async (e) => {
    e.preventDefault();

    const name = document.getElementById("fundingName").value.trim();
    const code = document.getElementById("fundingCode").value.trim();
    const active = document.getElementById("fundingActive").checked;

    if (!name || !code) {
      alert("All fields are required.");
      return;
    }

    const payload = {
      id: currentEditingId,
      name,
      code,
      active
    };

    try {
      const res = await fetch("/module/proxy/module4/api?action=saveFunding", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });

      if (!res.ok) {
        throw new Error("Failed to save funding");
      }

      const result = await res.json();
      console.log("Saved:", result);

      modal.hide();
      await loadFundings();
    } catch (err) {
      console.error("Error saving funding:", err);
      alert("Error saving funding");
    }
  });

  await loadFundings();
});