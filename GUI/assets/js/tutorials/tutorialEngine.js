// GLOBAL ENGINE
let tutorialSteps = [];
let tutorialIndex = 0;
let currentModule = "";
let currentPage = "";

export function initTutorial(steps, module, page) {
  tutorialSteps = steps;
  tutorialIndex = 0;
  currentModule = module;
  currentPage = page;

  checkTutorialStatus(module, page);
}

function checkTutorialStatus(module, page) {
  fetch(`/api/tutorial/status?module=${module}&page=${page}`)
    .then(r => r.json())
    .then(data => {
      if (!data.completed) startTutorial();
    })
    .catch(err => console.error("Error checking tutorial status:", err));
}

function startTutorial() {
  createOverlay();
  createTooltipContainer();
  showStep(0);
}

function showStep(index) {
  if (!tutorialSteps[index]) {
    completeTutorial();
    return;
  }

  const step = tutorialSteps[index];
  const target = document.querySelector(step.element);

  // Si un elemento no existe o esta oculto (por rol, filtro, tab, etc.), saltar al siguiente
  if (!target || !isTutorialTargetVisible(target)) return showStep(index + 1);

  ensureTargetInView(target);
  updateTooltip(step);
  highlightElement(target);
  positionTooltip(step, target);

  // Detectar si es último paso
  const isLast = index === tutorialSteps.length - 1;

  const nextBtn = document.getElementById("tNext");
  const skipBtn = document.getElementById("tSkip");
  const closeBtn = document.getElementById("tClose");

  if (isLast) {
    nextBtn.style.display = "none";
    skipBtn.textContent = "Finish tutorial and don't show again";      
  } else {
    nextBtn.style.display = "";
    skipBtn.textContent = "Don't show again";
  }

  tutorialIndex = index;
}

window.addEventListener("resize", () => {
  // reposicionar elemento seleccionado y tooltip actual
  removeHighlight();
  showStep(tutorialIndex);
});

function nextStep() {
  removeHighlight();
  tutorialIndex++;
  showStep(tutorialIndex);
}

function skipTutorial() {
  completeTutorial();
}

function closeTutorialTemporary() {
  cleanup();  
}


function completeTutorial() {
  fetch(`/api/tutorial/complete`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ module: currentModule, page: currentPage })
  });
  cleanup();
}

// ---------- UI ---------- //

function createOverlay() {
  const overlay = document.createElement("div");
  overlay.id = "tutorialOverlay";
  overlay.style.position = "fixed";
  overlay.style.inset = "0";
  overlay.style.background = "rgba(0,0,0,0.6)";
  overlay.style.backdropFilter = "blur(3px)";
  overlay.style.pointerEvents = "none";
  overlay.style.zIndex = "999998";
  document.body.appendChild(overlay);
}


function createTooltipContainer() {
  const tooltip = document.createElement("div");
  tooltip.id = "tutorialTooltip";
  tooltip.style.position = "fixed";
  tooltip.style.background = "#ffffff";
  tooltip.style.color = "#000000";
  tooltip.style.padding = "18px 22px";
  tooltip.style.borderRadius = "12px";
  tooltip.style.width = "min(350px, calc(100vw - 32px))";
  tooltip.style.maxWidth = "350px";
  tooltip.style.boxShadow = "0 0 30px rgba(0,0,0,0.5)";
  tooltip.style.zIndex = "2000002";
  tooltip.innerHTML = `
      <button id="tClose" class="tutorial-close">&times;</button>
      <h4 id="tutorialTitle"></h4>
      <p id="tutorialMsg"></p>
      <div class="tutorial-btns">
        <button id="tNext" class="btn btn-danger">Next</button>
        <button id="tSkip" class="btn btn-outline-secondary">Don't show again</button>
      </div>
  `;
  document.body.appendChild(tooltip);

  document.getElementById("tNext").onclick = nextStep;
  document.getElementById("tSkip").onclick = skipTutorial;
  document.getElementById("tClose").onclick = closeTutorialTemporary;
}

function updateTooltip(step) {
  document.getElementById("tutorialTitle").textContent = step.title;
  document.getElementById("tutorialMsg").textContent = step.message;
}

