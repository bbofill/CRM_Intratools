const PAGE_SIZE = 200;
let currentOffset = 0;
let lastBatchSize = 0;

export async function loadLogsTable(
  endpoint = "/module/proxy/module0/api/logs",
  offset = 0
) {
  const tbody = document.getElementById("log-table-body");
  const pageInfo = document.getElementById("page-info");
  const btnPrev = document.getElementById("btn-prev-page");
  const btnNext = document.getElementById("btn-next-page");

  if (!tbody) return;

  tbody.innerHTML = `<tr><td colspan="7" class="text-center text-muted">Loading logs...</td></tr>`;

  try {
    const url = `${endpoint}?limit=${PAGE_SIZE}&offset=${offset}`;
    const res = await fetch(url);
    if (!res.ok) throw new Error("Network or backend error");

    const logs = await res.json();
    if (!Array.isArray(logs)) throw new Error("Invalid response");

    currentOffset = offset;
    lastBatchSize = logs.length;

    tbody.innerHTML = "";

    if (logs.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7" class="text-muted">No logs available</td></tr>';
    } else {
      for (const l of logs) {
        const row = `
          <tr>
            <td>${l.id}</td>
            <td>${l.module}</td>
            <td>${new Date(l.timestamp * 1000).toLocaleString()}</td>
            <td class="text-break small">${l.msg}</td>
            <td>${l.code ?? '-'}</td>
            <td class="text-break small">${l.hash}</td>
            <td class="text-break small">${l.previous_hash}</td>
          </tr>`;
        tbody.insertAdjacentHTML("beforeend", row);
      }
    }

    if (pageInfo) {
      const pageNumber = Math.floor(currentOffset / PAGE_SIZE) + 1;
      pageInfo.textContent = `Page ${pageNumber} · Showing ${logs.length} logs`;
    }

    if (btnPrev) btnPrev.disabled = currentOffset === 0;
    if (btnNext) btnNext.disabled = logs.length < PAGE_SIZE;

  } catch (err) {
    console.error("Error loading logs:", err);
    tbody.innerHTML = `<tr><td colspan="7" class="text-danger text-center">Failed to load logs</td></tr>`;
    if (pageInfo) pageInfo.textContent = "Error loading page";
  }
}

export function setupLogsPagination() {
  const btnPrev = document.getElementById("btn-prev-page");
  const btnNext = document.getElementById("btn-next-page");

  if (btnPrev) {
    btnPrev.addEventListener("click", () => {
      if (currentOffset >= PAGE_SIZE) {
        loadLogsTable("/module/proxy/module0/api/logs", currentOffset - PAGE_SIZE);
      }
    });
  }

  if (btnNext) {
    btnNext.addEventListener("click", () => {
      if (lastBatchSize === PAGE_SIZE) {
        loadLogsTable("/module/proxy/module0/api/logs", currentOffset + PAGE_SIZE);
      }
    });
  }
}

export async function validateLogChain() {
  const status = document.getElementById("chain-status");
  if (!status) return;

  status.textContent = "Validating chain...";

  try {
    const res = await fetch("/module/proxy/module0/api/logs", {
      method: "POST"
    });
    const result = await res.json();

    if (result.status === "valid") {
      status.textContent = `✅ Chain is valid (${result.count} entries)`;
    } else {
      status.textContent = `❌ ${result.reason} at ID ${result.at}`;
    }
  } catch (err) {
    console.error(err);
    status.textContent = "❌ Error during validation";
  }
}