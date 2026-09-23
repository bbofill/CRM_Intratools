const PER_DIEM_TYPE = "per_diem";
const ACCOMMODATION_TYPE = "accommodation";
const MILEAGE_TYPE = "mileage";
const TRANSPORT_TYPE = "transport";

export function initExpenses() {
  const expensesContainer = document.getElementById("expensesContainer");
  const addExpenseBtn = document.getElementById("addExpenseBtn");

  if (!expensesContainer || !addExpenseBtn) return;

  addExpenseBtn.addEventListener("click", () => {
    createExpenseItem(expensesContainer);
  });

  initTravelDateSync(expensesContainer);

  expensesContainer.addEventListener("change", (event) => {
    const expenseTypeSelect = event.target.closest(".expense-type-select");
    if (!expenseTypeSelect) return;

    const expenseItem = expenseTypeSelect.closest(".expense-item");
    if (!expenseItem) return;

    updateExpenseItemByType(expenseItem);
  });

  expensesContainer.addEventListener("input", (event) => {
    const perDiemControl = event.target.closest(".per-diem-input, .per-diem-breakfast-input, .per-diem-covered-meals-input, .per-diem-covered-dinners-input");
    if (!perDiemControl) return;

    const expenseItem = perDiemControl.closest(".expense-item");
    if (expenseItem) updatePerDiemBreakdown(expenseItem);
  });

  expensesContainer.addEventListener("change", (event) => {
    const perDiemControl = event.target.closest(".per-diem-input, .per-diem-breakfast-input, .per-diem-event-input");
    if (!perDiemControl) return;

    const expenseItem = perDiemControl.closest(".expense-item");
    if (expenseItem) {
      updatePerDiemEventFields(expenseItem);
      updatePerDiemBreakdown(expenseItem);
    }
  });

  expensesContainer.addEventListener("click", (event) => {
    const removeBtn = event.target.closest(".remove-expense-btn");
    if (!removeBtn) return;

    const expenseItem = removeBtn.closest(".expense-item");
    const allExpenses = expensesContainer.querySelectorAll(".expense-item");

    if (expenseItem && allExpenses.length > 1) {
      expenseItem.remove();
      refreshExpenseLabels(expensesContainer);
    }
  });

  refreshExpenseLabels(expensesContainer);
}

export function resetExpenses() {
  const expensesContainer = document.getElementById("expensesContainer");
  if (!expensesContainer) return;

  const expenseItems = [...expensesContainer.querySelectorAll(".expense-item")];
  const firstExpense = expenseItems[0];
  if (!firstExpense) return;

  expenseItems.slice(1).forEach((item) => item.remove());

  firstExpense.querySelectorAll("input, select, textarea").forEach((field) => {
    field.classList.remove("is-invalid");

    if (field.type === "file") {
      field.value = "";
      field.disabled = false;
      return;
    }

    if (field.type === "checkbox") {
      field.checked = false;
      field.disabled = false;
      return;
    }

    if (field.tagName === "SELECT") {
      field.selectedIndex = 0;
      field.disabled = false;
      return;
    }

    if (field.classList.contains("per-diem-covered-meals-input") || field.classList.contains("per-diem-covered-dinners-input")) {
      field.value = "0";
    } else {
      field.value = "";
    }
    field.disabled = false;
  });

  resetExpenseVisualState(firstExpense);
  refreshExpenseLabels(expensesContainer);
}

