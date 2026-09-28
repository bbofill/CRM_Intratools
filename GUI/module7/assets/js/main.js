// ---- main.js ----
// Controlador principal del módulo 7
import { loadLists, renderListTable, setupEventHandlers } from './conditionTables.js';

document.addEventListener('DOMContentLoaded', async () => {

  try {
    const data = await loadLists();
    renderListTable(data);
  } catch (err) {
    console.error('Error initializing module 7:', err);
  }

});
