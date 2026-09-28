//////////////////////////////////////////////////////////////////////////////////////////////////
//             Trainings hub: formats training members edition and visualization                //
//////////////////////////////////////////////////////////////////////////////////////////////////

document.addEventListener('DOMContentLoaded', async () => {
  const params = new URLSearchParams(window.location.search);
  const trainingName = params.get("name");
  const trainingId = params.get("id");

  if (!trainingId) return;

  // Update title
  document.getElementById("groupHubTitle").textContent = `${trainingName}`;

  const container = document.getElementById("user-list");

  // Fetch members
  const res = await fetch(`/module/proxy/module4/api?action=getTrainingMembers&id=${trainingId}`);
  const members = await res.json();

  if (!members.length) {
    container.innerHTML = `<div class="col"><div class="alert alert-warning">No users found for this training.</div></div>`;
  }else{

  const htmlBlocks = await Promise.all(members.map(async member => {
    const {
      id,
      name,
      surname,
      secondSurname,
      picture_path,
      date,
      diploma_path
    } = member;

    let img = "";
    if (picture_path) {
      try {
        const res = await fetch(`/module/proxy/module4/Uploads?file=${encodeURIComponent(picture_path)}`);
        if (res.ok) {
          const blob = await res.blob();
          img = URL.createObjectURL(blob);
        }
      } catch (err) {
        console.warn("Error loading image:", err);
      }
    }


    return `
      <div class="col">
        <div class="card h-100 shadow-sm">
          <div class="card-body d-flex align-items-center">
            <img src="${img}" class="rounded me-3" width="60" height="60" alt="Profile picture">
            <div>
              <h5 class="mb-0">${name} ${surname ?? ""} ${secondSurname ?? ""}</h5>
              <p class="mb-0 small">
                Completion date: ${date?.slice(0,10) || "—"} <br>
                Diploma: ${diploma_path ? `<a href="/module/proxy/module4/Uploads?file=${encodeURIComponent(diploma_path)}" target="_blank">View</a>` : "—"}
              </p>
            </div>
              <button class="btn btn-sm btn-outline-danger ms-auto edit-training-info-btn" 
                data-people-id="${id}" 
                data-training-id="${trainingId}"
                data-name="${name}"
                data-surname="${surname}"
                data-date="${date || ''}" 
                data-diploma="${diploma_path || ''}">
                Edit training info
              </button>
          </div>
        </div>
      </div>`;
  }));

  container.innerHTML = htmlBlocks.join('');
}

  document.addEventListener("click", (e) => {
    const btn = e.target.closest(".edit-training-info-btn");
    if (btn) {
      document.getElementById("name").value = btn.dataset.name;
      document.getElementById("surname").value = btn.dataset.surname;
      document.getElementById("trainingUserId").value = btn.dataset.peopleId;
      document.getElementById("trainingId").value = btn.dataset.trainingId;
      document.getElementById("trainingDate").value = btn.dataset.date?.slice(0,10) || "";
  
      const diplomaPreview = document.getElementById("trainingDiplomaPreview");
      diplomaPreview.innerHTML = btn.dataset.diploma
        ? `<a href="/module/proxy/module4/Uploads?file=${encodeURIComponent(btn.dataset.diploma)}" target="_blank">View current diploma</a>`
        : "";
  
      new bootstrap.Modal(document.getElementById("editTrainingInfoModal")).show();
    }
  });

  document.getElementById("trainingInfoForm").addEventListener("submit", async (e) => {
    e.preventDefault();
  
    const formData = new FormData();
    formData.append("name", document.getElementById("name").value);
    formData.append("surname", document.getElementById("surname").value);
    formData.append("people_id", document.getElementById("trainingUserId").value);
    formData.append("training_id", document.getElementById("trainingId").value);
    formData.append("date", document.getElementById("trainingDate").value);
  
    const file = document.getElementById("trainingDiploma").files[0];
    if (file) formData.append("diploma", file);
  
    try {
      const res = await fetch("/module/proxy/module4/api?action=saveTrainingMember", {
        method: "POST",
        body: formData
      });
      const result = await res.json();
      if (result.status === "ok") {
        location.reload(); // refrescar para ver cambios
      } else {
        alert("Error saving training info");
      }
    } catch (err) {
      console.error("Error saving training info:", err);
      alert("Error saving training info");
    }
  });
  
  // Abrir modal "Add User to Training"
  document.getElementById("openAddUserModalBtn").addEventListener("click", async () => {
    const trainingId = new URLSearchParams(window.location.search).get("id");

    // Cargar usuarios disponibles desde el backend
    const res = await fetch(`/module/proxy/module4/api?action=getAvailableUsers&id=${trainingId}`);
    const users = await res.json();

    const datalist = document.getElementById("userOptions");
    datalist.innerHTML = users.map(u => 
      `<option 
        data-id="${u.id}" 
        data-name="${u.name}" 
        data-surname="${u.surname}" 
        value="${u.name} ${u.surname}"></option>`
    ).join("");

    // Resetear el formulario
    document.getElementById("addUserToTrainingForm").reset();
    document.getElementById("newUserId").value = "";
    document.getElementById("newUserName").value = "";
    document.getElementById("newUserSurname").value = "";

    new bootstrap.Modal(document.getElementById("addUserToTrainingModal")).show();
  });

  // Sincronizar input/datalist → id + name + surname
  document.getElementById("newUserInput").addEventListener("input", () => {
    const input = document.getElementById("newUserInput");
    const datalist = document.getElementById("userOptions");
    const option = [...datalist.options].find(o => o.value === input.value);
    if (option) {
      document.getElementById("newUserId").value = option.dataset.id;
      document.getElementById("newUserName").value = option.dataset.name;
      document.getElementById("newUserSurname").value = option.dataset.surname;
      input.classList.remove("is-invalid");
    } else {
      document.getElementById("newUserId").value = "";
      document.getElementById("newUserName").value = "";
      document.getElementById("newUserSurname").value = "";
      input.classList.add("is-invalid");
    }
  });

  // Enviar formulario
  document.getElementById("addUserToTrainingForm").addEventListener("submit", async (e) => {
    e.preventDefault();

    const trainingId = new URLSearchParams(window.location.search).get("id");
    const peopleId = document.getElementById("newUserId").value;
    const name = document.getElementById("newUserName").value;
    const surname = document.getElementById("newUserSurname").value;
    const date = document.getElementById("newTrainingDate").value;
    const file = document.getElementById("newTrainingDiploma").files[0];

    if (!peopleId) {
      document.getElementById("newUserInput").classList.add("is-invalid");
      return;
    }

    const formData = new FormData();
    formData.append("people_id", peopleId);
    formData.append("name", name);
    formData.append("surname", surname);
    formData.append("training_id", trainingId);
    formData.append("date", date);
    if (file) formData.append("diploma", file);

    const res = await fetch("/module/proxy/module4/api?action=addTrainingMember", {
      method: "POST",
      body: formData
    });

    const result = await res.json();
    if (result.status === "ok") {
      location.reload();
    } else {
      alert("Error adding user");
    }
  });



  
});
