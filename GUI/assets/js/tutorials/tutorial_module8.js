import { initTutorial } from "./tutorialEngine.js";

async function loadModule8Tutorial() {
  const isConfigurationPage =
    !!document.querySelector("#projectUsers") &&
    !!document.querySelector("#financeUsers");

  if (isConfigurationPage) {
    const steps = [
      {
        element: ".card-title",
        title: "Budgeting configuration",
        message: "This page lets administrators define who can manage project approvals and accounting reviews in the budgeting module.",
        position: "bottom"
      },
      {
        element: "#projectUsers",
        title: "Project users",
        message: "Select the users allowed to manage project-related budgeting requests.",
        position: "bottom"
      },
      {
        element: "#financeUsers",
        title: "Accounting users",
        message: "Select the users responsible for accounting validation and financial review.",
        position: "bottom"
      },
      {
        element: "#saveAccess",
        title: "Save changes",
        message: "When you finish assigning permissions, save them here so the new access rules apply to the module.",
        position: "top"
      }
    ];

    initTutorial(steps, "module8", "configuration");
    return;
  }

  let permissions = {};
  try {
    const res = await fetch("/module/proxy/module8/api?action=getBudgetPermissions");
    permissions = await res.json();
  } catch (error) {
    console.error("Error loading budgeting permissions for tutorial:", error);
  }

  const canManage =
    Boolean(permissions?.isIP) ||
    Boolean(permissions?.isProjects) ||
    Boolean(permissions?.isAccounting) ||
    Boolean(permissions?.isIT);

  const canSeeAll =
    Boolean(permissions?.isAdmin) ||
    Boolean(permissions?.isProjects) ||
    Boolean(permissions?.isAccounting) ||
    Boolean(permissions?.isIT);

  const steps = [
    {
      element: "#sidebar h5",
      title: "Budgeting module",
      message: "Welcome to the budgeting area. From here you can create requests and follow their approval process.",
      position: "right"
    },
    {
      element: "#sidebar",
      title: "Navigation menu",
      message: "Use this sidebar to move between request areas.",
      position: "right"
    },
    {
      element: '[data-section="newRequest"]',
      title: "New request",
      message: "This option opens the form to create a new budgeting request.",
      position: "right"
    },
    {
      element: "#category-ticks",
      title: "Request categories",
      message: "Choose one or more categories for the request. Equipment is exclusive and cannot be combined with the others.",
      position: "bottom"
    },
    {
      element: "#details-accordion",
      title: "Dynamic details",
      message: "After selecting categories, the corresponding request sections appear here.",
      position: "top"
    },
    {
      element: "#submit-btn",
      title: "Submit request",
      message: "When the form is complete, use this button to send the request for review.",
      position: "top"
    },
    {
      element: "#reset-btn",
      title: "Reset form",
      message: "Use this button to clear the form and start again.",
      position: "top"
    },
    {
      element: '[data-section="sentRequests"]',
      title: "Sent requests",
      message: "Open this section to review the requests you have already submitted and check their status.",
      position: "right"
    }
  ];

  if (canManage) {
    steps.push({
      element: "#managementAreaNav",
      title: "Management area",
      message: "Users with management permissions can open this section to review pending requests.",
      position: "right"
    });
  }

  if (canSeeAll) {
    steps.push({
      element: "#allRequestsNav",
      title: "All requests",
      message: "Privileged users can access the complete list of budgeting requests from here.",
      position: "right"
    });
  }

  initTutorial(steps, "module8", "budgeting");
}

loadModule8Tutorial();