function ensureTargetInView(target) {
  const rect = target.getBoundingClientRect();
  const margin = 24;
  const viewportHeight = window.innerHeight || document.documentElement.clientHeight;
  const viewportWidth = window.innerWidth || document.documentElement.clientWidth;
  const availableHeight = viewportHeight - margin * 2;
  const availableWidth = viewportWidth - margin * 2;

  const verticalVisible = rect.height > availableHeight
    ? rect.top <= viewportHeight - margin && rect.bottom >= margin
    : rect.top >= margin && rect.bottom <= viewportHeight - margin;
  const horizontalVisible = rect.width > availableWidth
    ? rect.left <= viewportWidth - margin && rect.right >= margin
    : rect.left >= margin && rect.right <= viewportWidth - margin;

  if (verticalVisible && horizontalVisible) return;

  target.scrollIntoView({ block: "nearest", inline: "nearest" });
}

function positionTooltip(step, target) {
  const rect = target.getBoundingClientRect();
  const tooltip = document.getElementById("tutorialTooltip");

  const offset = 15;
  const viewportMargin = 16;
  const viewportWidth = window.innerWidth || document.documentElement.clientWidth;
  const viewportHeight = window.innerHeight || document.documentElement.clientHeight;
  let top = rect.bottom + offset;
  let left = rect.left;

  switch(step.position) {
    case "right":
      top = rect.top;
      left = rect.right + offset;
      break;

    case "left":
      top = rect.top;
      left = rect.left - tooltip.offsetWidth - offset;
      break;

    case "top":
      top = rect.top - tooltip.offsetHeight - offset;
      left = rect.left;
      break;

    case "center":
      top = rect.top + rect.height/2 - tooltip.offsetHeight/2;
      left = rect.left + rect.width/2 - tooltip.offsetWidth/2;
      break;

    case "bottom":
    default:
      top = rect.bottom + offset;
      left = rect.left;
      break;
  }

  left = clampToViewport(left, tooltip.offsetWidth, viewportWidth, viewportMargin);
  top = clampToViewport(top, tooltip.offsetHeight, viewportHeight, viewportMargin);

  tooltip.style.top = `${top}px`;
  tooltip.style.left = `${left}px`;
}

function clampToViewport(value, size, viewportSize, margin) {
  const max = Math.max(margin, viewportSize - size - margin);
  return Math.min(Math.max(value, margin), max);
}


function highlightElement(el) {

  const rect = el.getBoundingClientRect();
  const overlay = document.getElementById("tutorialOverlay");
  const footer = document.querySelector("footer");

  const padding = 8;

  const footerRect = footer?.getBoundingClientRect();
  const footerTopLimit = footerRect?.top ?? window.innerHeight; // posición vertical donde empieza footer

  const left = rect.left - padding;
  const top = rect.top - padding;
  let bottom = rect.bottom + padding;

  // Limitar para no invadir el footer
  if (bottom > footerTopLimit - 10) {
    bottom = footerTopLimit - 10; // margen de 10 px
  }

  const right = rect.right + padding;

  overlay.style.clipPath = `polygon(
    0% 0%,
    100% 0%,
    100% 100%,
    0% 100%,
    0% ${top}px,
    ${left}px ${top}px,
    ${left}px ${bottom}px,
    ${right}px ${bottom}px,
    ${right}px ${top}px,
    0% ${top}px
  )`;

  const highlight = document.createElement("div");
  highlight.id = "tutorialHighlight";
  highlight.style.position = "fixed";
  highlight.style.top = top + "px";
  highlight.style.left = left + "px";
  highlight.style.width = rect.width + padding * 2 + "px";
  highlight.style.height = (bottom - top) + "px";
  highlight.style.border = "3px solid #4EA8DE";
  highlight.style.borderRadius = "8px";
  highlight.style.zIndex = "2000000";

  document.body.appendChild(highlight);
}

function isTutorialTargetVisible(target) {
  if (target.closest("[hidden], .d-none")) return false;

  const rect = target.getBoundingClientRect();
  return rect.width > 0 && rect.height > 0;
}


function removeHighlight() {
  document.getElementById("tutorialHighlight")?.remove();
  const overlay = document.getElementById("tutorialOverlay");
  if (overlay) overlay.style.clipPath = "none";
}


function cleanup() {
  removeHighlight();
  document.getElementById("tutorialOverlay")?.remove();
  document.getElementById("tutorialTooltip")?.remove();
}