function resetExpenseVisualState(expenseItem) {
  const otherBlock = expenseItem.querySelector(".other-expense-block");
  const perDiemBlock = expenseItem.querySelector(".per-diem-block");
  const breakfastBlock = expenseItem.querySelector(".per-diem-breakfast-block");
  const eventCounts = expenseItem.querySelector(".per-diem-event-counts");
  const amountBlock = expenseItem.querySelector(".amount-block");
  const receiptBlock = expenseItem.querySelector(".receipt-block");
  const paymentProofBlock = expenseItem.querySelector(".payment-proof-block");
  const mileageProofBlock = expenseItem.querySelector(".mileage-proof-block");
  const transportMethodBlock = expenseItem.querySelector(".transport-method-block");
  const transportMethodInput = expenseItem.querySelector(".transport-method-input");
  const breakdown = expenseItem.querySelector(".per-diem-breakdown");

  if (otherBlock) otherBlock.classList.add("d-none");
  if (perDiemBlock) perDiemBlock.classList.add("d-none");
  if (breakfastBlock) breakfastBlock.classList.add("d-none");
  if (eventCounts) eventCounts.classList.add("d-none");
  if (amountBlock) amountBlock.classList.remove("d-none");
  if (receiptBlock) receiptBlock.classList.remove("d-none");
  if (paymentProofBlock) paymentProofBlock.classList.remove("d-none");
  if (mileageProofBlock) mileageProofBlock.classList.add("d-none");
  if (transportMethodBlock) transportMethodBlock.classList.add("d-none");
  if (transportMethodInput) {
    transportMethodInput.required = false;
    transportMethodInput.disabled = true;
    transportMethodInput.selectedIndex = 0;
  }
  if (breakdown) breakdown.textContent = "Complete departure and return date/time to calculate per diem allowances.";
}

function sanitizeAmountInput(input) {
  let value = input.value;

  // Permitir escribir coma o punto
  value = value.replace(/,/g, ".");

  // Quitar todo excepto números y punto
  value = value.replace(/[^\d.]/g, "");

  // Permitir solo un punto decimal
  const firstDotIndex = value.indexOf(".");
  if (firstDotIndex !== -1) {
    const integerPart = value.slice(0, firstDotIndex);
    const decimalPart = value
      .slice(firstDotIndex + 1)
      .replace(/\./g, "")
      .slice(0, 2);

    value = integerPart + "." + decimalPart;
  }

  input.value = value;
}

function updateExpenseItemByType(expenseItem) {
  const expenseTypeSelect = expenseItem.querySelector(".expense-type-select");
  if (!expenseTypeSelect) return;

  const type = expenseTypeSelect.value;

  updatePerDiemFields(expenseItem, type);
  updateAmountFieldByType(expenseItem, type);
  updateSupportingDocumentHints(expenseItem, type);
  updateMileageFields(expenseItem, type);
  updateTransportMethodFields(expenseItem, type);
}

function updateAmountFieldByType(expenseItem, type) {
  const amountLabel = expenseItem.querySelector(".amount-label");
  const amountInput = expenseItem.querySelector(".amount-input");
  const isMileage = type === MILEAGE_TYPE;

  if (amountLabel) {
    amountLabel.textContent = isMileage ? "Distance (km)" : "Amount (EUR)";
  }

  if (amountInput) {
    amountInput.placeholder = isMileage ? "0.0" : "0.00";
    amountInput.setAttribute("aria-label", isMileage ? "Distance in kilometers" : "Amount in euros");
  }
}

function updatePerDiemFields(expenseItem, type) {
  const isPerDiem = type === PER_DIEM_TYPE;

  const amountBlock = expenseItem.querySelector(".amount-block");
  const amountInput = expenseItem.querySelector(".amount-input");
  const perDiemBlock = expenseItem.querySelector(".per-diem-block");
  const perDiemInputs = expenseItem.querySelectorAll(".per-diem-input");
  const perDiemExtraInputs = expenseItem.querySelectorAll(".per-diem-breakfast-input, .per-diem-event-input, .per-diem-covered-meals-input, .per-diem-covered-dinners-input");
  const receiptInput = expenseItem.querySelector(".receipt-input");
  const paymentProofInput = expenseItem.querySelector(".payment-proof-input");

  if (amountBlock) amountBlock.classList.toggle("d-none", isPerDiem);

  if (amountInput) {
    amountInput.required = !isPerDiem;
    amountInput.disabled = isPerDiem;
    if (isPerDiem) amountInput.value = "";
  }

  if (perDiemBlock) perDiemBlock.classList.toggle("d-none", !isPerDiem);

  perDiemInputs.forEach((input) => {
    input.required = isPerDiem;
    input.disabled = !isPerDiem;
    if (!isPerDiem) input.value = "";
  });

  perDiemExtraInputs.forEach((input) => {
    input.disabled = !isPerDiem;
    if (!isPerDiem) resetPerDiemExtraInput(input);
  });

  // Diets are calculated by table and normally do not need ticket/payment proof.
  // If your internal policy requires a proof for diets too, remove this exception.
  [receiptInput, paymentProofInput].forEach((input) => {
    if (!input) return;
    input.required = !isPerDiem;
    input.disabled = isPerDiem;
    if (isPerDiem) input.value = "";
  });

  if (isPerDiem) {
    applyTravelDatesToPerDiemExpense(expenseItem, true);
  }

  updatePerDiemEventFields(expenseItem);
  updatePerDiemBreakdown(expenseItem);
}

