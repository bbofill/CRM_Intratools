// ------------------ Subcategorías dinámicas ------------------

const subcategoryOptions = {
    IT: [
      "Hardware request",
      "Computer not working",
      "Intranet problems",
      "CRMIntratools problems",
      "Email problems",
      "Network connection",
      "Software installation",
      "Printer issue",
      "Other"
    ],
    Maintenance: [
      "Light not working",
      "Broken furniture",
      "Temperature issue",
      "Other"
    ]
  };
  
  const categorySelect = document.getElementById("ticketCategory");
  const subcategoryContainer = document.getElementById("subcategoryContainer");
  const subcategorySelect = document.getElementById("ticketSubcategory");
  const descriptionInput = document.getElementById("ticketMessage");
  
  // --- Cargar lista de departamentos dinámicamente ---
  async function loadDepartments() {
    try {
      const res = await fetch("/module/proxy/module6/api?action=getDepartments");
      if (!res.ok) throw new Error(`Error fetching departments: ${res.status}`);
      const departments = await res.json();
  
      // Limpia opciones previas
      categorySelect.innerHTML = `<option value="">Select a category...</option>`;
  
      if (!departments.length) {
        const opt = document.createElement("option");
        opt.disabled = true;
        opt.textContent = "No departments available";
        categorySelect.appendChild(opt);
        return;
      }
  
      // Añadir departamentos dinámicos
      departments.forEach(dept => {
        const opt = document.createElement("option");
        opt.value = dept.name;
        opt.textContent = dept.name;
        categorySelect.appendChild(opt);
      });
  
    } catch (err) {
      console.error("Error loading departments:", err);
      categorySelect.innerHTML = `<option disabled>Error loading departments</option>`;
    }
  }
  
  // --- Mostrar/ocultar subcategorías ---
  categorySelect.addEventListener("change", () => {
    const selected = categorySelect.value;
  
    // Limpia opciones anteriores
    subcategorySelect.innerHTML = "";
  
    if (selected && subcategoryOptions[selected]) {
      // Añade nuevas opciones
      subcategoryOptions[selected].forEach(optionText => {
        const opt = document.createElement("option");
        opt.value = optionText;
        opt.textContent = optionText;
        subcategorySelect.appendChild(opt);
      });
  
      subcategoryContainer.classList.remove("d-none");
    } else {
      // Oculta el campo si no hay subcategorías definidas
      subcategoryContainer.classList.add("d-none");
  
      // Mueve el foco directamente a la descripción
      if (descriptionInput) descriptionInput.focus();
    }
  });
  
  // --- Activar/desactivar email ---
  const notifyCheckbox = document.getElementById("notifyByEmail");
  const emailInput = document.getElementById("notificationEmail");
  
  if (notifyCheckbox && emailInput) {
    notifyCheckbox.addEventListener("change", () => {
      emailInput.disabled = !notifyCheckbox.checked;
      if (!notifyCheckbox.checked) emailInput.value = "";
    });
  }
  
  // --- Inicialización ---
  document.addEventListener("DOMContentLoaded", () => {
    loadDepartments(); // Cargar departamentos al inicio
  });
  