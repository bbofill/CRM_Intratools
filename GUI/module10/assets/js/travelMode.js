import { getTravelsWithoutCommission } from "./travelApi.js";
import {
  renderTravelLoading,
  renderTravelOptions,
  renderTravelError
} from "./travelSelectRenderer.js";
import {
  selectedTravelHasAttendanceCertificate,
  showAttendanceCertificateRequiredModal
} from "./attendanceCertificateGate.js";

let travelsAlreadyLoaded = false;
let travelModeUpdater = null;

export async function initTravelMode() {
  const useExistingTravel = document.getElementById("useExistingTravel");
  const createNewTravel = document.getElementById("createNewTravel");

  const existingTravelBlock = document.getElementById("existingTravelBlock");
  const newTravelBlock = document.getElementById("newTravelBlock");

  const budgetingTravelSelect = document.getElementById("budgetingTravel");
  const budgetingTravelFeedback = document.getElementById("budgetingTravelFeedback");

  if (
    !useExistingTravel ||
    !createNewTravel ||
    !existingTravelBlock ||
    !newTravelBlock
  ) {
    return;
  }

  async function updateTravelMode() {
    const isExisting = useExistingTravel.checked;

    existingTravelBlock.classList.toggle("d-none", !isExisting);
    newTravelBlock.classList.toggle("d-none", isExisting);

    updateTravelRequiredFields(isExisting);
    updateAttendanceCertificateGate();

    if (isExisting && !travelsAlreadyLoaded) {
      await loadAvailableTravels(budgetingTravelSelect, budgetingTravelFeedback);
      updateAttendanceCertificateGate();
    }
  }

  travelModeUpdater = updateTravelMode;

  useExistingTravel.addEventListener("change", updateTravelMode);
  createNewTravel.addEventListener("change", updateTravelMode);
  window.addEventListener("commission:created", async () => {
    travelsAlreadyLoaded = false;

    if (useExistingTravel.checked) {
      await loadAvailableTravels(budgetingTravelSelect, budgetingTravelFeedback);
      updateAttendanceCertificateGate();
    }
  });

  budgetingTravelSelect?.addEventListener("change", () => {
    const isMissingAttendanceCertificate = updateAttendanceCertificateGate();
    if (isMissingAttendanceCertificate) {
      showAttendanceCertificateRequiredModal();
    }
  });

  await updateTravelMode();

  function updateAttendanceCertificateGate() {
    const submitBtn = document.querySelector("#serviceCommissionForm button[type='submit']");
    const isMissingAttendanceCertificate = Boolean(
      useExistingTravel.checked &&
      budgetingTravelSelect?.value &&
      !selectedTravelHasAttendanceCertificate(budgetingTravelSelect)
    );

    budgetingTravelSelect?.classList.toggle("is-invalid", isMissingAttendanceCertificate);

    if (submitBtn) {
      submitBtn.disabled = isMissingAttendanceCertificate;
    }

    if (budgetingTravelFeedback && isMissingAttendanceCertificate) {
      budgetingTravelFeedback.textContent = "Attendance certificate required before creating a service commission for this travel.";
      budgetingTravelFeedback.classList.remove("text-muted");
      budgetingTravelFeedback.classList.add("text-danger");
    } else if (budgetingTravelFeedback && useExistingTravel.checked) {
      budgetingTravelFeedback.textContent = "Only travels without an associated service commission are shown.";
      budgetingTravelFeedback.classList.remove("text-danger");
      budgetingTravelFeedback.classList.add("text-muted");
    }

    return isMissingAttendanceCertificate;
  }
}

export async function resetTravelMode() {
  const useExistingTravel = document.getElementById("useExistingTravel");
  const createNewTravel = document.getElementById("createNewTravel");

  if (useExistingTravel) useExistingTravel.checked = true;
  if (createNewTravel) createNewTravel.checked = false;

  if (travelModeUpdater) {
    await travelModeUpdater();
    return;
  }

  document.getElementById("existingTravelBlock")?.classList.remove("d-none");
  document.getElementById("newTravelBlock")?.classList.add("d-none");
  updateTravelRequiredFields(true);
}

async function loadAvailableTravels(select, feedback) {
  try {
    travelsAlreadyLoaded = true;

    renderTravelLoading(select, feedback);

    const travels = await getTravelsWithoutCommission();

    renderTravelOptions(select, feedback, travels);
  } catch (error) {
    console.error(error);

    travelsAlreadyLoaded = false;

    renderTravelError(select, feedback);
  }
}

function updateTravelRequiredFields(isExisting) {
  const budgetingTravel = document.getElementById("budgetingTravel");

  const destination = document.getElementById("destination");
  const travelPurpose = document.getElementById("travelPurpose");
  const startDate = document.getElementById("startDate");
  const endDate = document.getElementById("endDate");
  const travelAttendanceCertificate = document.getElementById("travelAttendanceCertificate");

  if (budgetingTravel) {
    budgetingTravel.required = isExisting;
  }

  [destination, travelPurpose, startDate, endDate, travelAttendanceCertificate].forEach((field) => {
    if (field) {
      field.required = !isExisting;
    }
  });
}
