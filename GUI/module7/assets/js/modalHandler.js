// ---- modalHandler.js ----

export function openNewListModal(data = null) {
  const modalEl = document.getElementById('newListModal');
  const modal = bootstrap.Modal.getOrCreateInstance(modalEl);
  const form = document.getElementById('newListForm');
  const title = document.getElementById('newListModalLabel'); 

  // Inicializa TomSelects si no están inicializados aún
  initTomSelects();

  // Reset del formulario
  form.reset();
  form.classList.remove('was-validated');

  // Campos
  const nameInput = document.getElementById('listName');
  const descInput = document.getElementById('listDescription');
  const groupSelect = document.getElementById('groupSelect');
  const genderSelect = document.getElementById('genderSelect');
  const currentWorkersSelect = document.getElementById('currentWorkersSelect');
  const contractSelect = document.getElementById('contractSelect');
  const institutionSelect = document.getElementById('institutionSelect');
  const categorySelect = document.getElementById('categorySelect');
  const fundingSelect = document.getElementById('fundingSelect');
  const saveBtn = document.getElementById('saveListBtn');

  // Por defecto es modo creación
  saveBtn.textContent = 'Save list';
  saveBtn.dataset.editId = '';

  // Si recibimos datos es modo edición
  if (data) {
    nameInput.value = data.name || '';
    descInput.value = data.description || '';
    saveBtn.textContent = 'Save changes';
    saveBtn.dataset.editId = data.id;
    title.textContent = 'Edit Distribution List';

    const cond = data.conditions || {};

    if (cond.gender && cond.gender.length > 0)
      genderSelect.value = cond.gender[0];
    if (cond.current_workers)
      currentWorkersSelect.value = cond.current_workers;

    // Establecer valores en selects múltiples
    if (groupSelect.tomselect)
      groupSelect.tomselect.setValue(cond.groups || []);
    if (contractSelect.tomselect)
      contractSelect.tomselect.setValue(cond.contract_types || []);
    if (institutionSelect.tomselect)
      institutionSelect.tomselect.setValue(cond.institution || []);
    if (categorySelect.tomselect)
      categorySelect.tomselect.setValue(cond.category || []);
    if (fundingSelect.tomselect)
      fundingSelect.tomselect.setValue(cond.funding || []);
  }

  modal.show();
}

export function setupNewListForm() {
  const form = document.getElementById('newListForm');
  const saveBtn = document.getElementById('saveListBtn');

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    e.stopPropagation();

    if (!form.checkValidity()) {
      form.classList.add('was-validated');
      return;
    }

    const id = saveBtn.dataset.editId;
    const gender = document.getElementById("genderSelect").value;

    const data = {
      name: document.getElementById('listName').value.trim(),
      description: document.getElementById('listDescription').value.trim(),
      conditions: {
        gender: gender ? [gender] : [],
        groups: getSelectedValues('groupSelect'),
        contract_types: getSelectedValues('contractSelect'),
        institution: getSelectedValues('institutionSelect'),
        category: getSelectedValues('categorySelect'),
        funding: getSelectedValues('fundingSelect'),
        current_workers: getSelectedValue('currentWorkersSelect'),
      }
    };


    const action = id ? 'updateList' : 'createNewList';
    const url = `/module/proxy/module7/api?action=${action}${id ? `&id=${id}` : ''}`;

    try {
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
      });

      if (!res.ok) throw new Error('Failed to save list');
      alert(id ? 'List updated successfully!' : 'List created successfully!');
      form.reset();
      window.location.reload();
    } catch (err) {
      console.error(err);
      alert('Error saving list.');
    }
  });
}

// --------------------
// TomSelect initializer
// --------------------
function initTomSelects() {
  if (typeof TomSelect === 'undefined') {
    console.error('TomSelect no está disponible.');
    return;
  }

  const config = {
    plugins: ['remove_button'],
    persist: false,
    create: false,
    closeAfterSelect: false,
    placeholder: 'Select one or more options...',
    sortField: { field: 'text', direction: 'asc' },
  };

  const ids = ['groupSelect', 'fundingSelect', 'contractSelect', 'institutionSelect', 'categorySelect'];

  ids.forEach(id => {
    const el = document.getElementById(id);
    if (el && !el.tomselect) {
      el.classList.add('module7-tomselect');
      const tomselect = new TomSelect(`#${id}`, config);
      tomselect.wrapper.classList.remove('form-select');
      tomselect.wrapper.classList.add('module7-tomselect-wrapper');

      tomselect.control.addEventListener('click', (event) => {
        if (event.target.closest('.remove')) return;
        tomselect.focus();
        if (!tomselect.isOpen) {
          tomselect.open();
        }
      });
    }
  });
}

// --------------------
// Helpers
// --------------------
function getSelectedValues(selectId) {
  const el = document.getElementById(selectId);
  if (!el) return [];
  if (el.tomselect) return el.tomselect.getValue();
  return Array.from(el.selectedOptions).map(opt => opt.value);
}

function getSelectedValue(selectId) {
  const el = document.getElementById(selectId);
  return el.value || null;
}
