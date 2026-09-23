//////////////////////////////////////////////////////////////////////////////////////////////////
//                This file contains all the dependencies between fields                        //
//                             (why they are locked-unlocked)                                   //
//                                  and auxiliar functions                                      //
//////////////////////////////////////////////////////////////////////////////////////////////////


import { formOptions } from './formOptions.js';


// Function to visually lock or unlock a field
function setDisabled(block, selectorArray, disabled) {
  selectorArray.forEach(sel => {
    const elements = block.querySelectorAll(sel);

    elements.forEach(el => {
      el.disabled = disabled;
      const isTextInput = el.tagName === "INPUT" && (el.type === "text" || el.getAttribute("type") === "text");
      if (isTextInput) {
        if (disabled) {
          el.setAttribute("readonly", true);
          el.classList.add("disabled");
        } else {
          el.removeAttribute("readonly");
          el.classList.remove("disabled");
        }
      }
      const displayId = el.id ? `${el.id}_display` : null;
      if (displayId) {
        const displayEl = document.getElementById(displayId);
        if (displayEl) {
          displayEl.disabled = disabled;
          if (disabled) {
            displayEl.setAttribute("readonly", true);
            displayEl.classList.add("disabled");
          } else {
            displayEl.removeAttribute("readonly");
            displayEl.classList.remove("disabled");
          }
        }
      }
    });
  });
}

// Establishes a hidden value (code, saved in db) and a display value (front-end) to a field
function setValue(block, name, code, label) {
  const hidden = block.querySelector(`[name$="[${name}]"]`);
  const display = hidden ? document.getElementById(`${hidden.id}_display`) : null;

  if (hidden) hidden.value = code || "";
  if (display) display.value = label || "";
}


