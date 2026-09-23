import { createCommission, fetchProjects } from "./travelApi.js";
import { resetExpenses } from "./expenses.js";
import { resetTravelMode } from "./travelMode.js";
import {
  selectedTravelHasAttendanceCertificate,
  showAttendanceCertificateRequiredModal
} from "./attendanceCertificateGate.js";

const ATTENDANCE_CERTIFICATE_REQUIRED_MESSAGE =
  "Upload the attendance certificate before creating a service commission for this travel.";

export function initCommissionForm() {
  const form = document.getElementById("serviceCommissionForm");
  if (!form) return;

  loadProjectOptions();

  form.addEventListener("submit", async (event) => {
    event.preventDefault();

    const validationError = validateCommissionForm(form);
    if (validationError) {
      if (validationError !== ATTENDANCE_CERTIFICATE_REQUIRED_MESSAGE) {
        alert(validationError);
      }
      return;
    }

    const submitBtn = form.querySelector("button[type='submit']");
    submitBtn.disabled = true;
    submitBtn.textContent = "Submitting...";

    try {
      const formData = new FormData(form);
      const response = await createCommission(formData);

      alert("Service commission submitted successfully.");
      form.reset();
      clearFormValidation(form);
      resetExpenses();
      await resetTravelMode();
      await loadProjectOptions();

      window.dispatchEvent(new CustomEvent("commission:created", {
        detail: response
      }));
    } catch (error) {
      console.error(error);
      alert("There was an error submitting the service commission.");
    } finally {
      submitBtn.disabled = false;
      submitBtn.textContent = "Submit expense";
    }
  });
}

export function lockFutureTravelDates() {
  const today = new Date().toISOString().split("T")[0];

  const startDateInput = document.getElementById("startDate");
  const endDateInput = document.getElementById("endDate");

  if (startDateInput) {
    startDateInput.max = today;

    startDateInput.addEventListener("change", () => {
      if (startDateInput.value && startDateInput.value > today) {
        alert("Travel start date cannot be later than today.");
        startDateInput.value = "";
      }

      if (endDateInput && endDateInput.value && startDateInput.value && endDateInput.value < startDateInput.value) {
        alert("Travel end date cannot be earlier than travel start date.");
        endDateInput.value = "";
      }
    });
  }

  if (endDateInput) {
    endDateInput.max = today;

    endDateInput.addEventListener("change", () => {
      if (endDateInput.value && endDateInput.value > today) {
        alert("Travel end date cannot be later than today.");
        endDateInput.value = "";
      }

      if (startDateInput && startDateInput.value && endDateInput.value && endDateInput.value < startDateInput.value) {
        alert("Travel end date cannot be earlier than travel start date.");
        endDateInput.value = "";
      }
    });
  }
}

function clearFormValidation(form) {
  form.querySelectorAll(".is-invalid").forEach((field) => {
    field.classList.remove("is-invalid");
  });

  form.querySelectorAll(".selected-files-list").forEach((list) => {
    list.innerHTML = "";
  });

  form.querySelectorAll("[data-server-files]").forEach((container) => {
    container.innerHTML = "";
  });
}

