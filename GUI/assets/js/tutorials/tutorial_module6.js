import { initTutorial } from "./tutorialEngine.js";

async function loadTicketingTutorial() {

  // obtener rol
  const res = await fetch('/module/proxy/module6/api?action=checkRole');
  const data = await res.json();
  const userRole = data.role;

  const steps = [
    {
      element: "h5.text-danger",
      title: "Ticketing system",
      message: "Welcome to the CRM ticketing system, where you can report issues and follow their status.",
      position: "bottom"
    },
    {
      element: "#sidebar",
      title: "Navigation menu",
      message: "Use these options to navigate between ticket creation and tracking.",
      position: "right"
    },
    {
      element: '[data-section="send"]',
      title: "Send new ticket",
      message: "Click here to open the form to create a new ticket describing your issue.",
      position: "right"
    },
    {
      element: "#section-send",
      title: "Ticket form",
      message: "Fill this form to submit a support request.",
      position: "left"
    },
    {
      element: "#ticketCategory",
      title: "Category",
      message: "Select the type of issue you want to report. Subcategories will appear when needed.",
      position: "bottom"
    },
    {
      element: "#ticketMessage",
      title: "Description",
      message: "Describe the problem in detail and include supporting information like office number or schedule.",
      position: "bottom"
    },
    {
      element: "#notifyByEmail",
      title: "Email notifications",
      message: "Tick this box if you want to receive updates on the status of your ticket by email.",
      position: "right"
    },
    {
      element: 'button[type="submit"]',
      title: "Send ticket",
      message: "Submit your ticket. It will be reviewed and handled by support staff.",
      position: "top"
    },
    {
      element: '[data-section="myTickets"]',
      title: "My Tickets",
      message: "Check your submitted tickets and track their progress in real time.",
      position: "right"
    }
  ];

  // Opciones especiales según rol
  if (userRole === "admin" || userRole === "manager" || userRole === "responsible") {
    steps.push({
      element: '[data-section="admin"]',
      title: "Administrator area",
      message: "Access management tools for departments and ticket assignments.",
      position: "right"
    });
  }

  initTutorial(steps, "module3", "ticketing");
}

loadTicketingTutorial();
