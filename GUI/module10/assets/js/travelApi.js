const API_BASE_URL = "/module/proxy/module10/api";

async function parseResponse(response) {
  if (!response.ok) {
    const message = await response.text().catch(() => "");
    throw new Error(message || `Request failed with status ${response.status}`);
  }

  const contentType = response.headers.get("content-type") || "";
  if (contentType.includes("application/json")) {
    return response.json();
  }

  return response.text();
}

export async function getSession() {
  const response = await fetch(`${API_BASE_URL}?action=session`, {
    method: "GET",
    headers: { "Accept": "application/json" },
    credentials: "include"
  });

  return parseResponse(response);
}

export async function fetchProjects() {
  const response = await fetch(`${API_BASE_URL}?action=projects`, {
    credentials: "same-origin"
  });

  if (!response.ok) {
    throw new Error("Could not fetch projects");
  }

  return response.json();
}

export async function getTravelsWithoutCommission() {
  const response = await fetch(`${API_BASE_URL}?action=available-travels`, {
    method: "GET",
    headers: { "Accept": "application/json" },
    credentials: "include"
  });

  return parseResponse(response);
}

export async function createCommission(formData) {
  const response = await fetch(`${API_BASE_URL}?action=create-commission`, {
    method: "POST",
    body: formData,
    credentials: "include"
  });

  return parseResponse(response);
}

export async function getMyCommissions() {
  const response = await fetch(`${API_BASE_URL}?action=my-commissions`, {
    method: "GET",
    headers: { "Accept": "application/json" },
    credentials: "include"
  });

  return parseResponse(response);
}

export async function getProjectCommissions() {
  const response = await fetch(`${API_BASE_URL}?action=project-commissions`, {
    method: "GET",
    headers: { "Accept": "application/json" },
    credentials: "include"
  });

  return parseResponse(response);
}

export async function getStakeholderCommissions() {
  const response = await fetch(`${API_BASE_URL}?action=stakeholder-commissions`, {
    method: "GET",
    headers: { "Accept": "application/json" },
    credentials: "include"
  });

  return parseResponse(response);
}

export async function getCommissionDetail(id) {
  const response = await fetch(`${API_BASE_URL}?action=commission-detail&id=${encodeURIComponent(id)}`, {
    method: "GET",
    headers: { "Accept": "application/json" },
    credentials: "include"
  });

  return parseResponse(response);
}

export async function approveCommission(id, peopleCategory = null, relatedTask = null) {
  return updateCommissionStatus("approve-commission", id, peopleCategory, relatedTask);
}

export async function saveCommissionReviewFields(id, peopleCategory = null, relatedTask = null) {
  const payload = { id: Number(id) };
  if (peopleCategory !== null && peopleCategory !== undefined) {
    payload.people_category = Number(peopleCategory);
  }
  if (relatedTask !== null && relatedTask !== undefined) {
    payload.related_task = String(relatedTask);
  }

  const response = await fetch(`${API_BASE_URL}?action=save-commission-review-fields`, {
    method: "POST",
    headers: {
      "Accept": "application/json",
      "Content-Type": "application/json"
    },
    credentials: "include",
    body: JSON.stringify(payload)
  });

  return parseResponse(response);
}

export async function rejectCommission(id) {
  return updateCommissionStatus("reject-commission", id);
}

export async function approveStakeholder(approvalId, message = "") {
  const response = await fetch(`${API_BASE_URL}?action=approve-stakeholder`, {
    method: "POST",
    headers: {
      "Accept": "application/json",
      "Content-Type": "application/json"
    },
    credentials: "include",
    body: JSON.stringify({ approval_id: Number(approvalId), message })
  });

  return parseResponse(response);
}

export async function getDataExportOptions() {
  const response = await fetch(`${API_BASE_URL}?action=data-export-options`, {
    method: "GET",
    headers: { "Accept": "application/json" },
    credentials: "include"
  });

  return parseResponse(response);
}

