export function renderTravelLoading(select, feedback) {
  if (!select) return;

  select.innerHTML = `
    <option value="">Loading available travels...</option>
  `;

  select.disabled = true;

  if (feedback) {
    feedback.textContent = "Loading travels without service commission...";
    feedback.classList.remove("text-danger");
    feedback.classList.add("text-muted");
  }
}

export function renderTravelOptions(select, feedback, travels) {
  if (!select) return;

  select.innerHTML = "";

  const defaultOption = document.createElement("option");
  defaultOption.value = "";
  defaultOption.textContent = "Select travel...";
  select.appendChild(defaultOption);

  if (!Array.isArray(travels) || travels.length === 0) {
    select.disabled = true;

    const emptyOption = document.createElement("option");
    emptyOption.value = "";
    emptyOption.textContent = "No available travels found";
    select.appendChild(emptyOption);

    if (feedback) {
      feedback.textContent = "There are no Budgeting travels available without a service commission.";
      feedback.classList.remove("text-danger");
      feedback.classList.add("text-muted");
    }

    return;
  }

  travels.forEach((travel) => {
    const option = document.createElement("option");

    option.value = travel.id;

    option.textContent = formatTravelOption(travel);

    option.dataset.code = travel.code || "";
    option.dataset.destination = travel.destination || "";
    option.dataset.purpose = travel.purpose || "";
    option.dataset.startDate = travel.startDate || "";
    option.dataset.endDate = travel.endDate || "";
    option.dataset.attendanceCertificateUploaded = getAttendanceCertificateUploaded(travel) ? "1" : "0";

    select.appendChild(option);
  });

  select.disabled = false;

  if (feedback) {
    feedback.textContent = "Only travels without an associated service commission are shown.";
    feedback.classList.remove("text-danger");
    feedback.classList.add("text-muted");
  }
}

export function renderTravelError(select, feedback) {
  if (select) {
    select.innerHTML = `
      <option value="">Could not load travels</option>
    `;
    select.disabled = true;
  }

  if (feedback) {
    feedback.textContent = "There was an error loading available travels. Please try again later.";
    feedback.classList.remove("text-muted");
    feedback.classList.add("text-danger");
  }
}

function isUploaded(value) {
  return value === true ||
    value === 1 ||
    value === "1" ||
    String(value).toLowerCase() === "yes" ||
    String(value).toLowerCase() === "true";
}

function getAttendanceCertificateUploaded(travel) {
  return [
    travel.attendanceCertificateUploaded,
    travel.attendancecertificateuploaded,
    travel.attendance_certificate_uploaded,
    travel.file_uploaded
  ].some(isUploaded);
}

function formatTravelOption(travel) {
  const destination = travel.destination || "Unknown destination";
  const purpose = travel.purpose || "No purpose";
  const date = formatDate(travel.startDate);

  if (travel.code) {
    return `${travel.code} · ${destination} · ${purpose} · ${date}`;
  }

  return `${destination} · ${purpose} · ${date}`;
}

function formatDate(dateValue) {
  if (!dateValue) return "No date";

  const date = new Date(dateValue);

  if (Number.isNaN(date.getTime())) {
    return dateValue;
  }

  return date.toLocaleDateString("en-GB");
}
