import {
  getSession,
  getPricingTables,
  savePricingTable,
  deletePricingTable
} from "./travelApi.js";

let pricingTables = [];
let pricingTableInitialState = "";
let pricingTableIsSaving = false;

export async function initRatesManagement() {
  const session = await getSession().catch(() => null);
  const tabItem = document.getElementById("rates-management-tab-item");
  if (!session?.can_manage_rates) {
    tabItem?.classList.add("d-none");
    return;
  }

  tabItem?.classList.remove("d-none");
  bindRatesManagement();
  loadPricingTables();
}

function bindRatesManagement() {
  document.getElementById("addPricingTableBtn")?.addEventListener("click", () => openPricingTableModal());
  document.getElementById("addPricingRowBtn")?.addEventListener("click", () => addPricingRow());
  document.getElementById("cancelPricingTableBtn")?.addEventListener("click", requestClosePricingTableModal);
  document.getElementById("closePricingTableModalBtn")?.addEventListener("click", requestClosePricingTableModal);

  const pricingModal = document.getElementById("pricingTableModal");
  pricingModal?.addEventListener("hidePrevented.bs.modal", requestClosePricingTableModal);
  pricingModal?.addEventListener("hidden.bs.modal", () => {
    pricingTableInitialState = "";
    pricingTableIsSaving = false;
  });

  document.getElementById("pricingTableType")?.addEventListener("change", () => {
    if (!document.getElementById("pricingTableId")?.value) {
      resetPricingRowsForCurrentType();
    }

    updateMealColumnVisibility();
    normalizeRowsForTableType();
  });

  document.getElementById("pricingTablesBody")?.addEventListener("click", async (event) => {
    const editBtn = event.target.closest("[data-edit-pricing-table]");
    if (editBtn) {
      const table = pricingTables.find((item) => Number(item.id) === Number(editBtn.dataset.editPricingTable));
      if (table) openPricingTableModal(table);
      return;
    }

    const deleteBtn = event.target.closest("[data-delete-pricing-table]");
    if (deleteBtn) {
      if (!confirm("Delete this pricing table?")) return;
      try {
        await deletePricingTable(deleteBtn.dataset.deletePricingTable);
        await loadPricingTables();
      } catch (error) {
        console.error(error);
        alert("Could not delete pricing table.");
      }
    }
  });

  document.getElementById("pricingRowsBody")?.addEventListener("click", (event) => {
    const removeBtn = event.target.closest("[data-remove-pricing-row]");
    if (removeBtn) removeBtn.closest("tr")?.remove();
  });

  document.getElementById("pricingTableForm")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    await submitPricingTable();
  });
}

async function loadPricingTables() {
  const tbody = document.getElementById("pricingTablesBody");
  if (tbody) tbody.innerHTML = emptyRow(5, "Loading pricing tables...");

  try {
    const data = await getPricingTables();
    pricingTables = Array.isArray(data?.tables) ? data.tables.map(normalizePricingTable) : [];
    renderPricingTables();
  } catch (error) {
    console.error(error);
    if (tbody) tbody.innerHTML = emptyRow(5, "Could not load pricing tables. Check that the pricing migration has been applied.");
  }
}

function renderPricingTables() {
  const tbody = document.getElementById("pricingTablesBody");
  if (!tbody) return;

  if (!pricingTables.length) {
    tbody.innerHTML = emptyRow(5, "No pricing tables configured.");
    return;
  }

  tbody.innerHTML = pricingTables.map((table) => {
    const tableId = getRecordId(table);
    return `
    <tr>
      <td>${escapeHtml(table.title || "")}</td>
      <td>${escapeHtml(table.category || "")}</td>
      <td>${escapeHtml(formatTableType(table.table_type))}</td>
      <td>${Array.isArray(table.rows) ? table.rows.length : 0}</td>
      <td class="text-end">
        <div class="btn-group btn-group-sm">
          <button type="button" class="btn btn-outline-secondary" data-edit-pricing-table="${tableId}">Edit</button>
          <button type="button" class="btn btn-outline-danger" data-delete-pricing-table="${tableId}">Delete</button>
        </div>
      </td>
    </tr>
  `;
  }).join("");
}

