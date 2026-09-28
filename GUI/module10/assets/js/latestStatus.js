export function initLatestStatusModal() {
  const latestStatusNav = document.querySelector(".service-nav-item[href='#latestStatus']");
  const viewFullStatusBtn = document.getElementById("viewFullStatusBtn");
  const latestStatusModalEl = document.getElementById("latestCommissionModal");

  if (latestStatusNav && viewFullStatusBtn) {
    latestStatusNav.addEventListener("click", () => {
      setTimeout(() => {
        viewFullStatusBtn.classList.remove("status-attention");

        void viewFullStatusBtn.offsetWidth;

        viewFullStatusBtn.classList.add("status-attention");

        setTimeout(() => {
          viewFullStatusBtn.classList.remove("status-attention");
        }, 2600);
      }, 350);
    });
  }

  if (viewFullStatusBtn && latestStatusModalEl && window.bootstrap) {
    const latestStatusModal = new bootstrap.Modal(latestStatusModalEl);

    viewFullStatusBtn.addEventListener("click", () => {
      viewFullStatusBtn.classList.remove("status-attention");
      viewFullStatusBtn.blur();

      latestStatusModal.show();
    });
  }
}