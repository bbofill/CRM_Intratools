//////////////////////////////////////////////////////////////////////////////////////////////////
//                  Trainings hub: formats trainings edition and visualization                  //
//////////////////////////////////////////////////////////////////////////////////////////////////

import { formOptions } from './formOptions.js?v=${window.APP_VERSION}';

document.addEventListener('DOMContentLoaded', async () => {
  const container = document.getElementById("training-list");
  let currentEditingId = null;

  const res = await fetch("/module/proxy/module4/api?action=allTrainings");
  const trainings = await res.json();

  // Render cards
  const htmlBlocks = trainings.map(({ id, category, name, hours }) => `
    <div class="col">
      <div class="card h-100">
        <div class="card-body d-flex align-items-center">
          <div>
            <h5 class="mb-0">${name} (${category}, ${hours} hour(s))</h5>
            <div class="mt-2 d-flex gap-2 flex-wrap">
              <button class="btn btn-sm btn-danger" data-id="${id}">
                Modify training data
              </button>
              <button class="btn btn-sm btn-outline-danger" data-id="${id}">
                Modify who has done the training
              </button>
              <button class="btn btn-sm btn-outline-secondary btn-delete-training" data-id="${id}">
                ❌ Delete
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>`
  );
  container.innerHTML = htmlBlocks.join('');

  // Add training
  document.getElementById('openAddTrainingModalBtn').addEventListener('click', () => {
    currentEditingId = null;
    const modal = new bootstrap.Modal(document.getElementById('trainingFormModal'));

    const categoryOptions = formOptions.trainingCategories.map(opt => {
      return `<option value="${opt}">${opt}</option>`;
    }).join('');

    document.getElementById('formFieldsContainer').innerHTML = `
      <div class="col-md-6">
        <label for="trainingName" class="form-label">Training name</label>
        <input type="text" class="form-control" id="trainingName" name="trainingName" value="">
      </div>
      <div class="col-md-6">
        <label for="trainingCategory" class="form-label">Category</label>
        <select class="form-select" id="trainingCategory" name="trainingCategory">
          <option value="">-- Select --</option>
          ${categoryOptions}
        </select>
      </div>
      <div class="col-md-6">
        <label for="trainingHours" class="form-label">Hours</label>
        <input type="number" class="form-control" id="trainingHours" name="trainingHours" value="">
      </div>
    `;

    modal.show();
  });

  // Edit training
  container.querySelectorAll('button.btn-danger').forEach(btn => {
    btn.addEventListener('click', async (e) => {
      const trainingId = e.target.dataset.id;
      currentEditingId = trainingId;
      const training = trainings.find(t => t.id == trainingId);

      const categoryOptions = formOptions.trainingCategories.map(opt => {
        const selected = opt.trim().toLowerCase() === training.category?.trim().toLowerCase() ? 'selected' : '';
        return `<option value="${opt}" ${selected}>${opt}</option>`;
      }).join('');

      const modal = new bootstrap.Modal(document.getElementById('trainingFormModal'));
      document.getElementById('formFieldsContainer').innerHTML = `
        <div class="col-md-6">
          <label for="trainingName" class="form-label">Training name</label>
          <input type="text" class="form-control" id="trainingName" name="trainingName" value="${training.name ?? ''}">
        </div>
        <div class="col-md-6">
          <label for="trainingCategory" class="form-label">Category</label>
          <select class="form-select" id="trainingCategory" name="trainingCategory">
            <option value="">-- Select --</option>
            ${categoryOptions}
          </select>
        </div>
        <div class="col-md-6">
          <label for="trainingHours" class="form-label">Hours</label>
          <input type="number" class="form-control" id="trainingHours" name="trainingHours" value="${training.hours ?? ''}">
        </div>
      `;

      modal.show();
    });
  });

  // Edit who has done it
  container.querySelectorAll('button.btn-outline-danger').forEach(btn => {
    btn.addEventListener('click', (e) => {
      const trainingId = e.target.dataset.id;
      const training = trainings.find(t => t.id == trainingId);
      window.location.href = `trainingMembers.html?id=${trainingId}&name=${encodeURIComponent(training.name)}`;

    });
  });

  // -------------------------------
  // Submit handler (add/edit save)
  // -------------------------------
  document.getElementById("trainingForm").addEventListener("submit", async (e) => {
    e.preventDefault();

    const name = document.getElementById("trainingName").value.trim();
    const category = document.getElementById("trainingCategory").value;
    const hours = document.getElementById("trainingHours").value;

    if (!name || !category || !hours) {
      alert("All fields are required.");
      return;
    }

    const payload = {
      id: currentEditingId,
      name,
      category,
      hours: parseInt(hours, 10)
    };

    try {
      const res = await fetch("/module/proxy/module4/api?action=saveTraining", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });
      const result = await res.json();
      console.log("Saved:", result);

      // Refresh page or reload trainings
      location.reload();
    } catch (err) {
      console.error("Error saving training:", err);
      alert("Error saving training");
    }
  });

  // -------------------------------
  // Delete handler 
  // -------------------------------
  container.querySelectorAll('button.btn-delete-training').forEach(btn => {
    btn.addEventListener('click', async (e) => {
      const trainingId = e.target.dataset.id;

      if (!confirm("Are you sure you want to delete this training? This action cannot be undone and will delete all information of workers who have done this training.")) {
        return;
      }

      try {
        const res = await fetch("/module/proxy/module4/api?action=deleteTraining", {
          method: "POST", 
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ id: trainingId })
        });

        if (!res.ok) {
          throw new Error("Failed to delete training");
        }

        const result = await res.json();
        console.log("Deleted:", result);

        // Recargar lista
        location.reload();
      } catch (err) {
        console.error("Error deleting training:", err);
        alert("Error deleting training");
      }
    });
  });

});
