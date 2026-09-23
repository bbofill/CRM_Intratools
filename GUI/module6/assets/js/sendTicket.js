// --- sendTicket.js ---
// Maneja el envío del formulario de nuevo ticket

document.addEventListener("DOMContentLoaded", () => {
    const form = document.getElementById("ticketForm");
    if (!form) return;
  
    const notifyByEmailCheckbox = document.getElementById("notifyByEmail");
    const emailInput = document.getElementById("notificationEmail");
  
    // Cargar emails desde backend

    let crmEmail = "";
    let userEmail = "";
  
    async function fetchUserEmails() {
      try {
        const res = await fetch("/module/proxy/module6/api?action=getUserEmails");
        const data = await res.json();
        if (data.status === "ok") {
          crmEmail = data.crm_email || "";
          userEmail = data.user_email || "";
        }
      } catch (err) {
        console.warn("No se pudieron obtener los emails del usuario:", err);
      }
    }
  
    // Llamamos al cargar la página
    fetchUserEmails();
  
    // Cuando el usuario marca/desmarca "Enviar por email"
    notifyByEmailCheckbox.addEventListener("change", () => {
      if (notifyByEmailCheckbox.checked) {
        // Rellenar con el email disponible
        if (crmEmail) {
          emailInput.value = crmEmail;
        } else if (userEmail) {
          emailInput.value = userEmail;
        } else {
          emailInput.value = ""; // libre
        }
  
        emailInput.removeAttribute("disabled");
        emailInput.setAttribute("placeholder", "Enter your email address");
      } else {
        // Si se desmarca, limpiar y desactivar
        emailInput.value = "";
        emailInput.setAttribute("disabled", true);
      }
    });

    function sanitizeInput(str) {
      return str
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/script/gi, "")
        .trim();
    }
    
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
  
      // Campos base
      const category = document.getElementById("ticketCategory").value;
      const issueSelect = document.getElementById("ticketSubcategory");
      const issue = issueSelect && !issueSelect.classList.contains("d-none") ? issueSelect.value : "General issue";
      let description = document.getElementById("ticketMessage").value.trim();
      const email = document.getElementById("notificationEmail").value.trim();
      const notifyByEmail = document.getElementById("notifyByEmail").checked;
      const attachment = document.getElementById("ticketAttachment").files[0];
  
      if (!category || !description) {
        alert("Please select a category and provide a description.");
        return;
      }

      description = sanitizeInput(description);

      const allowedTypes = [
        "application/pdf",
        "image/jpeg", "image/png",
        "text/plain"
      ];

      const maxSize = 5 * 1024 * 1024;

      if (attachment) {
        if (!allowedTypes.includes(attachment.type)) {
          alert("File type not allowed. Only PDF, JPG or PNG are accepted.");
          return;
        }

        if (attachment.size > maxSize) {
          alert("File is too large (max 5MB).");
          return;
        }
      }
  
      // Crear FormData (permite archivo)
      const formData = new FormData();
      formData.append("department", category);
      formData.append("issue", issue);
      formData.append("description", description);
      if (notifyByEmail && email) formData.append("email", email);
      if (attachment) formData.append("file", attachment);
  
      try {
        const res = await fetch("/module/proxy/module6/api?action=createTicket", {
          method: "POST",
          body: formData,
        });
  
        const data = await res.json();
        if (data.status === "ok") {
          alert(`Ticket created successfully!`);
          form.reset();
          document.getElementById("subcategoryContainer").classList.add("d-none");
        } else {
          alert(`Error creating ticket: ${data.message || "Unknown error"}`);
        }
      } catch (err) {
        console.error("Error submitting ticket:", err);
        alert(`Server error while submitting the ticket: ${data.message || "Unknown error"}`);
      }
    });
  });
  