function initTravelDateSync(expensesContainer) {
  ["budgetingTravel", "startDate", "endDate", "useExistingTravel", "createNewTravel"].forEach((id) => {
    const field = document.getElementById(id);
    field?.addEventListener("change", () => {
      syncPerDiemDates(expensesContainer);
    });
  });
}

function syncPerDiemDates(expensesContainer) {
  expensesContainer.querySelectorAll(".expense-item").forEach((expenseItem) => {
    const type = expenseItem.querySelector(".expense-type-select")?.value;
    if (type === PER_DIEM_TYPE) {
      applyTravelDatesToPerDiemExpense(expenseItem, true);
    }
  });
}

function applyTravelDatesToPerDiemExpense(expenseItem, overwrite = false) {
  const { startDate, endDate } = getCurrentTravelDates();
  const departureDate = expenseItem.querySelector("input[id^='departureDate_']");
  const returnDate = expenseItem.querySelector("input[id^='returnDate_']");

  if (departureDate && startDate && (overwrite || !departureDate.value)) {
    departureDate.value = startDate;
  }

  if (returnDate && endDate && (overwrite || !returnDate.value)) {
    returnDate.value = endDate;
  }
}

function resetPerDiemExtraInput(input) {
  if (input.type === "checkbox") {
    input.checked = false;
    return;
  }

  if (input.classList.contains("per-diem-event-input")) {
    input.value = "no";
    return;
  }

  input.value = "0";
}

function updatePerDiemEventFields(expenseItem) {
  const type = expenseItem.querySelector(".expense-type-select")?.value;
  const isPerDiem = type === PER_DIEM_TYPE;
  const hasCoveredMeals = expenseItem.querySelector(".per-diem-event-input")?.value === "yes";
  const eventCounts = expenseItem.querySelector(".per-diem-event-counts");
  const coveredMeals = expenseItem.querySelector(".per-diem-covered-meals-input");
  const coveredDinners = expenseItem.querySelector(".per-diem-covered-dinners-input");

  if (eventCounts) eventCounts.classList.toggle("d-none", !isPerDiem || !hasCoveredMeals);

  [coveredMeals, coveredDinners].forEach((input) => {
    if (!input) return;
    input.disabled = !isPerDiem || !hasCoveredMeals;
    if (!hasCoveredMeals) input.value = "0";
  });
}

function updatePerDiemBreakdown(expenseItem) {
  const breakdown = expenseItem.querySelector(".per-diem-breakdown");
  const breakdownInput = expenseItem.querySelector(".per-diem-breakdown-input");
  if (!breakdown) return;

  const type = expenseItem.querySelector(".expense-type-select")?.value;
  if (type !== PER_DIEM_TYPE) {
    breakdown.textContent = "Complete departure and return date/time to calculate per diem allowances.";
    if (breakdownInput) breakdownInput.value = "";
    return;
  }

  const result = calculatePerDiemBreakdown(expenseItem);
  updateBreakfastQuestion(expenseItem, result.isMultiDay);

  if (!result.valid) {
    breakdown.textContent = result.message;
    if (breakdownInput) breakdownInput.value = "";
    return;
  }

  const summary = `Reimbursable allowances: Breakfasts A: ${result.final.A}, Meals B: ${result.final.B}, Dinners C: ${result.final.C}.`;
  const details = [
    `Initial calculation: A ${result.initial.A}, B ${result.initial.B}, C ${result.initial.C}.`,
    `Deductions: included breakfasts ${result.deductions.A}, event meals ${result.deductions.B}, event dinners ${result.deductions.C}.`
  ];

  breakdown.innerHTML = `
    <strong>${summary}</strong>
    <div class="small text-muted">${details.join(" ")}</div>
  `;

  if (breakdownInput) {
    breakdownInput.value = JSON.stringify({
      initial: result.initial,
      deductions: result.deductions,
      final: result.final
    });
  }
}

