// -- tutorial_main.js --
import { initTutorial } from "./tutorialEngine.js";

const moduleDescriptions = {
  M0: {
    title: "Administrator module",
    message: "Manage users and system configuration. Here you can create accounts, assign roles, reset passwords, inspect logs and control the status of every module.",
    position: "right"
  },
  M1: {
    title: "Room reservation module",
    message: "Book offices, classrooms, meeting rooms or auditoriums. Review, modify or cancel your reservations from the same panel.",
    position: "right"
  },
  M2: {
    title: "Hot Desks module",
    message: "Reserve flexible workspace desks for short periods or single sessions. Ideal when you only need a temporary working space.",
    position: "top",
  },
  M3: {
    title: "Profile module",
    message: "View your personal information stored at CRM, including academic and professional data. Here, you can also change your password.",
    position: "top",
  },
  M4: {
    title: "Manage Data module",
    message: "Create and maintain data records related to personnel, groups, academic background, announcements and other administrative elements.",
    position: "right",
  },
  M5: {
    title: "Export Data module",
    message: "Generate reports or export information from the CRM internal database, including UNEIX datasets.",
    position: "left",
  },
  M6: {
    title: "Ticketing module",
    message: "Report technical or maintenance issues quickly using support tickets, and follow their progress until resolution.",
    position: "top",
  },
  M7: {
    title: "Distribution Lists module",
    message: "Create mailing lists and send targeted notifications to specific teams, groups or audiences.",
    position: "top",
  },
  M8: {
    title: "Budgeting module",
    message: "Send your budget requests to the appropriate department through this module.",
    position: "top",
  },
  M9: {
    title: "Document viewer module",
    message: "Consult official documents from the center in one place, with access depending on your permissions.",
    position: "top",
  },
  M10: {
    title: "Service commissions module",
    message: "Submit expenses after work trips, attach receipts and payment proof, and follow the review and approval process.",
    position: "top",
  },
  M11: {
    title: "IT Tools module",
    message: "Access internal IT utilities such as the budget generator and other technical tools available to authorized users.",
    position: "top",
  }
};

function buildMainSteps() {
  const steps = [
    {
      element: ".hero-main",
      title: "Welcome to CRMIntratools",
      message: "This platform brings together all essential tools you will need during your time at CRM. Each module provides access to a different service.",
      position: "bottom"
    },
    {
      element: ".hero-side",
      title: "User and permissions",
      message: "This section shows your username and your permission level. Access to certain modules depends on this role.",
      position: "bottom"
    },
    {
      element: "#modulesContainer",
      title: "Available modules",
      message: "Here you will find the list of modules that you can access with your current role.",
      position: "top"
    },
  ];

  const visibleModules = document.querySelectorAll('#modulesContainer [data-module-id]');

  visibleModules.forEach((el, index) => {
    const modId = el.getAttribute("data-module-id");
    const info = moduleDescriptions[modId];
    if (!info) return;

    const tutorialId = `tutorial-module-${index}`;
    el.id = tutorialId;

    steps.push({
      element: `#${tutorialId}`,
      title: info.title,
      message: info.message,
      position: info.position || "center",
    });
  });

  steps.push(
    {
      element: "#notif",
      title: "Notifications center",
      message: "Receive system announcements, ticket updates, HR communications and other alerts.",
      position: "left"
    },
    {
      element: ".filter-buttons",
      title: "Module filters",
      message: "Filter modules by required permission level, useful when many modules are visible.",
      position: "bottom"
    },
    {
      element: "#searchInput",
      title: "Search bar",
      message: "Quickly locate a module by typing its name or description.",
      position: "bottom"
    },
    {
      element: "#modulesContainer",
      message: "You could start exploring by setting a new password on the Profile module.",
      position: "top",
    },
  );

  return steps;
}

const steps = buildMainSteps();
initTutorial(steps, "main", "home");
