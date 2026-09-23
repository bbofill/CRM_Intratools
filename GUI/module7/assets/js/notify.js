let distributionListId = null; // Variable global accesible

export async function initNotifyView() {
  const params = new URLSearchParams(window.location.search);
  distributionListId = params.get("id");

  const res = await fetch(`/module/proxy/module7/api?action=getList&id=${distributionListId}`);
  const list = await res.json();

  document.getElementById("distributionListName").value = list.name;
}

// submit handler
export function setupNotificationForm() {
  const form = document.getElementById("announcement-form");

  form.addEventListener("submit", async function (e) {
    e.preventDefault();

    const formData = new FormData(form);
    formData.append("distributionListId", distributionListId);

    try {
      const response = await fetch("/module/proxy/module7/api?action=notificationToList", {
        method: "POST",
        body: formData
      });

      if (response.ok) {
        alert("Notification sent successfully!");
        form.reset();
        initNotifyView();
        return;
      }

      const text = await response.text();
      alert("Error sending notification: " + text);

    } catch (err) {
      console.error("Error sending notification:", err);
      alert("Network error sending notification.");
    }
  });
}
