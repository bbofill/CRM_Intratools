document.addEventListener("DOMContentLoaded", () => {
  const startDateInput = document.getElementById("StartDate");
  const startTimeInput = document.getElementById("StartTime");
  const endDateInput = document.getElementById("EndDate");
  const endTimeInput = document.getElementById("EndTime");
  const repeatReservationBtn = document.getElementById("repeatReservationBtn");
  const repeatingReservationForm = document.getElementById("repeatingReservationForm");
  
  // Function to check if all date and time fields are filled
  function checkFields() {
    const startDate = startDateInput.value;
    const startTime = startTimeInput.value;
    const endDate = endDateInput.value;
    const endTime = endTimeInput.value;

    if (startDate && startTime && endDate && endTime) {
      repeatReservationBtn.disabled = false; // Enable the button
    } else {
      repeatReservationBtn.disabled = true; // Keep the button disabled
    }
  }

  // Attach the checkFields function to the change events of the date and time inputs
  startDateInput.addEventListener("change", checkFields);
  startTimeInput.addEventListener("change", checkFields);
  endDateInput.addEventListener("change", checkFields);
  endTimeInput.addEventListener("change", checkFields);

  // Initial check in case the fields have pre-filled values (for example, if it's preloaded)
  checkFields();

  // Handle the button click to toggle showing the repeating reservation options
  repeatReservationBtn.addEventListener("click", (e) => {
    e.preventDefault();

    // Check if the form is already visible
    if (repeatingReservationForm.style.display === "block") {
      // If it's visible, hide it and clear the form
      repeatingReservationForm.style.display = "none";
      clearRepeatingForm();
      repeatReservationBtn.disabled = false; // Keep the button enabled to show options later
    } else {
      // If it's not visible, show it and set the end date to be the same as the start date
      const startDate = startDateInput.value;
      const startTime = startTimeInput.value;

      if (!startDate || !startTime) {
        alert("Please select a valid start time.");
        return;
      }
      const date = new Date(startDate);
      const endPlusOne = new Date(date);
      endPlusOne.setDate(endPlusOne.getDate() + 1);
      const formattedEnd = endPlusOne.toISOString().split("T")[0];


      document.getElementById("weeklyEndDate").value = formattedEnd;
      document.getElementById("monthlyEndDate").value = formattedEnd;
      document.getElementById("yearlyEndDate").value = formattedEnd;

      // Show the repeating options form
      repeatingReservationForm.style.display = "block";

        // Default weekly checkbox selection

        const weekday = date.getDay();
        const dayMap = {
        1: "monday",
        2: "tuesday",
        3: "wednesday",
        4: "thursday",
        5: "friday"
        };

        const weeklyCheckboxes = document.querySelectorAll("#weeklyOptions input[type='checkbox']");
        weeklyCheckboxes.forEach(cb => cb.checked = false);

        if (dayMap[weekday]) {
        document.getElementById(dayMap[weekday]).checked = true;
        }

      // Show the corresponding options based on frequency selection
      const frequency = document.getElementById("frequency").value;
      showRepeatingOptions(frequency);
    }
  });

  // Show options based on frequency selection
  function showRepeatingOptions(frequency) {
    document.getElementById("weeklyOptions").style.display = "none";
    document.getElementById("monthlyOptions").style.display = "none";
    document.getElementById("yearlyOptions").style.display = "none";

    if (frequency === "weekly") {
      document.getElementById("weeklyOptions").style.display = "block";
    } else if (frequency === "monthly") {
      document.getElementById("monthlyOptions").style.display = "block";
    } else if (frequency === "yearly") {
      document.getElementById("yearlyOptions").style.display = "block";
    }
  }

  // Clear the repeating reservation form
  function clearRepeatingForm() {
    // Clear all inputs and reset to defaults
    document.getElementById("frequency").value = "weekly";
    document.getElementById("weeklyDuration").value = "";
    document.getElementById("weeklyEndDate").value = "";
    document.getElementById("monthlyDuration").value = "";
    document.getElementById("monthlyEndDate").value = "";
    document.getElementById("yearlyDuration").value = "";
    document.getElementById("yearlyEndDate").value = "";

    // Hide all the frequency-specific options
    document.getElementById("weeklyOptions").style.display = "none";
    document.getElementById("monthlyOptions").style.display = "none";
    document.getElementById("yearlyOptions").style.display = "none";

    // Uncheck checkboxes for weekly options
    const weeklyCheckboxes = document.querySelectorAll("#weeklyOptions input[type='checkbox']");
    weeklyCheckboxes.forEach(checkbox => checkbox.checked = false);
  }

  // Update the frequency options based on user input
  document.getElementById("frequency").addEventListener("change", (e) => {
    const frequency = e.target.value;
    showRepeatingOptions(frequency);
  });

  // Get the selected month repeat type
  document.querySelector('input[name="monthRepeatType"]').addEventListener('change', function () {
    const repeatBy = document.querySelector('input[name="monthRepeatType"]:checked').id === "monthByDay" ? "month_day" : "month_weekday";
    document.getElementById("repeatBy").value = repeatBy;
  });
});

