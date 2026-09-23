//////////////////////////////////////////////////////////////////////////////////////////////////
//              Formats visualization of notifications and notification list                    //
//////////////////////////////////////////////////////////////////////////////////////////////////


const list = document.getElementById("notification-list");
let notifications = [];

//Formats date in expected format
function formatDateTime(isoString) {
  const [datePart, timePart] = isoString.split("T");
  const [year, month, day] = datePart.split("-");
  const [hour, minute] = timePart.split(":");
  return `${day}-${month}-${year} ${hour}:${minute}`;
}
//Checks if JSON is valid before parsing
function isValidJSON(str) {
  try {
    JSON.parse(str);
    return true;
  } catch (e) {
    return false;
  }
}

//Returns intern BD name from visible label (unused, made to send RRHH notifications about incorrect profile fields)
export function getDbField(labelKey) {
  const internalName = fieldMap[labelKey] || labelKey;
  const match = Object.entries(formSources).find(([k, v]) => {
    if (Array.isArray(v)) return v.includes(internalName);
    return v === internalName;
  });
  return match?.[0] || internalName;
}

//Left side list of notifications
export async function loadNotifications() {
  const res = await fetch("/module/proxy/module3/api?action=notifications");
  notifications = await res.json();
  notifications.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
  const toggleBtn = document.getElementById("view-sent-btn");

  // Restore toggle button to go to Sent notifications
  toggleBtn.textContent = "Sent notifications";
  toggleBtn.onclick = async () => {
    await loadSentNotifications();
    clearNotificationDetail();
  };
  list.innerHTML = "";

  if (notifications.length === 0) {
    const emptyMsg = document.createElement("li");
    emptyMsg.className = "list-group-item text-muted text-center";
    emptyMsg.textContent = "You have not received any notifications yet.";
    list.appendChild(emptyMsg);
    return;
  }

  notifications.forEach((notif, i) => {
    const item = document.createElement("li");
    item.className = "list-group-item notif-item" + (notif.seen === 0 ? " unread" : "");
    item.dataset.index = i;
    item.innerHTML = `
      <div class="d-flex justify-content-between">
        <strong>${notif.title}</strong>
        <small>${new Date(notif.created_at).toLocaleDateString()}</small>
      </div>
    `;
    list.appendChild(item);
  });
}

function b64DecodeUnicode(str) {
  const bytes = Uint8Array.from(atob(str), c => c.charCodeAt(0));
  return new TextDecoder("utf-8").decode(bytes);
}


