const managementAreaNav = document.getElementById("managementAreaNav");
const allRequestsNav = document.getElementById("allRequestsNav");

async function loadUserPermissions() {
  try {
    const res = await fetch("/module/proxy/module8/api?action=getBudgetPermissions");
    const data = await res.json();

    const canManage =
      Boolean(data?.isIP) ||
      Boolean(data?.isProjects) ||
      Boolean(data?.isAccounting) ||
      Boolean(data?.isIT) ||
      Boolean(data?.isGerencia);

    const isAdmin = Boolean(data?.isAdmin)  ||
      Boolean(data?.isProjects) ||
      Boolean(data?.isAccounting) ||
      Boolean(data?.isIT) ||
      Boolean(data?.isGerencia);

    console.log("User permissions:", data, "Can manage:", canManage, "Is admin:", isAdmin);

    managementAreaNav?.classList.toggle("d-none", !canManage);
    allRequestsNav?.classList.toggle("d-none", !isAdmin);

    document
      .getElementById("section-viewRequests")
      ?.classList.toggle("d-none", !canManage);

    document
      .getElementById("section-allRequests")
      ?.classList.toggle("d-none", !isAdmin);

  } catch (error) {
    console.error("Error loading budgeting permissions:", error);
    managementAreaNav?.classList.add("d-none");
    allRequestsNav?.classList.add("d-none");
  }
}

document.addEventListener("DOMContentLoaded", async () => {
  const links = document.querySelectorAll("#sidebar .nav-link[data-section]");
  const sections = document.querySelectorAll("#content-area .content-section");

  function show(sectionKey, updateUrl = true) {
    if (!document.getElementById(`section-${sectionKey}`)) {
      sectionKey = "newRequest";
    }

    sections.forEach(s => s.classList.add("d-none"));
    const target = document.getElementById(`section-${sectionKey}`);
    if (target) target.classList.remove("d-none");

    links.forEach(l => l.classList.remove("active"));
    const active = document.querySelector(`#sidebar .nav-link[data-section="${sectionKey}"]`);
    if (active) active.classList.add("active");

    if (updateUrl) {
      window.history.replaceState(null, "", `#${sectionKey}`);
    }
  }

  links.forEach(l => {
    l.addEventListener("click", (e) => {
      e.preventDefault();
      show(l.dataset.section);
    });
  });

  await loadUserPermissions();

  const initialSection = getInitialSectionKey();
  show(initialSection, false);

  window.addEventListener("hashchange", () => {
    show(getInitialSectionKey(), false);
  });
});

function getInitialSectionKey() {
  const params = new URLSearchParams(window.location.search);
  const sectionParam = params.get("section");
  if (sectionParam) return sectionParam;

  const hashSection = window.location.hash.replace(/^#/, "");
  if (hashSection) return hashSection;

  return "newRequest";
}