export async function loadProjectOptions() {
  const select = document.querySelector("select[name='travel_project']");
  if (!select) return;

  select.innerHTML = `<option value="">Loading projects...</option>`;

  try {
    const projects = await fetchProjects();

    if (!Array.isArray(projects) || projects.length === 0) {
      select.innerHTML = `<option value="">No active projects available</option>`;
      return;
    }

    select.innerHTML = `
      <option value="">Select...</option>
      ${projects.map((project) => {
        const id = project.id ?? "";
        const name = project.short_name ?? project.name ?? project.id ?? "";
        return `<option value="${escapeHtml(String(id))}">${escapeHtml(String(name))}</option>`;
      }).join("")}
    `;
  } catch (error) {
    console.error(error);
    select.innerHTML = `<option value="">Could not load projects</option>`;
  }
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function validateCommissionForm(form) {
  const travelError = validateTravelFields(form);
  if (travelError) return travelError;

  const expenses = form.querySelectorAll(".expense-item");

  if (!expenses.length) {
    return "Add at least one expense.";
  }

  for (const [index, expense] of expenses.entries()) {
    const number = index + 1;
    const type = expense.querySelector(".expense-type-select")?.value;

    if (!type) {
      return `Select an expense type for expense ${number}.`;
    }

    const description = expense.querySelector("textarea[name*='[description]']");
    if (!String(description?.value || "").trim()) {
      description?.classList.add("is-invalid");
      return `Enter a description for expense ${number}.`;
    }
    description.classList.remove("is-invalid");

    const expenseError = type === "per_diem"
      ? validatePerDiemExpense(expense, number)
      : type === "mileage"
        ? validateMileageExpense(expense, number)
        : validateStandardExpense(expense, number);

    if (expenseError) return expenseError;
  }

  return "";
}

function isPdfFile(file) {
  return (
    file &&
    file.type === "application/pdf" &&
    file.name.toLowerCase().endsWith(".pdf")
  );
}

function validateTravelFields(form) {
  const source = form.querySelector("input[name='travelSource']:checked")?.value;

  if (source === "existing") {
    const budgetingTravel = form.querySelector("#budgetingTravel");

    if (!String(budgetingTravel?.value || "").trim()) {
      budgetingTravel?.classList.add("is-invalid");
      return "Select a Budgeting travel request.";
    }

    if (!selectedTravelHasAttendanceCertificate(budgetingTravel)) {
      budgetingTravel?.classList.add("is-invalid");
      showAttendanceCertificateRequiredModal();
      return ATTENDANCE_CERTIFICATE_REQUIRED_MESSAGE;
    }

    budgetingTravel.classList.remove("is-invalid");
    return "";
  }

  const requiredNewTravelFields = [
    ["input[name='travel_from_where']", "From where"],
    ["input[name='travel_to_where']", "To where"],
    ["input[name='destination']", "Destination"],
    ["input[name='travelPurpose']", "Purpose of travel"],
    ["input[name='startDate']", "Travel start date"],
    ["input[name='endDate']", "Travel end date"],
    ["select[name='travel_project']", "Project"]
  ];

  for (const [selector, label] of requiredNewTravelFields) {
    const field = form.querySelector(selector);

    if (!String(field?.value || "").trim()) {
      field?.classList.add("is-invalid");
      return `Complete the required field: ${label}.`;
    }

    field.classList.remove("is-invalid");
  }

  const attendanceCertificate = form.querySelector("input[name='travel_attendance_certificate']");
  if (!attendanceCertificate?.files?.length) {
    attendanceCertificate?.classList.add("is-invalid");
    return "Upload the attendance certificate.";
  }
  const invalidAttendanceCertificates = Array.from(attendanceCertificate.files)
    .filter((file) => !isPdfFile(file));

  if (invalidAttendanceCertificates.length > 0) {
    attendanceCertificate.classList.add("is-invalid");
    return "Only PDF files are allowed for attendance certificates.";
  }

  attendanceCertificate.classList.remove("is-invalid");

  const startDate = form.querySelector("input[name='startDate']")?.value;
  const endDate = form.querySelector("input[name='endDate']")?.value;

  if (startDate && endDate && new Date(endDate) < new Date(startDate)) {
    return "Travel end date cannot be before travel start date.";
  }

  return "";
}

function validatePerDiemExpense(expense, number) {
  const departureDate = expense.querySelector("input[id^='departureDate_']");
  const departureTime = expense.querySelector("input[id^='departureTime_']");
  const returnDate = expense.querySelector("input[id^='returnDate_']");
  const returnTime = expense.querySelector("input[id^='returnTime_']");

  const requiredFields = [
    [departureDate, "departure date"],
    [departureTime, "departure time"],
    [returnDate, "return date"],
    [returnTime, "return time"]
  ];

  for (const [field, label] of requiredFields) {
    if (!String(field?.value || "").trim()) {
      field?.classList.add("is-invalid");
      return `Enter ${label} for diet expense ${number}.`;
    }

    field.classList.remove("is-invalid");
  }

  const departure = new Date(`${departureDate.value}T${departureTime.value}`);
  const arrival = new Date(`${returnDate.value}T${returnTime.value}`);

  if (
    Number.isNaN(departure.getTime()) ||
    Number.isNaN(arrival.getTime()) ||
    arrival <= departure
  ) {
    return `The return date/time must be after the departure date/time for diet expense ${number}.`;
  }

  clearStandardFileValidation(expense);
  clearMileageFileValidation(expense);

  return "";
}

function validateMileageExpense(expense, number) {
  const distanceError = validateMileageDistance(expense, number);
  if (distanceError) return distanceError;

  const mileageProofInput = expense.querySelector(".mileage-proof-input");
  if (!mileageProofInput?.files?.length) {
    mileageProofInput?.classList.add("is-invalid");
    return `Upload the mileage screenshot for expense ${number}.`;
  }

  mileageProofInput.classList.remove("is-invalid");
  clearStandardFileValidation(expense);

  return "";
}

function validateMileageDistance(expense, number) {
  const distanceInput = expense.querySelector(".amount-input");
  const distanceValue = normalizeAmount(distanceInput?.value);
  const distance = Number(distanceValue);

  if (!isValidAmount(distanceValue) || !distance || distance <= 0) {
    distanceInput?.classList.add("is-invalid");
    return `Enter a valid distance in km for mileage expense ${number}.`;
  }

  distanceInput.value = distanceValue;
  distanceInput.classList.remove("is-invalid");

  return "";
}

function validateStandardExpense(expense, number) {
  const amountError = validateExpenseAmount(expense, number);
  if (amountError) return amountError;

  const type = expense.querySelector(".expense-type-select")?.value;
  if (type === "transport") {
    const transportMethod = expense.querySelector(".transport-method-input");
    if (!String(transportMethod?.value || "").trim()) {
      transportMethod?.classList.add("is-invalid");
      return `Enter the means of transport for expense ${number}.`;
    }
    transportMethod.classList.remove("is-invalid");
  }

  const receiptInput = expense.querySelector(".receipt-input");
  const paymentProofInput = expense.querySelector(".payment-proof-input");

  if (!receiptInput?.files?.length) {
    receiptInput?.classList.add("is-invalid");
    return `Upload the receipt/invoice for expense ${number}.`;
  }

  receiptInput.classList.remove("is-invalid");

  if (!paymentProofInput?.files?.length) {
    paymentProofInput?.classList.add("is-invalid");
    return `Upload the payment proof for expense ${number}.`;
  }

  paymentProofInput.classList.remove("is-invalid");
  clearMileageFileValidation(expense);

  return "";
}

function validateExpenseAmount(expense, number) {
  const amountInput = expense.querySelector(".amount-input");
  const amountValue = normalizeAmount(amountInput?.value);
  const amount = Number(amountValue);

  if (!isValidAmount(amountValue) || !amount || amount <= 0) {
    amountInput?.classList.add("is-invalid");
    return `Enter a valid amount for expense ${number}.`;
  }

  amountInput.value = amountValue;
  amountInput.classList.remove("is-invalid");

  return "";
}

function clearStandardFileValidation(expense) {
  expense.querySelectorAll(".receipt-input, .payment-proof-input").forEach((input) => {
    input.classList.remove("is-invalid");
  });
}

function clearMileageFileValidation(expense) {
  expense.querySelectorAll(".mileage-proof-input").forEach((input) => {
    input.classList.remove("is-invalid");
  });
}

function normalizeAmount(value) {
  return String(value || "")
    .trim()
    .replace(",", ".");
}

function isValidAmount(value) {
  return /^\d+(\.\d{1,2})?$/.test(value);
}