function openPricingTableModal(table = null) {
  const form = document.getElementById("pricingTableForm");
  form?.reset();

  document.getElementById("pricingTableId").value = table ? getRecordId(table) : "";
  document.getElementById("pricingTableTitle").value = table?.title || "";
  document.getElementById("pricingTableCategory").value = table?.category || "";
  document.getElementById("pricingTableType").value = table?.table_type || table?.type || "accommodation";

  const rowsBody = document.getElementById("pricingRowsBody");
  if (rowsBody) rowsBody.innerHTML = "";

  const rows = Array.isArray(table?.rows) && table.rows.length ? table.rows : defaultRows(document.getElementById("pricingTableType").value);
  renderPricingRows(rows);

  updateMealColumnVisibility();
  bootstrap.Modal.getOrCreateInstance(document.getElementById("pricingTableModal")).show();
  storePricingTableInitialState();
}

function renderPricingRows(rows) {
  const tbody = document.getElementById("pricingRowsBody");
  if (tbody) tbody.innerHTML = "";

  rows.forEach((row) => addPricingRow(row));
}

function resetPricingRowsForCurrentType() {
  const tableType = document.getElementById("pricingTableType")?.value || "accommodation";
  renderPricingRows(defaultRows(tableType));
}

function addPricingRow(row = {}) {
  const tbody = document.getElementById("pricingRowsBody");
  if (!tbody) return;

  const tableType = document.getElementById("pricingTableType")?.value || "accommodation";
  const tr = document.createElement("tr");
  tr.innerHTML = `
    <td><input type="text" class="form-control form-control-sm pricing-territory" value="${escapeAttribute(row.territory || "Spain")}" required></td>
    <td>
      <select class="form-select form-select-sm pricing-group">
        ${["G1", "G2", "G3"].map((group) => `<option value="${group}" ${String(row.group_code || "G1") === group ? "selected" : ""}>${group}</option>`).join("")}
      </select>
    </td>
    <td class="pricing-meal-cell">
      <select class="form-select form-select-sm pricing-meal">
        ${[
          ["A", "Breakfast A"],
          ["B", "Meal B"],
          ["C", "Dinner C"],
          ["FULL_BOARD", "Full board"]
        ].map(([value, label]) => `<option value="${value}" ${String(row.meal_type || "A") === value ? "selected" : ""}>${label}</option>`).join("")}
      </select>
    </td>
    <td><input type="number" min="0" step="0.01" class="form-control form-control-sm pricing-amount" value="${escapeAttribute(row.amount ?? 0)}" required></td>
    <td class="text-end"><button type="button" class="btn btn-sm btn-outline-danger" data-remove-pricing-row>Remove</button></td>
  `;

  tbody.appendChild(tr);
  tr.querySelector(".pricing-meal").disabled = tableType === "accommodation";
  tr.querySelector(".pricing-meal-cell").classList.toggle("d-none", tableType === "accommodation");
}

function normalizeRowsForTableType() {
  const tableType = document.getElementById("pricingTableType")?.value || "accommodation";
  document.querySelectorAll("#pricingRowsBody tr").forEach((row) => {
    const meal = row.querySelector(".pricing-meal");
    const mealCell = row.querySelector(".pricing-meal-cell");
    if (meal) meal.disabled = tableType === "accommodation";
    mealCell?.classList.toggle("d-none", tableType === "accommodation");
  });
}

function updateMealColumnVisibility() {
  const isAccommodation = document.getElementById("pricingTableType")?.value === "accommodation";
  document.querySelectorAll(".pricing-meal-column").forEach((cell) => {
    cell.classList.toggle("d-none", isAccommodation);
  });
}

