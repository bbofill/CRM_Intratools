// --- sidebarControl.js ---

let userRole = "guest";
async function getUserRole() {
    try {
      const res = await fetch("/module/proxy/module6/api?action=checkRole");
      const data = await res.json();
      userRole = data.role || "guest";
      localStorage.setItem("role", userRole);
      return userRole;
    } catch (err) {
      console.error("Error fetching role:", err);
      userRole = "guest";
      return userRole;
    }
  }
  
  // --- Obtiene info del usuario y sus departamentos ---
  async function getUserDepartments() {
    try {
      // Info del usuario actual
      const userRes = await fetch("/module/proxy/module6/api?action=getUserInfo");
      const userData = await userRes.json();
      if (!userData || !userData.user_id) {
        console.warn("No user data found");
        return { id: null, departments: [], managed: [] };
      }
  
      const userId = userData.user_id;
  
      // Departamentos en los que el usuario trabaja
      const deptRes = await fetch(`/module/proxy/module6/api?action=getUserDepartments&user_id=${userId}`);
      const departments = await deptRes.json();
  
      // Departamentos que el usuario gestiona
      const managedRes = await fetch(`/module/proxy/module6/api?action=getManagedDepartments&user_id=${userId}`);
      const managedDepts = await managedRes.json();
  
      return { id: userId, departments, managed: managedDepts };
    } catch (err) {
      console.error("Error fetching user departments:", err);
      return { id: null, departments: [], managed: [] };
    }
  }
  
  // --- Inicialización principal ---
  getUserRole().then(async (role) => {
    initializeSidebar(role);
  
    if (role !== "guest") {
      const { departments, managed } = await getUserDepartments();
  
      // Si pertenece a algún departamento → añadir Work Area
      if (Array.isArray(departments) && departments.length > 0) {
        addWorkAreaOption();
      }
  
      // Si gestiona alguno → añadir Department Admin
      if (Array.isArray(managed) && managed.length > 0) {
        addDepartmentAdminOption();
      }
    }
  });
  

  function initializeSidebar(userRole) {
    const adminLink = document.querySelector('[data-section="admin"]');
    if (userRole !== "admin") {
      adminLink.classList.add("disabled");
      adminLink.style.pointerEvents = "none";
      adminLink.style.opacity = "0.6";
    }
  }

  
// Elementos base
const sidebar = document.getElementById('sidebar');
const contentArea = document.getElementById('content-area');


// Función para mostrar solo una sección
function showSection(target) {
    const sections = document.querySelectorAll(".content-section");
    sections.forEach((s) => s.classList.add("d-none"));
  
    const targetSection = document.querySelector(`#section-${target}`);
    if (targetSection) {
      targetSection.classList.remove("d-none");
    }
  }

// Función principal para manejar clics del sidebar (delegación de eventos)
sidebar.addEventListener("click", async (e) => {
    const link = e.target.closest(".nav-link");
    if (!link) return;
  
    e.preventDefault();
    const target = link.dataset.section;
  
    // --- Control de acceso ---
    if (target === "admin" && userRole !== "admin") {
      showNotAllowedMessage();
      return;
    }
  
    // --- Activar enlace seleccionado ---
    sidebar.querySelectorAll(".nav-link").forEach((l) => l.classList.remove("active"));
    link.classList.add("active");

    // --- Mostrar sección ---
    showSection(target);

    // --- Importar JS solo cuando sea necesario ---
  if (target === "deptAdmin") {
    const module = await import("./departmentAdmin.js");
    if (module?.initDepartmentAdmin) {
      module.initDepartmentAdmin(); // función opcional de inicialización
    }
  }

  if (target === "workArea") {
    const module = await import("./workArea.js");
    if (module?.loadWorkTickets) {
      module.loadWorkTickets(); // función que carga los tickets asignados
    }
  }
  
  });

// Mostrar mensaje de no autorizado
function showNotAllowedMessage() {
  contentArea.innerHTML = `
    <div class="text-center mt-5">
      <h3 class="text-danger">You are not allowed in the administrator area</h3>
      <p class="text-muted">Please contact your system administrator if you think this is an error.</p>
    </div>
  `;
}

// Añadir Work Area dinámicamente
function addWorkAreaOption() {
    if (sidebar.querySelector('[data-section="workArea"]')) return;
  
    const newLink = document.createElement("a");
    newLink.className = "nav-link";
    newLink.href = "#";
    newLink.dataset.section = "workArea";
    newLink.textContent = "🧰 Work area";
  
    sidebar.querySelector(".nav").appendChild(newLink);
  
    if (!document.getElementById("section-workArea")) {
      const section = document.createElement("div");
      section.id = "section-workArea";
      section.className = "content-section d-none";
      section.innerHTML = `
        <h3 class="text-danger mb-3">Work area</h3>
        <p>Welcome to the work area. Here you can manage tickets assigned to your department.</p>
        <div class="alert alert-info mt-3">
          <strong>Tip:</strong> You will see only tickets from your departments.</div>
      `;
      contentArea.appendChild(section);
    }
  }


// Exponer funciones necesarias al ámbito global (porque usamos type="module")
function addDepartmentAdminOption() {
    if (sidebar.querySelector('[data-section="deptAdmin"]')) return;
  
    const newLink = document.createElement("a");
    newLink.className = "nav-link";
    newLink.href = "#";
    newLink.dataset.section = "deptAdmin";
    newLink.textContent = "🏢 Department Administrator";
  
    sidebar.querySelector(".nav").appendChild(newLink);
  }
  


window.showSection = showSection;