//Expanded view of selected notification
export function setupNotificationInteraction() {
  list.onclick = async e => {
    const item = e.target.closest(".notif-item");
    if (!item) return;
    const notif = notifications[parseInt(item.dataset.index)];

    if (!notif) {
    console.error("Notification not found for index:", item.dataset.index, notifications);
    return;
    }

    const notifTitle = document.getElementById("notif-title");
    const notifDate = document.getElementById("notif-date");
    const senderInfo = document.getElementById("notif-sender-info");
    const notifBody = document.getElementById("notif-body");
    const notifDelete = document.getElementById("notif-delete");
    const notifPreview = document.getElementById("notif-preview");
    const notifDownload = document.getElementById("notif-download");

    if (notif.sender_picture && !notif.sender_picture.startsWith("/")) {
        notif.sender_picture = "/" + notif.sender_picture;
    }

    if (senderInfo) {
      let imageUrl = "";
    
      if (notif.sender_picture) {
        try {
          const response = await fetch(`/module/proxy/module3/Uploads?file=${encodeURIComponent(notif.sender_picture)}`);
          if (response.ok) {
            const blob = await response.blob();
            imageUrl = URL.createObjectURL(blob);
          } else {
            console.error("Error fetching sender picture");
          }
        } catch (err) {
          console.error("Fetch error:", err);
        }
      }
    
      senderInfo.innerHTML = `
        <div class="d-flex align-items-center mt-2">
          ${imageUrl ? `<img src="${imageUrl}" class="rounded-circle me-2" style="width: 40px; height: 40px;">` : ""}
          <small class="text-muted">${notif.sender_name || "Unknown sender"}</small>
        </div>
      `;
    }

    notifDelete.classList.remove("d-none");
    notifBody.classList.remove("d-none");
    notifTitle.textContent = "";
    notifDate.textContent = "";
    notifBody.innerHTML = "";
    notifDownload.classList.add("d-none");

    //delete last iframe
    while (notifPreview.firstChild) {
        notifPreview.removeChild(notifPreview.firstChild);
    }

    notifDelete.onclick = async () => {
      if (!confirm("Are you sure you want to delete this notification?")) return;

      const response = await fetch("/module/proxy/module3/api?action=deleteNotification", {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: notif.id })
      });

      if (response.ok) {
        alert("Notification deleted.");
        await loadNotifications();
        notifTitle.textContent = "";
        notifDate.textContent = "";
        senderInfo.textContent = "";
        notifBody.textContent = "Notification deleted.";
        notifPreview.innerHTML = "";
        notifDownload.classList.add("d-none");
        notifDelete.classList.add("d-none");
        notifBody.classList.add("d-none");
      } else {
        alert("Error deleting notification.");
      }
    };

    item.classList.remove("unread");
    notifTitle.textContent = notif.title;
    notifDate.textContent = formatDateTime(notif.created_at);

    let parsed;
    try {
        let contentStr = notif.content?.trim?.() || "";
        if ((contentStr.startsWith("'") && contentStr.endsWith("'")) ||
            (contentStr.startsWith('"') && contentStr.endsWith('"'))) {
          contentStr = contentStr.slice(1, -1);
        }
        try {
          const decoded = b64DecodeUnicode(contentStr);
          if (isValidJSON(decoded)) {
            parsed = JSON.parse(decoded);
            notifBody.textContent = JSON.stringify(parsed, null, 2);
          } else {
            notifBody.textContent = decoded;
          }
        } catch {
          notifBody.textContent = contentStr || "No content.";
        }

    } catch (err) {
      notifBody.textContent = notif.content || "No content.";
      console.error("Error decoding or parsing content:", err, notif.content);
    }

    //delete previous content
    notifPreview.innerHTML = "";

    // Files visualization
    let fileUrl = notif.file_path || null;
    if (fileUrl) {
      fileUrl = `/module/proxy/module3/Uploads?file=${encodeURIComponent(fileUrl)}`;
    }

    const ext = notif.file_path?.split(".").pop()?.toLowerCase() || "";

    if (fileUrl && notif.can_download == 1) {
      notifDownload.href = fileUrl;
      notifDownload.classList.remove("d-none");
    } else {
      notifDownload.classList.add("d-none");
    }

    if (fileUrl) {
      if (["jpg", "jpeg", "png", "gif", "bmp", "webp"].includes(ext)) {
        notifPreview.innerHTML = `<img src="${fileUrl}" class="img-fluid" alt="Attachment">`;
      } else if (ext === "pdf") {
        const iframe = document.createElement("iframe");
        iframe.src = fileUrl;
        iframe.width = "100%";
        iframe.height = "500";
        iframe.style.border = "none";
        notifPreview.appendChild(iframe);
      } else {
        notifPreview.innerHTML = `<p class="text-muted">Unavailable visualization.</p>`;
      }
    }

    if (notif.seen === 0) {
      await fetch("/module/proxy/module3/api?action=markSeen", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: notif.id })
      });
      notif.seen = 1;
    }
  };
}


export async function loadSentNotifications() {
  const res = await fetch('/module/proxy/module3/api?action=getSentNotifications', {
    method: 'GET'
  });

  const data = await res.json();

  // IMPORTANT: update global notifications array
  notifications = data || [];

  notifications.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));

  const list = document.getElementById("notification-list");
  const toggleBtn = document.getElementById("view-sent-btn");

  toggleBtn.textContent = "Received notifications";
  toggleBtn.onclick = async () => {
    await loadNotifications();
    clearNotificationDetail();
  };

  list.innerHTML = "";

  const subtitle = document.createElement("li");
  subtitle.className = "list-group-item text-center fw-bold bg-light";
  subtitle.textContent = "Notifications sent by me";
  list.appendChild(subtitle);

  if (notifications.length === 0) {
    const emptyMsg = document.createElement("li");
    emptyMsg.className = "list-group-item text-muted text-center";
    emptyMsg.textContent = "You have not sent any notifications yet.";
    list.appendChild(emptyMsg);
    return;
  }

  notifications.forEach((notif, i) => {
    const item = document.createElement("li");
    item.className = "list-group-item notif-item";
    item.dataset.index = i;

    item.innerHTML = `
      <div class="d-flex justify-content-between">
        <strong>${notif.title}</strong>
        <small>${new Date(notif.created_at).toLocaleDateString()}</small>
      </div>
    `;

    list.appendChild(item);
  });
}

function clearNotificationDetail() {
  const notifTitle = document.getElementById("notif-title");
  const notifDate = document.getElementById("notif-date");
  const senderInfo = document.getElementById("notif-sender-info");
  const notifBody = document.getElementById("notif-body");
  const notifDelete = document.getElementById("notif-delete");
  const notifPreview = document.getElementById("notif-preview");
  const notifDownload = document.getElementById("notif-download");

  if (notifTitle) notifTitle.textContent = "";
  if (notifDate) notifDate.textContent = "";
  if (senderInfo) senderInfo.textContent = "";
  if (notifBody) {
    notifBody.textContent = "";
    notifBody.classList.add("d-none");
  }
  if (notifPreview) notifPreview.innerHTML = "";
  if (notifDownload) notifDownload.classList.add("d-none");
  if (notifDelete) notifDelete.classList.add("d-none");
}