import { initTutorial } from "./tutorialEngine.js";

async function loadModule2Tutorial() {
  const res = await fetch('/module/proxy/module2/api?action=checkRole');
  const data = await res.json();
  const userRole = data.role;

  const isAvailableTablesPage = !!document.querySelector("#mesasTable");

  if (!isAvailableTablesPage) {
    const steps = [
        {
        element: "h1.text-danger",
        title: "Hot Desks Service",
        message: "Here you can reserve shared workspace tables to work temporarily at CRM.",
        position: "bottom"
        },
        {
        element: "#form .row",
        title: "Book by time",
        message: "Choose the date and time when your reservation takes place to find available tables.",
        position: "bottom"
        },
        {
        element: ".image-row",
        title: "Fast booking by floor",
        message: "If you know where you want to work, click on a floor map to book instantly for the next two hours.",
        position: "top"
        },
        {
        element: 'a[href="modifyReservation.html"]',
        title: "Modify my reservations",
        message: "You can modify or cancel your reservations here.",
        position: "right"
        },
        {
        element: 'a[href="checkReservations.html"]',
        title: "Check your reservations",
        message: "Here you can review all your active bookings.",
        position: "bottom"
        },

    ];

    if (userRole === "admin" || userRole === "manager") {
        steps.push({
        element: "#viewAllReservationsBtn",
        title: "View all reservations",
        message: "Admins can inspect and manage all current reservations in the system.",
        position: "right"
        });
    }

    initTutorial(steps, "module2", "hottables");
    return
  }

const stepsTables = [
    {
      element: "h2.text-danger",
      title: "Available Tables",
      message: "These are the tables available with the range of dates and times previously selected.",
      position: "bottom"
    },
    {
      element: "#mesasTable th:nth-child(1)",
      title: "Table ID",
      message: "This is the unique identifier of the table. Use it to reference the location or ask support.",
      position: "bottom"
    },
    {
      element: "#mesasTable th:nth-child(2)",
      title: "Capacity",
      message: "Indicates how many people can use this table at the same time.",
      position: "bottom"
    },
    {
      element: "#mesasTable th:nth-child(3)",
      title: "Floor",
      message: "Indicates the floor on which the table is located: floor 0, 1 or 2.",
      position: "bottom"
    },
    {
      element: "#mesasTable th:nth-child(4)",
      title: "Start time",
      message: "Shows when your reservation would begin for this table.",
      position: "bottom"
    },
    {
      element: "#mesasTable th:nth-child(5)",
      title: "End time",
      message: "Shows when your reservation would finish for this table.",
      position: "bottom"
    },
    {
      element: "button.reservar-btn",
      title: "Book button",
      message: "Press this button to continue to the confirmation screen and finalize your table booking.",
      position: "left"
    }
  ];

  initTutorial(stepsTables, "module2", "available");

}

loadModule2Tutorial();
