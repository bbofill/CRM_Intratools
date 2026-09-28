//////////////////////////////////////////////////////////////////////////////////////////////////
//               Projects hub: formats project members edition and visualization                //
//////////////////////////////////////////////////////////////////////////////////////////////////

window.addEventListener('load', async () => {
  const params = new URLSearchParams(window.location.search);
  const projectId = params.get("id");

  if (!projectId) return;

  // Update title
  const titleEl = document.getElementById("projectHubTitle");
  if (titleEl) {
    titleEl.textContent = `${projectId} Hub`;
  }

  const container = document.getElementById("user-list");
  if (!container) return;

  const searchInput = document.getElementById("memberSearchInput");
  const statusFilter = document.getElementById("projectMemberStatusFilter");
  const clearFiltersBtn = document.getElementById("clearMemberFiltersBtn");
  const resultSummary = document.getElementById("memberResultSummary");
  let members = [];

  const escapeHtml = (value) => String(value ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");

  const normalizeDate = (value) => {
    const raw = String(value ?? "").trim();
    if (!raw) return null;

    const date = new Date(raw.slice(0, 10));
    return Number.isNaN(date.getTime()) ? null : date;
  };

  const todayOnly = () => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    return today;
  };

  const formatDate = (value) => {
    const raw = String(value ?? "").slice(0, 10);
    if (!raw) return "No date";

    const [year, month, day] = raw.split("-");
    return year && month && day ? `${day}/${month}/${year}` : raw;
  };

  const getRelationshipStatus = (member) => {
    const start = normalizeDate(member.start_date);
    const end = normalizeDate(member.end_date);
    const today = todayOnly();

    if (!start && !end) {
      return {
        key: "no-dates",
        label: "No dates",
        badgeClass: "bg-secondary",
        cardClass: "project-member-card-no-dates"
      };
    }

    if (end && end < today) {
      return {
        key: "ended",
        label: "Ended",
        badgeClass: "bg-secondary",
        cardClass: "project-member-card-ended"
      };
    }

    if (start && start > today) {
      return {
        key: "upcoming",
        label: "Upcoming",
        badgeClass: "bg-info text-dark",
        cardClass: "project-member-card-upcoming"
      };
    }

    return {
      key: "active",
      label: "Active",
      badgeClass: "bg-success",
      cardClass: "project-member-card-active"
    };
  };

  const getMemberName = (member) => [
    member.name,
    member.surname,
    member.secondSurname
  ].filter(Boolean).join(" ");

  const getSearchText = (member) => [
    member.id,
    member.name,
    member.surname,
    member.secondSurname,
    member.role,
    member.start_date,
    member.end_date
  ].map(value => String(value ?? "").toLowerCase()).join(" ");

  const loadMemberImage = async (picturePath) => {
    if (!picturePath) return "/assets/images/default_profile.png";

    try {
      const imgRes = await fetch(`/module/proxy/module4/Uploads?file=${encodeURIComponent(picturePath)}`);
      if (imgRes.ok) {
        const blob = await imgRes.blob();
        return URL.createObjectURL(blob);
      }
    } catch (err) {
      console.warn("Error loading image:", err);
    }

    return "/assets/images/default_profile.png";
  };

  const renderMembers = () => {
    const query = searchInput?.value.trim().toLowerCase() ?? "";
    const selectedStatus = statusFilter?.value ?? "all";

    const filteredMembers = members.filter(member => {
      const status = getRelationshipStatus(member);
      const matchesStatus = selectedStatus === "all" || status.key === selectedStatus;
      const matchesSearch = !query || getSearchText(member).includes(query);
      return matchesStatus && matchesSearch;
    });

    if (resultSummary) {
      resultSummary.textContent = `${filteredMembers.length} of ${members.length} members shown`;
    }

    if (!filteredMembers.length) {
      container.innerHTML = `<div class="col-12"><div class="alert alert-warning mb-0">No members match the current search.</div></div>`;
      return;
    }

    container.innerHTML = filteredMembers.map(member => {
      const {
        id: people_id,
        start_date,
        end_date,
        role,
        img
      } = member;
      const status = getRelationshipStatus(member);
      const fullName = getMemberName(member) || "Unnamed member";

      return `
        <div class="col">
          <div class="card h-100 shadow-sm project-member-card ${status.cardClass}">
            <div class="card-body">
              <div class="d-flex align-items-start gap-3">
                <img src="${img}" class="project-member-avatar" alt="Profile picture">
                <div class="flex-grow-1 min-w-0">
                  <div class="d-flex justify-content-between align-items-start gap-2">
                    <h5 class="mb-1 project-member-name">${escapeHtml(fullName)}</h5>
                    <span class="badge ${status.badgeClass}">${status.label}</span>
                  </div>
                  <div class="project-meta mt-2">
                    <span>${escapeHtml(role || "No role")}</span>
                    <span>From ${formatDate(start_date)}</span>
                    <span>Until ${formatDate(end_date)}</span>
                  </div>
                </div>
              </div>
              <div class="mt-3 d-flex gap-2 flex-wrap justify-content-end">
                <button
                  class="btn btn-sm btn-outline-primary edit-project-info-btn"
                  data-people-id="${escapeHtml(people_id)}"
                  data-project-id="${escapeHtml(projectId)}"
                  data-start="${escapeHtml(start_date || '')}"
                  data-end="${escapeHtml(end_date || '')}"
                  data-role-value="${escapeHtml(role || '')}">
                  Edit member info
                </button>
                <button
                  class="btn btn-sm btn-outline-danger delete-project-member-btn"
                  data-people-id="${escapeHtml(people_id)}"
                  data-project-id="${escapeHtml(projectId)}">
                  Delete member
                </button>
              </div>
            </div>
          </div>
        </div>`;
    }).join('');
  };

  const loadMembers = async () => {
    const res = await fetch(`/module/proxy/module4/api?action=getProjectMembers&id=${encodeURIComponent(projectId)}`);
    const rawMembers = await res.json();
    const enrichedMembers = await Promise.all(rawMembers.map(async member => ({
      ...member,
      img: await loadMemberImage(member.picture_path)
    })));

    const statusOrder = { active: 0, upcoming: 1, "no-dates": 2, ended: 3 };
    members = enrichedMembers.sort((a, b) => {
      const statusDiff = statusOrder[getRelationshipStatus(a).key] - statusOrder[getRelationshipStatus(b).key];
      if (statusDiff !== 0) return statusDiff;
      return getMemberName(a).localeCompare(getMemberName(b));
    });

    renderMembers();
  };

  await loadMembers();

  searchInput?.addEventListener("input", renderMembers);
  statusFilter?.addEventListener("change", renderMembers);
  clearFiltersBtn?.addEventListener("click", () => {
    if (searchInput) searchInput.value = "";
    if (statusFilter) statusFilter.value = "all";
    renderMembers();
  });

  // Open edit modal
  document.addEventListener("click", (e) => {
    const btn = e.target.closest(".edit-project-info-btn");
    if (!btn) return;

    document.getElementById("projectUserId").value = btn.dataset.peopleId;
    document.getElementById("projectId").value = btn.dataset.projectId;
    document.getElementById("projectStartDate").value = btn.dataset.start?.slice(0, 10) || "";
    document.getElementById("projectEndDate").value = btn.dataset.end?.slice(0, 10) || "";
    document.getElementById("projectRole").value = btn.dataset.roleValue || "";

    new bootstrap.Modal(document.getElementById("editProjectInfoModal")).show();
  });

  let pendingDelete = null;

  // Open delete modal
  document.addEventListener("click", (e) => {
    const btn = e.target.closest(".delete-project-member-btn");
    if (!btn) return;

    pendingDelete = {
      people_id: btn.dataset.peopleId,
      project_id: btn.dataset.projectId
    };

    new bootstrap.Modal(document.getElementById("deleteProjectMemberModal")).show();
  });

  // Save edited member
  const projectInfoForm = document.getElementById("projectInfoForm");
  if (projectInfoForm) {
    projectInfoForm.addEventListener("submit", async (e) => {
      e.preventDefault();

      const formData = new FormData();
      formData.append("people_id", document.getElementById("projectUserId").value);
      formData.append("project_id", document.getElementById("projectId").value);
      formData.append("start_date", document.getElementById("projectStartDate").value);
      formData.append("end_date", document.getElementById("projectEndDate").value);
      formData.append("role", document.getElementById("projectRole").value);

      try {
        const res = await fetch("/module/proxy/module4/api?action=saveProjectMember", {
          method: "POST",
          body: formData
        });

        const result = await res.json();

        if (result.status === "ok") {
          location.reload();
        } else {
          alert("Error saving project member info");
        }
      } catch (err) {
        console.error("Error saving project member info:", err);
        alert("Error saving project member info");
      }
    });
  }

  // Open add member modal
  const openAddBtn = document.getElementById("openAddUserModalBtn");
  if (openAddBtn) {
    openAddBtn.addEventListener("click", async () => {
      const currentProjectId = new URLSearchParams(window.location.search).get("id");

      const res = await fetch(`/module/proxy/module4/api?action=getAvailableUsersForProject&id=${encodeURIComponent(currentProjectId)}`);
      const users = await res.json();

      const datalist = document.getElementById("userOptions");
      datalist.innerHTML = users.map(u => `
        <option
          data-id="${u.id}"
          data-name="${u.name}"
          data-surname="${u.surname}"
          value="${u.name} ${u.surname}">
        </option>
      `).join("");

      const addForm = document.getElementById("addUserToProjectForm");
      addForm.reset();

      document.getElementById("newUserId").value = "";
      document.getElementById("newUserName").value = "";
      document.getElementById("newUserSurname").value = "";

      new bootstrap.Modal(document.getElementById("addUserToProjectModal")).show();
    });
  }

  // Sync datalist hidden fields
  const newUserInput = document.getElementById("newUserInput");
  if (newUserInput) {
    newUserInput.addEventListener("input", () => {
      const datalist = document.getElementById("userOptions");
      const option = [...datalist.options].find(o => o.value === newUserInput.value);

      if (option) {
        document.getElementById("newUserId").value = option.dataset.id;
        document.getElementById("newUserName").value = option.dataset.name;
        document.getElementById("newUserSurname").value = option.dataset.surname;
        newUserInput.classList.remove("is-invalid");
      } else {
        document.getElementById("newUserId").value = "";
        document.getElementById("newUserName").value = "";
        document.getElementById("newUserSurname").value = "";
        newUserInput.classList.add("is-invalid");
      }
    });
  }

  // Save new member
  const addUserToProjectForm = document.getElementById("addUserToProjectForm");
  if (addUserToProjectForm) {
    addUserToProjectForm.addEventListener("submit", async (e) => {
      e.preventDefault();

      const currentProjectId = new URLSearchParams(window.location.search).get("id");
      const peopleId = document.getElementById("newUserId").value;
      const start = document.getElementById("newProjectStartDate").value;
      const end = document.getElementById("newProjectEndDate").value;
      const role = document.getElementById("newProjectRole").value;

      if (!peopleId) {
        document.getElementById("newUserInput").classList.add("is-invalid");
        return;
      }

      if (!role) {
        document.getElementById("newProjectRole").classList.add("is-invalid");
        return;
      } else {
        document.getElementById("newProjectRole").classList.remove("is-invalid");
      }

      const formData = new FormData();
      formData.append("people_id", peopleId);
      formData.append("project_id", currentProjectId);
      formData.append("role", role);
      formData.append("start_date", start);
      formData.append("end_date", end);

      try {
        const res = await fetch("/module/proxy/module4/api?action=addProjectMember", {
          method: "POST",
          body: formData
        });

        const result = await res.json();

        if (result.status === "ok") {
          location.reload();
        } else {
          alert("Error adding user");
        }
      } catch (err) {
        console.error("Error adding user:", err);
        alert("Error adding user");
      }
    });
  }

  // Create delete modal if not present
  if (!document.getElementById("deleteProjectMemberModal")) {
    document.body.insertAdjacentHTML("beforeend", `
      <div class="modal fade" id="deleteProjectMemberModal" tabindex="-1">
        <div class="modal-dialog modal-dialog-centered">
          <div class="modal-content">
            <div class="modal-header bg-danger text-white">
              <h5 class="modal-title">Remove Project Member</h5>
              <button class="btn-close" data-bs-dismiss="modal"></button>
            </div>

            <div class="modal-body">
              <p class="mb-0">Are you sure you want to delete this project member?</p>
            </div>

            <div class="modal-footer">
              <button class="btn btn-secondary" data-bs-dismiss="modal">No</button>
              <button id="confirmDeleteProjectMemberBtn" class="btn btn-danger">Yes, delete</button>
            </div>
          </div>
        </div>
      </div>
    `);
  }

  // Confirm delete
  const confirmDeleteBtn = document.getElementById("confirmDeleteProjectMemberBtn");
  if (confirmDeleteBtn) {
    confirmDeleteBtn.addEventListener("click", async () => {
      if (!pendingDelete) return;

      const formData = new FormData();
      formData.append("people_id", pendingDelete.people_id);
      formData.append("project_id", pendingDelete.project_id);

      try {
        const res = await fetch("/module/proxy/module4/api?action=deleteProjectMember", {
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
  }
});
