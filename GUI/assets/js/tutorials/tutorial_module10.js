import { initTutorial } from "./tutorialEngine.js";

const MODULE10_API_URL = "/module/proxy/module10/api";

async function loadModule10Tutorial() {
  await waitForDocumentReady();

  const session = await fetchModule10Session();
  await waitForWorkspacePermissions(session);

  const canReview = Boolean(session?.can_review);
  const canManageRates = Boolean(session?.can_manage_rates) && isTargetAvailable("#rates-management-tab");
  const canApproveFinal = canReview || isTargetAvailable("#stakeholder-approval-tab");
  const steps = buildSteps({ canReview, canApproveFinal, canManageRates });
  const availableSteps = steps.filter((step) => isTargetAvailable(step.element));

  if (!availableSteps.length) return;

  initTutorial(availableSteps, "module10", tutorialPageKey({ canReview, canApproveFinal, canManageRates }));
}

function buildSteps({ canReview, canApproveFinal, canManageRates }) {
  const steps = [
    {
      element: ".service-hero",
      title: "Service commissions module",
      message: "Welcome to the service commissions area. From here you can submit travel expenses and follow the status of your service commissions.",
      position: "bottom"
    },
    {
      element: "#latestStatus",
      title: "Latest commission",
      message: "Here you can see your latest submitted service commission and quickly open its full status.",
      position: "bottom"
    }
  ];

  if (canReview) {
    steps.push(
      {
        element: "#pendingReviewSummaryCard",
        title: "Pending review",
        message: "This card shows how many commissions are waiting for project-team validation.",
        position: "bottom"
      },
      {
        element: "#readyToExportSummaryCard",
        title: "Ready to export",
        message: "This card shows approved commissions prepared for finance export.",
        position: "bottom"
      }
    );
  }

  steps.push(
    {
      element: "#serviceCommissionTabs",
      title: "Workspace tabs",
      message: workspaceTabsMessage({ canReview, canApproveFinal, canManageRates }),
      position: "bottom"
    },
    {
      element: "#submit-tab",
      title: "Submit expense",
      message: "Use this tab to create a new service commission and register expenses for a trip.",
      position: "bottom"
    },
    {
      element: "#my-commissions-tab",
      title: "My commissions",
      message: "Open this tab to review your previous service commissions and check their status.",
      position: "bottom"
    }
  );

  if (canReview) {
    steps.push({
      element: "#project-review-tab",
      title: "Project review",
      message: "Use this tab to approve, reject, export signed PDFs and download data-exploitation ZIP files for service commissions.",
      position: "bottom"
    });
  }

  if (canApproveFinal) {
    steps.push({
      element: "#stakeholder-approval-tab",
      title: "Final approvals",
      message: "This tab is used when a commission needs your final validation before signature.",
      position: "bottom"
    });
  }

  if (canManageRates) {
    steps.push({
      element: "#rates-management-tab",
      title: "Rates management",
      message: "Use this tab to manage accommodation and per diem rate tables.",
      position: "bottom"
    });
  }

  steps.push(
    {
      element: "#serviceCommissionForm",
      title: "Service commission form",
      message: "This form is used to select or create a travel and add the related expenses.",
      position: "right"
    },
    {
      element: ".travel-source-box",
      title: "Travel information",
      message: "Choose whether to use an existing approved Budgeting travel or create a new manual travel entry.",
      position: "right"
    },
    {
      element: "#expensesContainer",
      title: "Expense details",
      message: "Add the expenses related to the trip. Each expense may require receipts, payment proof or mileage evidence depending on its type.",
      position: "top"
    },
    {
      element: "#addExpenseBtn",
      title: "Add expense",
      message: "Use this button to add another expense line to the same service commission.",
      position: "left"
    },
    {
      element: "#expenseType_1",
      title: "Expense type",
      message: "Select the type of expense. The form adapts automatically depending on the selected expense type.",
      position: "bottom"
    },
    {
      element: "#amount_1",
      title: "Amount",
      message: "Enter the amount of the expense. For mileage, this field is used to enter the distance in kilometers.",
      position: "bottom"
    },
    {
      element: "#description_1",
      title: "Description",
      message: "Describe the expense briefly and explain its relation to the travel.",
      position: "top"
    },
    {
      element: "#receiptFile_1",
      title: "Receipt or invoice",
      message: "Upload the receipt or invoice associated with the expense.",
      position: "top"
    },
    {
      element: "#paymentProofFile_1",
      title: "Payment proof",
      message: "Upload the card, bank or equivalent payment proof for the expense.",
      position: "top"
    },
    {
      element: "#serviceCommissionForm button[type='submit']",
      title: "Submit commission",
      message: "When the form is complete, use this button to submit the service commission for review.",
      position: "top"
    }
  );

  return steps;
}

