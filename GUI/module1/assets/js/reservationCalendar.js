let calendarInstance = null;

document.getElementById("openCalendarBtn").addEventListener("click", () => {
    const modalEl = document.getElementById("calendarModal");
    const modal = new bootstrap.Modal(modalEl);
    modal.show();
  
    modalEl.addEventListener(
      "shown.bs.modal",
      () => {
        if (!calendarInstance) {
          initCalendar();
        } else {
          calendarInstance.updateSize(); 
        }
      },
      { once: true }
    );
  });
  

  function initCalendar() {
    const calendarEl = document.getElementById("reservationCalendar");
  
    calendarInstance = new FullCalendar.Calendar(calendarEl, {
        initialView: "dayGridMonth",
        height: "auto",
        firstDay: 1,
      
        fixedWeekCount: false,
        locale: "en",
        timeZone: "local",
        eventDisplay: "block", 

        eventTimeFormat: {
            hour: "2-digit",
            minute: "2-digit",
            hour12: false
        },
      
        headerToolbar: {
          left: "prev,next today",
          center: "title",
          right: ""
        },
        events: fetchCalendarEvents,
      
        eventDidMount(info) {
          if (info.event.extendedProps.tooltip) {
            new bootstrap.Tooltip(info.el, {
              title: info.event.extendedProps.tooltip,
              placement: "top",
              trigger: "hover",
              container: "body",
              html: true 
            });
          }
        }
      });
      
  
    calendarInstance.render();
  }
  

async function fetchCalendarEvents(info, successCallback, failureCallback) {
    try {
      const res = await fetch(
        "/module/proxy/module1/api?action=getReservationsCalendar",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            start: info.startStr,
            end: info.endStr
          })
        }
      );
  
      if (!res.ok) {
        throw new Error("Server error");
      }
  
      const data = await res.json();
  
      const events = data.map(r => {
        const colors = roomColor(r.uab_code);
      
        return {
          title: r.name,
          start: r.start_date.replace(/Z$/, ""),
          end: r.end_date.replace(/Z$/, ""),
          backgroundColor: colors.bg,
          borderColor: colors.bg,
          textColor: colors.text,
          extendedProps: {
            tooltip: `
            <strong>${r.name}</strong> - ${r.uab_code}<br>
            ${r.start_date.slice(11, 16)} - ${r.end_date.slice(11, 16)}<br>
            ${r.concept || "No reason"}<br>
            <i>${r.email || "Unknown booker"}</i>
          `
        }
    };
  });
      
  
      successCallback(events);
    } catch (e) {
      console.error("Calendar load error:", e);
      failureCallback(e);
    }
  }
  
  const ROOM_COLORS = {
    "C1/022": { bg: "#E0E7FF", text: "#1E3A8A" }, // Sala de reunions – pastel indigo
    "C3b/-106": { bg: "#F3E8FF", text: "#5B21B6" }, // Aula petita – pastel violet
    "C1/028": { bg: "#DCFCE7", text: "#166534" }, // POL 1 – pastel green
    "C3b/-104": { bg: "#FEF3C7", text: "#92400E" }, // POL 2 – pastel amber
    "C3b/-110": { bg: "#FEE2E2", text: "#991B1B" }, // Auditorium – pastel red
    "C1/014": { bg: "#ECFEFF", text: "#155E75" }, // Office 0 – pastel cyan
    "C3b/-102": { bg: "#DBEAFE", text: "#1E40AF" }
  };  

  
  function roomColor(uabCode) {
    return ROOM_COLORS[uabCode] || {
      bg: "#F3F4F6",    
      text: "#374151"
    };
  }
  