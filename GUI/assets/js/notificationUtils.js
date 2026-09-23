export function updateNotificationCount() {
  fetch("/module/proxy/module3/api?action=notifications", {
    cache: "no-store"
  })
    .then(res => res.json())
    .then(notifs => {
      const badge = document.getElementById("notif-count");
      if (!badge) return;

      const unseen = notifs.filter(n => Number(n.seen) === 0);

      if (unseen.length > 0) {
        badge.textContent = unseen.length;
        badge.style.display = "inline-block";
      } else {
        badge.textContent = "";
        badge.style.display = "none";
      }
    })
    .catch(err => console.error("Error updating notifications:", err));
}

export function initNotificationBadge() {
  updateNotificationCount();

  window.addEventListener("pageshow", () => {
    updateNotificationCount();
  });

  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) {
      updateNotificationCount();
    }
  });

  setInterval(updateNotificationCount, 10000);
}