export async function downloadDataExport(payload) {
  const response = await fetch(`${API_BASE_URL}?action=data-export`, {
    method: "POST",
    headers: {
      "Accept": "application/zip, text/plain",
      "Content-Type": "application/json"
    },
    credentials: "include",
    body: JSON.stringify(payload)
  });

  if (!response.ok) {
    const message = await response.text().catch(() => "");
    const normalizedMessage = message.trim().toLowerCase();

    if (normalizedMessage.includes("no signed commissions found")) {
      alert("No signed commissions found for the selected filters.");
      return;
    }

    throw new Error(message || `Request failed with status ${response.status}`);
  }

  const contentType = response.headers.get("content-type") || "";

  if (contentType.includes("text/plain")) {
    const message = await response.text().catch(() => "");
    const normalizedMessage = message.trim().toLowerCase();

    if (normalizedMessage.includes("no signed commissions found")) {
      alert("No signed commissions found for the selected filters.");
      return;
    }

    alert(message.trim() || "No data available to export.");
    return;
  }

  const blob = await response.blob();

  if (!blob.size) {
    alert("No data available to export.");
    return;
  }

  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = "service_commission_data_export.zip";
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export function finalPDFUrl(id) {
  return `${API_BASE_URL}?action=download-final-pdf&id=${encodeURIComponent(id)}`;
}

export function previewCommissionPDFUrl(id, peopleCategory = null, relatedTask = null) {
  const params = new URLSearchParams({
    action: "preview-commission-pdf",
    id: String(id)
  });
  if (peopleCategory !== null && peopleCategory !== undefined) {
    params.set("people_category", String(peopleCategory));
  }
  if (relatedTask !== null && relatedTask !== undefined) {
    params.set("related_task", String(relatedTask));
  }
  return `${API_BASE_URL}?${params.toString()}`;
}

async function updateCommissionStatus(action, id, peopleCategory = null, relatedTask = null) {
  const payload = { id: Number(id) };
  if (peopleCategory !== null && peopleCategory !== undefined) {
    payload.people_category = Number(peopleCategory);
  }
  if (relatedTask !== null && relatedTask !== undefined) {
    payload.related_task = String(relatedTask);
  }

  const response = await fetch(`${API_BASE_URL}?action=${action}`, {
    method: "POST",
    headers: {
      "Accept": "application/json",
      "Content-Type": "application/json"
    },
    credentials: "include",
    body: JSON.stringify(payload)
  });

  return parseResponse(response);
}

export async function addCenterExpense(payload) {
  const response = await fetch(`${API_BASE_URL}?action=add-center-expense`, {
    method: "POST",
    headers: {
      "Accept": "application/json",
      "Content-Type": "application/json"
    },
    credentials: "include",
    body: JSON.stringify(payload)
  });

  return parseResponse(response);
}

export async function deleteCenterExpense(expenseId) {
  const response = await fetch(`${API_BASE_URL}?action=delete-center-expense`, {
    method: "POST",
    headers: {
      "Accept": "application/json",
      "Content-Type": "application/json"
    },
    credentials: "include",
    body: JSON.stringify({ expense_id: Number(expenseId) })
  });

  return parseResponse(response);
}

export async function excludeExpense(expenseId) {
  const response = await fetch(`${API_BASE_URL}?action=exclude-expense`, {
    method: "POST",
    headers: {
      "Accept": "application/json",
      "Content-Type": "application/json"
    },
    credentials: "include",
    body: JSON.stringify({ expense_id: Number(expenseId) })
  });

  return parseResponse(response);
}

export async function getPricingTables() {
  const response = await fetch(`${API_BASE_URL}?action=pricing-tables`, {
    method: "GET",
    headers: { "Accept": "application/json" },
    credentials: "include"
  });

  return parseResponse(response);
}

export async function savePricingTable(payload) {
  const response = await fetch(`${API_BASE_URL}?action=save-pricing-table`, {
    method: "POST",
    headers: {
      "Accept": "application/json",
      "Content-Type": "application/json"
    },
    credentials: "include",
    body: JSON.stringify(payload)
  });

  return parseResponse(response);
}

export async function deletePricingTable(id) {
  const response = await fetch(`${API_BASE_URL}?action=delete-pricing-table`, {
    method: "POST",
    headers: {
      "Accept": "application/json",
      "Content-Type": "application/json"
    },
    credentials: "include",
    body: JSON.stringify({ id: Number(id) })
  });

  return parseResponse(response);
}

export async function saveExpensePricingReview(payload) {
  const response = await fetch(`${API_BASE_URL}?action=save-expense-pricing-review`, {
    method: "POST",
    headers: {
      "Accept": "application/json",
      "Content-Type": "application/json"
    },
    credentials: "include",
    body: JSON.stringify(payload)
  });

  return parseResponse(response);
}

export async function saveMileageReview(payload) {
  const response = await fetch(`${API_BASE_URL}?action=save-mileage-review`, {
    method: "POST",
    headers: {
      "Accept": "application/json",
      "Content-Type": "application/json"
    },
    credentials: "include",
    body: JSON.stringify(payload)
  });

  return parseResponse(response);
}

export async function saveExpenseAdvance(payload) {
  const response = await fetch(`${API_BASE_URL}?action=save-expense-advance`, {
    method: "POST",
    headers: {
      "Accept": "application/json",
      "Content-Type": "application/json"
    },
    credentials: "include",
    body: JSON.stringify(payload)
  });

  return parseResponse(response);
}

export async function saveExpenseEdit(payload) {
  const response = await fetch(`${API_BASE_URL}?action=save-expense-edit`, {
    method: "POST",
    headers: {
      "Accept": "application/json",
      "Content-Type": "application/json"
    },
    credentials: "include",
    body: JSON.stringify(payload)
  });

  return parseResponse(response);
}

function filenameFromContentDisposition(header) {
  if (!header) return "";

  const utfMatch = header.match(/filename\*=UTF-8''([^;]+)/i);
  if (utfMatch?.[1]) {
    try {
      return decodeURIComponent(utfMatch[1]);
    } catch {
      return utfMatch[1];
    }
  }

  const match = header.match(/filename="?([^";]+)"?/i);
  return match?.[1] || "";
}

function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename || "service_commission.pdf";
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export async function exportCommissions(ids = []) {
  const query = Array.isArray(ids) && ids.length
    ? `&ids=${ids.map((id) => encodeURIComponent(id)).join(",")}`
    : "";

  const response = await fetch(`${API_BASE_URL}?action=export-commissions${query}`, {
    method: "GET",
    headers: { "Accept": "application/pdf, application/zip, text/plain" },
    credentials: "include"
  });

  if (!response.ok) {
    const message = await response.text().catch(() => "");
    throw new Error(message || `Request failed with status ${response.status}`);
  }

  const blob = await response.blob();
  const contentType = response.headers.get("content-type") || blob.type || "";
  const filename = filenameFromContentDisposition(response.headers.get("content-disposition"))
    || (contentType.includes("zip") ? "signed_service_commissions.zip" : "service_commission.pdf");
  downloadBlob(blob, filename);
}
