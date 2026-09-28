//////////////////////////////////////////////////////////////////////////////////////////////////
//                             Formats field content DB to FrontEnd                             //
//////////////////////////////////////////////////////////////////////////////////////////////////


import { formOptions } from "./formOptions.js";
import { formSources, userFieldSections } from "./fieldConfig.js";

function formatDate(value) {
  if (!value) return "";
  const d = new Date(value);
  return isNaN(d) ? value : d.toLocaleDateString("es-ES", { year: "numeric", month: "short", day: "numeric" });
}

function resolveDisplayValue(key, value, type) {
  if (value == null || value === "") return "";

  // fechas
  if (type === "date") return formatDate(value);
  if (type === "year") return value; 

  // horas
  if (key === "totalDedication_hours") {
    const hours = Number(value) / 100;
    return `${hours.toLocaleString("es-ES", { minimumFractionDigits: 1, maximumFractionDigits: 2 })} h/week`;
  }

  // booleanos -> iconos
  if (type === "boolean") {
    return (value == 1 || value === "1" || value === true)
      ? `<span class="text">Yes</span>`
      : `<span class="text">No</span>`;
  }

  // archivos -> link
  if (type === "file") {
    const filePath = value;
    const fileName = filePath.split("/").pop(); // nombre corto
    const fileUrl = `/module/proxy/module3/Uploads?file=${encodeURIComponent(filePath)}`;

    return `
      <div class="d-flex align-items-center gap-2">
        <i class="bi bi-file-earmark-pdf text-danger fs-5"></i>
        <a href="${fileUrl}" target="_blank" class="link-primary">${fileName}</a>
      </div>`;
  }

  // opciones definidas en formSources
  const source = formSources[key];
  if (!source) return value;

  const list = Array.isArray(source)
    ? source.flatMap(s => formOptions[s] || [])
    : formOptions[source] || [];

  const match = list.find(opt => opt.code == value);
  return match ? match.name : value;
}

// Renderiza un array de campos (sección o subcampos)
function renderFields(fields, data, insideRepeater = false) {
    return fields.map(f => {
      if (f.type === "repeater") {
        const arr = data[f.key] || [];

        if (!Array.isArray(arr) || arr.length === 0) return "";
  
        const subFields = userFieldSections[f.label + "Item"] || [];
  
        return `
          <div class="w-100 mt-3"> 
            <div class="row g-3">
              ${arr
                .map((entry, idx) => {
                  const items = renderFields(subFields, entry, true); // insideRepeater = true
                  const colClass = arr.length === 1 ? "col-12" : "col-12 col-md-6";
  
                  return `
                    <div class="${colClass} d-flex">
                      <div class="card flex-fill border-0 shadow-sm bg-light">
                        <div class="card-header small fw-bold bg-white">${f.label} #${idx + 1}</div>
                        <div class="card-body p-2">
                          <ul class="list-unstyled mb-0">
                            ${items}
                          </ul>
                        </div>
                      </div>
                    </div>`;
                })
                .join("")}
            </div>
          </div>`;
      } else {
        const display = resolveDisplayValue(f.key, data[f.key], f.type);
        const colClass = insideRepeater ? "col-12" : "col-12 col-md-6";
  
        return `
          <li class="${colClass} mb-1"> 
            <span class="fw-semibold text-secondary">${f.label}:</span>
            <span class="ms-1">${display}</span>
          </li>`;
      }
    }).join("");
  }
  
  // Renderiza una sección completa
  export function renderProfileSection(sectionName, fields, data) {
    if (!fields || fields.length === 0) return "";
    if (sectionName.endsWith("Item")) return "";
  
    const items = renderFields(fields, data);
  
    if (!items.trim()) return "";

    return `
      <div class="card shadow-sm mb-4">
        <div class="card-header bg-primary bg-opacity-10">
          <h5 class="mb-0">${sectionName}</h5>
        </div>
        <div class="card-body">
          <ul class="list-unstyled row g-3 mb-0">
            ${items}
          </ul>
        </div>
      </div>`;
  }
  