function updateBreakfastQuestion(expenseItem, isMultiDay) {
  const block = expenseItem.querySelector(".per-diem-breakfast-block");
  const input = expenseItem.querySelector(".per-diem-breakfast-input");

  if (block) block.classList.toggle("d-none", !isMultiDay);

  if (input) {
    input.disabled = !isMultiDay;
    if (!isMultiDay) input.checked = false;
  }
}

function calculatePerDiemBreakdown(expenseItem) {
  const departureDate = expenseItem.querySelector("input[id^='departureDate_']")?.value;
  const departureTime = expenseItem.querySelector("input[id^='departureTime_']")?.value;
  const returnDate = expenseItem.querySelector("input[id^='returnDate_']")?.value;
  const returnTime = expenseItem.querySelector("input[id^='returnTime_']")?.value;

  if (!departureDate || !departureTime || !returnDate || !returnTime) {
    return { valid: false, message: "Complete departure and return date/time to calculate per diem allowances.", isMultiDay: false };
  }

  const departure = new Date(`${departureDate}T${departureTime}`);
  const arrival = new Date(`${returnDate}T${returnTime}`);

  if (Number.isNaN(departure.getTime()) || Number.isNaN(arrival.getTime()) || arrival <= departure) {
    return { valid: false, message: "Return date/time must be after departure date/time.", isMultiDay: false };
  }

  const initial = calculateInitialAllowances(departure, arrival);
  const isMultiDay = !isSameDate(departure, arrival);
  const breakfastIncluded = isMultiDay && Boolean(expenseItem.querySelector(".per-diem-breakfast-input")?.checked);
  const coveredMeals = getPositiveInteger(expenseItem.querySelector(".per-diem-covered-meals-input")?.value);
  const coveredDinners = getPositiveInteger(expenseItem.querySelector(".per-diem-covered-dinners-input")?.value);

  const deductions = {
    A: breakfastIncluded ? initial.A : 0,
    B: Math.min(initial.B, coveredMeals),
    C: Math.min(initial.C, coveredDinners)
  };

  return {
    valid: true,
    isMultiDay,
    initial,
    deductions,
    final: {
      A: Math.max(0, initial.A - deductions.A),
      B: Math.max(0, initial.B - deductions.B),
      C: Math.max(0, initial.C - deductions.C)
    }
  };
}

function calculateInitialAllowances(departure, arrival) {
  const hours = (arrival.getTime() - departure.getTime()) / 36e5;
  const startHour = decimalHour(departure);
  const endHour = decimalHour(arrival);
  const counts = { A: 0, B: 0, C: 0 };

  if (hours < 24) {
    if (startHour < 14 && endHour > 21) {
      counts.B += 1;
      counts.C += 1;
    } else if (hours >= 5 && startHour < 14 && endHour < 21) {
      counts.B += 1;
    } else if (hours >= 5 && startHour >= 14 && endHour > 21) {
      counts.C += 1;
    }

    return counts;
  }

  if (startHour < 14) {
    counts.B += 1;
    counts.C += 1;
  } else if (startHour < 21) {
    counts.C += 1;
  }

  const fullDaysBetween = Math.max(0, daysBetween(startOfDay(departure), startOfDay(arrival)) - 1);
  counts.A += fullDaysBetween;
  counts.B += fullDaysBetween;
  counts.C += fullDaysBetween;

  counts.A += 1;
  if (endHour >= 14) counts.B += 1;
  if (endHour > 21) counts.C += 1;

  return counts;
}

function decimalHour(date) {
  return date.getHours() + date.getMinutes() / 60;
}

