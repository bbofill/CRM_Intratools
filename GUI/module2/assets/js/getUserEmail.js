window.addEventListener("DOMContentLoaded", () => {
  fetch("/module/proxy/module2/api?action=getUserEmails", {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  })
    .then((response) => response.json())
    .then((data) => {
      if (data.status === "ok") {
        const crmEmail = data.crm_email || "";
        const userEmail = data.user_email || "";
        if (crmEmail) {
          document.getElementById("email").value = crmEmail;
        } else if (userEmail) {
          document.getElementById("email").value = userEmail;
        }
      } else {
        return;
      }
    })
    .catch((error) => {
      console.error("Error al obtener el correo del usuario:", error);
    });
});