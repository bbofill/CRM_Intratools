//////////////////////////////////////////////////////////////////////////////////////////////////
//                         Users hub: formats workers visualization                             //
//////////////////////////////////////////////////////////////////////////////////////////////////

import { requiredFields, importantFields } from './fieldRequirements.js';

document.addEventListener('DOMContentLoaded', async () => {
  const container = document.getElementById("user-list");

  const res = await fetch("/module/proxy/module4/api?action=allUneixUsersSummary");
  let users = [];
  try {
    users = await res.json();
    if (!Array.isArray(users)) {
      console.warn("La API no devolvió un array. Valor recibido:", users);
      users = [];
    }
    console.log(users)
  } catch (err) {
    console.error("Error parseando JSON de la API:", err);
    users = [];
  }
  const enriched = users.map(user => {
    const isEmpty = (value) => {
      if (value === null || value === undefined) return true;
      return String(value).trim() === "";
    };
    const missingRequired = requiredFields.filter(f => isEmpty(user[f]));
    const missingImportant = importantFields.filter(f => isEmpty(user[f]));
    const hasAllRequired = missingRequired.length === 0;
    const hasAllImportant = missingImportant.length === 0;
    let priority = 0;
    if (!hasAllRequired) priority = 0;        // danger
    else if (!hasAllImportant) priority = 1;  // warning
    else priority = 2;                        // normal
    if (!user.active) priority = 3;           // inactive al final

    return {
      ...user,
      hasAllRequired,
      hasAllImportant,
      priority
    };
  });

  window.users = enriched;
  enriched.sort((a, b) => {
    if (a.priority !== b.priority) return a.priority - b.priority;
    const aSurname = (a.surname || "").toLowerCase();
    const bSurname = (b.surname || "").toLowerCase();
    return aSurname.localeCompare(bSurname);
  });

  const htmlBlocks = enriched.map(user => {
    const {
      people_id,
      people_name,
      surname,
      secondSurname,
      picture_path,
      hasAllRequired,
      hasAllImportant
    } = user;

    const img = picture_path
      ? `/module/proxy/module4/Uploads?file=${encodeURIComponent(picture_path)}`
      : `/module/proxy/module4/Uploads?file=${encodeURIComponent("Uploads/00_Users/default_profile.png")}`;

    let borderClass = "";
    let btnClass = "";
    let iconText = "";

    if (!hasAllRequired) {
      borderClass = "border border-danger";
      btnClass = "btn-danger";
      iconText = "⚠️ Modify";
    } else if (!hasAllImportant) {
      borderClass = "border border-warning";
      btnClass = "btn-warning";
      iconText = "✏️ Modify";
    } else {
      btnClass = "btn-primary";
      iconText = "✏️ Modify";
    }

    const inactiveClass = !user.active ? "inactive-user" : "";

    return `
      <div class="col user-card ${inactiveClass}"
          data-name="${people_name || ''}"
          data-surname="${surname || ''}"
          data-secondsurname="${secondSurname || ''}">
        <div class="card h-100 shadow-sm ${borderClass}">
          <div class="card-body d-flex align-items-center">
            <img src="${img}"
                loading="lazy"
                class="rounded me-3"
                width="60"
                height="60"
                style="object-fit: cover;"
                alt="Profile picture"
                onerror="this.onerror=null;this.src='/module/proxy/module4/Uploads?file=${encodeURIComponent("Uploads/00_Users/default_profile.png")}'">
            <div>
              <h5 class="mb-0">${people_name || ""} ${surname || ""} ${secondSurname || ""}</h5>
              <div class="mt-2">
                <button class="btn btn-sm ${btnClass}" data-id="${people_id}">
                  ${iconText}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>`;
  });
  

  container.innerHTML = htmlBlocks.join('');
  const searchInput = document.getElementById('userSearchInput');
  if (searchInput) attachUserSearch(searchInput);
});

function attachUserSearch(inputEl) {
  inputEl.addEventListener('input', e => {
    const query = e.target.value.trim().toLowerCase();
    const cards = document.querySelectorAll('#user-list .user-card');

    cards.forEach(card => {
      const name = (card.dataset.name || '').toLowerCase();
      const surname = (card.dataset.surname || '').toLowerCase();
      const secSurname = (card.dataset.secondsurname || '').toLowerCase();
      const full = `${name} ${surname} ${secSurname}`;

      const match =
        !query ||                
        name.includes(query) ||
        surname.includes(query) ||
        secSurname.includes(query) ||
        full.includes(query);

      card.style.display = match ? '' : 'none';
    });
  });
}