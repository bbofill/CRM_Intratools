const MODULE8_ATTENDANCE_CERTIFICATES_URL = "/module/gui/module8/?section=attendanceCertificates#attendanceCertificates";

export function selectedTravelHasAttendanceCertificate(select) {
  const option = select?.selectedOptions?.[0];
  if (!option || !option.value) return true;

  return option.dataset.attendanceCertificateUploaded === "1";
}

export function showAttendanceCertificateRequiredModal() {
  const modal = ensureAttendanceCertificateModal();
  const instance = bootstrap.Modal.getOrCreateInstance(modal, {
    backdrop: true,
    keyboard: true,
    focus: true
  });

  instance.show();
}

function ensureAttendanceCertificateModal() {
  const modalId = "serviceCommissionAttendanceCertificateModal";
  let modal = document.getElementById(modalId);

  if (modal) return modal;

  modal = document.createElement("div");
  modal.className = "modal fade";
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
            To create a service commission for this travel, you must first upload the attendance certificate.
          </p>
          <p class="mb-0">
            You can upload it from the <strong>Attendance certificates</strong> tab in Budgeting.
          </p>
        </div>

        <div class="modal-footer">
          <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">
            Close
          </button>
          <a class="btn btn-danger" href="${MODULE8_ATTENDANCE_CERTIFICATES_URL}">
            Go to Attendance certificates
          </a>
        </div>
      </div>
    </div>
  `;

  document.body.appendChild(modal);
  return modal;
}
