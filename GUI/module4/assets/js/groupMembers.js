//////////////////////////////////////////////////////////////////////////////////////////////////
//               Groups hub: formats group members edition and visualization                    //
//////////////////////////////////////////////////////////////////////////////////////////////////

window.addEventListener('load', async () => {
  const params = new URLSearchParams(window.location.search);
  const groupId = params.get("code");

  if (!groupId) return;

  // Update title
  document.getElementById("groupHubTitle").textContent = `${groupId} Hub`;

  const container = document.getElementById("user-list");

  // Fetch members
  const res = await fetch(`/module/proxy/module4/api?action=getGroupMembers&id=${groupId}`);
  const members = await res.json();

  if (!members.length) {
    container.innerHTML = `<div class="col"><div class="alert alert-warning">No users found for this group.</div></div>`;
  } else {

  const htmlBlocks = await Promise.all(members.map(async member => {
    const {
      id: people_id,
      name,
      surname,
      secondSurname,
      picture_path,
      start_date,
      end_date,
      ip
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
            <div class="flex-grow-1">
              <h5 class="mb-0">${name} ${surname ?? ""} ${secondSurname ?? ""}</h5>
              <p class="mb-0 small">
                From: ${start_date?.slice(0,10) || "—"} <br>
                Until: ${end_date?.slice(0,10) || "—"} <br>
                Role: ${ip === 1 || ip === "1" ? "Principal Investigator" : "Member"}
              </p>
            </div>
              <button class="btn btn-sm btn-outline-primary ms-auto edit-group-info-btn" 
                data-people-id="${people_id}" 
                data-group-code="${groupId}"
                data-start="${start_date || ''}" 
                data-end="${end_date || ''}"
                data-ip="${ip}">
                Edit member info
              </button>
              <button class="btn btn-sm btn-outline-danger ms-auto delete-member-info-btn" 
                data-people-id="${people_id}" 
                data-group-code="${groupId}">
                Delete member
              </button>
          </div>
        </div>
      </div>`;
  }));

  container.innerHTML = htmlBlocks.join('');
  }
  // Abrir modal para editar miembro
  document.addEventListener("click", (e) => {
    const btn = e.target.closest(".edit-group-info-btn");
    if (btn) {
      document.getElementById("groupUserId").value = btn.dataset.peopleId;
      document.getElementById("groupCode").value = btn.dataset.groupCode;
      document.getElementById("groupStartDate").value = btn.dataset.start?.slice(0,10) || "";
      document.getElementById("groupEndDate").value = btn.dataset.end?.slice(0,10) || "";
      document.getElementById("groupIp").checked = btn.dataset.ip === "1";

      new bootstrap.Modal(document.getElementById("editGroupInfoModal")).show();
    }
  });

  //eliminar miembro del grupo 
  document.addEventListener("click", (e) => {
    const btn = e.target.closest(".delete-member-info-btn");
    if (!btn) return;

    pendingDelete = {
      people_id: btn.dataset.peopleId,
      group_code: btn.dataset.groupCode
    };

    new bootstrap.Modal(document.getElementById("deleteGroupMemberModal")).show();
  });

  // Guardar miembro editado
  document.getElementById("groupInfoForm").addEventListener("submit", async (e) => {
    e.preventDefault();

    const formData = new FormData();
    formData.append("people_id", document.getElementById("groupUserId").value);
    formData.append("group_intern_code", document.getElementById("groupCode").value);
    formData.append("start_date", document.getElementById("groupStartDate").value);
    formData.append("end_date", document.getElementById("groupEndDate").value);
    formData.append("ip", document.getElementById("groupIp").checked ? 1 : 0);

    try {
      const res = await fetch("/module/proxy/module4/api?action=saveGroupMember", {
        method: "POST",
        body: formData
      });
      const result = await res.json();
      if (result.status === "ok") {
        location.reload();
      } else {
        alert("Error saving group info");
      }
    } catch (err) {
      console.error("Error saving group info:", err);
      alert("Error saving group info");
    }
  });

  // Abrir modal "Add User to Group"
  document.getElementById("openAddUserModalBtn").addEventListener("click", async () => {
    const groupId = new URLSearchParams(window.location.search).get("code");

    const res = await fetch(`/module/proxy/module4/api?action=getAvailableUsersForGroup&id=${groupId}`);
    const users = await res.json();
 console.log("Available users for group", groupId, ":", users);
    const datalist = document.getElementById("userOptions");
    datalist.innerHTML = users.map(u => 
      `<option 
        data-id="${u.id}" 
        data-name="${u.name}" 
        data-surname="${u.surname}" 
        value="${u.name} ${u.surname}"></option>`
    ).join("");

    // Reset form
    document.getElementById("addUserToGroupForm").reset();
    document.getElementById("newUserId").value = "";
    document.getElementById("newUserName").value = "";
    document.getElementById("newUserSurname").value = "";

    new bootstrap.Modal(document.getElementById("addUserToGroupModal")).show();
  });

  // Input datalist → sync campos ocultos
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

  // Guardar nuevo miembro
  document.getElementById("addUserToGroupForm").addEventListener("submit", async (e) => {
    e.preventDefault();

    const groupId = new URLSearchParams(window.location.search).get("code");
    const peopleId = document.getElementById("newUserId").value;
    const start = document.getElementById("newGroupStartDate").value;
    const end = document.getElementById("newGroupEndDate").value;
    const ip = document.getElementById("newGroupIp").checked ? 1 : 0;

    if (!peopleId) {
      document.getElementById("newUserInput").classList.add("is-invalid");
      return;
    }

    const formData = new FormData();
    formData.append("people_id", peopleId);
    formData.append("group_id", groupId);
    formData.append("ip", ip);
    formData.append("start_date", start);
    formData.append("end_date", end);

    const res = await fetch("/module/proxy/module4/api?action=addGroupMember", {
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

  if (!document.getElementById("deleteGroupMemberModal")) {
    document.body.insertAdjacentHTML("beforeend", `
      <div class="modal fade" id="deleteGroupMemberModal" tabindex="-1">
        <div class="modal-dialog modal-dialog-centered">
          <div class="modal-content">
            <div class="modal-header bg-danger text-white">
              <h5 class="modal-title">Remove Group Member</h5>
              <button class="btn-close" data-bs-dismiss="modal"></button>
            </div>

            <div class="modal-body">
              <p class="mb-0">Are you sure you want to delete this group member?</p>
            </div>

            <div class="modal-footer">
              <button class="btn btn-secondary" data-bs-dismiss="modal">No</button>
              <button id="confirmDeleteMemberBtn" class="btn btn-danger">Yes, delete</button>
            </div>
          </div>
        </div>
      </div>
    `);
  }

  let pendingDelete = null;

  document.getElementById("confirmDeleteMemberBtn").addEventListener("click", async () => {
  if (!pendingDelete) return;

  const formData = new FormData();
  formData.append("people_id", pendingDelete.people_id);
  formData.append("group_intern_code", pendingDelete.group_code);

  try {
    const res = await fetch("/module/proxy/module4/api?action=deleteGroupMember", {
      method: "POST",
      body: formData
    });

    const result = await res.json();

    if (result.status === "ok") {
      location.reload();
    } else {
      alert("Error deleting member.");
    }
  } catch (err) {
    console.error("Error:", err);
    alert("Error deleting member.");
  }
});


});
