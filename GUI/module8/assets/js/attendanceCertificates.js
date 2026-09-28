const API_BASE = "/module/proxy/module8/api";

let pendingAttendanceCertificates = [];

document.addEventListener("DOMContentLoaded", () => {
  initAttendanceCertificates();
});

async function initAttendanceCertificates() {
  await loadPendingAttendanceCertificates();

  const nav = document.getElementById("attendanceCertificatesNav");
  if (nav) {
    nav.addEventListener("click", async () => {
      await loadPendingAttendanceCertificates();
      renderAttendanceCertificatesTable();
    });
  }
}

async function loadPendingAttendanceCertificates() {
  try {
    const response = await fetch(`${API_BASE}?action=getPendingAttendanceCertificates`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      throw new Error("Error loading pending attendance certificates");
    }

    const data = await response.json();
    pendingAttendanceCertificates = Array.isArray(data.pending) ? data.pending : [];

    updateAttendanceCertificatesBadge();

    if (pendingAttendanceCertificates.length > 0) {
      showAttendanceCertificatePopupOnce();
    }

    renderAttendanceCertificatesTable();
  } catch (error) {
    console.error("Error loading pending attendance certificates:", error);
  }
}

function updateAttendanceCertificatesBadge() {
  const badge = document.getElementById("attendanceCertificatesBadge");
  if (!badge) return;

  const count = pendingAttendanceCertificates.length;

  if (count > 0) {
    badge.textContent = count;
    badge.classList.remove("d-none");
  } else {
    badge.textContent = "0";
    badge.classList.add("d-none");
  }
}

function renderAttendanceCertificatesTable() {
  const tbody = document.getElementById("attendanceCertificatesBody");
  if (!tbody) return;

  if (pendingAttendanceCertificates.length === 0) {
    tbody.innerHTML = `
      <tr>
        <td colspan="7" class="text-center text-muted">
          No pending attendance certificates.
        </td>
      </tr>
    `;
    return;
  }

  tbody.innerHTML = pendingAttendanceCertificates.map(item => `
    <tr>
      <td>${escapeHtml(item.internalId || item.combinedId || "")}</td>
      <td>${escapeHtml(item.projectName || "")}</td>
      <td>${escapeHtml(item.requestSummary || "")}</td>
      <td>${escapeHtml(item.endDate || "")}</td>
      <td>${escapeHtml(item.lastReminderAt || "-")}</td>
      <td>${escapeHtml(String(item.reminderCount ?? 0))}</td>
      <td>
        <button
          type="button"
          class="btn btn-sm btn-danger"
          data-upload-attendance-certificate="${escapeHtml(String(item.combinedId))}"
        >
          Upload certificate
        </button>
      </td>
    </tr>
  `).join("");

  tbody.querySelectorAll("[data-upload-attendance-certificate]").forEach(button => {
    button.addEventListener("click", () => {
      const combinedId = button.dataset.uploadAttendanceCertificate;
      openAttendanceCertificateUploadDialog(combinedId);
    });
  });
}

function showAttendanceCertificatePopupOnce() {
  const firstPending = pendingAttendanceCertificates[0];
  if (!firstPending) return;

  const modalId = "attendanceCertificateReminderModal";
  let modal = document.getElementById(modalId);

  if (!modal) {
    modal = document.createElement("div");
    modal.className = "modal fade attendance-certificate-modal";
    modal.id = modalId;
    modal.tabIndex = -1;
    modal.setAttribute("aria-hidden", "true");

    modal.innerHTML = `
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title text-danger">Attendance certificate required</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
          </div>

          <div class="modal-body">
            <p>
              You have one or more approved travel/registration requests whose end date has already passed.
            </p>
            <p class="mb-0">
              Please upload the attendance certificate from the
              <strong>Attendance certificates</strong> tab.
            </p>
          </div>

          <div class="modal-footer">
            <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">
              I’ll do it later
            </button>
            <button type="button" class="btn btn-danger" id="goToAttendanceCertificatesBtn">
              Upload now
            </button>
          </div>
        </div>
      </div>
    `;

    document.body.appendChild(modal);
  }

const instance = bootstrap.Modal.getOrCreateInstance(modal, {
  backdrop: true,
  keyboard: true,
  focus: true
});

instance.show();

  const goBtn = document.getElementById("goToAttendanceCertificatesBtn");
  if (goBtn) {
    goBtn.onclick = () => {
      instance.hide();
      goToAttendanceCertificatesSection();
    };
  }
}