function workspaceTabsMessage({ canReview, canApproveFinal, canManageRates }) {
  const availableTabs = ["submitting expenses", "reviewing your commissions"];
  if (canReview) availableTabs.push("project review");
  if (canApproveFinal) availableTabs.push("final approvals");
  if (canManageRates) availableTabs.push("rates management");

  return `Use these tabs to move between ${joinWithAnd(availableTabs)}.`;
}

function joinWithAnd(values) {
  if (values.length <= 1) return values[0] || "";
  if (values.length === 2) return `${values[0]} and ${values[1]}`;
  return `${values.slice(0, -1).join(", ")} and ${values[values.length - 1]}`;
}

function tutorialPageKey({ canReview, canApproveFinal, canManageRates }) {
  const suffixes = [];
  if (canReview) suffixes.push("review");
  if (canApproveFinal) suffixes.push("approval");
  if (canManageRates) suffixes.push("rates");

  return suffixes.length
    ? `service-commissions-${suffixes.join("-")}`
    : "service-commissions-user";
}

async function fetchModule10Session() {
  try {
    const response = await fetch(`${MODULE10_API_URL}?action=session`, {
      headers: { "Accept": "application/json" },
      credentials: "same-origin"
    });

    if (!response.ok) return null;
    return await response.json();
  } catch (error) {
    console.error("Error loading module 10 tutorial permissions:", error);
    return null;
  }
}

async function waitForDocumentReady() {
  if (document.readyState !== "loading") return;
  await new Promise((resolve) => document.addEventListener("DOMContentLoaded", resolve, { once: true }));
}

async function waitForWorkspacePermissions(session) {
  await waitForElement("#serviceCommissionTabs");

  for (let attempt = 0; attempt < 30; attempt += 1) {
    if (workspacePermissionsApplied(session)) return;
    await sleep(100);
  }

  await sleep(300);
}

function workspacePermissionsApplied(session) {
  const projectTabItem = document.querySelector("#project-review-tab")?.closest("li");
  const ratesTabItem = document.getElementById("rates-management-tab-item");
  const summaryGrid = document.getElementById("serviceSummaryGrid");

  if (!projectTabItem || !ratesTabItem || !summaryGrid) return false;
  if (!session) return true;

  const canReview = Boolean(session.can_review);
  const canManageRates = Boolean(session.can_manage_rates);
  const projectVisibilityReady = canReview
    ? !projectTabItem.classList.contains("d-none")
    : projectTabItem.classList.contains("d-none");
  const ratesVisibilityReady = canManageRates
    ? !ratesTabItem.classList.contains("d-none")
    : ratesTabItem.classList.contains("d-none");
  const summaryReady = canReview
    ? !summaryGrid.classList.contains("service-summary-grid--single")
    : summaryGrid.classList.contains("service-summary-grid--single");

  return projectVisibilityReady && ratesVisibilityReady && summaryReady;
}

async function waitForElement(selector) {
  if (document.querySelector(selector)) return;

  await new Promise((resolve) => {
    const observer = new MutationObserver(() => {
      if (!document.querySelector(selector)) return;
      observer.disconnect();
      resolve();
    });

    observer.observe(document.documentElement, { childList: true, subtree: true });
    setTimeout(() => {
      observer.disconnect();
      resolve();
    }, 3000);
  });
}

function isTargetAvailable(selector) {
  const target = document.querySelector(selector);
  if (!target) return false;
  if (target.closest("[hidden], .d-none")) return false;

  const rect = target.getBoundingClientRect();
  return rect.width > 0 && rect.height > 0;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

loadModule10Tutorial();