// Dependencies flux
export function updateFieldDependencies() {

  // --- CONTRACT DEPENDENCIES ---

  // Vinculation type dependencies 
  const vincInputs = document.querySelectorAll(
    '[name$="[vinculation_type]"],[id$="_vinculation_type_display"]'
  );
  vincInputs.forEach((input, i) => {
    const block = input.closest('.contract-block');
    const targetBlock = block || document; 

    const vincText = input.value.trim();

    const traineeSelectors = [
    '[name$="[trainee_type]"]',
    '[name$="[trainee_studies]"]',
    '[name$="[internship]"]'
    ];
    const visitorOptions = [
      '[name$="[trainee_studies]"]',
      '[name$="[internship]"]',
      '[name$="[job_category]"',
     '[name$="[contract_type]"'
      ];
    const workerSelectors = [
    '[name$="[job_category]"',
    '[name$="[contract_type]"'
    ];
    const visitorSelectors = [
      '[name$="[trainee_type]"'
    ]

    if (vincText === "Contracted worker") {
      setValue(targetBlock, "contracting_institution", "0000001672", "CRM: Centre de Recerca Matemàtica");
      setDisabled(targetBlock, traineeSelectors, true);
      setDisabled(targetBlock, workerSelectors, false);
    } else if (vincText === "Affiliated") {
      setDisabled(targetBlock, traineeSelectors, true);
      setDisabled(targetBlock, workerSelectors, true);
    } else if (vincText === "Visitor") {
      setDisabled(targetBlock, visitorOptions, true);
      setDisabled(targetBlock, visitorSelectors, false);
    } else if (vincText === "Internship, TFG or TFM") {
      setDisabled(targetBlock, traineeSelectors, false);
      setDisabled(targetBlock, workerSelectors, true);
    }
  });
 
  document.querySelectorAll(".contract-block").forEach((block) => {
    const vincInput = block.querySelector('[name$="[vinculation_type]"]') || block.querySelector('[id*="vinculation_type"]');
  
    const traineeHidden = block.querySelector('[name$="[trainee_type]"]');
    const contractingHidden = block.querySelector('[name$="[contracting_institution]"]');
  
    const traineeDisplay = traineeHidden ? document.getElementById(`${traineeHidden.id}_display`) : null;
    const contractingDisplay = contractingHidden ? document.getElementById(`${contractingHidden.id}_display`) : null;
  
    const traineeDatalistId = traineeDisplay?.getAttribute("list");
    const contractingDatalistId = contractingDisplay?.getAttribute("list");
  
    const traineeDatalist = traineeDatalistId ? document.getElementById(traineeDatalistId) : null;
    const contractingDatalist = contractingDatalistId ? document.getElementById(contractingDatalistId) : null;
  
    if (!vincInput || !traineeDisplay || !traineeHidden || !traineeDatalist || !contractingDisplay || !contractingHidden || !contractingDatalist) {
      return;
    }
  
    const selected = vincInput.value;
    let traineeOptions = [];
    // Determinar opciones según vinculación
    if (selected === "Internship, TFG or TFM") {
      traineeOptions = formOptions.trainee;
    } else if (selected === "Visitor") {
      traineeOptions = formOptions.visitor;
    }
  
    // ---- Actualizar trainee_type ----
    traineeDatalist.innerHTML = traineeOptions
      .map(opt => `<option data-code="${opt.code}" value="${opt.name}"></option>`)
      .join("");
  
    let traineeMatch = traineeOptions.find(opt => opt.name === traineeDisplay.value);
    if (!traineeMatch) {
      traineeDisplay.value = "";
      traineeHidden.value = "";
    }
    traineeDisplay.addEventListener("input", () => {
      const selectedOpt = traineeOptions.find(opt => opt.name === traineeDisplay.value);
      traineeHidden.value = selectedOpt ? selectedOpt.code : "";
    });
      
    // ---- Actualizar contracting_institution ----
    let contractingOptions = [];

    if (selected === "Affiliated") {
      contractingOptions = formOptions.uniAffiliated || [];
    } else if (selected === "Visitor" || selected === "Internship, TFG or TFM") {
      contractingOptions = formOptions.universities || [];
    } else if (selected === "Contracted worker") {
      contractingOptions = formOptions.institution || [];
    }

    // Rellenar el datalist con opciones
    contractingDatalist.innerHTML = contractingOptions
      .map(opt => `<option data-code="${opt.code}" value="${opt.name}"></option>`)
      .join("");

    // Mantener código si el valor actual sigue existiendo
    const selectedOpt = contractingOptions.find(opt => opt.name === contractingDisplay.value);
    if (selectedOpt) {
      contractingHidden.value = selectedOpt.code;
    } else {
      contractingHidden.value = "";
    }

    // Reasignar evento de sincronización (por si fue borrado antes)
    contractingDisplay.addEventListener("change", () => {
      const selected = contractingOptions.find(opt => opt.name === contractingDisplay.value);
      contractingHidden.value = selected ? selected.code : "";
    });


  });
  
  // Job category dependencies (to unlock PhD section)
  const phdJobCategories = ["PhD"];
  let phdEnabled = false;
  
  document.querySelectorAll(".contract-block").forEach((block) => {
    const jobCatHidden = block.querySelector('[name$="[job_category]"]');
    const jobCatDisplay = jobCatHidden 
      ? document.getElementById(`${jobCatHidden.id}_display`) 
      : null;
  
    const jobCatValue = (jobCatHidden?.value || jobCatDisplay?.value || "").trim();
    if (phdJobCategories.includes(jobCatValue)) {
      phdEnabled = true; // Si cualquier contrato tiene PhD, habilitamos la pestaña
    }
  });
  
  const phdTabBtn  = document.querySelector('[data-bs-target="#tab-Phd"]');
  const phdTabPane = document.querySelector('#tab-Phd');
  if (phdTabBtn)  phdTabBtn.disabled = !phdEnabled;
  if (phdTabPane) phdTabPane.classList.toggle('disabled-tab-content', !phdEnabled);


  // --- GENERAL DEPENDENCIES ---
  // Education field only unlocks if academic_grade is superior to secondary education
  const academicSelect = document.querySelector('[name="academic_grade"]');
  if (!academicSelect) return;                          
  const academicGradeWithHigherEd = ["1", "2", "3", "6", "7"];
  const eduEnabled = academicGradeWithHigherEd.includes(academicSelect.value);
  const eduTabBtn  = document.querySelector('[data-bs-target="#tab-Education"]');
  const eduTabPane = document.querySelector('#tab-Education');
  if (eduTabBtn)  eduTabBtn.disabled = !eduEnabled;
  if (eduTabPane) eduTabPane.classList.toggle('disabled-tab-content', !eduEnabled);

  // If user's in a locked section, they're redirected to General section
  const activeButton = document.querySelector('.nav-link.active');
  const activeTarget = activeButton?.getAttribute('data-bs-target');
  const blockedTabs  = [];
  if (!phdEnabled) blockedTabs.push('#tab-Phd');
  if (!eduEnabled) blockedTabs.push('#tab-Education');

  if (blockedTabs.includes(activeTarget)) {
    document.querySelector('[data-bs-target="#tab-General"]')?.click();
  }

}



// Listens to changes in fields
export function setupFieldDependencies() {
  document.querySelector('[name="academic_grade"]')?.addEventListener('change', updateFieldDependencies);

  document.querySelectorAll('[name$="[job_category]"],[id$="_job_category_display"]').forEach(inp => inp.addEventListener('input', updateFieldDependencies));
}
