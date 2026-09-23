import { initTutorial } from "./tutorialEngine.js";

async function loadTutorial() {
  const res = await fetch('/module/proxy/module1/api?action=checkRole');
  const data = await res.json();
  const userRole = data.role;

  const steps = [
    {
      element: "h1.text-danger",
      title: "Room Reservation",
      message: "Here you can manage bookings for available rooms.",
      position: "bottom"
    },
    {
      element: "p",
      title: "Reservation Policy",
      message: "Please provide a clear and justified purpose for each reservation request.",
      position: "bottom"
    },
    {
      element: "#dateform",
      title: "Dates",
      message: "Choose the date and time when your reservation takes place. If you leave these values empty, no room will be shown.",
      position: "bottom"
    },
    {
      element: "#repeatReservationBtn",
      title: "Repeating reservations",
      message: "You may create recurring reservations (weekly, monthly, or yearly). This option will remain blocked until you choose a date.",
      position: "right"
    },
    {
      element: "#roomCode",
      title: "Available rooms",
      message: "After selecting a range of dates and times, the available rooms will appear here.",
      position: "top"
    },
  ];

  // Adaptar pasos según rol
  if (userRole === "admin" || userRole === "manager") {
    steps.push(
    {
      element: ".button-container",
      title: "Choose a room category",
      message: "Select the type of space you want to reserve. If you don't choose any type, all available options will be shown.",
      position: "bottom"
    },{
      element: "#viewAllReservationsBtn",
      title: "View all reservations",
      message: "Inspect all system reservations and filter or review them.",
      position: "right"
    });
  } else {
    steps.push({
      element: "#modifyUserReservationBtn",
      title: "Modify my reservations",
      message: "View and modify your personal reservations.",
      position: "right"
    });
  }

  initTutorial(steps, "module1", "reservation");
}

loadTutorial();