function startOfDay(date) {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function daysBetween(start, end) {
  return Math.round((end.getTime() - start.getTime()) / 86400000);
}

function isSameDate(a, b) {
  return a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate();
}

function getPositiveInteger(value) {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
}

function getCurrentTravelDates() {
  const useExistingTravel = document.getElementById("useExistingTravel");

  if (useExistingTravel?.checked) {
    const selectedTravel = document.getElementById("budgetingTravel")?.selectedOptions?.[0];
    return {
      startDate: normalizeDateForInput(selectedTravel?.dataset.startDate),
      endDate: normalizeDateForInput(selectedTravel?.dataset.endDate)
    };
  }

  return {
    startDate: normalizeDateForInput(document.getElementById("startDate")?.value),
    endDate: normalizeDateForInput(document.getElementById("endDate")?.value)
  };
}

function normalizeDateForInput(value) {
  const text = String(value || "").trim();
  if (!text) return "";

  const isoDateMatch = text.match(/^\d{4}-\d{2}-\d{2}/);
  if (isoDateMatch) return isoDateMatch[0];

  const date = new Date(text);
  if (Number.isNaN(date.getTime())) return "";

  return date.toISOString().slice(0, 10);
}

function updateSupportingDocumentHints(expenseItem, type) {
  const receiptHelp = expenseItem.querySelector(".receipt-help");
  if (!receiptHelp) return;

  if (type === ACCOMMODATION_TYPE) {
    receiptHelp.textContent = "For accommodation, upload an invoice whenever possible instead of a simple ticket.";
    receiptHelp.classList.add("text-danger");
    return;
  }

  if (type === PER_DIEM_TYPE) {
    receiptHelp.textContent = "Diets do not require an entered amount. The system calculates them from the allowance table.";
    receiptHelp.classList.remove("text-danger");
    return;
  }

  if (type === MILEAGE_TYPE) {
    receiptHelp.textContent = "Mileage requires a screenshot showing the kilometers made during the trip.";
    receiptHelp.classList.remove("text-danger");
    return;
  }

  receiptHelp.textContent = "Upload the ticket or invoice for this expense.";
  receiptHelp.classList.remove("text-danger");
}

function updateMileageFields(expenseItem, type) {
  const doesNotRequireDocuments = type === MILEAGE_TYPE || type === PER_DIEM_TYPE;
  const receiptBlock = expenseItem.querySelector(".receipt-block");
  const paymentProofBlock = expenseItem.querySelector(".payment-proof-block");
  const mileageProofBlock = expenseItem.querySelector(".mileage-proof-block");
  const receiptInput = expenseItem.querySelector(".receipt-input");
  const paymentProofInput = expenseItem.querySelector(".payment-proof-input");
  const mileageProofInput = expenseItem.querySelector(".mileage-proof-input");

  [receiptBlock, paymentProofBlock].forEach((block) => {
    if (block) block.classList.toggle("d-none", doesNotRequireDocuments);
  });

  [receiptInput, paymentProofInput].forEach((input) => {
    if (!input) return;
    input.required = !doesNotRequireDocuments;
    input.disabled = doesNotRequireDocuments;
    if (doesNotRequireDocuments) input.value = "";
  });

  if (mileageProofBlock) mileageProofBlock.classList.toggle("d-none", type !== MILEAGE_TYPE);

  if (mileageProofInput) {
    mileageProofInput.required = type === MILEAGE_TYPE;
    mileageProofInput.disabled = type !== MILEAGE_TYPE;
    if (type !== MILEAGE_TYPE) mileageProofInput.value = "";
  }
}

function updateTransportMethodFields(expenseItem, type) {
  const isTransport = type === TRANSPORT_TYPE;
  const block = expenseItem.querySelector(".transport-method-block");
  const input = expenseItem.querySelector(".transport-method-input");

  if (block) block.classList.toggle("d-none", !isTransport);
  if (!input) return;

  input.required = isTransport;
  input.disabled = !isTransport;
  input.classList.remove("is-invalid");
  if (!isTransport) input.selectedIndex = 0;
}

function refreshExpenseLabels(expensesContainer) {
  const expenseItems = expensesContainer.querySelectorAll(".expense-item");

  expenseItems.forEach((item, index) => {
    const number = index + 1;
    const nameIndex = index;

    item.dataset.expenseIndex = number;

    const chip = item.querySelector(".expense-chip");
    if (chip) chip.textContent = `Expense ${number}`;

    const removeBtn = item.querySelector(".remove-expense-btn");
    if (removeBtn) {
      removeBtn.classList.toggle("d-none", expenseItems.length === 1);
    }

    updateFieldIds(item, number);
    updateFieldNames(item, nameIndex);
    updateExpenseItemByType(item);
  });
}

function updateFieldIds(item, number) {
  item.querySelectorAll("[id]").forEach((field) => {
    const baseId = field.id.replace(/_\d+$/, "");
    field.id = `${baseId}_${number}`;
  });

  item.querySelectorAll("label[for]").forEach((label) => {
    const baseFor = label.getAttribute("for").replace(/_\d+$/, "");
    label.setAttribute("for", `${baseFor}_${number}`);
  });
}

function updateFieldNames(item, nameIndex) {
  const type = item.querySelector("select[id^='expenseType_']");
  const otherType = item.querySelector("input[id^='otherExpenseType_']");
  const amount = item.querySelector("input[id^='amount_']");
  const transportMethod = item.querySelector("select[id^='transportMethod_']");
  const description = item.querySelector("textarea[id^='description_']");
  const receipt = item.querySelector("input[id^='receiptFile_']");
  const paymentProof = item.querySelector("input[id^='paymentProofFile_']");
  const mileageProof = item.querySelector("input[id^='mileageProofFile_']");
  const departureDate = item.querySelector("input[id^='departureDate_']");
  const departureTime = item.querySelector("input[id^='departureTime_']");
  const returnDate = item.querySelector("input[id^='returnDate_']");
  const returnTime = item.querySelector("input[id^='returnTime_']");
  const breakfastIncluded = item.querySelector("input[id^='breakfastIncluded_']");
  const eventMealsCovered = item.querySelector("select[id^='eventMealsCovered_']");
  const coveredMeals = item.querySelector("input[id^='coveredMeals_']");
  const coveredDinners = item.querySelector("input[id^='coveredDinners_']");
  const perDiemBreakdown = item.querySelector(".per-diem-breakdown-input");

  if (type) type.name = `expenses[${nameIndex}][type]`;
  if (otherType) otherType.name = `expenses[${nameIndex}][other_type]`;
  if (amount) amount.name = `expenses[${nameIndex}][amount]`;
  if (transportMethod) transportMethod.name = `expenses[${nameIndex}][transport_method]`;
  if (description) description.name = `expenses[${nameIndex}][description]`;
  if (receipt) receipt.name = `expenses[${nameIndex}][receipt]`;
  if (paymentProof) paymentProof.name = `expenses[${nameIndex}][payment_proof]`;
  if (mileageProof) mileageProof.name = `expenses[${nameIndex}][mileage_proof]`;
  if (departureDate) departureDate.name = `expenses[${nameIndex}][departure_date]`;
  if (departureTime) departureTime.name = `expenses[${nameIndex}][departure_time]`;
  if (returnDate) returnDate.name = `expenses[${nameIndex}][return_date]`;
  if (returnTime) returnTime.name = `expenses[${nameIndex}][return_time]`;
  if (breakfastIncluded) breakfastIncluded.name = `expenses[${nameIndex}][breakfast_included]`;
  if (eventMealsCovered) eventMealsCovered.name = `expenses[${nameIndex}][event_meals_covered]`;
  if (coveredMeals) coveredMeals.name = `expenses[${nameIndex}][covered_meals]`;
  if (coveredDinners) coveredDinners.name = `expenses[${nameIndex}][covered_dinners]`;
  if (perDiemBreakdown) perDiemBreakdown.name = `expenses[${nameIndex}][per_diem_breakdown]`;
}

function createExpenseItem(expensesContainer) {
  const firstExpense = expensesContainer.querySelector(".expense-item");
  if (!firstExpense) return;

  const newExpense = firstExpense.cloneNode(true);

  newExpense.querySelectorAll("input, select, textarea").forEach((field) => {
    if (field.type === "file") {
      field.value = "";
      field.disabled = false;
    } else if (field.type === "checkbox") {
      field.checked = false;
      field.disabled = false;
    } else if (field.tagName === "SELECT") {
      field.selectedIndex = 0;
    } else {
      field.value = "";
      field.disabled = false;
    }
  });

  resetExpenseVisualState(newExpense);

  expensesContainer.appendChild(newExpense);
  refreshExpenseLabels(expensesContainer);
}

document.addEventListener("input", function (event) {
  if (event.target.classList.contains("amount-input")) {
    sanitizeAmountInput(event.target);
  }
});