async function submitPricingTable() {
  const payload = {
    id: Number(document.getElementById("pricingTableId")?.value || 0),
    title: document.getElementById("pricingTableTitle")?.value?.trim() || "",
    category: document.getElementById("pricingTableCategory")?.value?.trim() || "",
    type: document.getElementById("pricingTableType")?.value || "accommodation",
    rows: collectPricingRows()
  };

  if (!payload.title || !payload.category || !payload.rows.length) {
    alert("Title, category and at least one rate row are required.");
    return;
  }

  try {
    pricingTableIsSaving = true;
    await savePricingTable(payload);
    bootstrap.Modal.getInstance(document.getElementById("pricingTableModal"))?.hide();
    await loadPricingTables();
  } catch (error) {
    pricingTableIsSaving = false;
    console.error(error);
    alert("Could not save pricing table.");
  }
}

function requestClosePricingTableModal() {
  if (!hasUnsavedPricingChanges()) {
    closePricingTableModal();
    return;
  }

  if (confirm("You have unsaved changes. Are you sure you want to discard them?")) {
    closePricingTableModal();
  }
}

function closePricingTableModal() {
  const modal = bootstrap.Modal.getOrCreateInstance(document.getElementById("pricingTableModal"));
  modal.hide();
}

function storePricingTableInitialState() {
  pricingTableInitialState = serializePricingTableForm();
}

function hasUnsavedPricingChanges() {
  if (pricingTableIsSaving) return false;
  return pricingTableInitialState !== serializePricingTableForm();
}

function serializePricingTableForm() {
  return JSON.stringify({
    id: Number(document.getElementById("pricingTableId")?.value || 0),
    title: document.getElementById("pricingTableTitle")?.value || "",
    category: document.getElementById("pricingTableCategory")?.value || "",
    type: document.getElementById("pricingTableType")?.value || "accommodation",
    rows: collectPricingRows()
  });
}

function collectPricingRows() {
  const tableType = document.getElementById("pricingTableType")?.value || "accommodation";
  return [...document.querySelectorAll("#pricingRowsBody tr")].map((row, index) => ({
    territory: row.querySelector(".pricing-territory")?.value?.trim() || "",
    group_code: row.querySelector(".pricing-group")?.value || "G1",
    meal_type: tableType === "per_diem" ? row.querySelector(".pricing-meal")?.value || "A" : "",
    amount: Number(row.querySelector(".pricing-amount")?.value || 0),
    sort_order: index + 1
  })).filter((row) => row.territory);
}

function defaultRows(tableType) {
  const territories = ["Spain", "Europe", "USA", "Rest of world"];
  const groups = ["G1", "G2", "G3"];
  const meals = tableType === "per_diem" ? ["A", "B", "C", "FULL_BOARD"] : [""];

  return territories.flatMap((territory) =>
    groups.flatMap((group) =>
      meals.map((meal) => ({
        territory,
        group_code: group,
        meal_type: meal,
        amount: 0
      }))
    )
  );
}

function formatTableType(type) {
  return type === "per_diem" ? "Per diem" : "Accommodation";
}

function normalizePricingTable(table) {
  const rows = Array.isArray(table.rows) ? table.rows : [];

  return {
    ...table,
    id: getRecordId(table),
    title: table.title ?? table.Title ?? "",
    category: table.category ?? table.Category ?? "",
    table_type: table.table_type ?? table.tableType ?? table.type ?? table.Type ?? "accommodation",
    rows: rows.map((row) => ({
      ...row,
      id: getRecordId(row),
      table_id: row.table_id ?? row.tableId ?? row.TableID,
      territory: row.territory ?? row.Territory ?? "",
      group_code: row.group_code ?? row.groupCode ?? row.GroupCode ?? "G1",
      meal_type: normalizeMealType(row.meal_type ?? row.mealType ?? row.MealType ?? ""),
      amount: row.amount ?? row.Amount ?? 0,
      sort_order: row.sort_order ?? row.sortOrder ?? row.SortOrder ?? 0
    }))
  };
}

function getRecordId(record) {
  return Number(record?.id ?? record?.ID ?? record?.Id ?? 0);
}

function normalizeMealType(value) {
  const mealType = String(value || "").toUpperCase();
  return mealType === "HALF_BOARD" ? "FULL_BOARD" : mealType;
}

function emptyRow(colspan, message) {
  return `<tr><td colspan="${colspan}" class="text-center text-muted py-4">${escapeHtml(message)}</td></tr>`;
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function escapeAttribute(value) {
  return escapeHtml(value);
}