function goToAttendanceCertificatesSection() {
  const nav = document.getElementById("attendanceCertificatesNav");
  if (nav) {
    nav.click();
    return;
  }

  document.querySelectorAll(".content-section").forEach(section => {
    section.classList.add("d-none");
  });

  const section = document.getElementById("section-attendanceCertificates");
  if (section) {
    section.classList.remove("d-none");
  }

  renderAttendanceCertificatesTable();
}

function openAttendanceCertificateUploadDialog(combinedId) {
  const input = document.createElement("input");
  input.type = "file";
  input.accept = ".pdf,application/pdf";
  input.style.display = "none";

  input.addEventListener("change", () => {
    const file = input.files && input.files[0];

    input.remove();

    if (!file) {
      return;
    }

    const fileName = (file.name || "").toLowerCase();

    if (!fileName.endsWith(".pdf")) {
      alert("Only PDF files are allowed for attendance certificates.");
      return;
    }

    showAttendanceCertificateConfirmModal(combinedId, file);
  });

  document.body.appendChild(input);
  input.click();
}

function showAttendanceCertificateConfirmModal(combinedId, file) {
  const modalId = "attendanceCertificateConfirmUploadModal";
  let modal = document.getElementById(modalId);

  if (!modal) {
    modal = document.createElement("div");
    modal.className = "modal fade";
    modal.id = modalId;
    modal.tabIndex = -1;
    modal.setAttribute("aria-hidden", "true");

    modal.innerHTML = `
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title text-danger">Confirm attendance certificate</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
          </div>

          <div class="modal-body">
            <p class="mb-2">
              Please confirm that this is the attendance certificate you want to upload:
            </p>

            <div class="border rounded p-3 bg-light">
              <div class="fw-semibold" id="attendanceCertificateSelectedFileName"></div>
              <div class="text-muted small" id="attendanceCertificateSelectedFileSize"></div>
            </div>

          </div>

          <div class="modal-footer">

            <button type="button" class="btn btn-outline-secondary" id="changeAttendanceCertificateBtn">
              Change file
            </button>

            <button type="button" class="btn btn-danger" id="confirmAttendanceCertificateUploadBtn">
              Upload certificate
            </button>
          </div>
        </div>
      </div>
    `;

    document.body.appendChild(modal);
  }

  const fileNameElement = modal.querySelector("#attendanceCertificateSelectedFileName");
  const fileSizeElement = modal.querySelector("#attendanceCertificateSelectedFileSize");

  if (fileNameElement) {
    fileNameElement.textContent = file.name || "Selected PDF";
  }

  if (fileSizeElement) {
    fileSizeElement.textContent = formatFileSize(file.size);
  }

  const instance = bootstrap.Modal.getOrCreateInstance(modal, {
    backdrop: true,
    keyboard: true,
    focus: true
  });

  const uploadBtn = modal.querySelector("#confirmAttendanceCertificateUploadBtn");
  const changeBtn = modal.querySelector("#changeAttendanceCertificateBtn");
  const removeBtn = modal.querySelector("#removeAttendanceCertificateBtn");

  if (uploadBtn) {
    uploadBtn.onclick = async () => {
      uploadBtn.disabled = true;

      if (changeBtn) {
        changeBtn.disabled = true;
      }

      try {
        await uploadAttendanceCertificate(combinedId, file);
        instance.hide();
      } finally {
        uploadBtn.disabled = false;

        if (changeBtn) {
          changeBtn.disabled = false;
        }
      }
    };
  }

  if (changeBtn) {
    changeBtn.onclick = () => {
      instance.hide();
      openAttendanceCertificateUploadDialog(combinedId);
    };
  }

  if (removeBtn) {
    removeBtn.onclick = () => {
      instance.hide();
    };
  }

  instance.show();
}

function formatFileSize(bytes) {
  const size = Number(bytes) || 0;

  if (size < 1024) {
    return `${size} B`;
  }

  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }

  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

async function uploadAttendanceCertificate(combinedId, file) {
  const formData = new FormData();
  formData.append("idCombined", combinedId);
  formData.append("attendance_certificate", file);

  try {
    const response = await fetch(`${API_BASE}?action=uploadAttendanceCertificate`, {
      method: "POST",
      body: formData,
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || "Error uploading attendance certificate");
    }

    await loadPendingAttendanceCertificates();

    alert("Attendance certificate uploaded successfully.");
  } catch (error) {
    console.error("Error uploading attendance certificate:", error);
    alert("Error uploading attendance certificate.");
  }
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}