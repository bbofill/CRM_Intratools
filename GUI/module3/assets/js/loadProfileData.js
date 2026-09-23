//////////////////////////////////////////////////////////////////////////////////////////////////
//                        Gets all user information from backend                                //
//////////////////////////////////////////////////////////////////////////////////////////////////


import { fetchOptions } from "./formOptions.js";
import { userFieldSections } from "./fieldConfig.js";
import { renderProfileSection } from "./renderProfileField.js";

export async function loadProfileSections(secciones, container) {
  await fetchOptions();
  const res = await fetch("/module/proxy/module3/api?action=profile");
  const data = await res.json();

  const general = data.general || {};
  document.getElementById("fullname").textContent = `${general.people_name || ""} ${general.surname || ""} ${general.secondSurname || ""}`;

  if (general.picture_path) {
    const response = await fetch(`/module/proxy/module3/Uploads?file=${encodeURIComponent(general.picture_path)}`);
    if (response.ok) {
      const blob = await response.blob();
      const imageUrl = URL.createObjectURL(blob);
      document.getElementById("profile-img").src = imageUrl;
    }
  }

  // recorrer secciones
  let html = "";
  for (const [sectionName, fields] of Object.entries(userFieldSections)) {
    const sectionData = { ...general, ...data };
    html += renderProfileSection(sectionName, fields, sectionData);
  }

  container.innerHTML = `<div>${html}</div>`;
}
